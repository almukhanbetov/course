package achievements

import "lms-backend/internal/activity"

// Rule is the one place a code (matching an `achievements.code` row) is
// paired with the condition that earns it. Adding a ninth achievement later
// means appending one line here — never touching internal/learning,
// internal/assignments, internal/coding, internal/tests, or
// internal/certificates.
type Rule struct {
	Code  string
	Check func(counts activity.Counts, currentStreak int) bool
}

// Rules is Stage 17 item 11's minimum set. Streak rules check the
// *current* streak at the moment of evaluation (which runs right after
// every meaningful event, i.e. exactly when a streak could have just
// advanced) — once earned, the unique(user_id, achievement_id) constraint
// makes it permanent even if the streak later resets.
var Rules = []Rule{
	{Code: "FIRST_LESSON", Check: func(c activity.Counts, _ int) bool { return c.LessonsCompleted >= 1 }},
	{Code: "FIRST_COURSE", Check: func(c activity.Counts, _ int) bool { return c.CoursesCompleted >= 1 }},
	{Code: "FIRST_CERTIFICATE", Check: func(c activity.Counts, _ int) bool { return c.Certificates >= 1 }},
	{Code: "CODE_BEGINNER", Check: func(c activity.Counts, _ int) bool { return c.CodingExercisesPassed >= 1 }},
	{Code: "CODE_10", Check: func(c activity.Counts, _ int) bool { return c.CodingExercisesPassed >= 10 }},
	{Code: "STREAK_3", Check: func(_ activity.Counts, streak int) bool { return streak >= 3 }},
	{Code: "STREAK_7", Check: func(_ activity.Counts, streak int) bool { return streak >= 7 }},
	{Code: "STREAK_30", Check: func(_ activity.Counts, streak int) bool { return streak >= 30 }},
}
