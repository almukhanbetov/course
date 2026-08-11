package assignments

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"lms-backend/internal/access"
)

var (
	ErrInvalidFileType   = errors.New("unsupported file type")
	ErrTooLarge          = errors.New("file exceeds the maximum upload size")
	ErrNotEnrolled       = errors.New("not enrolled in this course")
	ErrAccessRequired    = errors.New("an active subscription is required to access this course")
	ErrLessonUnpublished = errors.New("lesson is not published")
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// Storage is the minimal object-storage surface assignments needs — the
// same method shapes *videos.S3Storage already implements, so main.go
// passes the existing video storage client straight in (same bucket, a
// distinct "assignments/" key prefix — see buildObjectKey). No second S3
// client, no duplicated credential wiring; Go's structural interfaces make
// this reuse free.
type Storage interface {
	PutObject(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	DeleteObject(ctx context.Context, key string) error
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
}

const downloadURLTTL = 5 * time.Minute

// maxFilesPerSubmission is a sane abuse guard (not from the spec) — nothing
// requires an unbounded number of attachments per homework submission.
const maxFilesPerSubmission = 10

type Service struct {
	repo           *Repository
	storage        Storage
	access         *access.Service
	maxUploadBytes int64
}

func NewService(repo *Repository, storage Storage, accessService *access.Service, maxUploadMB int64) *Service {
	return &Service{
		repo:           repo,
		storage:        storage,
		access:         accessService,
		maxUploadBytes: maxUploadMB * 1024 * 1024,
	}
}

func (s *Service) MaxUploadBytes() int64 {
	return s.maxUploadBytes
}

func validateAssignmentInput(input AssignmentInput) error {
	if strings.TrimSpace(input.Title) == "" {
		return &ValidationError{Message: "title is required"}
	}
	if input.MaxScore != nil && *input.MaxScore <= 0 {
		return &ValidationError{Message: "max_score must be positive"}
	}
	return nil
}

// --- instructor CRUD -----------------------------------------------------

func (s *Service) CreateAssignment(ctx context.Context, lessonID uuid.UUID, input AssignmentInput) (*Assignment, error) {
	if err := validateAssignmentInput(input); err != nil {
		return nil, err
	}
	return s.repo.CreateAssignment(ctx, lessonID, input)
}

func (s *Service) GetAssignmentByLesson(ctx context.Context, lessonID uuid.UUID) (*Assignment, error) {
	return s.repo.GetAssignmentByLesson(ctx, lessonID)
}

func (s *Service) GetAssignment(ctx context.Context, id uuid.UUID) (*Assignment, error) {
	return s.repo.GetAssignment(ctx, id)
}

func (s *Service) UpdateAssignment(ctx context.Context, id uuid.UUID, input AssignmentInput) (*Assignment, error) {
	if err := validateAssignmentInput(input); err != nil {
		return nil, err
	}
	return s.repo.UpdateAssignment(ctx, id, input)
}

func (s *Service) DeleteAssignment(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteAssignment(ctx, id)
}

// --- student access checks ------------------------------------------------

// checkStudentAccess is the one place that enforces "enrolled AND active
// course access" for every student-facing assignment/submission endpoint —
// mirrors tests.Service's own enrollment+access gate, duplicated per this
// codebase's cross-domain convention.
func (s *Service) checkStudentAccess(ctx context.Context, userID, courseID uuid.UUID) error {
	enrolled, err := s.repo.IsEnrolled(ctx, userID, courseID)
	if err != nil {
		return err
	}
	if !enrolled {
		return ErrNotEnrolled
	}
	canAccess, err := s.access.CanAccessCourse(ctx, userID, courseID)
	if err != nil {
		return err
	}
	if !canAccess {
		return ErrAccessRequired
	}
	return nil
}

// GetStudentAssignment resolves and authorizes the student-facing view of a
// lesson's assignment in one call: lesson must exist and be published, the
// student must be enrolled with active course access, and the assignment
// itself must be published (an unpublished/draft assignment is treated as
// not existing, same as an unpublished test).
func (s *Service) GetStudentAssignment(ctx context.Context, userID, lessonID uuid.UUID) (*StudentAssignment, error) {
	lc, err := s.repo.GetLessonContext(ctx, lessonID)
	if err != nil {
		return nil, err
	}
	if !lc.Published {
		return nil, ErrLessonUnpublished
	}
	if err := s.checkStudentAccess(ctx, userID, lc.CourseID); err != nil {
		return nil, err
	}

	a, err := s.repo.GetPublishedAssignmentByLesson(ctx, lessonID)
	if err != nil {
		return nil, err
	}
	return &StudentAssignment{
		ID:           a.ID,
		LessonID:     a.LessonID,
		Title:        a.Title,
		Description:  a.Description,
		Instructions: a.Instructions,
		Required:     a.Required,
		MaxScore:     a.MaxScore,
	}, nil
}

// authorizeForSubmission re-derives and checks student access from an
// assignment id alone — used by every /assignments/:id/submission* route.
func (s *Service) authorizeForSubmission(ctx context.Context, userID, assignmentID uuid.UUID) (*Assignment, error) {
	a, err := s.repo.GetAssignment(ctx, assignmentID)
	if err != nil {
		return nil, err
	}
	if !a.Published {
		return nil, ErrNotFound
	}
	lc, err := s.repo.GetLessonContext(ctx, a.LessonID)
	if err != nil {
		return nil, err
	}
	if err := s.checkStudentAccess(ctx, userID, lc.CourseID); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *Service) GetSubmission(ctx context.Context, userID, assignmentID uuid.UUID) (*Submission, error) {
	if _, err := s.authorizeForSubmission(ctx, userID, assignmentID); err != nil {
		return nil, err
	}
	return s.repo.GetSubmission(ctx, assignmentID, userID)
}

func (s *Service) SaveDraft(ctx context.Context, userID, assignmentID uuid.UUID, textContent string) (*Submission, error) {
	if _, err := s.authorizeForSubmission(ctx, userID, assignmentID); err != nil {
		return nil, err
	}
	if err := s.checkEditable(ctx, assignmentID, userID); err != nil {
		return nil, err
	}
	return s.repo.SaveDraftText(ctx, assignmentID, userID, textContent)
}

// checkEditable returns ErrNotEditable if a submission already exists and
// is locked (submitted/approved) — a student can't rewrite homework that's
// awaiting review or already graded. No existing row at all is always fine
// (an implicit fresh draft).
func (s *Service) checkEditable(ctx context.Context, assignmentID, userID uuid.UUID) error {
	existing, err := s.repo.GetSubmission(ctx, assignmentID, userID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !EditableStatuses[existing.Status] {
		return ErrNotEditable
	}
	return nil
}

// UploadInput mirrors videos.UploadInput's shape.
type UploadInput struct {
	Filename     string
	DeclaredMIME string
	SizeBytes    int64
	Body         io.ReadSeeker
}

func (s *Service) UploadFile(ctx context.Context, userID, assignmentID uuid.UUID, input UploadInput) (*SubmissionFile, error) {
	assignment, err := s.authorizeForSubmission(ctx, userID, assignmentID)
	if err != nil {
		return nil, err
	}
	if err := s.checkEditable(ctx, assignmentID, userID); err != nil {
		return nil, err
	}
	if input.SizeBytes > s.maxUploadBytes {
		return nil, ErrTooLarge
	}

	mimeType, ext, err := detectAllowedMIME(input.Body)
	if err != nil {
		return nil, err
	}

	submission, err := s.repo.EnsureDraft(ctx, assignmentID, userID)
	if err != nil {
		return nil, err
	}
	if len(submission.Files) >= maxFilesPerSubmission {
		return nil, &ValidationError{Message: fmt.Sprintf("a submission may have at most %d files", maxFilesPerSubmission)}
	}

	objectKey := buildObjectKey(assignment.ID, submission.ID, ext)
	if err := s.storage.PutObject(ctx, objectKey, input.Body, input.SizeBytes, mimeType); err != nil {
		return nil, err
	}

	file, err := s.repo.InsertFile(ctx, submission.ID, objectKey, input.Filename, mimeType, input.SizeBytes)
	if err != nil {
		// Storage already has the object but the DB write failed — clean up
		// best-effort so we don't leak an orphaned blob. Never the reverse
		// order (DB row before a confirmed PutObject): that would be the
		// "phantom file row" item 29 explicitly calls out to avoid.
		_ = s.storage.DeleteObject(ctx, objectKey)
		return nil, err
	}
	return file, nil
}

func (s *Service) Submit(ctx context.Context, userID, assignmentID uuid.UUID) (*Submission, error) {
	if _, err := s.authorizeForSubmission(ctx, userID, assignmentID); err != nil {
		return nil, err
	}
	return s.repo.Submit(ctx, assignmentID, userID)
}

// --- file download ---------------------------------------------------

// AuthorizeDownload returns a short-lived signed URL for one submission
// file, after checking the caller is either the submission's own student,
// an instructor who manages the file's course, or an admin. canManage is
// injected by the handler (ownership.Service.CanManageCourse) so this
// package never needs to know about roles/JWTs itself.
func (s *Service) AuthorizeDownload(ctx context.Context, userID uuid.UUID, fileID uuid.UUID, canManage func(courseID uuid.UUID) (bool, error)) (string, string, error) {
	fc, err := s.repo.GetFileDownloadContext(ctx, fileID)
	if err != nil {
		return "", "", err
	}

	if fc.SubmissionUserID != userID {
		allowed, err := canManage(fc.CourseID)
		if err != nil {
			return "", "", err
		}
		if !allowed {
			return "", "", ErrNotFound // 404, not 403 — never confirm the file exists to an unauthorized caller
		}
	}

	url, err := s.storage.PresignGet(ctx, fc.ObjectKey, downloadURLTTL)
	if err != nil {
		return "", "", err
	}
	return url, fc.OriginalFilename, nil
}

// --- instructor review ----------------------------------------------

func (s *Service) ListSubmissions(ctx context.Context, params ListSubmissionsParams) ([]InstructorSubmissionRow, int, error) {
	return s.repo.ListSubmissions(ctx, params)
}

func (s *Service) GetSubmissionDetail(ctx context.Context, id uuid.UUID) (*SubmissionDetail, error) {
	return s.repo.GetSubmissionDetail(ctx, id)
}

func validateReviewInput(input ReviewInput, maxScore *int) error {
	if !AllowedReviewStatuses[input.Status] {
		return &ValidationError{Message: "status must be one of: approved, needs_revision"}
	}
	if input.Score != nil {
		if *input.Score < 0 {
			return &ValidationError{Message: "score must not be negative"}
		}
		if maxScore != nil && *input.Score > *maxScore {
			return &ValidationError{Message: fmt.Sprintf("score must be between 0 and %d", *maxScore)}
		}
	}
	return nil
}

func (s *Service) Review(ctx context.Context, submissionID, reviewerID uuid.UUID, input ReviewInput) (*Submission, error) {
	detail, err := s.repo.GetSubmissionDetail(ctx, submissionID)
	if err != nil {
		return nil, err
	}
	if err := validateReviewInput(input, detail.MaxScore); err != nil {
		return nil, err
	}
	return s.repo.Review(ctx, submissionID, reviewerID, input)
}

// --- file type detection ----------------------------------------------

// detectAllowedMIME sniffs the actual bytes of an upload (never trusting
// the client-declared Content-Type or filename extension alone — item 4)
// against the Stage 15 whitelist: PDF, TXT, ZIP, PNG, JPEG.
func detectAllowedMIME(body io.ReadSeeker) (mimeType, ext string, err error) {
	buf := make([]byte, 512)
	n, rerr := body.Read(buf)
	if rerr != nil && rerr != io.EOF {
		return "", "", rerr
	}
	if _, serr := body.Seek(0, io.SeekStart); serr != nil {
		return "", "", serr
	}

	sniffed := http.DetectContentType(bytes.TrimRight(buf[:n], "\x00"))
	switch {
	case strings.HasPrefix(sniffed, "application/pdf"):
		return "application/pdf", ".pdf", nil
	case strings.HasPrefix(sniffed, "application/zip"):
		return "application/zip", ".zip", nil
	case strings.HasPrefix(sniffed, "image/png"):
		return "image/png", ".png", nil
	case strings.HasPrefix(sniffed, "image/jpeg"):
		return "image/jpeg", ".jpg", nil
	case strings.HasPrefix(sniffed, "text/plain"):
		return "text/plain", ".txt", nil
	default:
		return "", "", ErrInvalidFileType
	}
}

// buildObjectKey never uses the caller-supplied filename — every path
// segment is a server-generated UUID, so there is no way to path-traverse
// or collide with another submission's files (item 16).
func buildObjectKey(assignmentID, submissionID uuid.UUID, ext string) string {
	return fmt.Sprintf("assignments/%s/%s/%s%s", assignmentID, submissionID, uuid.New(), ext)
}
