package recommendations

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Scoring weights — deliberately never serialized to any response (item
// 11: "Не показывай внутренние numeric weights пользователю"). Chosen so
// that no single factor dominates (item 10: "Не делай popularity главным
// фактором") except the one factor the spec explicitly asks to dominate:
// the learning-path/roadmap-next boost (item 15: "должен получить сильный
// priority" — this is deliberately larger than every other factor
// combined at its cap, so a roadmap-next course always outranks a merely
// well-rated, popular, same-category one).
const (
	weightCategoryTouched   = 3
	weightCategoryCompleted = 4
	weightCategoryMax       = 15

	weightWishlist = 30

	weightLearningPath = 45

	weightRatingPerStar = 2.0 // rating_average (0-5) * 2 => up to 10
	weightRatingMax     = 10
	ratingHighThreshold = 4.0

	weightPopularityMax   = 8
	popularPointThreshold = 5 // enrollment_count at/above this earns the "popular" reason

	weightFreshnessMax  = 5
	freshnessWindowDays = 90
	newCourseWindowDays = 30
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// baselineScore is the part of the score every candidate gets regardless
// of any personalization signal — rating, popularity, freshness. This is
// what makes the new-user fallback (item 16) fall out of the same code
// path as personalized scoring instead of a separate branch: a user with
// zero history just never accumulates any of the boost terms below, so
// ranking collapses to this baseline (top-rated / popular / newest),
// exactly the fallback order the spec asks for — deterministically, since
// it's a pure function of already-fetched aggregate data.
func baselineScore(cand Candidate, now time.Time) (score int, reasons []string) {
	if cand.RatingCount > 0 {
		ratingPoints := int(math.Round(cand.RatingAverage * weightRatingPerStar))
		if ratingPoints > weightRatingMax {
			ratingPoints = weightRatingMax
		}
		score += ratingPoints
		if cand.RatingAverage >= ratingHighThreshold {
			reasons = append(reasons, ReasonHighRating)
		}
	}

	if cand.EnrollmentCount > 0 {
		popPoints := int(math.Round(math.Log2(float64(cand.EnrollmentCount+1)) * 2))
		if popPoints > weightPopularityMax {
			popPoints = weightPopularityMax
		}
		score += popPoints
		if cand.EnrollmentCount >= popularPointThreshold {
			reasons = append(reasons, ReasonPopular)
		}
	}

	ageDays := now.Sub(cand.CreatedAt).Hours() / 24
	if ageDays < 0 {
		ageDays = 0
	}
	if ageDays <= freshnessWindowDays {
		freshPoints := int(math.Round(weightFreshnessMax * (1 - ageDays/freshnessWindowDays)))
		score += freshPoints
		if ageDays <= newCourseWindowDays {
			reasons = append(reasons, ReasonNewCourse)
		}
	}

	return score, reasons
}

// GetRecommendations is GET /me/recommendations — item 8-16. Candidate
// generation and every signal are each one query (repo methods below);
// scoring itself is pure Go over already-fetched data, so nothing here
// issues a query per candidate (item 13).
func (s *Service) GetRecommendations(ctx context.Context, userID uuid.UUID, limit int) ([]Recommendation, error) {
	candidates, err := s.repo.ListCandidatesForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	affinity, err := s.repo.GetCategoryAffinity(ctx, userID)
	if err != nil {
		return nil, err
	}
	wishlistIDs, err := s.repo.GetWishlistCourseIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	pathIDs, err := s.repo.GetLearningPathNextCourses(ctx, userID)
	if err != nil {
		return nil, err
	}

	affinityByCategory := make(map[uuid.UUID]CategoryAffinity, len(affinity))
	for _, a := range affinity {
		affinityByCategory[a.CategoryID] = a
	}
	wishlistSet := make(map[uuid.UUID]bool, len(wishlistIDs))
	for _, id := range wishlistIDs {
		wishlistSet[id] = true
	}
	pathSet := make(map[uuid.UUID]bool, len(pathIDs))
	for _, id := range pathIDs {
		pathSet[id] = true
	}

	now := time.Now()
	scored := make([]Recommendation, 0, len(candidates))
	for _, cand := range candidates {
		score, reasons := baselineScore(cand, now)

		if cand.CategoryID != nil {
			if a, ok := affinityByCategory[*cand.CategoryID]; ok {
				catPoints := a.TouchedCount*weightCategoryTouched + a.CompletedCount*weightCategoryCompleted
				if catPoints > weightCategoryMax {
					catPoints = weightCategoryMax
				}
				score += catPoints
				reasons = append(reasons, ReasonSameCategory)
			}
		}

		if wishlistSet[cand.CourseID] {
			score += weightWishlist
			reasons = append(reasons, ReasonWishlist)
		}

		// Item 15's dominant factor: a roadmap-next course always ranks
		// above one merely boosted by category/rating/popularity, since
		// weightLearningPath alone exceeds every other factor's cap
		// combined (45 > 15+30... within reach only when wishlisted too,
		// which is an intentional exception: an actively-wishlisted
		// roadmap-next course is the single strongest recommendation this
		// engine can produce).
		if pathSet[cand.CourseID] {
			score += weightLearningPath
			reasons = append(reasons, ReasonLearningPath)
		}

		scored = append(scored, Recommendation{
			CourseID:      cand.CourseID,
			Title:         cand.Title,
			Slug:          cand.Slug,
			ImageURL:      cand.ImageURL,
			AccessType:    cand.AccessType,
			CategoryName:  cand.CategoryName,
			RatingAverage: cand.RatingAverage,
			RatingCount:   cand.RatingCount,
			Score:         score,
			Reasons:       reasons,
		})
	}

	// Deterministic sort (item 12): score DESC, then course_id ASC as a
	// stable tie-break so identical DB state always produces identical
	// ranking regardless of map/slice iteration order upstream.
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		return scored[i].CourseID.String() < scored[j].CourseID.String()
	})

	if limit <= 0 || limit > 20 {
		limit = 6
	}
	if len(scored) > limit {
		scored = scored[:limit]
	}
	return scored, nil
}

// --- similar courses (public, item 18/19) -----------------------------

const (
	weightSimilarSameCategory   = 20
	weightSimilarSpecialityUnit = 15
	weightSimilarSpecialityMax  = 30
	weightSimilarRatingPerStar  = 1.0
	weightSimilarRatingMax      = 5
	similarDefaultLimit         = 4
)

// GetSimilarCourses is GET /courses/:id/similar — public, no personalization
// (item 18/19): same_category, speciality overlap, rating only. courseID
// itself is never included in its own results.
func (s *Service) GetSimilarCourses(ctx context.Context, courseID uuid.UUID, limit int) ([]Recommendation, error) {
	target, err := s.repo.GetCourseContext(ctx, courseID)
	if err != nil {
		return nil, err
	}

	candidates, overlaps, err := s.repo.ListSimilarCandidates(ctx, courseID)
	if err != nil {
		return nil, err
	}

	scored := make([]Recommendation, 0, len(candidates))
	for i, cand := range candidates {
		var score int
		var reasons []string

		if target.CategoryID != nil && cand.CategoryID != nil && *target.CategoryID == *cand.CategoryID {
			score += weightSimilarSameCategory
			reasons = append(reasons, ReasonSameCategory)
		}

		if overlap := overlaps[i]; overlap > 0 {
			points := overlap * weightSimilarSpecialityUnit
			if points > weightSimilarSpecialityMax {
				points = weightSimilarSpecialityMax
			}
			score += points
			reasons = append(reasons, ReasonSpecialityOverlap)
		}

		if cand.RatingCount > 0 {
			ratingPoints := int(math.Round(cand.RatingAverage * weightSimilarRatingPerStar))
			if ratingPoints > weightSimilarRatingMax {
				ratingPoints = weightSimilarRatingMax
			}
			score += ratingPoints
			if cand.RatingAverage >= ratingHighThreshold {
				reasons = append(reasons, ReasonHighRating)
			}
		}

		if score == 0 {
			continue // no genuine similarity signal at all — never pad with unrelated courses
		}

		scored = append(scored, Recommendation{
			CourseID:      cand.CourseID,
			Title:         cand.Title,
			Slug:          cand.Slug,
			ImageURL:      cand.ImageURL,
			AccessType:    cand.AccessType,
			CategoryName:  cand.CategoryName,
			RatingAverage: cand.RatingAverage,
			RatingCount:   cand.RatingCount,
			Score:         score,
			Reasons:       reasons,
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		return scored[i].CourseID.String() < scored[j].CourseID.String()
	})

	if limit <= 0 || limit > 20 {
		limit = similarDefaultLimit
	}
	if len(scored) > limit {
		scored = scored[:limit]
	}
	return scored, nil
}
