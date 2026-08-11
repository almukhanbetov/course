package certificates

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrForbidden = errors.New("you may only view your own certificates")

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func generateCertificateNumber() (string, error) {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("CERT-%d-%s", time.Now().UTC().Year(), strings.ToUpper(hex.EncodeToString(buf))), nil
}

func (s *Service) issueCertificate(ctx context.Context, userID, courseID uuid.UUID) (*Certificate, error) {
	const maxAttempts = 5

	for attempt := 0; attempt < maxAttempts; attempt++ {
		number, err := generateCertificateNumber()
		if err != nil {
			return nil, err
		}

		cert, err := s.repo.CreateCertificate(ctx, userID, courseID, number)
		switch {
		case err == nil:
			return cert, nil
		case errors.Is(err, ErrDuplicateNumber):
			continue
		case errors.Is(err, ErrAlreadyIssued):
			return s.repo.GetByUserAndCourse(ctx, userID, courseID)
		default:
			return nil, err
		}
	}

	return nil, fmt.Errorf("failed to generate a unique certificate number after %d attempts", maxAttempts)
}

// ListMyCertificates lazily issues a certificate for any course the user has
// already completed but doesn't have one for yet, then returns the full
// list. This is deliberately the only place issuance happens: eligibility
// is derived entirely from course_enrollments.completed_at (owned by the
// learning/tests domains), never from anything the caller supplies, so a
// student can't request a certificate for an unfinished course. Because
// /dashboard/courses also calls this endpoint, a freshly-completed course
// gets its certificate the moment the student next looks at their courses —
// automatic in every way that matters, without a cross-domain event hook.
func (s *Service) ListMyCertificates(ctx context.Context, userID uuid.UUID) ([]CertificateSummary, error) {
	pendingCourseIDs, err := s.repo.GetCompletedCourseIDsWithoutCertificate(ctx, userID)
	if err != nil {
		return nil, err
	}

	for _, courseID := range pendingCourseIDs {
		if _, err := s.issueCertificate(ctx, userID, courseID); err != nil {
			return nil, err
		}
	}

	return s.repo.ListByUser(ctx, userID)
}

func (s *Service) GetMyCertificate(ctx context.Context, userID, certID uuid.UUID) (*CertificateDetail, error) {
	detail, ownerID, err := s.repo.GetDetailByID(ctx, certID)
	if err != nil {
		return nil, err
	}
	if ownerID != userID {
		return nil, ErrForbidden
	}
	return detail, nil
}

func (s *Service) VerifyCertificate(ctx context.Context, number string) (*PublicVerification, error) {
	return s.repo.VerifyByCertificateNumber(ctx, number)
}

// ListAdmin is intentionally the only admin-facing method this domain
// exposes — see the repository's ListAdmin docs for why there's no
// create/update/delete counterpart.
func (s *Service) ListAdmin(ctx context.Context, search string, limit, offset int) ([]AdminCertificateSummary, int, error) {
	return s.repo.ListAdmin(ctx, search, limit, offset)
}
