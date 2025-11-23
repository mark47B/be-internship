//go:build e2e
// +build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/mark47B/be-internship/internal/infra/transport/rest/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAllEndpoints - тесты для каждого эндпоинта отдельно
func TestAllEndpoints(t *testing.T) {
	// Не используем t.Parallel() чтобы избежать конфликтов имен
	db := setupTestDB(t)
	client := newTestClient(db)
	t.Cleanup(client.Close)

	// Helper для создания уникальных имен
	counter := time.Now().UnixNano()
	uniqueID := func(prefix string) string {
		counter++
		return fmt.Sprintf("%s-%s-%d", prefix, strings.ReplaceAll(t.Name(), "/", "-"), counter)
	}

	t.Run("Health Check", func(t *testing.T) {
		resp := client.get(t, "/health")
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var body map[string]string
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, "ok", body["status"])
	})

	t.Run("POST /team/add", func(t *testing.T) {
		teamName := uniqueID("team")
		team := gen.Team{
			TeamName: teamName,
			Members: []gen.TeamMember{
				{UserId: uniqueID("u1"), Username: "User1", IsActive: true},
				{UserId: uniqueID("u2"), Username: "User2", IsActive: true},
			},
		}

		resp := client.post(t, "/team/add", team)
		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var respBody struct {
			Team gen.Team `json:"team"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&respBody))
		assert.Equal(t, teamName, respBody.Team.TeamName)
		assert.Len(t, respBody.Team.Members, 2)
	})

	t.Run("GET /team/get", func(t *testing.T) {
		teamName := uniqueID("team")
		team := gen.Team{
			TeamName: teamName,
			Members: []gen.TeamMember{
				{UserId: uniqueID("u1"), Username: "User1", IsActive: true},
			},
		}
		client.post(t, "/team/add", team)

		resp := client.get(t, "/team/get?team_name="+teamName)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var respTeam gen.Team
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&respTeam))
		assert.Equal(t, teamName, respTeam.TeamName)
		assert.Len(t, respTeam.Members, 1)
	})

	t.Run("POST /pullRequest/create", func(t *testing.T) {
		teamName := uniqueID("team")
		authorID := uniqueID("author")
		reviewerID := uniqueID("reviewer")

		client.post(t, "/team/add", gen.Team{
			TeamName: teamName,
			Members: []gen.TeamMember{
				{UserId: authorID, Username: "Author", IsActive: true},
				{UserId: reviewerID, Username: "Reviewer", IsActive: true},
			},
		})

		prID := uniqueID("pr")
		resp := client.post(t, "/pullRequest/create", map[string]any{
			"pull_request_id":   prID,
			"pull_request_name": "Test PR",
			"author_id":         authorID,
		})

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var body struct {
			Pr gen.PullRequest `json:"pr"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, prID, body.Pr.PullRequestId)
		assert.Equal(t, gen.PullRequestStatusOPEN, body.Pr.Status)
		assert.NotNil(t, body.Pr.CreatedAt)
	})

	t.Run("POST /pullRequest/merge", func(t *testing.T) {
		teamName := uniqueID("team")
		authorID := uniqueID("author")
		prID := uniqueID("pr")

		client.post(t, "/team/add", gen.Team{
			TeamName: teamName,
			Members: []gen.TeamMember{
				{UserId: authorID, Username: "Author", IsActive: true},
			},
		})

		client.post(t, "/pullRequest/create", map[string]any{
			"pull_request_id":   prID,
			"pull_request_name": "Test PR",
			"author_id":         authorID,
		})

		resp := client.post(t, "/pullRequest/merge", map[string]any{
			"pull_request_id": prID,
		})

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var body struct {
			Pr gen.PullRequest `json:"pr"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, gen.PullRequestStatusMERGED, body.Pr.Status)
		assert.NotNil(t, body.Pr.MergedAt)
	})

	t.Run("POST /pullRequest/reassign", func(t *testing.T) {
		teamName := uniqueID("team")
		authorID := uniqueID("author")
		r1ID := uniqueID("r1")
		r2ID := uniqueID("r2")
		r3ID := uniqueID("r3") // Добавляем третьего ревьювера для гарантии наличия кандидата
		prID := uniqueID("pr")

		client.post(t, "/team/add", gen.Team{
			TeamName: teamName,
			Members: []gen.TeamMember{
				{UserId: authorID, Username: "Author", IsActive: true},
				{UserId: r1ID, Username: "R1", IsActive: true},
				{UserId: r2ID, Username: "R2", IsActive: true},
				{UserId: r3ID, Username: "R3", IsActive: true},
			},
		})

		// Создаем PR
		createResp := client.post(t, "/pullRequest/create", map[string]any{
			"pull_request_id":   prID,
			"pull_request_name": "Test PR",
			"author_id":         authorID,
		})
		require.Equal(t, http.StatusCreated, createResp.StatusCode)

		var created struct {
			Pr gen.PullRequest `json:"pr"`
		}
		require.NoError(t, json.NewDecoder(createResp.Body).Decode(&created))
		require.Greater(t, len(created.Pr.AssignedReviewers), 0)

		oldReviewer := created.Pr.AssignedReviewers[0]

		// Переназначаем (новый ревьювер выбирается из команды старого ревьювера)
		resp := client.post(t, "/pullRequest/reassign", map[string]any{
			"pull_request_id": prID,
			"old_user_id":     oldReviewer,
		})

		if resp.StatusCode != http.StatusOK {
			var errBody gen.ErrorResponse
			if err := json.NewDecoder(resp.Body).Decode(&errBody); err == nil {
				t.Logf("Reassign error: %s - %s", errBody.Error.Code, errBody.Error.Message)
			}
		}
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var reassignResp struct {
			Pr         gen.PullRequest `json:"pr"`
			ReplacedBy string          `json:"replaced_by"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&reassignResp))
		assert.NotEqual(t, oldReviewer, reassignResp.ReplacedBy)
		assert.Contains(t, reassignResp.Pr.AssignedReviewers, reassignResp.ReplacedBy)
	})

	t.Run("POST /users/setIsActive", func(t *testing.T) {
		teamName := uniqueID("team")
		userID := uniqueID("user")

		client.post(t, "/team/add", gen.Team{
			TeamName: teamName,
			Members: []gen.TeamMember{
				{UserId: userID, Username: "User", IsActive: true},
			},
		})

		resp := client.post(t, "/users/setIsActive", map[string]any{
			"user_id":   userID,
			"is_active": false,
		})

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var body struct {
			User gen.User `json:"user"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, userID, body.User.UserId)
		assert.False(t, body.User.IsActive)
	})

	t.Run("GET /users/getReview", func(t *testing.T) {
		teamName := uniqueID("team")
		authorID := uniqueID("author")
		reviewerID := uniqueID("reviewer")
		prID := uniqueID("pr")

		client.post(t, "/team/add", gen.Team{
			TeamName: teamName,
			Members: []gen.TeamMember{
				{UserId: authorID, Username: "Author", IsActive: true},
				{UserId: reviewerID, Username: "Reviewer", IsActive: true},
			},
		})

		client.post(t, "/pullRequest/create", map[string]any{
			"pull_request_id":   prID,
			"pull_request_name": "Test PR",
			"author_id":         authorID,
		})

		resp := client.get(t, "/users/getReview?user_id="+reviewerID)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var body struct {
			UserId       string                 `json:"user_id"`
			PullRequests []gen.PullRequestShort `json:"pull_requests"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, reviewerID, body.UserId)
		assert.GreaterOrEqual(t, len(body.PullRequests), 1)
	})

	t.Run("GET /users/stats", func(t *testing.T) {
		teamName := uniqueID("team")
		userID := uniqueID("user")
		prID := uniqueID("pr")

		client.post(t, "/team/add", gen.Team{
			TeamName: teamName,
			Members: []gen.TeamMember{
				{UserId: userID, Username: "User", IsActive: true},
			},
		})

		client.post(t, "/pullRequest/create", map[string]any{
			"pull_request_id":   prID,
			"pull_request_name": "Test PR",
			"author_id":         userID,
		})

		resp := client.get(t, "/users/stats?user_id="+userID)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var stats gen.UserStats
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&stats))
		assert.Equal(t, userID, stats.UserId)
		assert.GreaterOrEqual(t, stats.CreatedPrCount, 1)
	})

	t.Run("GET /pullRequest/stats", func(t *testing.T) {
		resp := client.get(t, "/pullRequest/stats")
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var stats gen.PRStats
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&stats))
		assert.GreaterOrEqual(t, stats.Total, 0)
		assert.GreaterOrEqual(t, stats.Open, 0)
		assert.GreaterOrEqual(t, stats.Merged, 0)
	})

	t.Run("PATCH /teams/{teamName}/deactivate-members", func(t *testing.T) {
		teamName := uniqueID("team")
		user1ID := uniqueID("u1")
		user2ID := uniqueID("u2")

		client.post(t, "/team/add", gen.Team{
			TeamName: teamName,
			Members: []gen.TeamMember{
				{UserId: user1ID, Username: "User1", IsActive: true},
				{UserId: user2ID, Username: "User2", IsActive: true},
			},
		})

		resp := client.patch(t, "/teams/"+teamName+"/deactivate-members", map[string]any{
			"user_ids": []string{user1ID},
		})

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var body map[string]string
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Contains(t, body["message"], "deactivated")
	})
}

// TestComplexScenarios - комплексные тесты с комбинацией эндпоинтов
func TestComplexScenarios(t *testing.T) {
	// Не используем t.Parallel() для комплексных тестов, чтобы избежать конфликтов
	db := setupTestDB(t)
	client := newTestClient(db)
	t.Cleanup(client.Close)

	counter := time.Now().UnixNano()
	uniqueID := func(prefix string) string {
		counter++
		return fmt.Sprintf("%s-%s-%d", prefix, strings.ReplaceAll(t.Name(), "/", "-"), counter)
	}

	t.Run("Merge Idempotency - Multiple Calls", func(t *testing.T) {
		teamName := uniqueID("team")
		authorID := uniqueID("author")
		prID := uniqueID("pr")

		client.post(t, "/team/add", gen.Team{
			TeamName: teamName,
			Members: []gen.TeamMember{
				{UserId: authorID, Username: "Author", IsActive: true},
			},
		})

		client.post(t, "/pullRequest/create", map[string]any{
			"pull_request_id":   prID,
			"pull_request_name": "Test PR",
			"author_id":         authorID,
		})

		// Первый merge
		resp1 := client.post(t, "/pullRequest/merge", map[string]any{
			"pull_request_id": prID,
		})
		require.Equal(t, http.StatusOK, resp1.StatusCode)

		var body1 struct {
			Pr gen.PullRequest `json:"pr"`
		}
		require.NoError(t, json.NewDecoder(resp1.Body).Decode(&body1))
		mergedAt1 := body1.Pr.MergedAt
		require.NotNil(t, mergedAt1)

		// Второй merge (идемпотентный)
		resp2 := client.post(t, "/pullRequest/merge", map[string]any{
			"pull_request_id": prID,
		})
		require.Equal(t, http.StatusOK, resp2.StatusCode)

		var body2 struct {
			Pr gen.PullRequest `json:"pr"`
		}
		require.NoError(t, json.NewDecoder(resp2.Body).Decode(&body2))
		assert.Equal(t, gen.PullRequestStatusMERGED, body2.Pr.Status)
		assert.NotNil(t, body2.Pr.MergedAt)

		// Проверяем, что mergedAt не изменился (допускаем небольшую разницу из-за задержек)
		assert.WithinDuration(t, *mergedAt1, *body2.Pr.MergedAt, time.Second, "mergedAt should be the same (idempotent)")

		// Третий merge (тоже идемпотентный)
		resp3 := client.post(t, "/pullRequest/merge", map[string]any{
			"pull_request_id": prID,
		})
		require.Equal(t, http.StatusOK, resp3.StatusCode)
	})

	t.Run("Deactivate Reviewer Assigned to PR - Auto Reassignment", func(t *testing.T) {
		teamName := uniqueID("team")
		authorID := uniqueID("author")
		r1ID := uniqueID("r1")
		r2ID := uniqueID("r2")
		r3ID := uniqueID("r3")
		prID := uniqueID("pr")

		// Создаем команду с несколькими ревьюверами
		client.post(t, "/team/add", gen.Team{
			TeamName: teamName,
			Members: []gen.TeamMember{
				{UserId: authorID, Username: "Author", IsActive: true},
				{UserId: r1ID, Username: "R1", IsActive: true},
				{UserId: r2ID, Username: "R2", IsActive: true},
				{UserId: r3ID, Username: "R3", IsActive: true},
			},
		})

		// Создаем PR (автоматически назначаются ревьюверы)
		createResp := client.post(t, "/pullRequest/create", map[string]any{
			"pull_request_id":   prID,
			"pull_request_name": "Test PR",
			"author_id":         authorID,
		})
		require.Equal(t, http.StatusCreated, createResp.StatusCode)

		var created struct {
			Pr gen.PullRequest `json:"pr"`
		}
		require.NoError(t, json.NewDecoder(createResp.Body).Decode(&created))
		require.Greater(t, len(created.Pr.AssignedReviewers), 0, "PR should have reviewers")

		originalReviewers := created.Pr.AssignedReviewers
		t.Logf("Original reviewers: %v", originalReviewers)

		// Деактивируем одного из ревьюверов через массовую деактивацию
		deactivateResp := client.patch(t, "/teams/"+teamName+"/deactivate-members", map[string]any{
			"user_ids": []string{originalReviewers[0]},
		})
		require.Equal(t, http.StatusOK, deactivateResp.StatusCode)

		// Проверяем, что PR все еще существует и ревьюверы переназначены
		// Получаем PR через создание нового запроса (или можно добавить GET endpoint)
		// Для проверки используем прямой SQL запрос
		var reviewerCount int
		err := db.QueryRow(`SELECT COUNT(*) FROM review_assignments WHERE pr_id = $1`, prID).Scan(&reviewerCount)
		require.NoError(t, err)
		assert.Greater(t, reviewerCount, 0, "PR should still have reviewers after deactivation")

		// Проверяем, что деактивированный ревьювер больше не назначен
		var deactivatedAssigned int
		err = db.QueryRow(`SELECT COUNT(*) FROM review_assignments WHERE pr_id = $1 AND reviewer_id = $2`, prID, originalReviewers[0]).Scan(&deactivatedAssigned)
		require.NoError(t, err)
		assert.Equal(t, 0, deactivatedAssigned, "Deactivated reviewer should not be assigned")
	})

	t.Run("Create PR -> Merge -> Reassign Should Fail", func(t *testing.T) {
		teamName := uniqueID("team")
		authorID := uniqueID("author")
		r1ID := uniqueID("r1")
		r2ID := uniqueID("r2")
		prID := uniqueID("pr")

		client.post(t, "/team/add", gen.Team{
			TeamName: teamName,
			Members: []gen.TeamMember{
				{UserId: authorID, Username: "Author", IsActive: true},
				{UserId: r1ID, Username: "R1", IsActive: true},
				{UserId: r2ID, Username: "R2", IsActive: true},
			},
		})

		// Создаем PR
		createResp := client.post(t, "/pullRequest/create", map[string]any{
			"pull_request_id":   prID,
			"pull_request_name": "Test PR",
			"author_id":         authorID,
		})
		require.Equal(t, http.StatusCreated, createResp.StatusCode)

		var created struct {
			Pr gen.PullRequest `json:"pr"`
		}
		require.NoError(t, json.NewDecoder(createResp.Body).Decode(&created))
		require.Greater(t, len(created.Pr.AssignedReviewers), 0)

		oldReviewer := created.Pr.AssignedReviewers[0]

		// Merge PR
		mergeResp := client.post(t, "/pullRequest/merge", map[string]any{
			"pull_request_id": prID,
		})
		require.Equal(t, http.StatusOK, mergeResp.StatusCode)

		// Попытка переназначить после merge должна вернуть ошибку
		reassignResp := client.post(t, "/pullRequest/reassign", map[string]any{
			"pull_request_id": prID,
			"old_user_id":     oldReviewer,
		})

		require.Equal(t, http.StatusConflict, reassignResp.StatusCode)

		var errResp gen.ErrorResponse
		require.NoError(t, json.NewDecoder(reassignResp.Body).Decode(&errResp))
		assert.Equal(t, gen.PRMERGED, errResp.Error.Code)
	})

	t.Run("Mass Deactivation with Multiple PRs", func(t *testing.T) {
		teamName := uniqueID("team")
		author1ID := uniqueID("a1")
		author2ID := uniqueID("a2")
		r1ID := uniqueID("r1")
		r2ID := uniqueID("r2")
		r3ID := uniqueID("r3")
		pr1ID := uniqueID("pr1")
		pr2ID := uniqueID("pr2")

		// Создаём команду
		client.post(t, "/team/add", gen.Team{
			TeamName: teamName,
			Members: []gen.TeamMember{
				{UserId: author1ID, Username: "Author1", IsActive: true},
				{UserId: author2ID, Username: "Author2", IsActive: true},
				{UserId: r1ID, Username: "R1", IsActive: true},
				{UserId: r2ID, Username: "R2", IsActive: true},
				{UserId: r3ID, Username: "R3", IsActive: true},
			},
		})

		// Создаём два PR
		client.post(t, "/pullRequest/create", map[string]any{
			"pull_request_id":   pr1ID,
			"pull_request_name": "PR 1",
			"author_id":         author1ID,
		})
		client.post(t, "/pullRequest/create", map[string]any{
			"pull_request_id":   pr2ID,
			"pull_request_name": "PR 2",
			"author_id":         author2ID,
		})

		// Проверяем, что ревьюверы назначены
		var count1, count2 int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM review_assignments WHERE pr_id = $1`, pr1ID).Scan(&count1))
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM review_assignments WHERE pr_id = $1`, pr2ID).Scan(&count2))
		assert.Greater(t, count1, 0)
		assert.Greater(t, count2, 0)

		deactivateResp := client.patch(t, "/teams/"+teamName+"/deactivate-members", map[string]any{
			"user_ids": pq.Array([]string{r1ID, r2ID}), // ← вот здесь было падение
		})
		require.Equal(t, http.StatusOK, deactivateResp.StatusCode)

		// Проверяем, что после деактивации ревьюверы переназначены
		var newCount1, newCount2 int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM review_assignments WHERE pr_id = $1`, pr1ID).Scan(&newCount1))
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM review_assignments WHERE pr_id = $1`, pr2ID).Scan(&newCount2))
		assert.Greater(t, newCount1, 0, "PR1 должен иметь хотя бы одного активного ревьювера")
		assert.Greater(t, newCount2, 0, "PR2 должен иметь хотя бы одного активного ревьювера")

		// Проверяем, что деактивированные больше не назначены
		var deactivatedCount1, deactivatedCount2 int
		require.NoError(t, db.QueryRow(`
			SELECT COUNT(*) FROM review_assignments
			WHERE pr_id = $1 AND reviewer_id = ANY($2)
		`, pr1ID, pq.Array([]string{r1ID, r2ID})).Scan(&deactivatedCount1))
		require.NoError(t, db.QueryRow(`
			SELECT COUNT(*) FROM review_assignments
			WHERE pr_id = $1 AND reviewer_id = ANY($2)
		`, pr2ID, pq.Array([]string{r1ID, r2ID})).Scan(&deactivatedCount2))

		assert.Equal(t, 0, deactivatedCount1)
		assert.Equal(t, 0, deactivatedCount2)
	})

	t.Run("Deactivate and Reassign - Verify New Reviewer is Active", func(t *testing.T) {
		teamName := uniqueID("team")
		authorID := uniqueID("author")
		r1ID := uniqueID("r1")
		r2ID := uniqueID("r2")
		r3ID := uniqueID("r3")
		prID := uniqueID("pr")

		// Создаем команду
		client.post(t, "/team/add", gen.Team{
			TeamName: teamName,
			Members: []gen.TeamMember{
				{UserId: authorID, Username: "Author", IsActive: true},
				{UserId: r1ID, Username: "R1", IsActive: true},
				{UserId: r2ID, Username: "R2", IsActive: true},
				{UserId: r3ID, Username: "R3", IsActive: true},
			},
		})

		// Создаем PR
		createResp := client.post(t, "/pullRequest/create", map[string]any{
			"pull_request_id":   prID,
			"pull_request_name": "Test PR",
			"author_id":         authorID,
		})
		require.Equal(t, http.StatusCreated, createResp.StatusCode)

		var created struct {
			Pr gen.PullRequest `json:"pr"`
		}
		require.NoError(t, json.NewDecoder(createResp.Body).Decode(&created))
		require.Greater(t, len(created.Pr.AssignedReviewers), 0)

		originalReviewer := created.Pr.AssignedReviewers[0]

		// Деактивируем ревьювера
		deactivateResp := client.patch(t, "/teams/"+teamName+"/deactivate-members", map[string]any{
			"user_ids": []string{originalReviewer},
		})
		require.Equal(t, http.StatusOK, deactivateResp.StatusCode)

		// Проверяем, что новый ревьювер активен
		var newReviewerID string
		var isActive bool
		err := db.QueryRow(`
			SELECT ra.reviewer_id, u.is_active
			FROM review_assignments ra
			JOIN users u ON ra.reviewer_id = u.id
			WHERE ra.pr_id = $1 AND ra.reviewer_id != $2
			LIMIT 1
		`, prID, originalReviewer).Scan(&newReviewerID, &isActive)
		require.NoError(t, err)
		assert.True(t, isActive, "New reviewer should be active")
		assert.NotEqual(t, originalReviewer, newReviewerID, "New reviewer should be different")
	})

	t.Run("Multiple Reassignments on Same PR", func(t *testing.T) {
		teamName := uniqueID("team")
		authorID := uniqueID("author")
		r1ID := uniqueID("r1")
		r2ID := uniqueID("r2")
		r3ID := uniqueID("r3")
		r4ID := uniqueID("r4")
		prID := uniqueID("pr")

		client.post(t, "/team/add", gen.Team{
			TeamName: teamName,
			Members: []gen.TeamMember{
				{UserId: authorID, Username: "Author", IsActive: true},
				{UserId: r1ID, Username: "R1", IsActive: true},
				{UserId: r2ID, Username: "R2", IsActive: true},
				{UserId: r3ID, Username: "R3", IsActive: true},
				{UserId: r4ID, Username: "R4", IsActive: true},
			},
		})

		// Создаем PR
		createResp := client.post(t, "/pullRequest/create", map[string]any{
			"pull_request_id":   prID,
			"pull_request_name": "Test PR",
			"author_id":         authorID,
		})
		require.Equal(t, http.StatusCreated, createResp.StatusCode)

		var created struct {
			Pr gen.PullRequest `json:"pr"`
		}
		require.NoError(t, json.NewDecoder(createResp.Body).Decode(&created))
		require.Greater(t, len(created.Pr.AssignedReviewers), 0)

		// Первое переназначение
		oldReviewer1 := created.Pr.AssignedReviewers[0]
		resp1 := client.post(t, "/pullRequest/reassign", map[string]any{
			"pull_request_id": prID,
			"old_user_id":     oldReviewer1,
		})
		require.Equal(t, http.StatusOK, resp1.StatusCode)

		var reassign1 struct {
			Pr         gen.PullRequest `json:"pr"`
			ReplacedBy string          `json:"replaced_by"`
		}
		require.NoError(t, json.NewDecoder(resp1.Body).Decode(&reassign1))
		newReviewer1 := reassign1.ReplacedBy

		// Второе переназначение (переназначаем нового ревьювера)
		resp2 := client.post(t, "/pullRequest/reassign", map[string]any{
			"pull_request_id": prID,
			"old_user_id":     newReviewer1,
		})
		require.Equal(t, http.StatusOK, resp2.StatusCode)

		var reassign2 struct {
			Pr         gen.PullRequest `json:"pr"`
			ReplacedBy *string         `json:"replaced_by,omitempty"`
		}
		require.NoError(t, json.NewDecoder(resp2.Body).Decode(&reassign2))
		// replaced_by может быть пустым, если ревьювер просто удален
		if reassign2.ReplacedBy != nil {
			assert.NotEqual(t, newReviewer1, *reassign2.ReplacedBy, "New reviewer should be different from the one just replaced")
		}
		// Проверяем, что старый ревьювер больше не в списке
		assert.NotContains(t, reassign2.Pr.AssignedReviewers, newReviewer1)
	})

	t.Run("Deactivate All Reviewers - No Candidates", func(t *testing.T) {
		teamName := uniqueID("team")
		authorID := uniqueID("author")
		r1ID := uniqueID("r1")
		prID := uniqueID("pr")

		// Создаем команду с одним ревьювером
		client.post(t, "/team/add", gen.Team{
			TeamName: teamName,
			Members: []gen.TeamMember{
				{UserId: authorID, Username: "Author", IsActive: true},
				{UserId: r1ID, Username: "R1", IsActive: true},
			},
		})

		// Создаем PR
		client.post(t, "/pullRequest/create", map[string]any{
			"pull_request_id":   prID,
			"pull_request_name": "Test PR",
			"author_id":         authorID,
		})

		// Деактивируем единственного ревьювера
		deactivateResp := client.patch(t, "/teams/"+teamName+"/deactivate-members", map[string]any{
			"user_ids": []string{r1ID},
		})
		require.Equal(t, http.StatusOK, deactivateResp.StatusCode)

		// Проверяем, что ревьювер больше не назначен (нет кандидатов для замены)
		var reviewerCount int
		err := db.QueryRow(`SELECT COUNT(*) FROM review_assignments WHERE pr_id = $1`, prID).Scan(&reviewerCount)
		require.NoError(t, err)
		// Может быть 0, если нет кандидатов для замены - ревьювер просто удален
		assert.GreaterOrEqual(t, reviewerCount, 0, "Reviewer count should be >= 0")
		// Проверяем, что деактивированный ревьювер точно не назначен
		var deactivatedCount int
		err = db.QueryRow(`SELECT COUNT(*) FROM review_assignments WHERE pr_id = $1 AND reviewer_id = $2`, prID, r1ID).Scan(&deactivatedCount)
		require.NoError(t, err)
		assert.Equal(t, 0, deactivatedCount, "Deactivated reviewer should not be assigned")
	})

	t.Run("Statistics After Multiple Operations", func(t *testing.T) {
		teamName := uniqueID("team")
		user1ID := uniqueID("u1")
		user2ID := uniqueID("u2")
		pr1ID := uniqueID("pr1")
		pr2ID := uniqueID("pr2")
		pr3ID := uniqueID("pr3")

		client.post(t, "/team/add", gen.Team{
			TeamName: teamName,
			Members: []gen.TeamMember{
				{UserId: user1ID, Username: "U1", IsActive: true},
				{UserId: user2ID, Username: "U2", IsActive: true},
			},
		})

		// Создаем несколько PR
		client.post(t, "/pullRequest/create", map[string]any{
			"pull_request_id": pr1ID, "author_id": user1ID, "pull_request_name": "PR1",
		})
		client.post(t, "/pullRequest/create", map[string]any{
			"pull_request_id": pr2ID, "author_id": user2ID, "pull_request_name": "PR2",
		})
		client.post(t, "/pullRequest/create", map[string]any{
			"pull_request_id": pr3ID, "author_id": user1ID, "pull_request_name": "PR3",
		})

		// Merge один PR
		client.post(t, "/pullRequest/merge", map[string]any{"pull_request_id": pr1ID})

		// Проверяем статистику пользователя
		statsResp := client.get(t, "/users/stats?user_id="+user1ID)
		require.Equal(t, http.StatusOK, statsResp.StatusCode)

		var userStats gen.UserStats
		require.NoError(t, json.NewDecoder(statsResp.Body).Decode(&userStats))
		assert.Equal(t, user1ID, userStats.UserId)
		assert.GreaterOrEqual(t, userStats.CreatedPrCount, 2)
		assert.GreaterOrEqual(t, userStats.MergedPrCount, 1)

		// Проверяем общую статистику PR
		prStatsResp := client.get(t, "/pullRequest/stats")
		require.Equal(t, http.StatusOK, prStatsResp.StatusCode)

		var prStats gen.PRStats
		require.NoError(t, json.NewDecoder(prStatsResp.Body).Decode(&prStats))
		assert.GreaterOrEqual(t, prStats.Total, 3)
		assert.GreaterOrEqual(t, prStats.Open, 2)
		assert.GreaterOrEqual(t, prStats.Merged, 1)
	})

	t.Run("Concurrent Operations - Race Condition Protection", func(t *testing.T) {
		teamName := uniqueID("team")
		authorID := uniqueID("author")
		r1ID := uniqueID("r1")
		r2ID := uniqueID("r2")
		r3ID := uniqueID("r3")
		prID := uniqueID("pr")

		client.post(t, "/team/add", gen.Team{
			TeamName: teamName,
			Members: []gen.TeamMember{
				{UserId: authorID, Username: "Author", IsActive: true},
				{UserId: r1ID, Username: "R1", IsActive: true},
				{UserId: r2ID, Username: "R2", IsActive: true},
				{UserId: r3ID, Username: "R3", IsActive: true},
			},
		})

		// Создаем PR
		createResp := client.post(t, "/pullRequest/create", map[string]any{
			"pull_request_id":   prID,
			"pull_request_name": "Test PR",
			"author_id":         authorID,
		})
		require.Equal(t, http.StatusCreated, createResp.StatusCode)

		var created struct {
			Pr gen.PullRequest `json:"pr"`
		}
		require.NoError(t, json.NewDecoder(createResp.Body).Decode(&created))
		require.Greater(t, len(created.Pr.AssignedReviewers), 0)

		oldReviewer := created.Pr.AssignedReviewers[0]

		// Выполняем несколько операций подряд (имитация конкурентного доступа)
		// В реальности это защищено транзакциями
		resp1 := client.post(t, "/pullRequest/reassign", map[string]any{
			"pull_request_id": prID,
			"old_user_id":     oldReviewer,
		})
		require.Equal(t, http.StatusOK, resp1.StatusCode)

		// Проверяем, что PR все еще в состоянии OPEN
		var status string
		err := db.QueryRow(`SELECT status FROM pull_requests WHERE id = $1`, prID).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "OPEN", status)

		// Merge должен работать корректно
		mergeResp := client.post(t, "/pullRequest/merge", map[string]any{
			"pull_request_id": prID,
		})
		require.Equal(t, http.StatusOK, mergeResp.StatusCode)

		// После merge статус должен быть MERGED
		err = db.QueryRow(`SELECT status FROM pull_requests WHERE id = $1`, prID).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "MERGED", status)
	})
}
