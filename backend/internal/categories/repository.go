package categories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound      = errors.New("resource not found")
	ErrDuplicateSlug = errors.New("slug already exists")
)

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

const categoryColumns = "id, name, slug, description, position, active, created_at, updated_at"

// scanner is satisfied by both pgx.Row (QueryRow) and pgx.Rows (Query, one
// row at a time), so a single scan function serves both the list and
// single-row call sites below.
type scanner interface {
	Scan(dest ...any) error
}

func scanCategory(row scanner) (*Category, error) {
	var c Category
	err := row.Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.Position, &c.Active, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ListActive is the public catalog — active categories only, ordered the
// way an admin arranged them (position), with name as the tiebreaker.
func (r *Repository) ListActive(ctx context.Context) ([]Category, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+categoryColumns+` FROM categories WHERE active = true ORDER BY position ASC, name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Category{}
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *c)
	}
	return result, rows.Err()
}

func (r *Repository) ListAllAdmin(ctx context.Context) ([]Category, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+categoryColumns+` FROM categories ORDER BY position ASC, name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Category{}
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *c)
	}
	return result, rows.Err()
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*Category, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+categoryColumns+` FROM categories WHERE id = $1`, id)
	return scanCategory(row)
}

func (r *Repository) GetBySlug(ctx context.Context, slug string) (*Category, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+categoryColumns+` FROM categories WHERE slug = $1 AND active = true`, slug)
	return scanCategory(row)
}

func (r *Repository) Create(ctx context.Context, input CategoryInput) (*Category, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO categories (name, slug, description, position, active)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5)
		RETURNING `+categoryColumns,
		input.Name, input.Slug, input.Description, input.Position, input.Active)
	c, err := scanCategory(row)
	if isUniqueViolation(err) {
		return nil, ErrDuplicateSlug
	}
	return c, err
}

// Update is also how a category is deactivated (Active=false) — there is no
// Delete: a category referenced by courses must never disappear from under
// them (courses.category_id would either dangle or need cascading cleanup,
// neither of which is desirable). See handler_admin.go for confirmation
// there is genuinely no DELETE route registered.
func (r *Repository) Update(ctx context.Context, id uuid.UUID, input CategoryInput) (*Category, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE categories
		SET name = $2, slug = $3, description = NULLIF($4, ''), position = $5, active = $6, updated_at = now()
		WHERE id = $1
		RETURNING `+categoryColumns,
		id, input.Name, input.Slug, input.Description, input.Position, input.Active)
	c, err := scanCategory(row)
	if isUniqueViolation(err) {
		return nil, ErrDuplicateSlug
	}
	return c, err
}
