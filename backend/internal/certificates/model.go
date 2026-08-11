package certificates

import (
	"time"

	"github.com/google/uuid"
)

type Certificate struct {
	ID                uuid.UUID `json:"id"`
	UserID            uuid.UUID `json:"user_id"`
	CourseID          uuid.UUID `json:"course_id"`
	CertificateNumber string    `json:"certificate_number"`
	IssuedAt          time.Time `json:"issued_at"`
	CreatedAt         time.Time `json:"created_at"`
}

// CertificateSummary is the shape returned by GET /api/v1/me/certificates.
type CertificateSummary struct {
	ID                uuid.UUID `json:"id"`
	CertificateNumber string    `json:"certificate_number"`
	CourseID          uuid.UUID `json:"course_id"`
	CourseTitle       string    `json:"course_title"`
	IssuedAt          time.Time `json:"issued_at"`
}

// CertificateDetail is the "pretty view" shape used to render the
// certificate document at GET /api/v1/me/certificates/:id.
type CertificateDetail struct {
	ID                uuid.UUID `json:"id"`
	CertificateNumber string    `json:"certificate_number"`
	CourseID          uuid.UUID `json:"course_id"`
	CourseTitle       string    `json:"course_title"`
	StudentName       string    `json:"student_name"`
	IssuedAt          time.Time `json:"issued_at"`
}

// PublicVerification is the only shape the public verify endpoint ever
// returns — no email, no user id, no JWT/auth data of any kind, on purpose.
type PublicVerification struct {
	Valid             bool       `json:"valid"`
	CertificateNumber string     `json:"certificate_number,omitempty"`
	StudentName       string     `json:"student_name,omitempty"`
	CourseTitle       string     `json:"course_title,omitempty"`
	IssuedAt          *time.Time `json:"issued_at,omitempty"`
}

// AdminCertificateSummary is for the admin-only certificates list — unlike
// PublicVerification, it's fine to include the student's email here since
// the caller is already an authenticated admin.
type AdminCertificateSummary struct {
	ID                uuid.UUID `json:"id"`
	CertificateNumber string    `json:"certificate_number"`
	StudentName       string    `json:"student_name"`
	StudentEmail      string    `json:"student_email"`
	CourseTitle       string    `json:"course_title"`
	IssuedAt          time.Time `json:"issued_at"`
}
