package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mark47B/be-internship/internal/domain/usecase"
	"github.com/mark47B/be-internship/internal/infra/transport/rest/gen"
)

// POST /pullRequest/create
func (h *Handlers) PostPullRequestCreate(w http.ResponseWriter, r *http.Request) {
	var req gen.PostPullRequestCreateJSONRequestBody
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

	pr, err := h.service.CreatePR(r.Context(), req.PullRequestId, req.PullRequestName, req.AuthorId)
	if err != nil {
		if errors.Is(err, usecase.ErrUserNotFound) {
			WriteError(w, http.StatusNotFound, gen.ErrorResponse{
				Error: struct {
					Code    gen.ErrorResponseErrorCode `json:"code"`
					Message string                     `json:"message"`
				}{
					Code:    gen.NOTFOUND,
					Message: "author or team not found",
				},
			})
			return
		}
		if errors.Is(err, usecase.ErrPRExists) {
			WriteError(w, http.StatusConflict, gen.ErrorResponse{
				Error: struct {
					Code    gen.ErrorResponseErrorCode `json:"code"`
					Message string                     `json:"message"`
				}{
					Code:    gen.PREXISTS,
					Message: "PR id already exists",
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

	resp := gen.PullRequest{
		PullRequestId:     pr.ID,
		PullRequestName:   pr.Name,
		AuthorId:          pr.AuthorID,
		Status:            gen.PullRequestStatus(pr.Status),
		AssignedReviewers: pr.Reviewers,
		CreatedAt:         pr.CreatedAt,
		MergedAt:          pr.MergedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"pr": resp,
	})
}

// POST /pullRequest/merge
func (h *Handlers) PostPullRequestMerge(w http.ResponseWriter, r *http.Request) {
	var req gen.PostPullRequestMergeJSONRequestBody
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

	pr, err := h.service.MergePR(r.Context(), req.PullRequestId)
	if err != nil {
		if errors.Is(err, usecase.ErrPRNotFound) {
			WriteError(w, http.StatusNotFound, gen.ErrorResponse{
				Error: struct {
					Code    gen.ErrorResponseErrorCode `json:"code"`
					Message string                     `json:"message"`
				}{
					Code:    gen.NOTFOUND,
					Message: "pull request not found",
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

	resp := gen.PullRequest{
		PullRequestId:     pr.ID,
		PullRequestName:   pr.Name,
		AuthorId:          pr.AuthorID,
		Status:            gen.PullRequestStatus(pr.Status),
		AssignedReviewers: pr.Reviewers,
		CreatedAt:         pr.CreatedAt,
		MergedAt:          pr.MergedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"pr": resp,
	})
}

// POST /pullRequest/reassign
func (h *Handlers) PostPullRequestReassign(w http.ResponseWriter, r *http.Request) {
	var req gen.PostPullRequestReassignJSONRequestBody
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

	pr, replacedBy, err := h.service.ReassignReviewer(r.Context(), req.PullRequestId, req.OldUserId)
	if err != nil {
		if errors.Is(err, usecase.ErrPRNotFound) {
			WriteError(w, http.StatusNotFound, gen.ErrorResponse{
				Error: struct {
					Code    gen.ErrorResponseErrorCode `json:"code"`
					Message string                     `json:"message"`
				}{
					Code:    gen.NOTFOUND,
					Message: "pull request or user not found",
				},
			})
			return
		}
		if errors.Is(err, usecase.ErrAlreadyMerged) {
			WriteError(w, http.StatusConflict, gen.ErrorResponse{
				Error: struct {
					Code    gen.ErrorResponseErrorCode `json:"code"`
					Message string                     `json:"message"`
				}{
					Code:    gen.PRMERGED,
					Message: "cannot reassign on merged PR",
				},
			})
			return
		}
		if errors.Is(err, usecase.ErrNotReviewer) {
			WriteError(w, http.StatusConflict, gen.ErrorResponse{
				Error: struct {
					Code    gen.ErrorResponseErrorCode `json:"code"`
					Message string                     `json:"message"`
				}{
					Code:    gen.NOTASSIGNED,
					Message: "reviewer is not assigned to this PR",
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

	resp := gen.PullRequest{
		PullRequestId:     pr.ID,
		PullRequestName:   pr.Name,
		AuthorId:          pr.AuthorID,
		Status:            gen.PullRequestStatus(pr.Status),
		AssignedReviewers: pr.Reviewers,
		CreatedAt:         pr.CreatedAt,
		MergedAt:          pr.MergedAt,
	}

	responseBody := map[string]interface{}{
		"pr": resp,
	}
	// replaced_by может быть пустой строкой, если ревьювер просто удален
	if replacedBy != "" {
		responseBody["replaced_by"] = replacedBy
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(responseBody)
}

// GET /pullRequest/stats
func (h *Handlers) GetPullRequestStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.GetPRStats(r.Context())
	if err != nil {
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

	var avgReviewers *float32
	if stats.AvgReviewers > 0 {
		avg := float32(stats.AvgReviewers)
		avgReviewers = &avg
	}

	resp := gen.PRStats{
		Total:        stats.Total,
		Open:         stats.Open,
		Merged:       stats.Merged,
		AvgReviewers: avgReviewers,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
