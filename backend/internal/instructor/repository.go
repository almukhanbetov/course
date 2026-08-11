package instructor

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func calculatePercent(completed, total int) int {
	if total == 0 {
		return 0
	}
	return completed * 100 / total
}

// lessonProgressJoins is shared by ListStudents/ListCourseStudents/Stats/
// CourseStats — every one of them needs "lessons published in this course,
// joined against this enrollment's own lesson_progress rows" to compute a
// completion percentage, and duplicating the join instead of the whole
// query keeps each call site's WHERE/GROUP BY readable.
const lessonProgressJoins = `
	LEFT JOIN modules m ON m.course_id = c.id
	LEFT JOIN lessons l ON l.module_id = m.id
	LEFT JOIN lesson_progress lp ON lp.lesson_id = l.id AND lp.user_id = ce.user_id
`

// ListStudents is every (student, course) pair across all courses this
// instructor owns — a single aggregated query, no per-student round trips.
func (r *Repository) ListStudents(ctx context.Context, instructorID uuid.UUID, limit, offset int) ([]StudentRow, int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			u.id, TRIM(u.first_name || ' ' || u.last_name),
			c.id, c.title, ce.enrolled_at, ce.completed_at,
			COUNT(l.id) FILTER (WHERE l.published) AS total_lessons,
			COUNT(lp.id) FILTER (WHERE l.published AND lp.completed) AS completed_lessons,
			COUNT(*) OVER() AS total
		FROM course_enrollments ce
		JOIN courses c ON c.id = ce.course_id
		JOIN users u ON u.id = ce.user_id
		`+lessonProgressJoins+`
		WHERE c.instructor_id = $1
		GROUP BY u.id, u.first_name, u.last_name, c.id, c.title, ce.enrolled_at, ce.completed_at
		ORDER BY ce.enrolled_at DESC
		LIMIT $2 OFFSET $3
	`, instructorID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := []StudentRow{}
	total := 0
	for rows.Next() {
		var s StudentRow
		var totalLessons, completedLessons int
		if err := rows.Scan(&s.UserID, &s.DisplayName, &s.CourseID, &s.CourseTitle, &s.EnrolledAt, &s.CompletedAt,
			&totalLessons, &completedLessons, &total); err != nil {
			return nil, 0, err
		}
		s.ProgressPercent = calculatePercent(completedLessons, totalLessons)
		s.Completed = s.CompletedAt != nil
		result = append(result, s)
	}
	return result, total, rows.Err()
}

// ListCourseStudents is the per-course roster. Ownership of courseID is the
// handler's responsibility (via ownership.Service, which allows both the
// owning instructor and any admin) — this query only scopes by courseID, to
// avoid rejecting a legitimate admin caller who isn't the course's instructor.
func (r *Repository) ListCourseStudents(ctx context.Context, courseID uuid.UUID, limit, offset int) ([]CourseStudentRow, int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			u.id, TRIM(u.first_name || ' ' || u.last_name), ce.enrolled_at, ce.completed_at,
			COUNT(l.id) FILTER (WHERE l.published) AS total_lessons,
			COUNT(lp.id) FILTER (WHERE l.published AND lp.completed) AS completed_lessons,
			EXISTS (
				SELECT 1 FROM test_attempts ta
				JOIN tests t ON t.id = ta.test_id
				WHERE t.course_id = c.id AND t.published = true AND t.is_final = true
				  AND ta.user_id = ce.user_id AND ta.passed = true
			) AS final_test_passed,
			COUNT(*) OVER() AS total
		FROM course_enrollments ce
		JOIN courses c ON c.id = ce.course_id
		JOIN users u ON u.id = ce.user_id
		`+lessonProgressJoins+`
		WHERE c.id = $1
		GROUP BY u.id, u.first_name, u.last_name, ce.user_id, ce.enrolled_at, ce.completed_at, c.id
		ORDER BY ce.enrolled_at DESC
		LIMIT $2 OFFSET $3
	`, courseID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := []CourseStudentRow{}
	total := 0
	for rows.Next() {
		var s CourseStudentRow
		var totalLessons, completedLessons int
		if err := rows.Scan(&s.UserID, &s.DisplayName, &s.EnrolledAt, &s.CompletedAt,
			&totalLessons, &completedLessons, &s.FinalTestPassed, &total); err != nil {
			return nil, 0, err
		}
		s.TotalLessons = totalLessons
		s.CompletedLessons = completedLessons
		s.ProgressPercent = calculatePercent(completedLessons, totalLessons)
		s.Completed = s.CompletedAt != nil
		result = append(result, s)
	}
	return result, total, rows.Err()
}

// Stats is the dashboard-level summary — one round trip, each column its
// own scalar subquery over the instructor's courses.
func (r *Repository) Stats(ctx context.Context, instructorID uuid.UUID) (Stats, error) {
	var s Stats
	err := r.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM courses WHERE instructor_id = $1),
			(SELECT COUNT(*) FROM courses WHERE instructor_id = $1 AND publication_status = 'published'),
			(SELECT COUNT(DISTINCT ce.user_id) FROM course_enrollments ce JOIN courses c ON c.id = ce.course_id WHERE c.instructor_id = $1),
			(SELECT COUNT(*) FROM course_enrollments ce JOIN courses c ON c.id = ce.course_id WHERE c.instructor_id = $1 AND ce.completed_at IS NULL),
			(SELECT COUNT(*) FROM course_enrollments ce JOIN courses c ON c.id = ce.course_id WHERE c.instructor_id = $1 AND ce.completed_at IS NOT NULL),
			(SELECT COALESCE(AVG(sub.pct), 0) FROM (
				SELECT
					CASE WHEN COUNT(l.id) FILTER (WHERE l.published) = 0 THEN 0
						ELSE 100.0 * COUNT(lp.id) FILTER (WHERE l.published AND lp.completed) / COUNT(l.id) FILTER (WHERE l.published)
					END AS pct
				FROM course_enrollments ce
				JOIN courses c ON c.id = ce.course_id
				`+lessonProgressJoins+`
				WHERE c.instructor_id = $1
				GROUP BY ce.user_id, ce.course_id
			) sub),
			(SELECT COUNT(*) FROM certificates cert JOIN courses c ON c.id = cert.course_id WHERE c.instructor_id = $1),
			(SELECT COUNT(*) FROM assignment_submissions s
				JOIN assignments a ON a.id = s.assignment_id
				JOIN lessons l ON l.id = a.lesson_id
				JOIN modules m ON m.id = l.module_id
				JOIN courses c ON c.id = m.course_id
				WHERE c.instructor_id = $1 AND s.status = 'submitted'),
			(SELECT COUNT(*) FROM assignment_submissions s
				JOIN assignments a ON a.id = s.assignment_id
				JOIN lessons l ON l.id = a.lesson_id
				JOIN modules m ON m.id = l.module_id
				JOIN courses c ON c.id = m.course_id
				WHERE c.instructor_id = $1 AND s.status = 'needs_revision')
	`, instructorID).Scan(&s.CoursesCount, &s.PublishedCourses, &s.StudentsCount, &s.ActiveEnrollments,
		&s.CompletedEnrollments, &s.AverageCompletionPct, &s.CertificatesIssued,
		&s.SubmissionsAwaitingReview, &s.SubmissionsNeedsRevision)
	return s, err
}

// CourseStats is the per-course analytics panel. courseID is trusted here —
// the handler already resolved ownership before calling this.
func (r *Repository) CourseStats(ctx context.Context, courseID uuid.UUID) (CourseStats, error) {
	var s CourseStats
	var enrollments, completedEnrollments int
	err := r.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM course_enrollments WHERE course_id = $1),
			(SELECT COUNT(*) FROM course_enrollments WHERE course_id = $1 AND completed_at IS NOT NULL),
			(SELECT COALESCE(AVG(sub.pct), 0) FROM (
				SELECT
					CASE WHEN COUNT(l.id) FILTER (WHERE l.published) = 0 THEN 0
						ELSE 100.0 * COUNT(lp.id) FILTER (WHERE l.published AND lp.completed) / COUNT(l.id) FILTER (WHERE l.published)
					END AS pct
				FROM course_enrollments ce
				JOIN courses c ON c.id = ce.course_id
				`+lessonProgressJoins+`
				WHERE c.id = $1
				GROUP BY ce.user_id, ce.course_id
			) sub),
			(SELECT
				CASE WHEN COUNT(DISTINCT ta.user_id) = 0 THEN 0
					ELSE 100.0 * COUNT(DISTINCT ta.user_id) FILTER (WHERE ta.passed) / COUNT(DISTINCT ta.user_id)
				END
			FROM test_attempts ta JOIN tests t ON t.id = ta.test_id
			WHERE t.course_id = $1 AND t.is_final = true),
			(SELECT COALESCE(AVG(rating), 0) FROM course_reviews WHERE course_id = $1 AND published = true),
			(SELECT COUNT(*) FROM course_reviews WHERE course_id = $1 AND published = true),
			(SELECT COUNT(*) FROM assignments a JOIN lessons l ON l.id = a.lesson_id JOIN modules m ON m.id = l.module_id WHERE m.course_id = $1),
			(SELECT COUNT(*) FROM assignment_submissions s
				JOIN assignments a ON a.id = s.assignment_id JOIN lessons l ON l.id = a.lesson_id JOIN modules m ON m.id = l.module_id
				WHERE m.course_id = $1 AND s.status != 'draft'),
			(SELECT COUNT(*) FROM assignment_submissions s
				JOIN assignments a ON a.id = s.assignment_id JOIN lessons l ON l.id = a.lesson_id JOIN modules m ON m.id = l.module_id
				WHERE m.course_id = $1 AND s.status = 'submitted'),
			(SELECT CASE WHEN COUNT(*) FILTER (WHERE s.status IN ('approved', 'needs_revision')) = 0 THEN 0
				ELSE 100.0 * COUNT(*) FILTER (WHERE s.status = 'approved') / COUNT(*) FILTER (WHERE s.status IN ('approved', 'needs_revision'))
			END
				FROM assignment_submissions s
				JOIN assignments a ON a.id = s.assignment_id JOIN lessons l ON l.id = a.lesson_id JOIN modules m ON m.id = l.module_id
				WHERE m.course_id = $1),
			(SELECT COALESCE(AVG(s.score), 0) FROM assignment_submissions s
				JOIN assignments a ON a.id = s.assignment_id JOIN lessons l ON l.id = a.lesson_id JOIN modules m ON m.id = l.module_id
				WHERE m.course_id = $1 AND s.score IS NOT NULL),
			(SELECT COUNT(*) FROM coding_exercises ce JOIN lessons l ON l.id = ce.lesson_id JOIN modules m ON m.id = l.module_id WHERE m.course_id = $1),
			(SELECT COUNT(*) FROM code_submissions cs
				JOIN coding_exercises ce ON ce.id = cs.exercise_id JOIN lessons l ON l.id = ce.lesson_id JOIN modules m ON m.id = l.module_id
				WHERE m.course_id = $1 AND cs.mode = 'submit'),
			(SELECT CASE WHEN COUNT(*) = 0 THEN 0 ELSE 100.0 * COUNT(*) FILTER (WHERE cs.status = 'passed') / COUNT(*) END
				FROM code_submissions cs
				JOIN coding_exercises ce ON ce.id = cs.exercise_id JOIN lessons l ON l.id = ce.lesson_id JOIN modules m ON m.id = l.module_id
				WHERE m.course_id = $1 AND cs.mode = 'submit'),
			-- Average, across every (user, exercise) pair that has ever
			-- passed, of how many submit-mode submissions it took (counting
			-- every attempt up to and including the first pass) — no N+1,
			-- one correlated subquery per pair via the self-join on
			-- MIN(created_at) of the first passing submission.
			(SELECT COALESCE(AVG(cnt), 0) FROM (
				SELECT COUNT(*) AS cnt
				FROM code_submissions cs
				JOIN coding_exercises ce ON ce.id = cs.exercise_id JOIN lessons l ON l.id = ce.lesson_id JOIN modules m ON m.id = l.module_id
				WHERE m.course_id = $1 AND cs.mode = 'submit'
				  AND cs.created_at <= (
					SELECT MIN(cs2.created_at) FROM code_submissions cs2
					WHERE cs2.user_id = cs.user_id AND cs2.exercise_id = cs.exercise_id
					  AND cs2.mode = 'submit' AND cs2.status = 'passed'
				  )
				GROUP BY cs.user_id, cs.exercise_id
			) attempts_to_pass),
			-- Stage 17 item 18: aggregate 7-day activity, course-scoped via
			-- the same lessons->modules join as every field above — never
			-- learning_activity, so no individual student's streak/activity
			-- calendar is reachable through this endpoint.
			(SELECT COUNT(DISTINCT user_id) FROM (
				SELECT lp.user_id FROM lesson_progress lp
					JOIN lessons l ON l.id = lp.lesson_id JOIN modules m ON m.id = l.module_id
					WHERE m.course_id = $1 AND lp.updated_at >= now() - interval '7 days'
				UNION
				SELECT s2.user_id FROM assignment_submissions s2
					JOIN assignments a2 ON a2.id = s2.assignment_id JOIN lessons l ON l.id = a2.lesson_id JOIN modules m ON m.id = l.module_id
					WHERE m.course_id = $1 AND s2.updated_at >= now() - interval '7 days'
				UNION
				SELECT cs3.user_id FROM code_submissions cs3
					JOIN coding_exercises ce3 ON ce3.id = cs3.exercise_id JOIN lessons l ON l.id = ce3.lesson_id JOIN modules m ON m.id = l.module_id
					WHERE m.course_id = $1 AND cs3.created_at >= now() - interval '7 days'
			) active_users),
			(SELECT COUNT(*) FROM lesson_progress lp
				JOIN lessons l ON l.id = lp.lesson_id JOIN modules m ON m.id = l.module_id
				WHERE m.course_id = $1 AND lp.completed = true AND lp.updated_at >= now() - interval '7 days'),
			(SELECT COUNT(*) FROM assignment_submissions s2
				JOIN assignments a2 ON a2.id = s2.assignment_id JOIN lessons l ON l.id = a2.lesson_id JOIN modules m ON m.id = l.module_id
				WHERE m.course_id = $1 AND s2.status != 'draft' AND s2.submitted_at >= now() - interval '7 days'),
			(SELECT COUNT(*) FROM code_submissions cs3
				JOIN coding_exercises ce3 ON ce3.id = cs3.exercise_id JOIN lessons l ON l.id = ce3.lesson_id JOIN modules m ON m.id = l.module_id
				WHERE m.course_id = $1 AND cs3.mode = 'submit' AND cs3.created_at >= now() - interval '7 days'),
			(SELECT CASE WHEN COUNT(*) = 0 THEN 0 ELSE 100.0 * COUNT(*) FILTER (WHERE cs3.status = 'passed') / COUNT(*) END
				FROM code_submissions cs3
				JOIN coding_exercises ce3 ON ce3.id = cs3.exercise_id JOIN lessons l ON l.id = ce3.lesson_id JOIN modules m ON m.id = l.module_id
				WHERE m.course_id = $1 AND cs3.mode = 'submit' AND cs3.created_at >= now() - interval '7 days')
	`, courseID).Scan(&enrollments, &completedEnrollments, &s.AverageLessonProgress, &s.FinalTestPassRatePct,
		&s.AverageRating, &s.ReviewCount, &s.AssignmentsCount, &s.SubmittedCount, &s.AwaitingReviewCount,
		&s.ApprovalRatePct, &s.AverageScore, &s.CodingExercisesCount, &s.CodeSubmissionsCount,
		&s.CodePassRatePct, &s.CodeAverageAttemptsBeforePass,
		&s.ActiveStudentsLast7Days, &s.LessonsCompletedLast7Days, &s.AssignmentSubmissionsLast7Days,
		&s.CodingSubmissionsLast7Days, &s.CodingPassRateLast7DaysPct)
	if err != nil {
		return CourseStats{}, err
	}
	s.Enrollments = enrollments
	s.CompletionRatePct = float64(calculatePercent(completedEnrollments, enrollments))
	return s, nil
}

// ListCourseReviews mirrors reviews.Repository.ListPublicByCourse (published
// only, display name never email) — duplicated rather than imported per
// this codebase's "share schema, not code" convention between domains.
func (r *Repository) ListCourseReviews(ctx context.Context, courseID uuid.UUID, limit, offset int) ([]CourseReview, int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT cr.id, TRIM(u.first_name || ' ' || u.last_name), cr.rating, cr.review_text, cr.created_at,
			COUNT(*) OVER() AS total
		FROM course_reviews cr
		JOIN users u ON u.id = cr.user_id
		WHERE cr.course_id = $1 AND cr.published = true
		ORDER BY cr.created_at DESC
		LIMIT $2 OFFSET $3
	`, courseID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := []CourseReview{}
	total := 0
	for rows.Next() {
		var cr CourseReview
		if err := rows.Scan(&cr.ID, &cr.DisplayName, &cr.Rating, &cr.ReviewText, &cr.CreatedAt, &total); err != nil {
			return nil, 0, err
		}
		result = append(result, cr)
	}
	return result, total, rows.Err()
}

// ListCourseTests resolves every test attached to courseID directly, or to
// one of its modules, or to one of its modules' lessons — same resolution
// tests.Repository.ListTestsAdmin uses, scoped to a single course.
func (r *Repository) ListCourseTests(ctx context.Context, courseID uuid.UUID) ([]TestSummary, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.id, t.course_id, t.module_id, t.lesson_id, t.title, t.passing_score, t.published, t.is_final
		FROM tests t
		LEFT JOIN modules m ON m.id = t.module_id
		LEFT JOIN lessons l ON l.id = t.lesson_id
		LEFT JOIN modules lm ON lm.id = l.module_id
		WHERE t.course_id = $1 OR m.course_id = $1 OR lm.course_id = $1
		ORDER BY t.created_at DESC
	`, courseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []TestSummary{}
	for rows.Next() {
		var ts TestSummary
		if err := rows.Scan(&ts.ID, &ts.CourseID, &ts.ModuleID, &ts.LessonID, &ts.Title, &ts.PassingScore, &ts.Published, &ts.IsFinal); err != nil {
			return nil, err
		}
		result = append(result, ts)
	}
	return result, rows.Err()
}
