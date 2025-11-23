package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mark47B/be-internship/internal/domain/usecase"
	"github.com/mark47B/be-internship/internal/infra/transport/rest/gen"
)

// POST /users/setIsActive
func (h *Handlers) PostUsersSetIsActive(w http.ResponseWriter, r *http.Request) {
	var req gen.PostUsersSetIsActiveJSONRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, gen.ErrorResponse{
			Error: struct {
				Code    gen.ErrorResponseErrorCode `json:"code"`
				Message string                     `json:"message"`
			}{
				Code:    gen.NOTFOUND,
				Message: "invalid json body",
			},
		})
		return
	}

	user, err := h.service.SetUserActive(r.Context(), req.UserId, req.IsActive)
	if err != nil {
		if err == sql.ErrNoRows || errors.Is(err, usecase.ErrUserNotFound) {
			WriteError(w, http.StatusNotFound, gen.ErrorResponse{
				Error: struct {
					Code    gen.ErrorResponseErrorCode `json:"code"`
					Message string                     `json:"message"`
				}{
					Code:    gen.NOTFOUND,
					Message: "user not found",
				},
			})
			return
		}
		WriteError(w, http.StatusInternalServerError, gen.ErrorResponse{
			Error: struct {
				Code    gen.ErrorResponseErrorCode `json:"code"`
				Message string                     `json:"message"`
			}{
				Code:    gen.NOTFOUND,
				Message: err.Error(),
			},
		})
		return
	}

	resp := gen.User{
		UserId:   user.ID,
		Username: user.Username,
		TeamName: user.TeamName,
		IsActive: user.IsActive,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"user": resp,
	})
}

// GET /users/getReview
func (h *Handlers) GetUsersGetReview(w http.ResponseWriter, r *http.Request, params gen.GetUsersGetReviewParams) {
	prs, err := h.service.GetUserReviewPRs(r.Context(), params.UserId)
	if err != nil {
		if errors.Is(err, usecase.ErrUserNotFound) {
			WriteError(w, http.StatusNotFound, gen.ErrorResponse{
				Error: struct {
					Code    gen.ErrorResponseErrorCode `json:"code"`
					Message string                     `json:"message"`
				}{
					Code:    gen.NOTFOUND,
					Message: "user not found",
				},
			})
			return
		}
		WriteError(w, http.StatusInternalServerError, gen.ErrorResponse{
			Error: struct {
				Code    gen.ErrorResponseErrorCode `json:"code"`
				Message string                     `json:"message"`
			}{
				Code:    gen.NOTFOUND,
				Message: err.Error(),
			},
		})
		return
	}

	prShorts := make([]gen.PullRequestShort, 0, len(prs))
	for _, pr := range prs {
		prShorts = append(prShorts, gen.PullRequestShort{
			PullRequestId:   pr.ID,
			PullRequestName: pr.Name,
			AuthorId:         pr.AuthorID,
			Status:           gen.PullRequestShortStatus(pr.Status),
		})
	}

	resp := map[string]interface{}{
		"user_id":       params.UserId,
		"pull_requests": prShorts,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// GET /users/stats
func (h *Handlers) GetUsersStats(w http.ResponseWriter, r *http.Request, params gen.GetUsersStatsParams) {
	stats, err := h.service.GetUserStats(r.Context(), params.UserId)
	if err != nil {
		if errors.Is(err, usecase.ErrUserNotFound) {
			WriteError(w, http.StatusNotFound, gen.ErrorResponse{
				Error: struct {
					Code    gen.ErrorResponseErrorCode `json:"code"`
					Message string                     `json:"message"`
				}{
					Code:    gen.NOTFOUND,
					Message: "user not found",
				},
			})
			return
		}
		WriteError(w, http.StatusInternalServerError, gen.ErrorResponse{
			Error: struct {
				Code    gen.ErrorResponseErrorCode `json:"code"`
				Message string                     `json:"message"`
			}{
				Code:    gen.NOTFOUND,
				Message: err.Error(),
			},
		})
		return
	}

	resp := gen.UserStats{
		UserId:          stats.UserID,
		CreatedPrCount:  stats.CreatedPRCount,
		ReviewedPrCount: stats.ReviewedPRCount,
		MergedPrCount:   stats.MergedPRCount,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
