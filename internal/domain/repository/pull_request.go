package repository

type PullRequestRepository interface {
	// Save(ctx context.Context, pr entity.PullRequest) error
	// Get(ctx context.Context, id string) (entity.PullRequest, error)
	// GetByReviewer(ctx context.Context, reviewerID string) ([]entity.PullRequest, error)
	// GetByTeam(ctx context.Context, team string) ([]entity.PullRequest, error)
	// Update(ctx context.Context, pr entity.PullRequest) error

	// GetReviewers(ctx context.Context, prID string) ([]string, error)
	// AssignReviewers(ctx context.Context, prID string, reviewerIDs []string) error
	// ReplaceReviewer(ctx context.Context, prID, oldReviewerID, newReviewerID string) error

	// GetAll(ctx context.Context) ([]entity.PullRequest, error)
}
