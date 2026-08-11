// Package achievements evaluates and awards achievements after a
// meaningful learning event. Evaluate is table-driven (see rules.go) —
// Stage 17 item 12 explicitly asks for this instead of "a giant switch
// scattered across every domain": the switch, such as it is, lives here
// and only here, as a slice of rules, not inside internal/learning,
// internal/assignments, etc.
package achievements

import (
	"time"

	"github.com/google/uuid"
)

type Achievement struct {
	ID          uuid.UUID `json:"id"`
	Code        string    `json:"code"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Icon        string    `json:"icon"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
}

type UserAchievement struct {
	AchievementID uuid.UUID `json:"achievement_id"`
	EarnedAt      time.Time `json:"earned_at"`
}

// EarnedAchievement is one row of GET /me/achievements' "earned" list.
type EarnedAchievement struct {
	Code        string    `json:"code"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Icon        string    `json:"icon"`
	EarnedAt    time.Time `json:"earned_at"`
}

// LockedAchievement never exposes the rule's internal threshold logic
// beyond the same human-readable description already shown for earned
// achievements (item 16: "не раскрывать чувствительную/internal logic") —
// there is structurally no field here for e.g. the exact SQL or current
// progress count.
type LockedAchievement struct {
	Code        string `json:"code"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

type AchievementsView struct {
	Earned []EarnedAchievement `json:"earned"`
	Locked []LockedAchievement `json:"locked"`
}
