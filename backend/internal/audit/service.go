package audit

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"lms-backend/internal/pagination"
)

// Bounds on action/entity_type — a length cap, not a whitelist (see
// model.go's doc comment on why this package never enforces one). Prevents
// a pathological caller from writing an unbounded string into a column
// that's meant to stay short and queryable.
const (
	maxActionLength     = 100
	maxEntityTypeLength = 100
)

var (
	ErrActionRequired     = errors.New("action is required")
	ErrEntityTypeRequired = errors.New("entity_type is required")
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// LogInput is Service.Log's argument shape. ActorUserID/ActorRole should
// always be sourced from a verified identity the caller already has (e.g.
// authctx.UserID/authctx.Role in whatever future handler calls this) — this
// package takes them as trusted Go values, not raw HTTP input, so it does
// not re-validate ActorRole against the known role vocabulary the way a
// public-facing endpoint would validate user-supplied text. Pass a nil
// ActorUserID only for a genuine system-generated event with no human
// actor (see model.go's doc comment) — this is a deliberate choice for
// each call site to make, not a default this method silently falls back
// to.
type LogInput struct {
	ActorUserID *uuid.UUID
	ActorRole   *string
	Action      string
	EntityType  string
	EntityID    *uuid.UUID
	Metadata    map[string]any
}

// Log appends one audit event. Only structural validation (non-empty,
// bounded length) — action/entity_type membership is never checked
// against a whitelist (see model.go). Callers are responsible for never
// putting a password, token, session cookie, or payment secret into
// Metadata; this method has no way to detect that from arbitrary
// map[string]any content, so the discipline lives at each future call
// site, not here (matches the roadmap's own security requirement: "explicit
// review of every call site").
func (s *Service) Log(ctx context.Context, input LogInput) (*AuditLog, error) {
	action := strings.TrimSpace(input.Action)
	if action == "" {
		return nil, ErrActionRequired
	}
	if len(action) > maxActionLength {
		action = action[:maxActionLength]
	}

	entityType := strings.TrimSpace(input.EntityType)
	if entityType == "" {
		return nil, ErrEntityTypeRequired
	}
	if len(entityType) > maxEntityTypeLength {
		entityType = entityType[:maxEntityTypeLength]
	}

	return s.repo.Create(ctx, AuditLog{
		ActorUserID: input.ActorUserID,
		ActorRole:   input.ActorRole,
		Action:      action,
		EntityType:  entityType,
		EntityID:    input.EntityID,
		Metadata:    input.Metadata,
	})
}

// ListAdmin is the moderation/accountability read surface (Stage 25A3) —
// admin-only, enforced by whichever route group calls this (see
// handler_admin.go's RegisterAdminRoutes, mounted under the /admin group's
// own auth+role middleware, the same choke point every other admin
// endpoint in this codebase goes through). Filters are optional and
// unvalidated (see AdminListParams' own doc comment).
func (s *Service) ListAdmin(ctx context.Context, params AdminListParams) (pagination.Result[AdminAuditLog], error) {
	items, total, err := s.repo.ListAdmin(ctx, params)
	if err != nil {
		return pagination.Result[AdminAuditLog]{}, err
	}
	return pagination.New(items, params.Page, params.Limit, total), nil
}
