package app

import (
	"context"
	"database/sql"
	"errors"

	"github.com/mark47B/be-internship/internal/domain/entity"
	"github.com/mark47B/be-internship/internal/domain/repository"
	"github.com/mark47B/be-internship/internal/domain/usecase"
)

// compile-time proof
var _ usecase.Service = (*ServiceImpl)(nil)

type ServiceImpl struct {
	teams repository.TeamRepository
	users repository.UserRepository
	prs   repository.PullRequestRepository
}

func NewService(
	teams repository.TeamRepository,
	users repository.UserRepository,
	prs repository.PullRequestRepository,
) usecase.Service {
	return &ServiceImpl{
		teams: teams,
		users: users,
		prs:   prs,
	}
}

func (s *ServiceImpl) GetTeam(ctx context.Context, name string) (entity.Team, error) {
	team, err := s.teams.Get(ctx, name)
	if err != nil {
		if err == sql.ErrNoRows || errors.Is(err, usecase.ErrTeamNotFound) {
			return entity.Team{}, usecase.ErrTeamNotFound
		}
		return entity.Team{}, err
	}
	return team, nil
}
