package audit

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	"lms-backend/internal/pagination"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create inserts one audit_logs row and returns it as actually stored.
// There is deliberately no Update or Delete method anywhere in this
// package — the table is append-only by construction (see the migration's
// own doc comment), enforced one layer below the future HTTP surface, not
// only at that surface. Metadata marshaling mirrors
// internal/notifications.Enqueue's exact json.Marshal-to-[]byte-into-jsonb
// idiom, the established convention for a JSON column in this codebase.
func (r *Repository) Create(ctx context.Context, entry AuditLog) (*AuditLog, error) {
	var metadataJSON []byte
	if entry.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(entry.Metadata)
		if err != nil {
			return nil, err
		}
	}

	var a AuditLog
	var metadataRaw []byte
	err := r.pool.QueryRow(ctx, `
		INSERT INTO audit_logs (actor_user_id, actor_role, action, entity_type, entity_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, actor_user_id, actor_role, action, entity_type, entity_id, metadata, created_at
	`, entry.ActorUserID, entry.ActorRole, entry.Action, entry.EntityType, entry.EntityID, metadataJSON).Scan(
		&a.ID, &a.ActorUserID, &a.ActorRole, &a.Action, &a.EntityType, &a.EntityID, &metadataRaw, &a.CreatedAt)
	if err != nil {
		return nil, err
	}

	if len(metadataRaw) > 0 {
		if err := json.Unmarshal(metadataRaw, &a.Metadata); err != nil {
			return nil, err
		}
	}
	return &a, nil
}

// ListAdmin is the admin-only read surface (Stage 25A3): every audit
// event, newest first, optionally filtered by actor_role/action/
// entity_type, paginated the same way every other admin list endpoint in
// this codebase is (see internal/reports.ListAdmin's identical shape).
// LEFT JOINs users — not an inner join — since actor_user_id is nullable;
// a system-generated event or an event whose actor account was later
// deleted must still appear in the list, just with a NULL actor name
// (TRIM(NULL || ' ' || NULL) is itself NULL, which scans cleanly into
// AdminAuditLog.ActorName's *string).
func (r *Repository) ListAdmin(ctx context.Context, params AdminListParams) ([]AdminAuditLog, int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT a.id, a.actor_user_id, TRIM(u.first_name || ' ' || u.last_name), a.actor_role,
			a.action, a.entity_type, a.entity_id, a.metadata, a.created_at,
			COUNT(*) OVER() AS total
		FROM audit_logs a
		LEFT JOIN users u ON u.id = a.actor_user_id
		WHERE ($1 = '' OR a.actor_role = $1)
		  AND ($2 = '' OR a.action = $2)
		  AND ($3 = '' OR a.entity_type = $3)
		ORDER BY a.created_at DESC
		LIMIT $4 OFFSET $5
	`, params.ActorRole, params.Action, params.EntityType, params.Limit, pagination.Offset(params.Page, params.Limit))
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := []AdminAuditLog{}
	total := 0
	for rows.Next() {
		var a AdminAuditLog
		var metadataRaw []byte
		if err := rows.Scan(&a.ID, &a.ActorUserID, &a.ActorName, &a.ActorRole,
			&a.Action, &a.EntityType, &a.EntityID, &metadataRaw, &a.CreatedAt, &total); err != nil {
			return nil, 0, err
		}
		if len(metadataRaw) > 0 {
			if err := json.Unmarshal(metadataRaw, &a.Metadata); err != nil {
				return nil, 0, err
			}
		}
		result = append(result, a)
	}
	return result, total, rows.Err()
}
