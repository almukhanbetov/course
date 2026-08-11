package videos

import (
	"time"

	"github.com/google/uuid"
)

// Legacy Stage 10 status values, kept only for the column's own sake — new
// code reads ProcessingStatus instead (see the migration comment on why
// pre-Stage-11 rows are backfilled to ProcessingStatus "ready").
const (
	StatusUploading = "uploading"
	StatusReady     = "ready"
	StatusFailed    = "failed"
)

// ProcessingStatus values (Stage 11 HLS pipeline). "Uploaded" and
// "processing" are both reported to students as a single "processing"
// state — see Service.GetPlayback.
const (
	ProcessingUploaded   = "uploaded"
	ProcessingProcessing = "processing"
	ProcessingReady      = "ready"
	ProcessingFailed     = "failed"
)

const (
	RenditionPending    = "pending"
	RenditionProcessing = "processing"
	RenditionReady      = "ready"
	RenditionFailed     = "failed"
)

const (
	JobPending    = "pending"
	JobProcessing = "processing"
	JobCompleted  = "completed"
	JobFailed     = "failed"
)

// AllowedContentTypes maps an accepted upload content type to the file
// extension used when generating the object key (never the client's
// original filename — see Service.UploadVideo).
var AllowedContentTypes = map[string]string{
	"video/mp4":  ".mp4",
	"video/webm": ".webm",
}

// LessonVideo is one upload attempt for a lesson. IsActive=true is the
// version currently served to students; at most one row per lesson may be
// active (enforced by a partial unique index), and a second, not-yet-active
// row may exist while a replacement is being transcoded — see
// Repository.ActivateVideo and the migration's comments for why this shape
// was chosen over a separate versions table.
type LessonVideo struct {
	ID               uuid.UUID `json:"id"`
	LessonID         uuid.UUID `json:"lesson_id"`
	StorageProvider  string    `json:"storage_provider"`
	Bucket           string    `json:"bucket"`
	ObjectKey        string    `json:"-"`
	OriginalFilename string    `json:"original_filename"`
	MimeType         string    `json:"mime_type"`
	SizeBytes        int64     `json:"size_bytes"`
	DurationSeconds  *int      `json:"duration_seconds,omitempty"`
	Status           string    `json:"status"` // legacy Stage 10 field
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`

	ProcessingStatus   string     `json:"processing_status"`
	SourceObjectKey    string     `json:"-"`
	HLSMasterObjectKey *string    `json:"-"`
	ProcessingError    *string    `json:"processing_error,omitempty"`
	ProcessedAt        *time.Time `json:"processed_at,omitempty"`
	IsActive           bool       `json:"is_active"`
}

// SignedVideoURL is kept for the legacy MP4 fallback shape (StreamType
// "mp4") — see VideoPlayback for the unified response.
type SignedVideoURL struct {
	URL       string `json:"url"`
	ExpiresIn int    `json:"expires_in"`
}

// VideoPlayback is the unified response for GET /api/v1/lessons/:id/video —
// see Service.GetPlayback for the state machine that fills it in.
type VideoPlayback struct {
	Status     string `json:"status"` // "processing" | "ready" | "failed"
	StreamType string `json:"stream_type,omitempty"`
	URL        string `json:"url,omitempty"`
	ExpiresIn  int    `json:"expires_in,omitempty"`
	Error      string `json:"error,omitempty"`
}

type VideoRendition struct {
	ID                uuid.UUID `json:"id"`
	LessonVideoID     uuid.UUID `json:"lesson_video_id"`
	Quality           string    `json:"quality"`
	Width             int       `json:"width"`
	Height            int       `json:"height"`
	BitrateKbps       int       `json:"bitrate_kbps"`
	PlaylistObjectKey *string   `json:"-"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type VideoJob struct {
	ID            uuid.UUID  `json:"id"`
	LessonVideoID uuid.UUID  `json:"lesson_video_id"`
	Status        string     `json:"status"`
	Attempts      int        `json:"attempts"`
	AvailableAt   time.Time  `json:"available_at"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
	LastError     *string    `json:"-"`
	CreatedAt     time.Time  `json:"created_at"`
}

// RenditionStatusSummary/AttemptStatusSummary/ProcessingStatusResponse back
// GET /api/v1/admin/lessons/:id/video/status (item 19) — "active" is always
// present once a video has ever been uploaded; "pending" only appears while
// a replacement is in flight or has failed (see Service.GetProcessingStatus).
type RenditionStatusSummary struct {
	Quality string `json:"quality"`
	Status  string `json:"status"`
}

type AttemptStatusSummary struct {
	Status     string                   `json:"status"`
	Error      string                   `json:"error,omitempty"`
	Renditions []RenditionStatusSummary `json:"renditions"`
}

type ProcessingStatusResponse struct {
	Active  *AttemptStatusSummary `json:"active,omitempty"`
	Pending *AttemptStatusSummary `json:"pending,omitempty"`
}
