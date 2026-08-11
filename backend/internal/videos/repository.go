package videos

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrLessonNotFound = errors.New("lesson not found")
	ErrVideoNotFound  = errors.New("lesson has no video")
	ErrNoJobAvailable = errors.New("no video job available")
	ErrJobNotFound    = errors.New("video job not found")
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const videoColumns = `id, lesson_id, storage_provider, bucket, object_key, original_filename, mime_type, size_bytes,
	duration_seconds, status, created_at, updated_at, processing_status, source_object_key,
	hls_master_object_key, processing_error, processed_at, is_active`

func scanVideo(row pgx.Row) (*LessonVideo, error) {
	var v LessonVideo
	err := row.Scan(&v.ID, &v.LessonID, &v.StorageProvider, &v.Bucket, &v.ObjectKey, &v.OriginalFilename,
		&v.MimeType, &v.SizeBytes, &v.DurationSeconds, &v.Status, &v.CreatedAt, &v.UpdatedAt,
		&v.ProcessingStatus, &v.SourceObjectKey, &v.HLSMasterObjectKey, &v.ProcessingError, &v.ProcessedAt, &v.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrVideoNotFound
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// LessonExists duplicates a one-line existence check against the courses
// domain's table rather than importing that package — same
// share-the-schema, don't-share-the-code convention used throughout this
// codebase (see e.g. courses.Repository.CountEnrollments).
func (r *Repository) LessonExists(ctx context.Context, lessonID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM lessons WHERE id = $1)`, lessonID).Scan(&exists)
	return exists, err
}

type lessonCourseInfo struct {
	CourseID        uuid.UUID
	DurationSeconds int
}

func (r *Repository) GetLessonCourseInfo(ctx context.Context, lessonID uuid.UUID) (*lessonCourseInfo, error) {
	var info lessonCourseInfo
	err := r.pool.QueryRow(ctx, `
		SELECT m.course_id, l.duration_seconds
		FROM lessons l JOIN modules m ON m.id = l.module_id
		WHERE l.id = $1
	`, lessonID).Scan(&info.CourseID, &info.DurationSeconds)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrLessonNotFound
	}
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func (r *Repository) IsEnrolled(ctx context.Context, userID, courseID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM course_enrollments WHERE user_id = $1 AND course_id = $2)
	`, userID, courseID).Scan(&exists)
	return exists, err
}

// --- lesson_videos (active / pending) -------------------------------------

func (r *Repository) GetActiveByLessonID(ctx context.Context, lessonID uuid.UUID) (*LessonVideo, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+videoColumns+` FROM lesson_videos WHERE lesson_id = $1 AND is_active = true`, lessonID)
	return scanVideo(row)
}

// GetPendingByLessonID returns the not-yet-active replacement attempt for a
// lesson, if one is currently in flight (or failed without ever having
// replaced the active video) — see the migration's comments on why this is
// modeled as a second row rather than a separate versions table.
func (r *Repository) GetPendingByLessonID(ctx context.Context, lessonID uuid.UUID) (*LessonVideo, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+videoColumns+` FROM lesson_videos WHERE lesson_id = $1 AND is_active = false`, lessonID)
	return scanVideo(row)
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*LessonVideo, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+videoColumns+` FROM lesson_videos WHERE id = $1`, id)
	return scanVideo(row)
}

// InsertVideoAttempt always creates a brand new row (never updates one in
// place) with a caller-supplied id, so the object storage key — which is
// derived from that id — can be computed and the object uploaded before the
// database ever knows about it.
func (r *Repository) InsertVideoAttempt(ctx context.Context, id, lessonID uuid.UUID, isActive bool, v LessonVideo) (*LessonVideo, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO lesson_videos (
			id, lesson_id, storage_provider, bucket, object_key, original_filename, mime_type, size_bytes,
			status, processing_status, source_object_key, is_active
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING `+videoColumns,
		id, lessonID, v.StorageProvider, v.Bucket, v.ObjectKey, v.OriginalFilename, v.MimeType, v.SizeBytes,
		v.Status, v.ProcessingStatus, v.SourceObjectKey, isActive)
	return scanVideo(row)
}

// DeleteVideoRowByID cascades to that row's renditions and jobs.
func (r *Repository) DeleteVideoRowByID(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM lesson_videos WHERE id = $1`, id)
	return err
}

func (r *Repository) MarkVideoProcessing(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE lesson_videos SET processing_status = 'processing', updated_at = now() WHERE id = $1
	`, id)
	return err
}

func (r *Repository) MarkVideoFailed(ctx context.Context, id uuid.UUID, message string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE lesson_videos SET processing_status = 'failed', processing_error = $2, updated_at = now()
		WHERE id = $1
	`, id, message)
	return err
}

// ActivateVideo marks id's HLS pipeline complete and, if id was not already
// the active row, atomically swaps it in (deactivating the previous active
// row first so the partial unique index on (lesson_id) WHERE is_active is
// never briefly violated by two active rows at once). It returns the row
// that got replaced, if any, so the caller can clean up its storage
// afterward — deliberately outside this transaction, since a storage
// failure must never roll back an already-successful activation.
func (r *Repository) ActivateVideo(ctx context.Context, videoID uuid.UUID, hlsMasterKey string) (*LessonVideo, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var lessonID uuid.UUID
	var isActive bool
	err = tx.QueryRow(ctx, `SELECT lesson_id, is_active FROM lesson_videos WHERE id = $1 FOR UPDATE`, videoID).Scan(&lessonID, &isActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrVideoNotFound
	}
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE lesson_videos SET processing_status = 'ready', hls_master_object_key = $2, processed_at = now(), updated_at = now()
		WHERE id = $1
	`, videoID, hlsMasterKey); err != nil {
		return nil, err
	}

	var replaced *LessonVideo
	if !isActive {
		row := tx.QueryRow(ctx, `SELECT `+videoColumns+` FROM lesson_videos WHERE lesson_id = $1 AND is_active = true`, lessonID)
		old, scanErr := scanVideo(row)
		if scanErr != nil && !errors.Is(scanErr, ErrVideoNotFound) {
			return nil, scanErr
		}
		if scanErr == nil {
			if _, err := tx.Exec(ctx, `UPDATE lesson_videos SET is_active = false, updated_at = now() WHERE id = $1`, old.ID); err != nil {
				return nil, err
			}
			replaced = old
		}
		if _, err := tx.Exec(ctx, `UPDATE lesson_videos SET is_active = true, updated_at = now() WHERE id = $1`, videoID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return replaced, nil
}

// --- video_renditions -------------------------------------------------

// UpsertRendition is called repeatedly through one rendition's lifecycle
// (pending -> processing -> ready|failed), each time with the fields known
// so far — playlistKey is nil until the encode actually succeeds.
func (r *Repository) UpsertRendition(ctx context.Context, lessonVideoID uuid.UUID, quality string, width, height, bitrateKbps int, status string, playlistKey *string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO video_renditions (lesson_video_id, quality, width, height, bitrate_kbps, status, playlist_object_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (lesson_video_id, quality) DO UPDATE SET
			width = EXCLUDED.width,
			height = EXCLUDED.height,
			bitrate_kbps = EXCLUDED.bitrate_kbps,
			status = EXCLUDED.status,
			playlist_object_key = COALESCE(EXCLUDED.playlist_object_key, video_renditions.playlist_object_key),
			updated_at = now()
	`, lessonVideoID, quality, width, height, bitrateKbps, status, playlistKey)
	return err
}

func (r *Repository) ListRenditions(ctx context.Context, lessonVideoID uuid.UUID) ([]VideoRendition, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, lesson_video_id, quality, width, height, bitrate_kbps, playlist_object_key, status, created_at, updated_at
		FROM video_renditions WHERE lesson_video_id = $1 ORDER BY height ASC
	`, lessonVideoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []VideoRendition{}
	for rows.Next() {
		var v VideoRendition
		if err := rows.Scan(&v.ID, &v.LessonVideoID, &v.Quality, &v.Width, &v.Height, &v.BitrateKbps,
			&v.PlaylistObjectKey, &v.Status, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

func (r *Repository) ReadyRenditionPlaylistKeys(ctx context.Context, lessonVideoID uuid.UUID) (map[string]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT quality, playlist_object_key FROM video_renditions
		WHERE lesson_video_id = $1 AND status = 'ready' AND playlist_object_key IS NOT NULL
		ORDER BY height ASC
	`, lessonVideoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string]string{}
	for rows.Next() {
		var quality, key string
		if err := rows.Scan(&quality, &key); err != nil {
			return nil, err
		}
		result[quality] = key
	}
	return result, rows.Err()
}

// --- video_jobs -------------------------------------------------------

func (r *Repository) CreateJob(ctx context.Context, lessonVideoID uuid.UUID) (*VideoJob, error) {
	var j VideoJob
	err := r.pool.QueryRow(ctx, `
		INSERT INTO video_jobs (lesson_video_id) VALUES ($1)
		RETURNING id, lesson_video_id, status, attempts, available_at, started_at, finished_at, last_error, created_at
	`, lessonVideoID).Scan(&j.ID, &j.LessonVideoID, &j.Status, &j.Attempts, &j.AvailableAt, &j.StartedAt, &j.FinishedAt, &j.LastError, &j.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// ClaimNextJob atomically picks one pending, due job and marks it
// processing — FOR UPDATE SKIP LOCKED means a second worker polling at the
// same instant simply skips a row already locked by this query instead of
// blocking or double-claiming it, so N workers can run against the same
// queue with no coordination beyond the database itself.
func (r *Repository) ClaimNextJob(ctx context.Context) (*VideoJob, error) {
	var j VideoJob
	err := r.pool.QueryRow(ctx, `
		UPDATE video_jobs
		SET status = 'processing', started_at = now(), attempts = attempts + 1
		WHERE id = (
			SELECT id FROM video_jobs
			WHERE status = 'pending' AND available_at <= now()
			ORDER BY available_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING id, lesson_video_id, status, attempts, available_at, started_at, finished_at, last_error, created_at
	`).Scan(&j.ID, &j.LessonVideoID, &j.Status, &j.Attempts, &j.AvailableAt, &j.StartedAt, &j.FinishedAt, &j.LastError, &j.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoJobAvailable
	}
	if err != nil {
		return nil, err
	}
	return &j, nil
}

func (r *Repository) MarkJobCompleted(ctx context.Context, jobID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE video_jobs SET status = 'completed', finished_at = now() WHERE id = $1
	`, jobID)
	return err
}

// MarkJobFailedOrRetry requeues the job (available after a backoff delay)
// if it still has attempts left, or marks it permanently failed once
// attempts (already incremented by ClaimNextJob) reaches maxAttempts —
// bounding retries so a permanently-broken source file can't loop forever.
func (r *Repository) MarkJobFailedOrRetry(ctx context.Context, jobID uuid.UUID, attempts, maxAttempts int, errMsg string, backoff time.Duration) (retried bool, err error) {
	if attempts >= maxAttempts {
		_, err = r.pool.Exec(ctx, `
			UPDATE video_jobs SET status = 'failed', finished_at = now(), last_error = $2 WHERE id = $1
		`, jobID, errMsg)
		return false, err
	}

	_, err = r.pool.Exec(ctx, `
		UPDATE video_jobs SET status = 'pending', available_at = now() + make_interval(secs => $2), last_error = $3 WHERE id = $1
	`, jobID, backoff.Seconds(), errMsg)
	return true, err
}
