package reports

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lms-backend/internal/pagination"
)

// ErrDuplicateActiveReport is returned when the same reporter already has
// an open report against the same content — the partial unique index
// (idx_content_reports_active_unique) is what actually enforces this; this
// error is how Create surfaces that to its caller instead of a raw
// constraint-violation error.
var ErrDuplicateActiveReport = errors.New("an open report from this user for this content already exists")

// ErrNotFound is returned when a report id doesn't exist — see UpdateStatus.
var ErrNotFound = errors.New("report not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// contentTable maps a pre-validated content_type to its real table name —
// mirrors internal/courses.courseSortClause's own doc comment convention
// exactly: this never sees raw, attacker-controlled text, since the
// service rejects anything not in allowedContentTypes before ContentExists
// is ever called, and the switch below only ever assigns one of three
// fixed Go string literals. Safe to use directly in a query string for
// that reason — there is no path from user input into this identifier.
func contentTable(contentType string) (table string, ok bool) {
	switch contentType {
	case ContentTypeQuestion:
		return "lesson_questions", true
	case ContentTypeAnswer:
		return "question_answers", true
	case ContentTypeReview:
		return "course_reviews", true
	default:
		return "", false
	}
}

// ContentExists checks whether the referenced content actually exists —
// the substitute for the real foreign key content_id can't have (see the
// migration's doc comment: one column can't conditionally reference three
// different tables). Called by the service before ever inserting a report.
func (r *Repository) ContentExists(ctx context.Context, contentType string, contentID uuid.UUID) (bool, error) {
	table, ok := contentTable(contentType)
	if !ok {
		return false, nil
	}

	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM `+table+` WHERE id = $1)`, contentID).Scan(&exists)
	return exists, err
}

// Create inserts a new report with status defaulting to 'open' (the
// column's own DB default — never set explicitly here). ON CONFLICT
// against the partial unique index makes a duplicate active report a
// detectable no-op rather than a raw constraint-violation error: the
// INSERT affects zero rows, so the RETURNING clause yields no row, which
// pgx surfaces as pgx.ErrNoRows — mapped here to ErrDuplicateActiveReport.
func (r *Repository) Create(ctx context.Context, reporterUserID uuid.UUID, contentType string, contentID uuid.UUID, reason string) (*Report, error) {
	var rpt Report
	err := r.pool.QueryRow(ctx, `
		INSERT INTO content_reports (reporter_user_id, content_type, content_id, reason)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (reporter_user_id, content_type, content_id) WHERE status = 'open' DO NOTHING
		RETURNING id, reporter_user_id, content_type, content_id, reason, status, created_at, updated_at
	`, reporterUserID, contentType, contentID, reason).Scan(
		&rpt.ID, &rpt.ReporterUserID, &rpt.ContentType, &rpt.ContentID, &rpt.Reason, &rpt.Status, &rpt.CreatedAt, &rpt.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrDuplicateActiveReport
	}
	if err != nil {
		return nil, err
	}
	return &rpt, nil
}

// ListAdmin is the moderation queue (Stage 24A3): every report regardless
// of status by default, optionally filtered to one status and/or one
// content_type, newest first, paginated the same way every other admin
// list endpoint in this codebase is (see internal/reviews.ListAdmin's
// identical shape). Joins users only for the reporter's display name — no
// join to the reported content itself is possible generically, since
// content_type/content_id are polymorphic (see this package's doc comment).
func (r *Repository) ListAdmin(ctx context.Context, params AdminListParams) ([]AdminReport, int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT cr.id, cr.reporter_user_id, TRIM(u.first_name || ' ' || u.last_name),
			cr.content_type, cr.content_id, cr.reason, cr.status, cr.created_at,
			COUNT(*) OVER() AS total
		FROM content_reports cr
		JOIN users u ON u.id = cr.reporter_user_id
		WHERE ($1 = '' OR cr.status = $1)
		  AND ($2 = '' OR cr.content_type = $2)
		ORDER BY cr.created_at DESC
		LIMIT $3 OFFSET $4
	`, params.Status, params.ContentType, params.Limit, pagination.Offset(params.Page, params.Limit))
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := []AdminReport{}
	total := 0
	for rows.Next() {
		var ar AdminReport
		if err := rows.Scan(&ar.ID, &ar.ReporterUserID, &ar.ReporterName,
			&ar.ContentType, &ar.ContentID, &ar.Reason, &ar.Status, &ar.CreatedAt, &total); err != nil {
			return nil, 0, err
		}
		result = append(result, ar)
	}
	return result, total, rows.Err()
}

// UpdateStatus moves a report to a new status (Stage 24A3) — a status
// change only, never touching the reported content itself: actually
// hiding a question/answer/review still goes through Stage 21's existing
// hide/show or the review-publish toggle, a deliberately separate action
// from this one (see this package's doc comment).
func (r *Repository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) (*Report, error) {
	var rpt Report
	err := r.pool.QueryRow(ctx, `
		UPDATE content_reports SET status = $2, updated_at = now()
		WHERE id = $1
		RETURNING id, reporter_user_id, content_type, content_id, reason, status, created_at, updated_at
	`, id, status).Scan(&rpt.ID, &rpt.ReporterUserID, &rpt.ContentType, &rpt.ContentID, &rpt.Reason, &rpt.Status, &rpt.CreatedAt, &rpt.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rpt, nil
}
