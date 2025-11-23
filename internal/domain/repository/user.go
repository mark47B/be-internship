package repository

import (
	"context"

	"github.com/mark47B/be-internship/internal/domain/entity"
)

type UserRepository interface {
	SaveUpdateMany(ctx context.Context, user []entity.User) error
	Get(ctx context.Context, id string) (entity.User, error)
	GetByTeam(ctx context.Context, teamName string) ([]entity.User, error)
	GetActiveByTeam(ctx context.Context, teamName string, excludeUserID string) ([]entity.User, error)
	UpdateMany(ctx context.Context, users []entity.User) error
	GetUserStats(ctx context.Context, userID string) (entity.UserStats, error)
	DeactivateMany(ctx context.Context, userIDs []string) error
}
