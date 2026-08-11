package health

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

type Status struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

func (s *Service) Check(ctx context.Context) Status {
	if err := s.pool.Ping(ctx); err != nil {
		return Status{Status: "degraded", Database: "unreachable"}
	}
	return Status{Status: "ok", Database: "ok"}
}
