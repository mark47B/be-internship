package pg

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/mark47B/be-internship/internal/domain/entity"
	"github.com/mark47B/be-internship/internal/domain/repository"
	"github.com/mark47B/be-internship/internal/domain/usecase"
)

type TeamStorage struct {
	db *sql.DB
}

func NewTeamStorage(db *sql.DB) repository.TeamRepository {
	return &TeamStorage{db: db}
}

func (s *TeamStorage) getQuerier(ctx context.Context) Querier {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok && tx != nil {
		return tx
	}
	return s.db
}

func (s *TeamStorage) Get(ctx context.Context, name string) (entity.Team, error) {
	q := s.getQuerier(ctx)

	// Проверяем существование команды
	var exists bool
	err := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM teams WHERE name = $1)`, name).Scan(&exists)
	if err != nil {
		return entity.Team{}, fmt.Errorf("check team exists: %w", err)
	}
	if !exists {
		return entity.Team{}, usecase.ErrTeamNotFound
	}

	rows, err := q.QueryContext(ctx, `
        SELECT id, name, is_active, team_name
        FROM users
        WHERE team_name = $1
        ORDER BY id
    `, name)
	if err != nil {
		return entity.Team{}, fmt.Errorf("query members: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("ERROR closing rows: %v", closeErr)
		}
	}()

	var members []entity.User
	for rows.Next() {
		var u entity.User
		var teamName sql.NullString
		if err := rows.Scan(&u.ID, &u.Username, &u.IsActive, &teamName); err != nil {
			return entity.Team{}, fmt.Errorf("scan user: %w", err)
		}
		if teamName.Valid {
			u.TeamName = teamName.String
		}
		members = append(members, u)
	}
	if err := rows.Err(); err != nil {
		return entity.Team{}, err
	}

	return entity.Team{
		Name:    name,
		Members: members,
	}, nil
}

func (s *TeamStorage) Save(ctx context.Context, team entity.Team) error {
	q := s.getQuerier(ctx)

	_, err := q.ExecContext(ctx, `
        INSERT INTO teams (name) VALUES ($1)
        ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
    `, team.Name)
	if err != nil {
		return fmt.Errorf("upsert team: %w", err)
	}
	return nil
}
