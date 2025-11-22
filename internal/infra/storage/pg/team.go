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

func (s *TeamStorage) Get(ctx context.Context, name string) (entity.Team, error) {
	// Check if team exists
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM teams WHERE name = $1)
	`, name).Scan(&exists)
	if err != nil {
		return entity.Team{}, fmt.Errorf("check team exists: %w", err)
	}
	if !exists {
		return entity.Team{}, usecase.ErrTeamNotFound
	}

	// Get all members
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, is_active, team_name
		FROM users
		WHERE team_name = $1
		ORDER BY id
	`, name)
	if err != nil {
		return entity.Team{}, fmt.Errorf("query team members: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	members := make([]entity.User, 0)
	for rows.Next() {
		var u entity.User
		var teamName sql.NullString
		if err := rows.Scan(&u.ID, &u.Username, &u.IsActive, &teamName); err != nil {
			return entity.Team{}, fmt.Errorf("scan member: %w", err)
		}
		if teamName.Valid {
			u.TeamName = teamName.String
		} else {
			u.TeamName = ""
		}
		members = append(members, u)
	}
	if err := rows.Err(); err != nil {
		return entity.Team{}, fmt.Errorf("iterate members: %w", err)
	}

	return entity.Team{
		Name:    name,
		Members: members,
	}, nil
}
