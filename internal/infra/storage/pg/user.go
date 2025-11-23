package pg

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
	"github.com/mark47B/be-internship/internal/domain/entity"
	"github.com/mark47B/be-internship/internal/domain/repository"
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

func (s *UserStorage) Save(ctx context.Context, user entity.User) error {
	return nil
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
