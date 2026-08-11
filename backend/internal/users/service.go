package users

import (
	"context"
	"time"

	// See internal/activity/model.go's identical import for why this is
	// required: none of this repo's (Alpine-based) Dockerfiles ship
	// /usr/share/zoneinfo, so time.LoadLocation needs the tzdata database
	// embedded directly in the binary to validate a real IANA zone name.
	_ "time/tzdata"

	"github.com/google/uuid"
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) GetByEmail(ctx context.Context, email string) (*User, error) {
	return s.repo.GetByEmail(ctx, email)
}

func (s *Service) GetRoleByName(ctx context.Context, name string) (*Role, error) {
	return s.repo.GetRoleByName(ctx, name)
}

func (s *Service) Create(ctx context.Context, input CreateUserInput) (*User, error) {
	return s.repo.Create(ctx, input)
}

func (s *Service) ListAdmin(ctx context.Context, search string, limit, offset int) ([]User, int, error) {
	return s.repo.ListAdmin(ctx, search, limit, offset)
}

func (s *Service) UpdateAdmin(ctx context.Context, id uuid.UUID, input UpdateAdminInput) (*User, error) {
	if !AllowedRoles[input.RoleName] {
		return nil, &ValidationError{Message: "role must be one of: student, instructor, admin"}
	}

	role, err := s.repo.GetRoleByName(ctx, input.RoleName)
	if err != nil {
		return nil, err
	}

	return s.repo.UpdateAdmin(ctx, id, input, role.ID)
}

// UpdateTimezone validates timezone as a real IANA zone name via Go's own
// tzdata lookup before ever writing it — item 7: "Не принимай arbitrary
// invalid timezone. Валидируй IANA timezone." time.LoadLocation is exactly
// that validation: it succeeds only for a name the Go runtime's embedded
// (or system) tzdata actually recognizes, so "Europe/Moscow" passes and
// "Not/AZone" or an offset string like "+03:00" both fail.
func (s *Service) UpdateTimezone(ctx context.Context, userID uuid.UUID, timezone string) (*User, error) {
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, &ValidationError{Message: "timezone must be a valid IANA timezone name (e.g. Europe/Moscow)"}
	}
	return s.repo.UpdateTimezone(ctx, userID, timezone)
}
