package admin

import "context"

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetStats(ctx context.Context) (*Stats, error) {
	return s.repo.GetStats(ctx)
}
