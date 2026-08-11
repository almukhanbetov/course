package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"lms-backend/internal/access"
	"lms-backend/internal/achievements"
	"lms-backend/internal/activity"
	"lms-backend/internal/admin"
	"lms-backend/internal/assignments"
	"lms-backend/internal/auth"
	"lms-backend/internal/categories"
	"lms-backend/internal/certificates"
	"lms-backend/internal/coding"
	"lms-backend/internal/config"
	"lms-backend/internal/courses"
	"lms-backend/internal/db"
	"lms-backend/internal/health"
	"lms-backend/internal/instructor"
	"lms-backend/internal/learning"
	"lms-backend/internal/notifications"
	"lms-backend/internal/ownership"
	"lms-backend/internal/qa"
	"lms-backend/internal/recommendations"
	"lms-backend/internal/reviews"
	"lms-backend/internal/specialities"
	"lms-backend/internal/subscriptions"
	"lms-backend/internal/tests"
	"lms-backend/internal/users"
	"lms-backend/internal/videos"
	"lms-backend/internal/wishlist"
)

func main() {
	cfg := config.Load()
	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer pool.Close()

	healthService := health.NewService(pool)
	healthHandler := health.NewHandler(healthService)

	coursesRepo := courses.NewRepository(pool)
	coursesService := courses.NewService(coursesRepo)
	coursesHandler := courses.NewHandler(coursesService)

	categoriesRepo := categories.NewRepository(pool)
	categoriesService := categories.NewService(categoriesRepo)
	categoriesHandler := categories.NewHandler(categoriesService)

	reviewsRepo := reviews.NewRepository(pool)
	reviewsService := reviews.NewService(reviewsRepo)
	reviewsHandler := reviews.NewHandler(reviewsService)

	usersRepo := users.NewRepository(pool)
	usersService := users.NewService(usersRepo)
	usersHandler := users.NewHandler(usersService)

	authService := auth.NewService(usersService, cfg.JWTSecret)
	authHandler := auth.NewHandler(authService)
	authMiddleware := auth.NewMiddleware(cfg.JWTSecret)

	accessService := access.NewService(pool)

	learningRepo := learning.NewRepository(pool)
	learningService := learning.NewService(learningRepo, accessService)
	learningHandler := learning.NewHandler(learningService)

	specialitiesRepo := specialities.NewRepository(pool)
	specialitiesService := specialities.NewService(specialitiesRepo)
	specialitiesHandler := specialities.NewHandler(specialitiesService)

	testsRepo := tests.NewRepository(pool)
	testsService := tests.NewService(testsRepo, accessService)
	testsHandler := tests.NewHandler(testsService)

	certificatesRepo := certificates.NewRepository(pool)
	certificatesService := certificates.NewService(certificatesRepo)
	certificatesHandler := certificates.NewHandler(certificatesService)

	adminRepo := admin.NewRepository(pool)
	adminService := admin.NewService(adminRepo)
	adminHandler := admin.NewHandler(adminService)

	paymentProvider := subscriptions.NewProvider(cfg.PaymentProvider)
	subscriptionsRepo := subscriptions.NewRepository(pool)
	subscriptionsService := subscriptions.NewService(subscriptionsRepo, paymentProvider, cfg.PaymentProvider)
	subscriptionsHandler := subscriptions.NewHandler(subscriptionsService)

	videoStorage, err := videos.NewS3Storage(ctx, videos.S3Config{
		Endpoint:       cfg.S3Endpoint,
		PublicEndpoint: cfg.S3PublicEndpoint,
		AccessKey:      cfg.S3AccessKey,
		SecretKey:      cfg.S3SecretKey,
		Bucket:         cfg.S3Bucket,
		Region:         cfg.S3Region,
	})
	if err != nil {
		log.Fatalf("failed to configure video storage client: %v", err)
	}
	// The storage client itself never fails to construct (it doesn't dial
	// anything), so confirm the bucket is actually reachable here — but only
	// log a warning rather than exiting. Every other domain (courses, tests,
	// certificates, ...) must keep working even if MinIO is misconfigured;
	// only the video endpoints will fail, and only when actually called.
	pingCtx, cancelPing := context.WithTimeout(ctx, 5*time.Second)
	if err := videoStorage.Ping(pingCtx); err != nil {
		log.Printf("WARNING: video storage is not reachable yet (bucket %q via %s): %v", cfg.S3Bucket, cfg.S3Endpoint, err)
	}
	cancelPing()

	videosRepo := videos.NewRepository(pool)
	videosService := videos.NewService(videosRepo, videoStorage, accessService, cfg.S3Bucket, cfg.VideoMaxUploadMB, cfg.VideoSignedURLTTLSec)
	videosHandler := videos.NewHandler(videosService)

	// Assignments (Stage 15) reuses the exact same video storage client —
	// *videos.S3Storage structurally satisfies assignments.Storage's smaller
	// method set, so this is the same MinIO bucket/credentials, just a
	// distinct "assignments/" object key prefix. No second S3 client.
	assignmentsRepo := assignments.NewRepository(pool)
	assignmentsService := assignments.NewService(assignmentsRepo, videoStorage, accessService, cfg.AssignmentMaxUploadMB)

	// Coding exercises (Stage 16). The API backend never executes student
	// code itself — it only authorizes requests, validates authoring input
	// against the same system ceilings cmd/code-runner enforces at
	// execution time, and enqueues code_execution_jobs for that separate
	// process to pick up. See internal/coding/runner.go for the actual
	// sandbox.
	codingRepo := coding.NewRepository(pool)
	codingService := coding.NewService(codingRepo, accessService, coding.Limits{
		SystemTimeoutMS:         cfg.CodeRunnerTimeoutMS,
		SystemMemoryMB:          cfg.CodeRunnerMemoryMB,
		MaxSubmissionsPerMinute: cfg.CodeRunnerMaxSubmissionsPerMin,
		MaxConcurrentPerUser:    cfg.CodeRunnerMaxConcurrentPerUser,
	})

	// The API backend only ever enqueues jobs (from the trigger points
	// embedded in users/learning/tests/certificates/subscriptions) and
	// serves the read endpoints below — it never sends email itself, so it
	// needs no SMTP config at all. See cmd/notification-worker for the
	// process that actually processes the queue.
	notificationsRepo := notifications.NewRepository(pool)
	notificationsService := notifications.NewService(notificationsRepo)
	notificationsHandler := notifications.NewHandler(notificationsService)

	// Stage 14: instructor dashboard + course authoring. ownershipService is
	// the single centralized "may this user manage this course" check;
	// instructorHandler is a thin authorization+routing layer that reuses
	// coursesHandler/videosHandler/testsHandler verbatim wherever their
	// existing DTOs are already safe for an instructor caller — see
	// internal/instructor's doc comments for exactly where and why.
	ownershipService := ownership.NewService(pool)
	instructorRepo := instructor.NewRepository(pool)
	instructorService := instructor.NewService(instructorRepo)
	instructorHandler := instructor.NewHandler(
		instructorService, coursesService, coursesHandler, videosHandler, testsService, testsHandler, ownershipService,
	)
	assignmentsHandler := assignments.NewHandler(assignmentsService, ownershipService)
	codingHandler := coding.NewHandler(codingService, ownershipService)

	// Stage 17: learning analytics/streaks (internal/activity) and
	// achievements (internal/achievements) — the write side of both lives
	// inside learning/assignments/coding/tests/certificates' own
	// transactions (see each domain's repository.go), so the API layer
	// only ever needs their read-facing services/handlers here.
	activityRepo := activity.NewRepository(pool)
	activityService := activity.NewService(activityRepo)
	activityHandler := activity.NewHandler(activityService)
	achievementsRepo := achievements.NewRepository(pool)
	achievementsService := achievements.NewService(achievementsRepo)
	achievementsHandler := achievements.NewHandler(achievementsService)

	// Stage 18: rule-based recommendations (internal/recommendations) —
	// read-only, no ML, no separate recommendation service. Registers both
	// the personalized GET /me/recommendations and the public
	// GET /courses/:id/similar (see RegisterRoutes' own doc comment for why
	// they share one handler despite different auth requirements).
	recommendationsRepo := recommendations.NewRepository(pool)
	recommendationsService := recommendations.NewService(recommendationsRepo)
	recommendationsHandler := recommendations.NewHandler(recommendationsService)

	// Stage 18: wishlist (internal/wishlist) — a pure intent signal, no
	// side effects of its own (see the package doc comment). Its write
	// side is also called directly from internal/learning.CreateEnrollment
	// (auto-remove-on-enroll, in that transaction), but its own HTTP
	// routes are registered here like every other domain.
	wishlistRepo := wishlist.NewRepository(pool)
	wishlistService := wishlist.NewService(wishlistRepo)
	wishlistHandler := wishlist.NewHandler(wishlistService)

	// Stage 20A: lesson Q&A (internal/qa) — student-facing only this stage
	// (ask/answer/list/delete-own); instructor/admin moderation routes come
	// in a later stage. Depends on ownershipService (constructed above,
	// Stage 14) for the "course-owning instructor may also answer" half of
	// the answer-authorization rule.
	qaRepo := qa.NewRepository(pool)
	qaService := qa.NewService(qaRepo, ownershipService)
	qaHandler := qa.NewHandler(qaService)

	router := gin.Default()

	v1 := router.Group("/api/v1")
	healthHandler.RegisterRoutes(v1)
	coursesHandler.RegisterRoutes(v1)
	categoriesHandler.RegisterRoutes(v1)
	reviewsHandler.RegisterRoutes(v1, authMiddleware.RequireAuth())
	authHandler.RegisterRoutes(v1)
	usersHandler.RegisterRoutes(v1, authMiddleware.RequireAuth())
	learningHandler.RegisterRoutes(v1, authMiddleware.RequireAuth())
	specialitiesHandler.RegisterRoutes(v1, authMiddleware.RequireAuth())
	testsHandler.RegisterRoutes(v1, authMiddleware.RequireAuth())
	certificatesHandler.RegisterRoutes(v1, authMiddleware.RequireAuth())
	subscriptionsHandler.RegisterRoutes(v1, authMiddleware.RequireAuth())
	videosHandler.RegisterRoutes(v1, authMiddleware.RequireAuth())
	notificationsHandler.RegisterRoutes(v1, authMiddleware.RequireAuth())
	assignmentsHandler.RegisterRoutes(v1, authMiddleware.RequireAuth())
	codingHandler.RegisterRoutes(v1, authMiddleware.RequireAuth())
	activityHandler.RegisterRoutes(v1, authMiddleware.RequireAuth())
	achievementsHandler.RegisterRoutes(v1, authMiddleware.RequireAuth())
	recommendationsHandler.RegisterRoutes(v1, authMiddleware.RequireAuth())
	wishlistHandler.RegisterRoutes(v1, authMiddleware.RequireAuth())
	qaHandler.RegisterRoutes(v1, authMiddleware.RequireAuth())

	// /instructor/* requires a valid JWT and either the instructor or admin
	// role — finer-grained per-course ownership is checked inside
	// instructorHandler itself (see internal/ownership), not here.
	instructorGroup := v1.Group("/instructor")
	instructorGroup.Use(authMiddleware.RequireAuth(), authMiddleware.RequireAnyRole("instructor", "admin"))
	instructorHandler.RegisterRoutes(instructorGroup)
	assignmentsHandler.RegisterInstructorRoutes(instructorGroup)
	codingHandler.RegisterInstructorRoutes(instructorGroup)

	// Every /admin/* route requires a valid JWT AND the admin role, checked
	// here on the backend — this is the single choke point all admin
	// sub-routers below register onto, so no domain can accidentally expose
	// an admin endpoint without this gate.
	adminGroup := v1.Group("/admin")
	adminGroup.Use(authMiddleware.RequireAuth(), authMiddleware.RequireRole("admin"))
	adminGroup.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	adminHandler.RegisterAdminRoutes(adminGroup)
	usersHandler.RegisterAdminRoutes(adminGroup)
	coursesHandler.RegisterAdminRoutes(adminGroup)
	categoriesHandler.RegisterAdminRoutes(adminGroup)
	reviewsHandler.RegisterAdminRoutes(adminGroup)
	specialitiesHandler.RegisterAdminRoutes(adminGroup)
	testsHandler.RegisterAdminRoutes(adminGroup)
	certificatesHandler.RegisterAdminRoutes(adminGroup)
	subscriptionsHandler.RegisterAdminRoutes(adminGroup)
	videosHandler.RegisterAdminRoutes(adminGroup)
	notificationsHandler.RegisterAdminRoutes(adminGroup)

	log.Printf("starting server on port %s", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
