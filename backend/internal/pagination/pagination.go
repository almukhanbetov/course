// Package pagination provides a shared page/limit envelope and query-param
// parsing so every admin list endpoint (users, courses, certificates, ...)
// answers with the same shape instead of each domain rolling its own.
package pagination

import "strconv"

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

type Result[T any] struct {
	Items      []T `json:"items"`
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

func New[T any](items []T, page, limit, total int) Result[T] {
	totalPages := 0
	if limit > 0 {
		totalPages = (total + limit - 1) / limit
	}
	return Result[T]{Items: items, Page: page, Limit: limit, Total: total, TotalPages: totalPages}
}

// ParseParams parses raw "page"/"limit" query values, defaulting and
// clamping them to sane bounds. It never errors — invalid input just falls
// back to the defaults.
func ParseParams(pageStr, limitStr string) (page, limit int) {
	page = 1
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}

	limit = DefaultLimit
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}

	return page, limit
}

func Offset(page, limit int) int {
	return (page - 1) * limit
}
