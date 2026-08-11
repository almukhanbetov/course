package assignments

import (
	"time"

	"github.com/google/uuid"
)

var AllowedReviewStatuses = map[string]bool{
	"approved":       true,
	"needs_revision": true,
}

// EditableStatuses are the submission states a student may still write to
// (save a draft, add a file, submit/resubmit). "submitted" (awaiting
// review) and "approved" are locked — see service.go's checks.
var EditableStatuses = map[string]bool{
	"draft":          true,
	"needs_revision": true,
}

// Assignment is the full instructor-owned row.
type Assignment struct {
	ID           uuid.UUID `json:"id"`
	LessonID     uuid.UUID `json:"lesson_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Instructions string    `json:"instructions"`
	Required     bool      `json:"required"`
	MaxScore     *int      `json:"max_score,omitempty"`
	Published    bool      `json:"published"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type AssignmentInput struct {
	Title        string
	Description  string
	Instructions string
	Required     bool
	MaxScore     *int
	Published    bool
}

// StudentAssignment is what GET /lessons/:id/assignment returns — published
// only, and structurally incapable of leaking instructor-private fields
// (there simply is no Published/CreatedAt/UpdatedAt field here).
type StudentAssignment struct {
	ID           uuid.UUID `json:"id"`
	LessonID     uuid.UUID `json:"lesson_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Instructions string    `json:"instructions"`
	Required     bool      `json:"required"`
	MaxScore     *int      `json:"max_score,omitempty"`
}

type SubmissionFile struct {
	ID               uuid.UUID `json:"id"`
	OriginalFilename string    `json:"original_filename"`
	MimeType         string    `json:"mime_type"`
	SizeBytes        int64     `json:"size_bytes"`
	CreatedAt        time.Time `json:"created_at"`
}

// Submission is the current-state snapshot. Status/score/feedback reflect
// the most recent review, if any — the full history is in SubmissionReview
// rows, never overwritten (Stage 15 item 10).
type Submission struct {
	ID                 uuid.UUID        `json:"id"`
	AssignmentID       uuid.UUID        `json:"assignment_id"`
	UserID             uuid.UUID        `json:"user_id"`
	TextContent        *string          `json:"text_content,omitempty"`
	Status             string           `json:"status"`
	Score              *int             `json:"score,omitempty"`
	InstructorFeedback *string          `json:"instructor_feedback,omitempty"`
	SubmittedAt        *time.Time       `json:"submitted_at,omitempty"`
	ReviewedAt         *time.Time       `json:"reviewed_at,omitempty"`
	ReviewedBy         *uuid.UUID       `json:"reviewed_by,omitempty"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
	Files              []SubmissionFile `json:"files"`
}

type SubmissionReview struct {
	ID           uuid.UUID `json:"id"`
	SubmissionID uuid.UUID `json:"submission_id"`
	ReviewerID   uuid.UUID `json:"reviewer_id"`
	Status       string    `json:"status"`
	Score        *int      `json:"score,omitempty"`
	Feedback     string    `json:"feedback"`
	CreatedAt    time.Time `json:"created_at"`
}

type ReviewInput struct {
	Status   string
	Score    *int
	Feedback string
}

// InstructorSubmissionRow is one row of the instructor inbox — both the
// global GET /instructor/submissions and the per-assignment
// GET /instructor/assignments/:id/submissions reuse this shape.
type InstructorSubmissionRow struct {
	SubmissionID    uuid.UUID  `json:"submission_id"`
	AssignmentID    uuid.UUID  `json:"assignment_id"`
	AssignmentTitle string     `json:"assignment_title"`
	LessonID        uuid.UUID  `json:"lesson_id"`
	LessonTitle     string     `json:"lesson_title"`
	CourseID        uuid.UUID  `json:"course_id"`
	CourseTitle     string     `json:"course_title"`
	StudentID       uuid.UUID  `json:"student_id"`
	StudentName     string     `json:"student_name"`
	Status          string     `json:"status"`
	Score           *int       `json:"score,omitempty"`
	SubmittedAt     *time.Time `json:"submitted_at,omitempty"`
}

// SubmissionDetail is the full review-screen payload: the submission, its
// files, its resolved course/lesson/assignment context, and full review
// history.
type SubmissionDetail struct {
	Submission
	AssignmentTitle string             `json:"assignment_title"`
	MaxScore        *int               `json:"max_score,omitempty"`
	LessonID        uuid.UUID          `json:"lesson_id"`
	LessonTitle     string             `json:"lesson_title"`
	CourseID        uuid.UUID          `json:"course_id"`
	CourseTitle     string             `json:"course_title"`
	StudentName     string             `json:"student_name"`
	Reviews         []SubmissionReview `json:"reviews"`
}

type ListSubmissionsParams struct {
	// InstructorID scopes results to one instructor's courses; nil means no
	// scoping (admin — every course).
	InstructorID *uuid.UUID
	CourseID     *uuid.UUID
	AssignmentID *uuid.UUID
	Status       string
	Page         int
	Limit        int
}
