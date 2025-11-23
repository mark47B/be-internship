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
	teams     repository.TeamRepository
	users     repository.UserRepository
	prs       repository.PullRequestRepository
	txManager repository.TxManager
}

func NewService(
	teams repository.TeamRepository,
	users repository.UserRepository,
	prs repository.PullRequestRepository,
	txManager repository.TxManager,
) usecase.Service {
	return &ServiceImpl{
		teams:     teams,
		users:     users,
		prs:       prs,
		txManager: txManager,
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

func (s *ServiceImpl) AddOrUpdateTeam(ctx context.Context, team entity.Team) (entity.Team, error) {

	if team.Name == "" {
		return entity.Team{}, errors.New("team name is required")
	}

	existing, err := s.teams.Get(ctx, team.Name)
	if err == nil {
		// Команда найдена
		return existing, usecase.ErrTeamExists
	}
	if err != usecase.ErrTeamNotFound {
		// Неизвестная ошибка
		return entity.Team{}, err
	}

	for i := range team.Members {
		m := &team.Members[i]
		m.TeamName = team.Name // гарантируем привязку
	}
	// Команды нет транзакционно создаём и обновляем пользователей
	createdTeam, err := s.txManager.DoTx(ctx, func(txCtx context.Context) (any, error) {
		// Создаём команду
		if err := s.teams.Save(txCtx, team); err != nil {
			return nil, err
		}
		// создаём или обновляем
		if err := s.users.SaveUpdateMany(txCtx, team.Members); err != nil {
			return nil, err
		}

		// Возвращаем свежесозданную команду с участниками
		return s.teams.Get(txCtx, team.Name)
	})

	if err != nil {
		return entity.Team{}, err
	}

	return createdTeam.(entity.Team), nil
}
