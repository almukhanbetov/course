package certificates

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"lms-backend/internal/achievements"
	"lms-backend/internal/activity"
	"lms-backend/internal/notifications"
)

var (
	ErrNotFound        = errors.New("resource not found")
	ErrAlreadyIssued   = errors.New("certificate already issued for this user and course")
	ErrDuplicateNumber = errors.New("certificate number collision")
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// GetCompletedCourseIDsWithoutCertificate is the entire "is this student
// eligible" check — it only ever looks at course_enrollments.completed_at,
// which is maintained exclusively by the learning/tests domains' own
// completion logic. There is no course_id input from the caller anywhere in
// this package, so the frontend has no way to request a certificate for a
// course it hasn't actually completed.
func (r *Repository) GetCompletedCourseIDsWithoutCertificate(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT ce.course_id
		FROM course_enrollments ce
		LEFT JOIN certificates cert ON cert.user_id = ce.user_id AND cert.course_id = ce.course_id
		WHERE ce.user_id = $1 AND ce.completed_at IS NOT NULL AND cert.id IS NULL
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, rows.Err()
}

// CreateCertificate returns ErrAlreadyIssued if (user_id, course_id) already
// has one (race with a concurrent request), or ErrDuplicateNumber if the
// generated certificate_number happens to collide with an existing one —
// callers distinguish these via the violated constraint name. On genuine
// first issuance, the "certificate issued" notification is enqueued in the
// same transaction — this is the only moment a certificate is ever newly
// created (see Service.issueCertificate), so there is no separate
// "just now issued" detection needed here.
func (r *Repository) CreateCertificate(ctx context.Context, userID, courseID uuid.UUID, number string) (*Certificate, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var cert Certificate
	err = tx.QueryRow(ctx, `
		INSERT INTO certificates (user_id, course_id, certificate_number, issued_at)
		VALUES ($1, $2, $3, now())
		RETURNING id, user_id, course_id, certificate_number, issued_at, created_at
	`, userID, courseID, number).Scan(&cert.ID, &cert.UserID, &cert.CourseID, &cert.CertificateNumber, &cert.IssuedAt, &cert.CreatedAt)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "certificates_user_course_unique":
			return nil, ErrAlreadyIssued
		case "certificates_number_unique":
			return nil, ErrDuplicateNumber
		}
	}
	if err != nil {
		return nil, err
	}

	var courseTitle string
	if err := tx.QueryRow(ctx, `SELECT title FROM courses WHERE id = $1`, courseID).Scan(&courseTitle); err != nil {
		return nil, err
	}

	if err := notifications.Enqueue(ctx, tx, notifications.EnqueueInput{
		UserID: userID,
		Type:   notifications.TypeCertificateIssued,
		Data: map[string]any{
			"certificate_id":     cert.ID,
			"certificate_number": cert.CertificateNumber,
			"course_id":          courseID,
			"course_title":       courseTitle,
		},
		DedupeKey: "certificate_issued:" + cert.ID.String(),
		Channels:  []string{notifications.ChannelInApp, notifications.ChannelEmail},
	}); err != nil {
		return nil, err
	}

	if err := activity.Record(ctx, tx, activity.RecordInput{
		UserID: userID, Type: activity.TypeCertificateIssued,
		EntityType: "certificate", EntityID: &cert.ID,
		Metadata:  map[string]any{"title": courseTitle},
		DedupeKey: "certificate_issued_activity:" + cert.ID.String(),
	}); err != nil {
		return nil, err
	}
	if err := achievements.Evaluate(ctx, tx, userID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &cert, nil
}

func (r *Repository) GetByUserAndCourse(ctx context.Context, userID, courseID uuid.UUID) (*Certificate, error) {
	var cert Certificate
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, course_id, certificate_number, issued_at, created_at
		FROM certificates
		WHERE user_id = $1 AND course_id = $2
	`, userID, courseID).Scan(&cert.ID, &cert.UserID, &cert.CourseID, &cert.CertificateNumber, &cert.IssuedAt, &cert.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

func (r *Repository) ListByUser(ctx context.Context, userID uuid.UUID) ([]CertificateSummary, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT cert.id, cert.certificate_number, cert.course_id, c.title, cert.issued_at
		FROM certificates cert
		JOIN courses c ON c.id = cert.course_id
		WHERE cert.user_id = $1
		ORDER BY cert.issued_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []CertificateSummary{}
	for rows.Next() {
		var cs CertificateSummary
		if err := rows.Scan(&cs.ID, &cs.CertificateNumber, &cs.CourseID, &cs.CourseTitle, &cs.IssuedAt); err != nil {
			return nil, err
		}
		result = append(result, cs)
	}
	return result, rows.Err()
}

// GetDetailByID returns the certificate detail plus its owning user id, so
// the service can enforce ownership before handing back any data.
func (r *Repository) GetDetailByID(ctx context.Context, certID uuid.UUID) (*CertificateDetail, uuid.UUID, error) {
	var d CertificateDetail
	var ownerID uuid.UUID
	var firstName, lastName string
	err := r.pool.QueryRow(ctx, `
		SELECT cert.id, cert.certificate_number, cert.course_id, c.title, u.first_name, u.last_name, cert.issued_at, cert.user_id
		FROM certificates cert
		JOIN courses c ON c.id = cert.course_id
		JOIN users u ON u.id = cert.user_id
		WHERE cert.id = $1
	`, certID).Scan(&d.ID, &d.CertificateNumber, &d.CourseID, &d.CourseTitle, &firstName, &lastName, &d.IssuedAt, &ownerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, uuid.UUID{}, ErrNotFound
	}
	if err != nil {
		return nil, uuid.UUID{}, err
	}
	d.StudentName = firstName + " " + lastName
	return &d, ownerID, nil
}

// VerifyByCertificateNumber never returns ErrNotFound — an unknown number is
// a normal outcome for a public verification check, not an error.
func (r *Repository) VerifyByCertificateNumber(ctx context.Context, number string) (*PublicVerification, error) {
	var courseTitle, firstName, lastName string
	var issuedAt time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT c.title, u.first_name, u.last_name, cert.issued_at
		FROM certificates cert
		JOIN courses c ON c.id = cert.course_id
		JOIN users u ON u.id = cert.user_id
		WHERE cert.certificate_number = $1
	`, number).Scan(&courseTitle, &firstName, &lastName, &issuedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return &PublicVerification{Valid: false}, nil
	}
	if err != nil {
		return nil, err
	}

	return &PublicVerification{
		Valid:             true,
		CertificateNumber: number,
		StudentName:       firstName + " " + lastName,
		CourseTitle:       courseTitle,
		IssuedAt:          &issuedAt,
	}, nil
}

// ListAdmin is read-only by design (see package docs / Stage 8 report): a
// certificate is never created directly through the admin API, only ever
// through the completion-driven issuance in service.go, so this list has no
// corresponding create/update/delete counterpart.
func (r *Repository) ListAdmin(ctx context.Context, search string, limit, offset int) ([]AdminCertificateSummary, int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT cert.id, cert.certificate_number, u.first_name, u.last_name, u.email, c.title, cert.issued_at,
			COUNT(*) OVER() AS total
		FROM certificates cert
		JOIN courses c ON c.id = cert.course_id
		JOIN users u ON u.id = cert.user_id
		WHERE $1 = '' OR cert.certificate_number ILIKE '%'||$1||'%'
		ORDER BY cert.issued_at DESC
		LIMIT $2 OFFSET $3
	`, search, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := []AdminCertificateSummary{}
	total := 0
	for rows.Next() {
		var cs AdminCertificateSummary
		var firstName, lastName string
		if err := rows.Scan(&cs.ID, &cs.CertificateNumber, &firstName, &lastName, &cs.StudentEmail, &cs.CourseTitle, &cs.IssuedAt, &total); err != nil {
			return nil, 0, err
		}
		cs.StudentName = firstName + " " + lastName
		result = append(result, cs)
	}
	return result, total, rows.Err()
}
