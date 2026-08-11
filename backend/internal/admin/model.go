package admin

type Stats struct {
	UsersCount            int `json:"users_count"`
	CoursesCount          int `json:"courses_count"`
	PublishedCoursesCount int `json:"published_courses_count"`
	SpecialitiesCount     int `json:"specialities_count"`
	EnrollmentsCount      int `json:"enrollments_count"`
	CompletedCoursesCount int `json:"completed_courses_count"`
	CertificatesCount     int `json:"certificates_count"`
	TestAttemptsCount     int `json:"test_attempts_count"`

	// Stage 17 item 19-21 additions. DailyActiveLearners/WeeklyActiveLearners
	// count distinct users with at least one *meaningful* learning_activity
	// row (item 20: never counts a plain login) — daily is the current UTC
	// calendar day, weekly is a rolling 7-day window (both plain PostgreSQL
	// aggregates, no ClickHouse).
	DailyActiveLearners       int `json:"daily_active_learners"`
	WeeklyActiveLearners      int `json:"weekly_active_learners"`
	LessonsCompletedToday     int `json:"lessons_completed_today"`
	CoursesCompletedThisMonth int `json:"courses_completed_this_month"`
	CertificatesThisMonth     int `json:"certificates_this_month"`
}
