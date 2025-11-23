package repository

import (
	"context"

	"github.com/mark47B/be-internship/internal/domain/entity"
)

type UserRepository interface {
	// Save(ctx context.Context, user entity.User) error
	SaveUpdateMany(ctx context.Context, user []entity.User) error
	// Get(ctx context.Context, id string) (entity.User, error)
	// GetByTeam(ctx context.Context, teamName string) ([]entity.User, error)
	// GetReviewPRs(ctx context.Context, userID string) ([]entity.PullRequest, error)
	// UpdateMany(ctx context.Context, users []entity.User) error
}
