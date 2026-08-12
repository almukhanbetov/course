package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"lms-backend/internal/users"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes wires /auth/register and /auth/login behind the shared
// rate limiter (Stage 26A1) — every call to either endpoint passes through
// RateLimiter.Limit first; Register/Login themselves are completely
// unchanged, since the limiter only ever turns away requests past the
// threshold, never touching what a normal request receives.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, rateLimiter *RateLimiter) {
	auth := rg.Group("/auth")
	auth.POST("/register", rateLimiter.Limit("register"), h.Register)
	auth.POST("/login", rateLimiter.Limit("login"), h.Login)
}

type registerRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string           `json:"token"`
	User  users.PublicUser `json:"user"`
}

func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	user, err := h.service.Register(c.Request.Context(), RegisterInput(req))

	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(http.StatusCreated, user.Public())
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, users.ErrEmailExists):
		respondError(c, http.StatusConflict, "EMAIL_ALREADY_EXISTS", "a user with this email already exists")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to register user")
	}
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	token, user, err := h.service.Login(c.Request.Context(), LoginInput(req))
	switch {
	case err == nil:
		c.JSON(http.StatusOK, loginResponse{Token: token, User: user.Public()})
	case errors.Is(err, ErrInvalidCredentials):
		respondError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid email or password")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to login")
	}
}
