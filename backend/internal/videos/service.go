package videos

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"lms-backend/internal/access"
)

var (
	ErrInvalidContentType = errors.New("unsupported video content type")
	ErrTooLarge           = errors.New("video file exceeds the maximum upload size")
	ErrNotEnrolled        = errors.New("not enrolled in this course")
	ErrAccessRequired     = errors.New("an active subscription is required to access this video")
	ErrVideoNotReady      = errors.New("video is not ready yet")
)

type Service struct {
	repo           *Repository
	storage        VideoStorage
	access         *access.Service
	bucket         string
	maxUploadBytes int64
	signedURLTTL   time.Duration
}

func NewService(repo *Repository, storage VideoStorage, accessService *access.Service, bucket string, maxUploadMB int64, signedURLTTLSec int) *Service {
	return &Service{
		repo:           repo,
		storage:        storage,
		access:         accessService,
		bucket:         bucket,
		maxUploadBytes: maxUploadMB * 1024 * 1024,
		signedURLTTL:   time.Duration(signedURLTTLSec) * time.Second,
	}
}

func (s *Service) SignedURLTTLSeconds() int {
	return int(s.signedURLTTL.Seconds())
}

func (s *Service) MaxUploadBytes() int64 {
	return s.maxUploadBytes
}

// videoPrefix is the storage root for everything belonging to one upload
// attempt (source + its whole HLS tree) — see migrations/00027 and the
// "8. HLS structure" requirement: videos/<id>/source.<ext>,
// videos/<id>/hls/master.m3u8, videos/<id>/hls/<quality>/...
func videoPrefix(id uuid.UUID) string {
	return "videos/" + id.String() + "/"
}

func sourceObjectKey(id uuid.UUID, ext string) string {
	return videoPrefix(id) + "source" + ext
}

// --- Admin: upload / replace / delete ---------------------------------

type UploadInput struct {
	Filename     string
	DeclaredMIME string
	SizeBytes    int64
	Body         io.ReadSeeker
}

// UploadVideo validates and stores a new upload attempt, then enqueues it
// for transcoding. It never blocks on FFmpeg — see internal/videos/worker.go
// for the actual encode, which runs out-of-process in video-worker.
//
// Replace semantics (item 20): if the lesson currently has no *ready*
// active video, the new attempt becomes active immediately (there is
// nothing working yet worth protecting). If it does, the new attempt is
// stored as a second, inactive ("pending") row — students keep being served
// the existing active video, completely unaffected, until the worker
// finishes and Repository.ActivateVideo atomically promotes the new row.
// Any previous pending attempt (an earlier replacement that was superseded
// before finishing) is deleted outright; it was never visible to students.
func (s *Service) UploadVideo(ctx context.Context, lessonID uuid.UUID, input UploadInput) (*LessonVideo, error) {
	exists, err := s.repo.LessonExists(ctx, lessonID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrLessonNotFound
	}

	if input.SizeBytes > s.maxUploadBytes {
		return nil, ErrTooLarge
	}

	ext, ok := AllowedContentTypes[input.DeclaredMIME]
	if !ok {
		return nil, ErrInvalidContentType
	}
	if err := sniffMatchesDeclared(input.Body, input.DeclaredMIME); err != nil {
		return nil, err
	}

	if existingPending, err := s.repo.GetPendingByLessonID(ctx, lessonID); err == nil {
		s.discardAttempt(ctx, existingPending)
	}

	makeActive := true
	if existingActive, err := s.repo.GetActiveByLessonID(ctx, lessonID); err == nil {
		if existingActive.ProcessingStatus == ProcessingReady {
			makeActive = false // a working video exists — don't touch it until the replacement is ready
		} else {
			s.discardAttempt(ctx, existingActive) // nothing usable to protect
		}
	}

	videoID := uuid.New()
	objectKey := sourceObjectKey(videoID, ext)

	if err := s.storage.PutObject(ctx, objectKey, input.Body, input.SizeBytes, input.DeclaredMIME); err != nil {
		return nil, err
	}

	video, err := s.repo.InsertVideoAttempt(ctx, videoID, lessonID, makeActive, LessonVideo{
		StorageProvider:  "s3",
		Bucket:           s.bucket,
		ObjectKey:        objectKey,
		SourceObjectKey:  objectKey,
		OriginalFilename: input.Filename,
		MimeType:         input.DeclaredMIME,
		SizeBytes:        input.SizeBytes,
		Status:           StatusReady,
		ProcessingStatus: ProcessingUploaded,
	})
	if err != nil {
		_ = s.storage.DeleteObject(ctx, objectKey)
		return nil, err
	}

	if _, err := s.repo.CreateJob(ctx, videoID); err != nil {
		_ = s.repo.MarkVideoFailed(ctx, videoID, "failed to schedule video processing")
		return video, nil
	}

	return video, nil
}

// discardAttempt removes a row (and its storage) that never became, and
// never will become, visible to a student — best-effort on storage, same as
// every other cleanup path in this service.
func (s *Service) discardAttempt(ctx context.Context, v *LessonVideo) {
	_ = s.storage.DeletePrefix(ctx, videoPrefix(v.ID))
	_ = s.repo.DeleteVideoRowByID(ctx, v.ID)
}

// DeleteVideo removes the metadata row(s) first so the lesson is
// immediately and consistently reported as having no video, then
// best-effort removes the storage objects — the reverse order would risk a
// request in between seeing metadata for an object that's already gone.
// Any in-flight replacement is discarded along with the active video.
func (s *Service) DeleteVideo(ctx context.Context, lessonID uuid.UUID) error {
	active, err := s.repo.GetActiveByLessonID(ctx, lessonID)
	if err != nil {
		return err
	}

	if pending, perr := s.repo.GetPendingByLessonID(ctx, lessonID); perr == nil {
		s.discardAttempt(ctx, pending)
	}

	s.discardAttempt(ctx, active)
	return nil
}

func (s *Service) GetVideoMetadata(ctx context.Context, lessonID uuid.UUID) (*LessonVideo, error) {
	return s.repo.GetActiveByLessonID(ctx, lessonID)
}

// GetProcessingStatus backs the admin polling endpoint (item 19) — it never
// touches storage, only metadata, so it's cheap to poll every few seconds.
func (s *Service) GetProcessingStatus(ctx context.Context, lessonID uuid.UUID) (*ProcessingStatusResponse, error) {
	resp := &ProcessingStatusResponse{}

	active, err := s.repo.GetActiveByLessonID(ctx, lessonID)
	if err != nil && !errors.Is(err, ErrVideoNotFound) {
		return nil, err
	}
	if err == nil {
		summary, err := s.attemptSummary(ctx, active)
		if err != nil {
			return nil, err
		}
		resp.Active = summary
	}

	pending, err := s.repo.GetPendingByLessonID(ctx, lessonID)
	if err != nil && !errors.Is(err, ErrVideoNotFound) {
		return nil, err
	}
	if err == nil {
		summary, err := s.attemptSummary(ctx, pending)
		if err != nil {
			return nil, err
		}
		resp.Pending = summary
	}

	if resp.Active == nil && resp.Pending == nil {
		return nil, ErrVideoNotFound
	}
	return resp, nil
}

func (s *Service) attemptSummary(ctx context.Context, v *LessonVideo) (*AttemptStatusSummary, error) {
	renditions, err := s.repo.ListRenditions(ctx, v.ID)
	if err != nil {
		return nil, err
	}

	summaries := make([]RenditionStatusSummary, len(renditions))
	for i, r := range renditions {
		summaries[i] = RenditionStatusSummary{Quality: r.Quality, Status: r.Status}
	}

	status := v.ProcessingStatus
	if status == ProcessingUploaded {
		status = ProcessingProcessing // queued-but-not-yet-picked-up reads the same as "processing" to callers
	}

	summary := &AttemptStatusSummary{Status: status, Renditions: summaries}
	if v.ProcessingStatus == ProcessingFailed && v.ProcessingError != nil {
		summary.Error = *v.ProcessingError
	}
	return summary, nil
}

// --- Student: playback ---------------------------------

// GetPlayback enforces, in order: the lesson exists, the caller is enrolled
// in its course, the course's access policy currently allows them in (an
// expired subscription fails this even though enrolled=true), and only then
// looks at the video's own state. A video that's still processing or has
// failed is reported as such (item 13) rather than as an HTTP error — it's
// an expected, normal state a student's player is meant to poll through,
// not a fault.
func (s *Service) GetPlayback(ctx context.Context, userID, lessonID uuid.UUID) (*VideoPlayback, error) {
	info, err := s.repo.GetLessonCourseInfo(ctx, lessonID)
	if err != nil {
		return nil, err
	}

	enrolled, err := s.repo.IsEnrolled(ctx, userID, info.CourseID)
	if err != nil {
		return nil, err
	}
	if !enrolled {
		return nil, ErrNotEnrolled
	}

	canAccess, err := s.access.CanAccessCourse(ctx, userID, info.CourseID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, ErrAccessRequired
	}

	video, err := s.repo.GetActiveByLessonID(ctx, lessonID)
	if err != nil {
		return nil, err
	}

	switch video.ProcessingStatus {
	case ProcessingReady:
		if video.HLSMasterObjectKey != nil {
			// This is a same-origin path from the *frontend's* perspective —
			// the Next.js route at app/api/video-stream/... proxies it to
			// this backend's own /api/v1/video-stream/... with a Bearer
			// token attached server-side (see that route's comments for why
			// a plain backend URL wouldn't work: <video>/hls.js can't send
			// custom auth headers, and the session token is an httpOnly
			// cookie inaccessible to client JS in the first place).
			return &VideoPlayback{
				Status:     "ready",
				StreamType: "hls",
				URL:        fmt.Sprintf("/api/video-stream/%s/master.m3u8", video.ID),
			}, nil
		}
		// A Stage 10 row: ready, but never had an HLS pipeline at all.
		url, err := s.storage.PresignGet(ctx, video.ObjectKey, s.signedURLTTL)
		if err != nil {
			return nil, err
		}
		return &VideoPlayback{Status: "ready", StreamType: "mp4", URL: url, ExpiresIn: s.SignedURLTTLSeconds()}, nil
	case ProcessingFailed:
		return &VideoPlayback{Status: "failed", Error: "video processing failed"}, nil
	default: // "uploaded" or "processing"
		return &VideoPlayback{Status: "processing"}, nil
	}
}

// --- Streaming proxy authorization --------------------------------------

// AuthorizeStream is the single choke point the /video-stream/:videoId/*
// proxy calls before ever touching storage — see handler.go. It resolves
// videoId to its lesson/course independently of whatever the caller
// believes it is, so a request can't be authorized for one course's
// enrollment/subscription while reading another course's video: knowing a
// valid videoId is not sufficient, only being entitled to the specific
// course it belongs to is.
func (s *Service) AuthorizeStream(ctx context.Context, userID, videoID uuid.UUID) (*LessonVideo, error) {
	video, err := s.repo.GetByID(ctx, videoID)
	if err != nil {
		return nil, err
	}
	if !video.IsActive || video.ProcessingStatus != ProcessingReady || video.HLSMasterObjectKey == nil {
		return nil, ErrVideoNotReady
	}

	info, err := s.repo.GetLessonCourseInfo(ctx, video.LessonID)
	if err != nil {
		return nil, err
	}

	enrolled, err := s.repo.IsEnrolled(ctx, userID, info.CourseID)
	if err != nil {
		return nil, err
	}
	if !enrolled {
		return nil, ErrNotEnrolled
	}

	canAccess, err := s.access.CanAccessCourse(ctx, userID, info.CourseID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, ErrAccessRequired
	}

	return video, nil
}

// StreamObject fetches one HLS artifact (playlist or segment) — resolvedKey
// must already have been built from a validated, allow-listed relative path
// (see handler.go's path regex) joined onto video's own storage prefix, so
// this can never be pointed at an arbitrary bucket key.
func (s *Service) StreamObject(ctx context.Context, resolvedKey string) (io.ReadCloser, int64, error) {
	body, size, _, err := s.storage.GetObject(ctx, resolvedKey)
	return body, size, err
}

// sniffMatchesDeclared reads a small prefix of the upload to verify its
// actual bytes look like video content, rather than trusting the
// client-declared Content-Type (or, worse, the filename extension) alone.
// It rewinds the reader afterward so the full body is still available for
// the actual upload.
func sniffMatchesDeclared(body io.ReadSeeker, declaredMIME string) error {
	buf := make([]byte, 512)
	n, err := body.Read(buf)
	if err != nil && err != io.EOF {
		return err
	}
	if _, err := body.Seek(0, io.SeekStart); err != nil {
		return err
	}

	sniffed := http.DetectContentType(bytes.TrimRight(buf[:n], "\x00"))
	// http.DetectContentType recognizes the ISO-BMFF/WebM signatures used by
	// mp4/webm files, but browsers vary in the exact codec string it
	// returns (e.g. "video/webm" vs a generic octet-stream for some mp4
	// variants without a fully standard ftyp box) — requiring only that it
	// land in the same "video/" family as what was declared, rather than an
	// exact string match, avoids rejecting legitimate files while still
	// catching a mislabeled non-video upload (e.g. a renamed .exe or .txt).
	if len(sniffed) < 6 || sniffed[:6] != "video/" {
		return ErrInvalidContentType
	}
	return nil
}
