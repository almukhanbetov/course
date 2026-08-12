package reports

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"lms-backend/internal/pagination"
)

// ValidationError mirrors every other domain's pattern in this codebase
// (internal/qa, internal/courses, internal/recommendations, ...) — a typed
// error the future HTTP handler layer maps to 400, distinct from a plain
// internal error.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// ErrContentNotFound is returned when content_type/content_id don't
// resolve to a real row — see Repository.ContentExists.
var ErrContentNotFound = errors.New("reported content not found")

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// CreateReport validates content_type and reason, confirms the referenced
// content actually exists, then creates the report. reporterUserID is
// always the caller's own id — this service has no alternate identity
// parameter for a caller to substitute another user's id into; whichever
// future handler calls this must read it from authctx (authctx.UserID),
// the same convention internal/qa and internal/recommendations already
// use for every identity-bearing write in this codebase. Duplicate active
// reports are rejected via Repository.ErrDuplicateActiveReport, passed
// through unchanged so the caller can distinguish it from every other
// failure mode.
func (s *Service) CreateReport(ctx context.Context, reporterUserID uuid.UUID, contentType string, contentID uuid.UUID, reason string) (*Report, error) {
	if !allowedContentTypes[contentType] {
		return nil, &ValidationError{Message: "content_type must be one of: question, answer, review"}
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, &ValidationError{Message: "reason is required"}
	}

	exists, err := s.repo.ContentExists(ctx, contentType, contentID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrContentNotFound
	}

	return s.repo.Create(ctx, reporterUserID, contentType, contentID, reason)
}

// ListAdmin is the moderation queue (Stage 24A3) — admin-only, enforced by
// whichever route group calls this (see handler_admin.go's RegisterAdminRoutes,
// mounted under the /admin group's own auth+role middleware, the same
// choke point every other admin endpoint in this codebase goes through).
// Status/ContentType are validated against the same whitelists CreateReport
// uses when non-empty; an empty filter means "no filter," not "reject."
func (s *Service) ListAdmin(ctx context.Context, params AdminListParams) (pagination.Result[AdminReport], error) {
	if params.Status != "" && !allowedStatuses[params.Status] {
		return pagination.Result[AdminReport]{}, &ValidationError{Message: "status must be one of: open, resolved, dismissed"}
	}
	if params.ContentType != "" && !allowedContentTypes[params.ContentType] {
		return pagination.Result[AdminReport]{}, &ValidationError{Message: "content_type must be one of: question, answer, review"}
	}

	items, total, err := s.repo.ListAdmin(ctx, params)
	if err != nil {
		return pagination.Result[AdminReport]{}, err
	}
	return pagination.New(items, params.Page, params.Limit, total), nil
}

// UpdateStatus moves a report to a new status — validated against the same
// whitelist as everything else in this package. This is a status change
// only: it never touches the reported content itself (see this package's
// doc comment) — actually hiding a question/answer/review is a separate
// action through Stage 21's existing hide/show or the review-publish
// toggle, deliberately not invoked from here.
func (s *Service) UpdateStatus(ctx context.Context, id uuid.UUID, status string) (*Report, error) {
	if !allowedStatuses[status] {
		return nil, &ValidationError{Message: "status must be one of: open, resolved, dismissed"}
	}
	return s.repo.UpdateStatus(ctx, id, status)
}
