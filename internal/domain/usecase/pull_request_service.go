package usecase

import (
	"context"

	"github.com/mark47B/be-internship/internal/infra/transport/rest/gen"
)

type PullRequestService interface {
	CreateTeam(ctx context.Context, t gen.Team) (gen.Team, error)
	GetTeam(ctx context.Context, teamName string) (gen.Team, error)
	ReassignReviewer(ctx context.Context, prID, oldReviewerID string) (string, *gen.PullRequest, error)
	GetPullRequestsByReviewer(ctx context.Context, reviewerID string) ([]gen.PullRequestShort, error)
}
