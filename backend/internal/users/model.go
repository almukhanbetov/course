package users

import (
	"time"

	"github.com/google/uuid"
)

type Role struct {
	ID   uuid.UUID
	Name string
}

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	FirstName    string
	LastName     string
	RoleID       uuid.UUID
	RoleName     string
	Active       bool
	Timezone     string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type PublicUser struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Role      string    `json:"role"`
	Active    bool      `json:"active"`
	Timezone  string    `json:"timezone"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (u User) Public() PublicUser {
	return PublicUser{
		ID:        u.ID,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Role:      u.RoleName,
		Active:    u.Active,
		Timezone:  u.Timezone,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

type CreateUserInput struct {
	Email        string
	PasswordHash string
	FirstName    string
	LastName     string
	RoleID       uuid.UUID
	RoleName     string
}

var AllowedRoles = map[string]bool{
	"student":    true,
	"instructor": true,
	"admin":      true,
}

// UpdateAdminInput is deliberately narrow: no email, no password_hash, no
// id. Those simply have no field here for the admin update path to set.
type UpdateAdminInput struct {
	FirstName string
	LastName  string
	Active    bool
	RoleName  string
}
