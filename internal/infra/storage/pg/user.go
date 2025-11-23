package pg

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
	"github.com/mark47B/be-internship/internal/domain/entity"
	"github.com/mark47B/be-internship/internal/domain/repository"
	"github.com/mark47B/be-internship/internal/domain/usecase"
)

type UserStorage struct {
	db *sql.DB
}

func NewUserStorage(db *sql.DB) repository.UserRepository {
	return &UserStorage{db: db}
}

func (s *UserStorage) getQuerier(ctx context.Context) Querier {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok && tx != nil {
		return tx
	}
	return s.db
}

func (s *UserStorage) Get(ctx context.Context, id string) (entity.User, error) {
	q := s.getQuerier(ctx)

	var u entity.User
	var teamName sql.NullString

	err := q.QueryRowContext(ctx, `
		SELECT id, name, team_name, is_active
		FROM users
		WHERE id = $1
	`, id).Scan(&u.ID, &u.Username, &teamName, &u.IsActive)
	if err != nil {
		if err == sql.ErrNoRows {
			return entity.User{}, usecase.ErrUserNotFound
		}
		return entity.User{}, fmt.Errorf("get user: %w", err)
	}

	if teamName.Valid {
		u.TeamName = teamName.String
	}

	return u, nil
}

func (s *UserStorage) GetByTeam(ctx context.Context, teamName string) ([]entity.User, error) {
	q := s.getQuerier(ctx)

	rows, err := q.QueryContext(ctx, `
		SELECT id, name, team_name, is_active
		FROM users
		WHERE team_name = $1
		ORDER BY id
	`, teamName)
	if err != nil {
		return nil, fmt.Errorf("get users by team: %w", err)
	}
	defer CloseRows(rows)

	var users []entity.User
	for rows.Next() {
		var u entity.User
		var teamName sql.NullString

		if err := rows.Scan(&u.ID, &u.Username, &teamName, &u.IsActive); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}

		if teamName.Valid {
			u.TeamName = teamName.String
		}

		users = append(users, u)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func (s *UserStorage) GetActiveByTeam(ctx context.Context, teamName string, excludeUserID string) ([]entity.User, error) {
	q := s.getQuerier(ctx)

	rows, err := q.QueryContext(ctx, `
		SELECT id, name, team_name, is_active
		FROM users
		WHERE team_name = $1 AND is_active = true AND id != $2
		ORDER BY id
	`, teamName, excludeUserID)
	if err != nil {
		return nil, fmt.Errorf("get active users by team: %w", err)
	}
	defer CloseRows(rows)

	var users []entity.User
	for rows.Next() {
		var u entity.User
		var teamName sql.NullString

		if err := rows.Scan(&u.ID, &u.Username, &teamName, &u.IsActive); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}

		if teamName.Valid {
			u.TeamName = teamName.String
		}

		users = append(users, u)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func (s *UserStorage) UpdateMany(ctx context.Context, users []entity.User) error {
	if len(users) == 0 {
		return nil
	}

	q := s.getQuerier(ctx)

	ids := make([]string, 0, len(users))
	isActives := make([]bool, 0, len(users))

	for _, u := range users {
		ids = append(ids, u.ID)
		isActives = append(isActives, u.IsActive)
	}

	query := `
		UPDATE users
		SET is_active = data.is_active
		FROM (
			SELECT unnest($1::text[]) as id, unnest($2::boolean[]) as is_active
		) as data
		WHERE users.id = data.id
	`

	_, err := q.ExecContext(ctx, query, pq.Array(ids), pq.Array(isActives))
	if err != nil {
		return fmt.Errorf("update many users: %w", err)
	}
	return nil
}

func (s *UserStorage) GetUserStats(ctx context.Context, userID string) (entity.UserStats, error) {
	q := s.getQuerier(ctx)

	var stats entity.UserStats
	stats.UserID = userID

	err := q.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE author_id = $1) as created_pr_count,
			COUNT(*) FILTER (WHERE EXISTS (
				SELECT 1 FROM review_assignments ra
				WHERE ra.pr_id = pr.id AND ra.reviewer_id = $1
			)) as reviewed_pr_count,
			COUNT(*) FILTER (WHERE author_id = $1 AND status = 'MERGED') as merged_pr_count
		FROM pull_requests pr
	`, userID).Scan(&stats.CreatedPRCount, &stats.ReviewedPRCount, &stats.MergedPRCount)
	if err != nil {
		return entity.UserStats{}, fmt.Errorf("get user stats: %w", err)
	}

	return stats, nil
}

func (s *UserStorage) SaveUpdateMany(ctx context.Context, users []entity.User) error {
	if len(users) == 0 {
		return nil
	}

	q := s.getQuerier(ctx)

	ids := make([]string, 0, len(users))
	usernames := make([]string, 0, len(users))
	teamNames := make([]string, 0, len(users))
	isActives := make([]bool, 0, len(users))

	for _, u := range users {
		ids = append(ids, u.ID)
		usernames = append(usernames, u.Username)
		teamNames = append(teamNames, u.TeamName)
		isActives = append(isActives, u.IsActive)
	}

	query := `
        INSERT INTO users (id, name, team_name, is_active)
        SELECT
            unnest($1::text[]),
            unnest($2::text[]),
            unnest($3::text[]),
            unnest($4::boolean[])
        ON CONFLICT (id) DO UPDATE SET
            name      = EXCLUDED.name,
            team_name = EXCLUDED.team_name,
            is_active = EXCLUDED.is_active
    `

	_, err := q.ExecContext(ctx, query,
		pq.Array(ids),
		pq.Array(usernames),
		pq.Array(teamNames),
		pq.Array(isActives),
	)
	if err != nil {
		return fmt.Errorf("bulk save/update users: %w", err)
	}
	return nil
}

func (s *UserStorage) DeactivateMany(ctx context.Context, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}

	q := s.getQuerier(ctx)

	query := `
        UPDATE users
        SET is_active = false
        WHERE id = ANY($1)
          AND is_active = true
    `

	_, err := q.ExecContext(ctx, query, pq.Array(userIDs))
	if err != nil {
		return fmt.Errorf("deactivate many users: %w", err)
	}

	return nil
}
