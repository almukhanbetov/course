package users

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"lms-backend/internal/notifications"
)

var (
	ErrNotFound     = errors.New("resource not found")
	ErrEmailExists  = errors.New("email already exists")
	ErrRoleNotFound = errors.New("role not found")
)

const userSelectColumns = `
	u.id, u.email, u.password_hash, u.first_name, u.last_name,
	u.role_id, r.name, u.active, u.timezone, u.created_at, u.updated_at
`

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.FirstName, &u.LastName,
		&u.RoleID, &u.RoleName, &u.Active, &u.Timezone, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repository) GetRoleByName(ctx context.Context, name string) (*Role, error) {
	var role Role
	err := r.pool.QueryRow(ctx, `SELECT id, name FROM roles WHERE name = $1`, name).Scan(&role.ID, &role.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRoleNotFound
	}
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *Repository) GetByEmail(ctx context.Context, email string) (*User, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+userSelectColumns+`
		FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE u.email = $1
	`, email)
	return scanUser(row)
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+userSelectColumns+`
		FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE u.id = $1
	`, id)
	return scanUser(row)
}

// ListAdmin returns a page of users (optionally filtered by a case-insensitive
// match against email/first_name/last_name) plus the total matching count,
// computed in the same query via a window function.
func (r *Repository) ListAdmin(ctx context.Context, search string, limit, offset int) ([]User, int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+userSelectColumns+`, COUNT(*) OVER() AS total
		FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE $1 = '' OR u.email ILIKE '%'||$1||'%' OR u.first_name ILIKE '%'||$1||'%' OR u.last_name ILIKE '%'||$1||'%'
		ORDER BY u.created_at DESC
		LIMIT $2 OFFSET $3
	`, search, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := []User{}
	total := 0
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.FirstName, &u.LastName,
			&u.RoleID, &u.RoleName, &u.Active, &u.Timezone, &u.CreatedAt, &u.UpdatedAt, &total); err != nil {
			return nil, 0, err
		}
		result = append(result, u)
	}
	return result, total, rows.Err()
}

// UpdateAdmin never touches email or password_hash — those columns have no
// place in this query at all, by design.
func (r *Repository) UpdateAdmin(ctx context.Context, id uuid.UUID, input UpdateAdminInput, roleID uuid.UUID) (*User, error) {
	var u User
	err := r.pool.QueryRow(ctx, `
		UPDATE users
		SET first_name = $2, last_name = $3, active = $4, role_id = $5, updated_at = now()
		WHERE id = $1
		RETURNING id, email, password_hash, first_name, last_name, role_id,
			(SELECT name FROM roles WHERE id = $5), active, timezone, created_at, updated_at
	`, id, input.FirstName, input.LastName, input.Active, roleID).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.FirstName, &u.LastName, &u.RoleID, &u.RoleName, &u.Active, &u.Timezone, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// Create inserts the new user and enqueues their welcome notification in
// the same transaction — registration succeeding and the welcome event
// being durably queued are one atomic outcome, so a crash right after
// commit can never lose the event (see internal/notifications).
func (r *Repository) Create(ctx context.Context, input CreateUserInput) (*User, error) {
	u := User{
		Email:        input.Email,
		PasswordHash: input.PasswordHash,
		FirstName:    input.FirstName,
		LastName:     input.LastName,
		RoleID:       input.RoleID,
		RoleName:     input.RoleName,
		Active:       true,
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, first_name, last_name, role_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, active, timezone, created_at, updated_at
	`, input.Email, input.PasswordHash, input.FirstName, input.LastName, input.RoleID).
		Scan(&u.ID, &u.Active, &u.Timezone, &u.CreatedAt, &u.UpdatedAt)
	if isUniqueViolation(err) {
		return nil, ErrEmailExists
	}
	if err != nil {
		return nil, err
	}

	if err := notifications.Enqueue(ctx, tx, notifications.EnqueueInput{
		UserID:    u.ID,
		Type:      notifications.TypeWelcome,
		Data:      map[string]any{"first_name": u.FirstName},
		DedupeKey: "welcome:" + u.ID.String(),
		Channels:  []string{notifications.ChannelInApp, notifications.ChannelEmail},
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &u, nil
}

// UpdateTimezone is the one field a student can self-service update (Stage
// 17 item 7) — everything else about a user's own row (email, name, role)
// has no self-service path at all, by design. The caller (service.go) has
// already validated timezone as a real IANA zone name before this query
// ever runs. userSelectColumns expects a joined `r` (roles) alias, which a
// plain UPDATE...RETURNING doesn't have, so this uses an UPDATE...FROM to
// join roles in the same statement.
func (r *Repository) UpdateTimezone(ctx context.Context, userID uuid.UUID, timezone string) (*User, error) {
	var u User
	err := r.pool.QueryRow(ctx, `
		UPDATE users u SET timezone = $2, updated_at = now()
		FROM roles r
		WHERE u.id = $1 AND u.role_id = r.id
		RETURNING `+userSelectColumns+`
	`, userID, timezone).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.FirstName, &u.LastName,
		&u.RoleID, &u.RoleName, &u.Active, &u.Timezone, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}
