package usecase

import (
	"context"

	"github.com/mark47B/be-internship/internal/domain/entity"
)

type TeamUseCase interface {
	// Создать/обновить команду и её участников
	AddOrUpdateTeam(ctx context.Context, team entity.Team) (entity.Team, error)

	// Получить команду по имени
	GetTeam(ctx context.Context, teamName string) (entity.Team, error)

	// Массовая деактивация пользователей + безопасное переназначение PR
	DeactivateUsersAndReassign(ctx context.Context, teamName string, userIDs []string) error
}

// Управление пользователями
type UserUseCase interface {
	// Установить активность пользователя
	SetUserActive(ctx context.Context, userID string, active bool) (entity.User, error)

	// Получить PR где пользователь — ревьювер
	GetUserReviewPRs(ctx context.Context, userID string) ([]entity.PullRequest, error)

	// Статистика по пользователю
	GetUserStats(ctx context.Context, userID string) (entity.UserStats, error)
}

// Управление PR
type PullRequestUseCase interface {
	// Создать PR + автоприсвоение ревьюверов
	CreatePR(ctx context.Context, id, name, authorID string) (entity.PullRequest, error)

	// Идемпотентный merge
	MergePR(ctx context.Context, id string) (entity.PullRequest, error)

	// Переназначить ревьювера
	ReassignReviewer(ctx context.Context, prID, oldReviewerID string) (updated entity.PullRequest, replacedBy string, err error)

	// Получить aggregated stats
	GetPRStats(ctx context.Context) (entity.PRStats, error)
}

// Фасад для агрегации интерфейсов сервиса
type Service interface {
	TeamUseCase
	UserUseCase
	PullRequestUseCase
}
