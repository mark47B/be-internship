package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/lib/pq"
	"github.com/mark47B/be-internship/internal/domain/entity"
	"github.com/mark47B/be-internship/internal/domain/repository"
	"github.com/mark47B/be-internship/internal/domain/usecase"
)

type PullRequestStorage struct {
	db *sql.DB
}

func NewPullRequestStorage(db *sql.DB) repository.PullRequestRepository {
	return &PullRequestStorage{db: db}
}

func (s *PullRequestStorage) getQuerier(ctx context.Context) Querier {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok && tx != nil {
		return tx
	}
	return s.db
}

func (s *PullRequestStorage) Save(ctx context.Context, pr entity.PullRequest) error {
	q := s.getQuerier(ctx)

	var createdAt time.Time
	if pr.CreatedAt != nil {
		createdAt = *pr.CreatedAt
	} else {
		createdAt = time.Now()
	}

	var mergedAt sql.NullTime
	if pr.MergedAt != nil {
		mergedAt = sql.NullTime{Time: *pr.MergedAt, Valid: true}
	}

	_, err := q.ExecContext(ctx, `
		INSERT INTO pull_requests (id, name, author_id, status, created_at, merged_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			author_id = EXCLUDED.author_id,
			status = EXCLUDED.status,
			merged_at = EXCLUDED.merged_at
	`, pr.ID, pr.Name, pr.AuthorID, string(pr.Status), createdAt, mergedAt)
	if err != nil {
		return fmt.Errorf("save pull request: %w", err)
	}
	return nil
}

func (s *PullRequestStorage) Get(ctx context.Context, id string) (entity.PullRequest, error) {
	q := s.getQuerier(ctx)

	var pr entity.PullRequest
	var createdAt time.Time
	var mergedAt sql.NullTime
	var statusStr string

	err := q.QueryRowContext(ctx, `
		SELECT id, name, author_id, status, created_at, merged_at
		FROM pull_requests
		WHERE id = $1
	`, id).Scan(&pr.ID, &pr.Name, &pr.AuthorID, &statusStr, &createdAt, &mergedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.PullRequest{}, usecase.ErrPRNotFound
		}
		return entity.PullRequest{}, fmt.Errorf("get pull request: %w", err)
	}

	pr.Status = entity.PRStatus(statusStr)
	pr.CreatedAt = &createdAt
	if mergedAt.Valid {
		pr.MergedAt = &mergedAt.Time
	}

	// Загружаем reviewers
	reviewers, err := s.GetReviewers(ctx, id)
	if err != nil {
		return entity.PullRequest{}, fmt.Errorf("get reviewers: %w", err)
	}
	pr.Reviewers = reviewers

	return pr, nil
}

func (s *PullRequestStorage) GetByReviewer(ctx context.Context, reviewerID string) ([]entity.PullRequest, error) {
	q := s.getQuerier(ctx)

	rows, err := q.QueryContext(ctx, `
		SELECT pr.id, pr.name, pr.author_id, pr.status, pr.created_at, pr.merged_at
		FROM pull_requests pr
		INNER JOIN review_assignments ra ON pr.id = ra.pr_id
		WHERE ra.reviewer_id = $1
		ORDER BY pr.created_at DESC
	`, reviewerID)
	if err != nil {
		return nil, fmt.Errorf("get PRs by reviewer: %w", err)
	}
	defer CloseRows(rows)

	var prs []entity.PullRequest
	for rows.Next() {
		var pr entity.PullRequest
		var createdAt time.Time
		var mergedAt sql.NullTime
		var statusStr string

		if err := rows.Scan(&pr.ID, &pr.Name, &pr.AuthorID, &statusStr, &createdAt, &mergedAt); err != nil {
			return nil, fmt.Errorf("scan PR: %w", err)
		}

		pr.Status = entity.PRStatus(statusStr)
		pr.CreatedAt = &createdAt
		if mergedAt.Valid {
			pr.MergedAt = &mergedAt.Time
		}

		// Загружаем reviewers для каждого PR
		reviewers, err := s.GetReviewers(ctx, pr.ID)
		if err != nil {
			return nil, fmt.Errorf("get reviewers for PR %s: %w", pr.ID, err)
		}
		pr.Reviewers = reviewers

		prs = append(prs, pr)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return prs, nil
}

func (s *PullRequestStorage) Update(ctx context.Context, pr entity.PullRequest) error {
	q := s.getQuerier(ctx)

	var mergedAt sql.NullTime
	if pr.MergedAt != nil {
		mergedAt = sql.NullTime{Time: *pr.MergedAt, Valid: true}
	}

	_, err := q.ExecContext(ctx, `
		UPDATE pull_requests
		SET name = $2, author_id = $3, status = $4, merged_at = $5
		WHERE id = $1
	`, pr.ID, pr.Name, pr.AuthorID, string(pr.Status), mergedAt)
	if err != nil {
		return fmt.Errorf("update pull request: %w", err)
	}
	return nil
}

func (s *PullRequestStorage) GetReviewers(ctx context.Context, prID string) ([]string, error) {
	q := s.getQuerier(ctx)

	rows, err := q.QueryContext(ctx, `
		SELECT reviewer_id
		FROM review_assignments
		WHERE pr_id = $1
		ORDER BY reviewer_id
	`, prID)
	if err != nil {
		return nil, fmt.Errorf("get reviewers: %w", err)
	}
	defer CloseRows(rows)

	var reviewers []string
	for rows.Next() {
		var reviewerID string
		if err := rows.Scan(&reviewerID); err != nil {
			return nil, fmt.Errorf("scan reviewer: %w", err)
		}
		reviewers = append(reviewers, reviewerID)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return reviewers, nil
}

func (s *PullRequestStorage) AssignReviewers(ctx context.Context, prID string, reviewerIDs []string) error {
	if len(reviewerIDs) == 0 {
		return nil
	}

	q := s.getQuerier(ctx)

	query := `
		INSERT INTO review_assignments (pr_id, reviewer_id)
		SELECT $1, unnest($2::text[])
		ON CONFLICT (pr_id, reviewer_id) DO NOTHING
	`

	_, err := q.ExecContext(ctx, query, prID, pq.Array(reviewerIDs))
	if err != nil {
		return fmt.Errorf("assign reviewers: %w", err)
	}
	return nil
}

func (s *PullRequestStorage) ReplaceReviewer(ctx context.Context, prID, oldReviewerID, newReviewerID string) error {
	q := s.getQuerier(ctx)

	// Используем DELETE + INSERT вместо UPDATE, чтобы избежать конфликтов с unique constraint
	// если новый ревьювер уже назначен
	_, err := q.ExecContext(ctx, `
		DELETE FROM review_assignments
		WHERE pr_id = $1 AND reviewer_id = $2
	`, prID, oldReviewerID)
	if err != nil {
		return fmt.Errorf("remove old reviewer: %w", err)
	}

	// Добавляем нового ревьювера (если его еще нет)
	_, err = q.ExecContext(ctx, `
		INSERT INTO review_assignments (pr_id, reviewer_id)
		VALUES ($1, $2)
		ON CONFLICT (pr_id, reviewer_id) DO NOTHING
	`, prID, newReviewerID)
	if err != nil {
		return fmt.Errorf("add new reviewer: %w", err)
	}
	return nil
}

func (s *PullRequestStorage) RemoveReviewer(ctx context.Context, prID, reviewerID string) error {
	q := s.getQuerier(ctx)

	_, err := q.ExecContext(ctx, `
		DELETE FROM review_assignments
		WHERE pr_id = $1 AND reviewer_id = $2
	`, prID, reviewerID)
	if err != nil {
		return fmt.Errorf("remove reviewer: %w", err)
	}
	return nil
}

func (s *PullRequestStorage) GetStats(ctx context.Context) (entity.PRStats, error) {
	q := s.getQuerier(ctx)

	var stats entity.PRStats
	var avgReviewers sql.NullFloat64

	err := q.QueryRowContext(ctx, `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE status = 'OPEN') as open,
			COUNT(*) FILTER (WHERE status = 'MERGED') as merged,
			COALESCE(AVG(reviewer_count), 0) as avg_reviewers
		FROM pull_requests pr
		LEFT JOIN (
			SELECT pr_id, COUNT(*) as reviewer_count
			FROM review_assignments
			GROUP BY pr_id
		) ra ON pr.id = ra.pr_id
	`).Scan(&stats.Total, &stats.Open, &stats.Merged, &avgReviewers)
	if err != nil {
		return entity.PRStats{}, fmt.Errorf("get PR stats: %w", err)
	}

	if avgReviewers.Valid {
		stats.AvgReviewers = avgReviewers.Float64
	}

	return stats, nil
}

func (s *PullRequestStorage) GetOpenPRsByReviewers(ctx context.Context, reviewerIDs []string) ([]entity.PullRequest, error) {
	log.Println("[DEBUG] GetOpenPRsByReviewers: start, reviewerIDs:", reviewerIDs)
	if len(reviewerIDs) == 0 {
		return []entity.PullRequest{}, nil
	}

	q := s.getQuerier(ctx)
	log.Println("[DEBUG] GetOpenPRsByReviewers: got querier")

	rows, err := q.QueryContext(ctx, `
		SELECT DISTINCT pr.id, pr.name, pr.author_id, pr.status, pr.created_at, pr.merged_at
		FROM pull_requests pr
		INNER JOIN review_assignments ra ON pr.id = ra.pr_id
		WHERE ra.reviewer_id = ANY($1::text[]) AND pr.status = 'OPEN'
		ORDER BY pr.created_at DESC
	`, pq.Array(reviewerIDs))
	if err != nil {
		log.Printf("[DEBUG] GetOpenPRsByReviewers: query error: %v", err)
		return nil, fmt.Errorf("get open PRs by reviewers: %w", err)
	}
	defer CloseRows(rows)

	var prs []entity.PullRequest
	for rows.Next() {
		var pr entity.PullRequest
		var createdAt time.Time
		var mergedAt sql.NullTime
		var statusStr string

		if err := rows.Scan(&pr.ID, &pr.Name, &pr.AuthorID, &statusStr, &createdAt, &mergedAt); err != nil {
			log.Printf("[DEBUG] GetOpenPRsByReviewers: scan error: %v", err)
			return nil, fmt.Errorf("scan PR: %w", err)
		}

		pr.Status = entity.PRStatus(statusStr)
		pr.CreatedAt = &createdAt
		if mergedAt.Valid {
			pr.MergedAt = &mergedAt.Time
		}

		// НЕ загружаем reviewers здесь, так как это может быть внутри транзакции
		// Reviewers будут загружены позже при необходимости через GetReviewers в service.go
		pr.Reviewers = []string{}
		log.Printf("[DEBUG] GetOpenPRsByReviewers: added PR %s without reviewers", pr.ID)

		prs = append(prs, pr)
	}

	if err := rows.Err(); err != nil {
		log.Printf("[DEBUG] GetOpenPRsByReviewers: rows error: %v", err)
		return nil, err
	}

	log.Printf("[DEBUG] GetOpenPRsByReviewers: returning %d PRs", len(prs))
	return prs, nil
}

func (s *PullRequestStorage) GetOpenPRsByTeam(ctx context.Context, teamName string) ([]entity.PullRequest, error) {
	q := s.getQuerier(ctx)

	rows, err := q.QueryContext(ctx, `
        SELECT
            pr.id,
            pr.name,
            pr.author_id,
            pr.status,
            pr.created_at,
            pr.merged_at
        FROM pull_requests pr
        INNER JOIN users u ON pr.author_id = u.id
        WHERE u.team_name = $1
          AND u.is_active = true      -- только активные авторы (опционально, но логично)
          AND pr.status = 'OPEN'
        ORDER BY pr.created_at DESC
    `, teamName)
	if err != nil {
		return nil, fmt.Errorf("get open PRs by team: query: %w", err)
	}
	defer CloseRows(rows)

	var prs []entity.PullRequest
	for rows.Next() {
		var pr entity.PullRequest
		var createdAt time.Time
		var mergedAt sql.NullTime
		var statusStr string

		if err := rows.Scan(
			&pr.ID,
			&pr.Name,
			&pr.AuthorID,
			&statusStr,
			&createdAt,
			&mergedAt,
		); err != nil {
			return nil, fmt.Errorf("get open PRs by team: scan: %w", err)
		}

		pr.Status = entity.PRStatus(statusStr)
		pr.CreatedAt = &createdAt
		if mergedAt.Valid {
			pr.MergedAt = &mergedAt.Time
		}

		// НЕ загружаем ревьюверов здесь!
		pr.Reviewers = []string{}

		prs = append(prs, pr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get open PRs by team: rows error: %w", err)
	}

	return prs, nil
}

func (s *PullRequestStorage) GetReviewersBatch(ctx context.Context, prIDs []string) (map[string][]string, error) {
	if len(prIDs) == 0 {
		return map[string][]string{}, nil
	}

	q := s.getQuerier(ctx)

	// Один запрос: все назначения ревьюверов для указанных PR
	rows, err := q.QueryContext(ctx, `
        SELECT pr_id, reviewer_id
        FROM review_assignments
        WHERE pr_id = ANY($1::text[])
        ORDER BY pr_id, reviewer_id
    `, pq.Array(prIDs))
	if err != nil {
		return nil, fmt.Errorf("get reviewers batch: query: %w", err)
	}
	defer CloseRows(rows)

	result := make(map[string][]string, len(prIDs))

	// Инициализируем пустые слайсы для всех PR
	for _, prID := range prIDs {
		result[prID] = []string{}
	}

	for rows.Next() {
		var prID, reviewerID string
		if err := rows.Scan(&prID, &reviewerID); err != nil {
			return nil, fmt.Errorf("get reviewers batch: scan: %w", err)
		}
		result[prID] = append(result[prID], reviewerID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get reviewers batch: rows error: %w", err)
	}

	return result, nil
}
