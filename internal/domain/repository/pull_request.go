package repository

import (
	"context"

	"github.com/mark47B/be-internship/internal/domain/entity"
)

type PullRequestRepository interface {
	Save(ctx context.Context, pr entity.PullRequest) error
	Get(ctx context.Context, id string) (entity.PullRequest, error)
	GetByReviewer(ctx context.Context, reviewerID string) ([]entity.PullRequest, error)
	Update(ctx context.Context, pr entity.PullRequest) error
	GetReviewers(ctx context.Context, prID string) ([]string, error)
	AssignReviewers(ctx context.Context, prID string, reviewerIDs []string) error
	ReplaceReviewer(ctx context.Context, prID, oldReviewerID, newReviewerID string) error
	RemoveReviewer(ctx context.Context, prID, reviewerID string) error
	GetStats(ctx context.Context) (entity.PRStats, error)
	GetOpenPRsByReviewers(ctx context.Context, reviewerIDs []string) ([]entity.PullRequest, error)
	GetReviewersBatch(ctx context.Context, prIDs []string) (map[string][]string, error)
	GetOpenPRsByTeam(ctx context.Context, teamName string) ([]entity.PullRequest, error)
}
