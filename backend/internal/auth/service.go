package auth

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"lms-backend/internal/users"
)

const studentRoleName = "student"

var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

var ErrInvalidCredentials = errors.New("invalid email or password")

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

type RegisterInput struct {
	Email     string
	Password  string
	FirstName string
	LastName  string
}

type LoginInput struct {
	Email    string
	Password string
}

type Service struct {
	users          *users.Service
	jwtSecret      string
	accessTokenTTL time.Duration
}

func NewService(usersService *users.Service, jwtSecret string, accessTokenTTL time.Duration) *Service {
	return &Service{users: usersService, jwtSecret: jwtSecret, accessTokenTTL: accessTokenTTL}
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (*users.User, error) {
	if err := validateRegisterInput(input); err != nil {
		return nil, err
	}

	role, err := s.users.GetRoleByName(ctx, studentRoleName)
	if err != nil {
		return nil, err
	}

	passwordHash, err := HashPassword(input.Password)
	if err != nil {
		return nil, err
	}

	return s.users.Create(ctx, users.CreateUserInput{
		Email:        strings.ToLower(strings.TrimSpace(input.Email)),
		PasswordHash: passwordHash,
		FirstName:    strings.TrimSpace(input.FirstName),
		LastName:     strings.TrimSpace(input.LastName),
		RoleID:       role.ID,
		RoleName:     role.Name,
	})
}

func (s *Service) Login(ctx context.Context, input LoginInput) (string, *users.User, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))

	user, err := s.users.GetByEmail(ctx, email)
	if errors.Is(err, users.ErrNotFound) {
		return "", nil, ErrInvalidCredentials
	}
	if err != nil {
		return "", nil, err
	}

	if !user.Active || !ComparePassword(user.PasswordHash, input.Password) {
		return "", nil, ErrInvalidCredentials
	}

	token, err := GenerateToken(s.jwtSecret, user.ID, user.RoleName, s.accessTokenTTL)
	if err != nil {
		return "", nil, err
	}

	return token, user, nil
}

func validateRegisterInput(input RegisterInput) error {
	if !emailPattern.MatchString(strings.TrimSpace(input.Email)) {
		return &ValidationError{Message: "a valid email is required"}
	}
	if len(input.Password) < 8 {
		return &ValidationError{Message: "password must be at least 8 characters"}
	}
	if strings.TrimSpace(input.FirstName) == "" {
		return &ValidationError{Message: "first name is required"}
	}
	if strings.TrimSpace(input.LastName) == "" {
		return &ValidationError{Message: "last name is required"}
	}
	return nil
}
