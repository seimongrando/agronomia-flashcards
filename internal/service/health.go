package service

import (
	"context"
	"database/sql"
)

type HealthService struct {
	db *sql.DB
}

func NewHealthService(db *sql.DB) *HealthService {
	return &HealthService{db: db}
}

func (s *HealthService) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}
