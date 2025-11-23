package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mark47B/be-internship/internal/domain/usecase"
	"github.com/mark47B/be-internship/internal/infra/transport/rest/gen"
)

// PATCH /teams/{teamName}/deactivate-members
func (h *Handlers) PatchTeamsTeamNameDeactivateMembers(w http.ResponseWriter, r *http.Request, teamName string) {
	var req gen.PatchTeamsTeamNameDeactivateMembersJSONRequestBody
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

	err := h.service.DeactivateUsersAndReassign(r.Context(), teamName, req.UserIds)
	if err != nil {
		if errors.Is(err, usecase.ErrTeamNotFound) {
			WriteError(w, http.StatusNotFound, gen.ErrorResponse{
				Error: struct {
					Code    gen.ErrorResponseErrorCode `json:"code"`
					Message string                     `json:"message"`
				}{
					Code:    gen.NOTFOUND,
					Message: "team not found",
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

	resp := map[string]string{
		"message": "Users deactivated and PRs reassigned",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
