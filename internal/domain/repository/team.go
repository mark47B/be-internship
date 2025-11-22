package repository

import (
	"context"

	"github.com/mark47B/be-internship/internal/domain/entity"
)

type TeamRepository interface {
	// Save(ctx context.Context, team entity.Team) error
	Get(ctx context.Context, name string) (entity.Team, error)
}
