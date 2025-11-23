package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"slices"
	"time"

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

func (s *ServiceImpl) SetUserActive(ctx context.Context, userID string, active bool) (entity.User, error) {
	user, err := s.users.Get(ctx, userID)
	if err != nil {
		if err == sql.ErrNoRows || errors.Is(err, usecase.ErrUserNotFound) {
			return entity.User{}, usecase.ErrUserNotFound
		}
		return entity.User{}, err
	}

	user.IsActive = active
	if err := s.users.UpdateMany(ctx, []entity.User{user}); err != nil {
		return entity.User{}, err
	}

	return user, nil
}

func (s *ServiceImpl) GetUserReviewPRs(ctx context.Context, userID string) ([]entity.PullRequest, error) {
	// Проверяем существование пользователя
	_, err := s.users.Get(ctx, userID)
	if err != nil {
		if err == sql.ErrNoRows || errors.Is(err, usecase.ErrUserNotFound) {
			return nil, usecase.ErrUserNotFound
		}
		return nil, err
	}

	prs, err := s.prs.GetByReviewer(ctx, userID)
	if err != nil {
		return nil, err
	}

	return prs, nil
}

func (s *ServiceImpl) GetUserStats(ctx context.Context, userID string) (entity.UserStats, error) {
	// Проверяем существование пользователя
	_, err := s.users.Get(ctx, userID)
	if err != nil {
		if err == sql.ErrNoRows || errors.Is(err, usecase.ErrUserNotFound) {
			return entity.UserStats{}, usecase.ErrUserNotFound
		}
		return entity.UserStats{}, err
	}

	stats, err := s.users.GetUserStats(ctx, userID)
	if err != nil {
		return entity.UserStats{}, err
	}

	return stats, nil
}

// PullRequestUseCase methods

func (s *ServiceImpl) CreatePR(ctx context.Context, id, name, authorID string) (entity.PullRequest, error) {
	// Проверяем существование автора
	author, err := s.users.Get(ctx, authorID)
	if err != nil {
		if errors.Is(err, usecase.ErrUserNotFound) {
			return entity.PullRequest{}, usecase.ErrUserNotFound
		}
		return entity.PullRequest{}, err
	}

	// Проверяем, не существует ли уже PR с таким ID
	_, err = s.prs.Get(ctx, id)
	if err == nil {
		return entity.PullRequest{}, usecase.ErrPRExists
	}
	if !errors.Is(err, usecase.ErrPRNotFound) {
		return entity.PullRequest{}, err
	}

	// Получаем активных пользователей из команды автора (исключая автора)
	candidates, err := s.users.GetActiveByTeam(ctx, author.TeamName, authorID)
	if err != nil {
		return entity.PullRequest{}, err
	}

	// Случайный выбор до 2 ревьюверов
	reviewerIDs := selectRandomReviewers(candidates, 2)

	// Создаём PR и назначаем ревьюверов в транзакции
	var pr entity.PullRequest
	createdPR, err := s.txManager.DoTx(ctx, func(txCtx context.Context) (any, error) {
		now := time.Now()
		pr = entity.PullRequest{
			ID:        id,
			Name:      name,
			AuthorID:  authorID,
			Status:    entity.PROpen,
			CreatedAt: &now,
		}

		if err := s.prs.Save(txCtx, pr); err != nil {
			return nil, err
		}

		if len(reviewerIDs) > 0 {
			if err := s.prs.AssignReviewers(txCtx, id, reviewerIDs); err != nil {
				return nil, err
			}
		}

		// Получаем полный PR с ревьюверами
		return s.prs.Get(txCtx, id)
	})

	if err != nil {
		return entity.PullRequest{}, err
	}

	return createdPR.(entity.PullRequest), nil
}

func (s *ServiceImpl) MergePR(ctx context.Context, id string) (entity.PullRequest, error) {
	pr, err := s.prs.Get(ctx, id)
	if err != nil {
		if errors.Is(err, usecase.ErrPRNotFound) {
			return entity.PullRequest{}, usecase.ErrPRNotFound
		}
		return entity.PullRequest{}, err
	}

	// Идемпотентность: если уже MERGED, возвращаем как есть
	if pr.Status == entity.PRMerged {
		reviewers, err := s.prs.GetReviewers(ctx, id)
		if err != nil {
			return entity.PullRequest{}, err
		}
		pr.Reviewers = reviewers
		return pr, nil
	}

	mergedPR, err := s.txManager.DoTx(ctx, func(txCtx context.Context) (any, error) {
		// Перечитываем PR в транзакции (на случай, если статус изменился параллельно)
		current, err := s.prs.Get(txCtx, id)
		if err != nil {
			return nil, err
		}
		if current.Status == entity.PRMerged {
			// Уже кто-то успел смержить — идемпотентность
			return current, nil
		}

		now := time.Now()
		current.Status = entity.PRMerged
		current.MergedAt = &now

		if err := s.prs.Update(txCtx, current); err != nil {
			return nil, err
		}

		// Перечитываем с ревьюверами уже в транзакции
		finalPR, err := s.prs.Get(txCtx, id)
		if err != nil {
			return nil, err
		}
		reviewers, err := s.prs.GetReviewers(txCtx, id)
		if err != nil {
			return nil, err
		}
		finalPR.Reviewers = reviewers

		return finalPR, nil
	})
	if err != nil {
		return entity.PullRequest{}, err
	}
	result := mergedPR.(entity.PullRequest)

	return result, nil
}

func (s *ServiceImpl) ReassignReviewer(ctx context.Context, prID, oldReviewerID string) (entity.PullRequest, string, error) {
	// Проверяем существование PR
	pr, err := s.prs.Get(ctx, prID)
	if err != nil {
		if errors.Is(err, usecase.ErrPRNotFound) {
			return entity.PullRequest{}, "", usecase.ErrPRNotFound
		}
		return entity.PullRequest{}, "", err
	}

	// Проверка: нельзя переназначать для MERGED PR
	if pr.Status == entity.PRMerged {
		return entity.PullRequest{}, "", usecase.ErrAlreadyMerged
	}

	// Выбираем нового user-a для ревью
	result, err := s.txManager.DoTx(ctx, func(txCtx context.Context) (any, error) {
		// 1. Перечитываем PR в транзакции
		currentPR, err := s.prs.Get(txCtx, prID)
		if err != nil {
			return nil, err
		}
		if currentPR.Status == entity.PRMerged {
			return nil, usecase.ErrAlreadyMerged
		}

		// 2. Проверяем, что oldReviewerID всё ещё назначен
		currentReviewers, err := s.prs.GetReviewers(txCtx, prID)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(currentReviewers, oldReviewerID) {
			return nil, usecase.ErrNotReviewer
		}

		// 3. Получаем данные старого ревьювера
		oldReviewer, err := s.users.Get(txCtx, oldReviewerID)
		if err != nil {
			return nil, err
		}

		// 4. Получаем актуальных кандидатов из команды (исключаем автора PR и старого ревьювера)
		candidates, err := s.users.GetActiveByTeam(txCtx, oldReviewer.TeamName, currentPR.AuthorID)
		if err != nil {
			return nil, err
		}

		var validCandidates []entity.User
		for _, c := range candidates {
			if c.ID != oldReviewerID {
				validCandidates = append(validCandidates, c)
			}
		}

		var newReviewerID string

		if len(validCandidates) == 0 {
			// Просто удаляем старого ревьювера
			if err := s.prs.RemoveReviewer(txCtx, prID, oldReviewerID); err != nil {
				return nil, err
			}
		} else {
			// Выбираем нового — детерминированно внутри транзакции!
			newReviewerID = selectRandomReviewers(validCandidates, 1)[0]
			if err := s.prs.ReplaceReviewer(txCtx, prID, oldReviewerID, newReviewerID); err != nil {
				return nil, err
			}
		}

		// 5. Читаем финальный PR с актуальными ревьюверами — всё в транзакции!
		finalPR, err := s.prs.Get(txCtx, prID)
		if err != nil {
			return nil, err
		}
		finalReviewers, err := s.prs.GetReviewers(txCtx, prID)
		if err != nil {
			return nil, err
		}
		finalPR.Reviewers = finalReviewers

		return struct {
			PR            entity.PullRequest
			NewReviewerID string
		}{
			PR:            finalPR,
			NewReviewerID: newReviewerID,
		}, nil
	})
	if err != nil {
		return entity.PullRequest{}, "", err
	}

	typedResult := result.(struct {
		PR            entity.PullRequest
		NewReviewerID string
	})

	return typedResult.PR, typedResult.NewReviewerID, nil
}

func (s *ServiceImpl) GetPRStats(ctx context.Context) (entity.PRStats, error) {
	stats, err := s.prs.GetStats(ctx)
	if err != nil {
		return entity.PRStats{}, err
	}
	return stats, nil
}

// Массовая деактивация с переназначением
func (s *ServiceImpl) DeactivateUsersAndReassign(ctx context.Context, teamName string, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}

	// === 1. Предварительные проверки вне транзакции ===

	// Проверяем существование команды
	if _, err := s.teams.Get(ctx, teamName); err != nil {
		if errors.Is(err, usecase.ErrTeamNotFound) {
			return usecase.ErrTeamNotFound
		}
		return err
	}

	// Проверяем, что ВСЕ userIDs принадлежат этой команде
	usersInTeam, err := s.users.GetByTeam(ctx, teamName)
	if err != nil {
		return fmt.Errorf("failed to fetch team users: %w", err)
	}

	teamUserIDs := make(map[string]bool, len(usersInTeam))
	for _, u := range usersInTeam {
		teamUserIDs[u.ID] = true
	}

	for _, id := range userIDs {
		if !teamUserIDs[id] {
			return usecase.ErrUserNotInTeam
		}
	}

	// === 2. Атомарная операция в транзакции ===
	return s.txManager.Do(ctx, func(txCtx context.Context) error {
		// 1. Деактивируем
		if err := s.users.DeactivateMany(txCtx, userIDs); err != nil {
			return err
		}

		// 2. Берём все открытые PR команды
		teamOpenPRs, err := s.prs.GetOpenPRsByTeam(txCtx, teamName)
		if err != nil {
			return err
		}
		if len(teamOpenPRs) == 0 {
			return nil
		}

		// 3. все ревьюверы для этих PR
		prIDs := make([]string, 0, len(teamOpenPRs))
		prByID := make(map[string]entity.PullRequest, len(teamOpenPRs))
		for _, pr := range teamOpenPRs {
			prIDs = append(prIDs, pr.ID)
			prByID[pr.ID] = pr
		}

		allReviewers, err := s.prs.GetReviewersBatch(txCtx, prIDs)
		if err != nil {
			return err
		}

		// 4. Активные пользователи команды
		activeTeamUsers, err := s.users.GetActiveByTeam(txCtx, teamName, "")
		if err != nil {
			return err
		}

		activeUserIDs := make(map[string]bool, len(activeTeamUsers))
		for _, u := range activeTeamUsers {
			activeUserIDs[u.ID] = true
		}

		deactivatedSet := make(map[string]bool, len(userIDs))
		for _, id := range userIDs {
			deactivatedSet[id] = true
		}

		// 5. Обрабатываем каждый PR
		for prID, pr := range prByID {
			currentReviewers := allReviewers[prID]
			var toReplace []string

			for _, rID := range currentReviewers {
				if deactivatedSet[rID] {
					toReplace = append(toReplace, rID)
				}
			}
			if len(toReplace) == 0 {
				continue
			}

			// Кандидаты: активные из команды, не автор и не уже назначены
			var candidates []string
			for _, u := range activeTeamUsers {
				if u.ID != pr.AuthorID && !contains(currentReviewers, u.ID) {
					candidates = append(candidates, u.ID)
				}
			}

			for _, oldID := range toReplace {
				if len(candidates) > 0 {
					idx := rand.Intn(len(candidates))
					newID := candidates[idx]
					candidates[idx] = candidates[len(candidates)-1]
					candidates = candidates[:len(candidates)-1]

					if err := s.prs.ReplaceReviewer(txCtx, prID, oldID, newID); err != nil {
						return err
					}
				} else {
					if err := s.prs.RemoveReviewer(txCtx, prID, oldID); err != nil {
						return err
					}
				}
			}
		}

		return nil
	})
}

func contains(slice []string, val string) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

// selectRandomReviewers выбирает случайных ревьюверов из списка кандидатов (до maxCount)
func selectRandomReviewers(candidates []entity.User, maxCount int) []string {
	n := len(candidates)
	if n == 0 {
		return []string{}
	}
	if maxCount > n {
		maxCount = n
	}

	// rand.Perm возвращает случайную перестановку 0..n-1
	perm := rand.Perm(n)

	result := make([]string, 0, maxCount)
	for i := 0; i < maxCount; i++ {
		result = append(result, candidates[perm[i]].ID)
	}
	return result
}
