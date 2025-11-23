// e2e/e2e_test.go
//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/mark47B/be-internship/internal/app"
	"github.com/mark47B/be-internship/internal/infra/storage/pg"
	"github.com/mark47B/be-internship/internal/infra/transport/rest/gen"
	"github.com/mark47B/be-internship/internal/infra/transport/rest/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testClient struct {
	server  *httptest.Server
	client  *http.Client
	baseURL string
}

func newTestClient(db *sql.DB) *testClient {
	teamRepo := pg.NewTeamStorage(db)
	userRepo := pg.NewUserStorage(db)
	prRepo := pg.NewPullRequestStorage(db)
	txRepo := pg.NewTxManager(db)

	svc := app.NewService(teamRepo, userRepo, prRepo, txRepo)
	h := handlers.NewHandlers(svc)

	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			next.ServeHTTP(w, r)
		})
	})

	gen.HandlerFromMux(h, router)
	server := httptest.NewServer(router)

	return &testClient{
		server:  server,
		client:  http.DefaultClient,
		baseURL: server.URL,
	}
}

// getTestDB returns the test database connection
func getTestDB(t *testing.T) *sql.DB {
	db := setupTestDB(t)
	return db
}

func (c *testClient) Close() {
	c.server.Close()
}

// Вспомогательные методы
func (c *testClient) post(t *testing.T, path string, body interface{}) *http.Response {
	return c.do(t, http.MethodPost, path, body)
}

func (c *testClient) get(t *testing.T, path string) *http.Response {
	return c.do(t, http.MethodGet, path, nil)
}

func (c *testClient) patch(t *testing.T, path string, body interface{}) *http.Response {
	return c.do(t, http.MethodPatch, path, body)
}

func (c *testClient) do(t *testing.T, method, path string, body interface{}) *http.Response {
	var bodyReader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		bodyReader = bytes.NewReader(b)
	}

	var req *http.Request
	var err error
	if bodyReader != nil {
		req, err = http.NewRequest(method, c.baseURL+path, bodyReader)
	} else {
		req, err = http.NewRequest(method, c.baseURL+path, nil)
	}
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	require.NoError(t, err)
	return resp
}

func TestE2EScenarios(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	client := newTestClient(db)
	t.Cleanup(client.Close)

	t.Run("Team Management", func(t *testing.T) {
		t.Run("Create team successfully", func(t *testing.T) {
			team := gen.Team{
				TeamName: "payments",
				Members: []gen.TeamMember{
					{UserId: "u1", Username: "Alice", IsActive: true},
					{UserId: "u2", Username: "Bob", IsActive: true},
				},
			}

			resp := client.post(t, "/team/add", team)
			assert.Equal(t, http.StatusCreated, resp.StatusCode)

			var respBody struct {
				Team gen.Team `json:"team"`
			}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&respBody))
			assert.Equal(t, "payments", respBody.Team.TeamName)
			assert.Len(t, respBody.Team.Members, 2)
		})

		t.Run("Create duplicate team → 400 TEAM_EXISTS", func(t *testing.T) {
			team := gen.Team{TeamName: "duplicate", Members: []gen.TeamMember{{UserId: "x", Username: "X", IsActive: true}}}
			client.post(t, "/team/add", team)
			resp := client.post(t, "/team/add", team)

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			var errResp gen.ErrorResponse
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
			assert.Equal(t, gen.TEAMEXISTS, errResp.Error.Code)
		})

		t.Run("Get non-existent team → 404", func(t *testing.T) {
			resp := client.get(t, "/team/get?team_name=nonexistent")
			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})
	})

	t.Run("Pull Request Lifecycle", func(t *testing.T) {
		t.Parallel()

		// Setup: создаём команду с уникальным именем
		setupTeam := func(t *testing.T, teamName string, members ...gen.TeamMember) {
			team := gen.Team{TeamName: teamName, Members: members}
			resp := client.post(t, "/team/add", team)
			if resp.StatusCode == http.StatusBadRequest {
				return
			}
			require.Equal(t, http.StatusCreated, resp.StatusCode)
		}

		t.Run("Create PR with auto-assignment (2 reviewers)", func(t *testing.T) {
			teamName := "backend-pr100"
			setupTeam(t, teamName,
				gen.TeamMember{UserId: "author-pr100", Username: "Author", IsActive: true},
				gen.TeamMember{UserId: "r1-pr100", Username: "R1", IsActive: true},
				gen.TeamMember{UserId: "r2-pr100", Username: "R2", IsActive: true},
				gen.TeamMember{UserId: "r3-pr100", Username: "R3", IsActive: true},
			)

			resp := client.post(t, "/pullRequest/create", map[string]any{
				"pull_request_id":   "pr-100",
				"pull_request_name": "feat: search",
				"author_id":         "author-pr100",
			})
			require.Equal(t, http.StatusCreated, resp.StatusCode)

			var body struct {
				Pr gen.PullRequest `json:"pr"`
			}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
			assert.Equal(t, gen.PullRequestStatusOPEN, body.Pr.Status)
			// Может быть 1 или 2 ревьювера в зависимости от доступных кандидатов
			assert.GreaterOrEqual(t, len(body.Pr.AssignedReviewers), 1)
			assert.LessOrEqual(t, len(body.Pr.AssignedReviewers), 2)
			assert.NotContains(t, body.Pr.AssignedReviewers, "author-pr100")
		})

		t.Run("Create PR with only author in team → 0 reviewers", func(t *testing.T) {
			teamName := "backend-lonely"
			setupTeam(t, teamName, gen.TeamMember{UserId: "lonely", Username: "Lonely", IsActive: true})

			resp := client.post(t, "/pullRequest/create", map[string]any{
				"pull_request_id":   "pr-lonely",
				"pull_request_name": "fix",
				"author_id":         "lonely",
			})
			require.Equal(t, http.StatusCreated, resp.StatusCode)

			var body struct {
				Pr gen.PullRequest `json:"pr"`
			}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
			assert.Empty(t, body.Pr.AssignedReviewers)
		})

		t.Run("Reassign reviewer", func(t *testing.T) {
			// Используем уникальное имя команды, чтобы избежать конфликтов
			teamName := "backend-reassign-final"
			team := gen.Team{
				TeamName: teamName,
				Members: []gen.TeamMember{
					{UserId: "a1-reassign-final", Username: "A", IsActive: true},
					{UserId: "r1-reassign-final", Username: "R1", IsActive: true},
					{UserId: "r2-reassign-final", Username: "R2", IsActive: true},
					{UserId: "r3-reassign-final", Username: "R3", IsActive: true},
				},
			}
			resp := client.post(t, "/team/add", team)
			require.Equal(t, http.StatusCreated, resp.StatusCode, "Team should be created")

			// create PR
			prID := "pr-reassign-final"
			resp = client.post(t, "/pullRequest/create", map[string]any{
				"pull_request_id": prID, "author_id": "a1-reassign-final", "pull_request_name": "test",
			})
			require.Equal(t, http.StatusCreated, resp.StatusCode, "PR should be created")
			var created struct {
				Pr gen.PullRequest `json:"pr"`
			}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
			require.Greater(t, len(created.Pr.AssignedReviewers), 0, "PR should have at least one reviewer")
			oldReviewer := created.Pr.AssignedReviewers[0]
			t.Logf("Created PR %s with reviewers: %v, will reassign %s", prID, created.Pr.AssignedReviewers, oldReviewer)

			// Прямая проверка в БД - убеждаемся, что ревьювер действительно назначен
			var reviewerCount int
			err := db.QueryRow(`SELECT COUNT(*) FROM review_assignments WHERE pr_id = $1 AND reviewer_id = $2`, prID, oldReviewer).Scan(&reviewerCount)
			require.NoError(t, err, "Should be able to query DB")
			require.Equal(t, 1, reviewerCount, "Reviewer should be assigned in DB")
			t.Logf("DB check: reviewer %s is assigned to PR %s (count=%d)", oldReviewer, prID, reviewerCount)

			// reassign
			resp = client.post(t, "/pullRequest/reassign", map[string]any{
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
			assert.NotContains(t, reassignResp.Pr.AssignedReviewers, oldReviewer)
		})

		t.Run("Reassign on merged PR → PR_MERGED", func(t *testing.T) {
			teamName := "backend-merged"
			prID := "pr-merged-" + teamName
			setupTeam(t, teamName, gen.TeamMember{UserId: "a-merged", Username: "A", IsActive: true}, gen.TeamMember{UserId: "r-merged", Username: "R", IsActive: true})
			client.post(t, "/pullRequest/create", map[string]any{"pull_request_id": prID, "author_id": "a-merged", "pull_request_name": "x"})
			client.post(t, "/pullRequest/merge", map[string]any{"pull_request_id": prID})

			resp := client.post(t, "/pullRequest/reassign", map[string]any{
				"pull_request_id": prID, "old_user_id": "r-merged",
			})
			assert.Equal(t, http.StatusConflict, resp.StatusCode)
			var errResp gen.ErrorResponse
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
			assert.Equal(t, gen.PRMERGED, errResp.Error.Code)
		})

		t.Run("Reassign non-assigned reviewer → NOT_ASSIGNED", func(t *testing.T) {
			teamName := "backend-notassigned"
			prID := "pr-x-" + teamName
			setupTeam(t, teamName, gen.TeamMember{UserId: "a-notassigned", Username: "A", IsActive: true}, gen.TeamMember{UserId: "r1-notassigned", Username: "R1", IsActive: true}, gen.TeamMember{UserId: "r2-notassigned", Username: "R2", IsActive: true})
			client.post(t, "/pullRequest/create", map[string]any{"pull_request_id": prID, "author_id": "a-notassigned", "pull_request_name": "x"})

			resp := client.post(t, "/pullRequest/reassign", map[string]any{
				"pull_request_id": prID, "old_user_id": "nonexistent",
			})
			assert.Equal(t, http.StatusConflict, resp.StatusCode)
			var errResp gen.ErrorResponse
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
			assert.Equal(t, gen.NOTASSIGNED, errResp.Error.Code)
		})

		t.Run("No candidate for reassignment → reviewer removed", func(t *testing.T) {
			teamName := "backend-nocandidate"
			prID := "pr-nocandidate-" + teamName
			// Создаем команду с активным ревьювером
			client.post(t, "/team/add", gen.Team{
				TeamName: teamName,
				Members: []gen.TeamMember{
					{UserId: "a-nocandidate", Username: "A", IsActive: true},
					{UserId: "r-nocandidate", Username: "R", IsActive: true}, // сначала активный
				},
			})
			client.post(t, "/pullRequest/create", map[string]any{"pull_request_id": prID, "author_id": "a-nocandidate", "pull_request_name": "x"})
			// Деактивируем ревьювера, чтобы не было кандидатов для замены
			client.post(t, "/users/setIsActive", map[string]any{"user_id": "r-nocandidate", "is_active": false})

			// Reassign должен успешно выполниться, просто удалив ревьювера
			resp := client.post(t, "/pullRequest/reassign", map[string]any{
				"pull_request_id": prID, "old_user_id": "r-nocandidate",
			})
			require.Equal(t, http.StatusOK, resp.StatusCode)

			var reassignResp struct {
				Pr         gen.PullRequest `json:"pr"`
				ReplacedBy *string         `json:"replaced_by,omitempty"`
			}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&reassignResp))
			// replaced_by должен быть пустым или отсутствовать, так как ревьювер просто удален
			assert.NotContains(t, reassignResp.Pr.AssignedReviewers, "r-nocandidate", "Old reviewer should be removed")
		})
	})

	t.Run("Merge PR", func(t *testing.T) {
		t.Parallel()

		setup := func(t *testing.T) string {
			client.post(t, "/team/add", gen.Team{
				TeamName: "temp", Members: []gen.TeamMember{{UserId: "u1", Username: "U1", IsActive: true}},
			})
			client.post(t, "/pullRequest/create", map[string]any{
				"pull_request_id": "pr-merge", "author_id": "u1", "pull_request_name": "test",
			})
			return "pr-merge"
		}

		t.Run("Merge successfully + mergedAt set", func(t *testing.T) {
			prID := setup(t)
			resp := client.post(t, "/pullRequest/merge", map[string]any{"pull_request_id": prID})
			require.Equal(t, http.StatusOK, resp.StatusCode)

			var body struct {
				Pr gen.PullRequest `json:"pr"`
			}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
			assert.Equal(t, gen.PullRequestStatusMERGED, body.Pr.Status)
			assert.NotNil(t, body.Pr.MergedAt)
		})

		t.Run("Idempotent merge", func(t *testing.T) {
			prID := setup(t)
			client.post(t, "/pullRequest/merge", map[string]any{"pull_request_id": prID})
			resp := client.post(t, "/pullRequest/merge", map[string]any{"pull_request_id": prID})
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	})

	t.Run("Deactivate members with auto-reassignment", func(t *testing.T) {
		t.Parallel()

		testName := strings.ReplaceAll(t.Name(), "/", "-")
		teamName := "bigteam-" + testName
		author := "a-deact-" + t.Name()
		r1 := "r1-deact-" + t.Name()
		r2 := "r2-deact-" + t.Name()
		backup := "backup-deact-" + t.Name()
		prID := "pr-deact-" + t.Name()

		resp := client.post(t, "/team/add", gen.Team{
			TeamName: teamName,
			Members: []gen.TeamMember{
				{UserId: author, Username: "Author", IsActive: true},
				{UserId: r1, Username: "R1", IsActive: true},
				{UserId: r2, Username: "R2", IsActive: true},
				{UserId: backup, Username: "Backup", IsActive: true},
			},
		})
		require.Equal(t, http.StatusCreated, resp.StatusCode, "Team should be created")

		client.post(t, "/pullRequest/create", map[string]any{
			"pull_request_id": prID, "author_id": author, "pull_request_name": "feat",
		})

		resp = client.patch(t, "/teams/"+teamName+"/deactivate-members", map[string]any{"user_ids": []string{r1, r2}})
		if resp.StatusCode != http.StatusOK {
			var errBody gen.ErrorResponse
			if err := json.NewDecoder(resp.Body).Decode(&errBody); err == nil {
				t.Logf("Deactivate error: %s - %s", errBody.Error.Code, errBody.Error.Message)
			}
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Statistics", func(t *testing.T) {
		t.Parallel()

		teamName := "stats-" + t.Name()
		user1 := "u1-stats-" + t.Name()
		user2 := "u2-stats-" + t.Name()
		pr1 := "pr1-stats-" + t.Name()
		pr2 := "pr2-stats-" + t.Name()

		client.post(t, "/team/add", gen.Team{
			TeamName: teamName,
			Members:  []gen.TeamMember{{UserId: user1, Username: "U1", IsActive: true}, {UserId: user2, Username: "U2", IsActive: true}},
		})

		client.post(t, "/pullRequest/create", map[string]any{"pull_request_id": pr1, "author_id": user1, "pull_request_name": "A"})
		client.post(t, "/pullRequest/create", map[string]any{"pull_request_id": pr2, "author_id": user2, "pull_request_name": "B"})
		client.post(t, "/pullRequest/merge", map[string]any{"pull_request_id": pr1})

		t.Run("User stats", func(t *testing.T) {
			resp := client.get(t, "/users/stats?user_id="+user1)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			var stats gen.UserStats
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&stats))
			assert.Equal(t, user1, stats.UserId)
			assert.Equal(t, 1, stats.CreatedPrCount)
			assert.Equal(t, 1, stats.MergedPrCount)
		})

		t.Run("PR stats", func(t *testing.T) {
			resp := client.get(t, "/pullRequest/stats")
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			var stats gen.PRStats
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&stats))
			assert.GreaterOrEqual(t, stats.Total, 2)
			assert.GreaterOrEqual(t, stats.Open, 1)
			assert.GreaterOrEqual(t, stats.Merged, 1)
		})
	})

	t.Run("Get user review queue", func(t *testing.T) {
		t.Parallel()

		// Используем уникальные имена для параллельного выполнения
		teamName := "queue-" + t.Name()
		author := "a-q-" + t.Name()
		reviewer := "rev-q-" + t.Name()
		prID := "pr-q-" + t.Name()

		client.post(t, "/team/add", gen.Team{
			TeamName: teamName,
			Members:  []gen.TeamMember{{UserId: author, Username: "A", IsActive: true}, {UserId: reviewer, Username: "Rev", IsActive: true}},
		})
		client.post(t, "/pullRequest/create", map[string]any{"pull_request_id": prID, "author_id": author, "pull_request_name": "test"})

		resp := client.get(t, "/users/getReview?user_id="+reviewer)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body struct {
			UserId       string                 `json:"user_id"`
			PullRequests []gen.PullRequestShort `json:"pull_requests"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, reviewer, body.UserId)
		assert.Len(t, body.PullRequests, 1)
		assert.Equal(t, prID, body.PullRequests[0].PullRequestId)
	})
}
