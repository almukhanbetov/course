package wishlist

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lms-backend/internal/authctx"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

func currentUser(c *gin.Context) (uuid.UUID, bool) {
	userID, ok := authctx.UserID(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid authorization header")
		return uuid.UUID{}, false
	}
	return userID, true
}

// RegisterRoutes wires both the course-scoped wishlist toggle and the
// student's own wishlist views — every route requires auth (item 25: no
// anonymous wishlist access), and every one reads only the caller's own
// userID from the JWT, never a client-supplied id (item 25/26).
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, requireAuth gin.HandlerFunc) {
	rg.POST("/courses/:id/wishlist", requireAuth, h.AddToWishlist)
	rg.DELETE("/courses/:id/wishlist", requireAuth, h.RemoveFromWishlist)
	rg.GET("/me/wishlist", requireAuth, h.ListMyWishlist)
	rg.GET("/me/wishlist/course-ids", requireAuth, h.ListMyWishlistCourseIDs)
}

func (h *Handler) AddToWishlist(c *gin.Context) {
	courseID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_COURSE_ID", "course id must be a valid UUID")
		return
	}
	userID, ok := currentUser(c)
	if !ok {
		return
	}

	err = h.service.Add(c.Request.Context(), userID, courseID)
	if errors.Is(err, ErrCourseNotFound) {
		respondError(c, http.StatusNotFound, "COURSE_NOT_FOUND", "course not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to add course to wishlist")
		return
	}
	// 200, not 201 — Add is idempotent (item 2), so "already there" and
	// "just added" are indistinguishable to the caller on purpose.
	c.JSON(http.StatusOK, gin.H{"course_id": courseID, "in_wishlist": true})
}

func (h *Handler) RemoveFromWishlist(c *gin.Context) {
	courseID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_COURSE_ID", "course id must be a valid UUID")
		return
	}
	userID, ok := currentUser(c)
	if !ok {
		return
	}

	if err := h.service.Remove(c.Request.Context(), userID, courseID); err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to remove course from wishlist")
		return
	}
	c.JSON(http.StatusOK, gin.H{"course_id": courseID, "in_wishlist": false})
}

func (h *Handler) ListMyWishlist(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}

	items, err := h.service.ListForUser(c.Request.Context(), userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load wishlist")
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) ListMyWishlistCourseIDs(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}

	ids, err := h.service.ListCourseIDsForUser(c.Request.Context(), userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load wishlist")
		return
	}
	c.JSON(http.StatusOK, ids)
}
