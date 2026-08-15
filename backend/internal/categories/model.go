package categories

import (
	"time"

	"github.com/google/uuid"
)

type Category struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description *string   `json:"description,omitempty"`
	Position    int       `json:"position"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Name is always Russian; these are optional per-locale overrides —
	// same convention as courses.Course.TitleKk/TitleEn.
	NameKk *string `json:"name_kk,omitempty"`
	NameEn *string `json:"name_en,omitempty"`
}

type CategoryInput struct {
	Name        string
	Slug        string
	Description string
	Position    int
	Active      bool
}
