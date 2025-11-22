package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mark47B/be-internship/internal/domain/usecase"
	"github.com/mark47B/be-internship/internal/infra/transport/rest/gen"
)

func (h *Handlers) GetTeamGet(w http.ResponseWriter, r *http.Request, params gen.GetTeamGetParams) {
	team, err := h.service.GetTeam(r.Context(), params.TeamName)
	if err != nil {

		if err == sql.ErrNoRows || errors.Is(err, usecase.ErrTeamNotFound) {
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

	resp := gen.Team{
		TeamName: team.Name,
		Members:  make([]gen.TeamMember, len(team.Members)),
	}

	for i, m := range team.Members {
		resp.Members[i] = gen.TeamMember{
			UserId:   m.ID,
			Username: m.Username,
			IsActive: m.IsActive,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)

}
