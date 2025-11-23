package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mark47B/be-internship/internal/domain/entity"
	"github.com/mark47B/be-internship/internal/domain/usecase"
	"github.com/mark47B/be-internship/internal/infra/transport/rest/gen"
)

// GET /team/get
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

// POST /team/add
func (h *Handlers) PostTeamAdd(w http.ResponseWriter, r *http.Request) {
	var req gen.PostTeamAddJSONRequestBody
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

	// Map gen.Team to entity.Team
	teamEntity := entity.Team{
		Name:    req.TeamName,
		Members: make([]entity.User, 0, len(req.Members)),
	}

	for _, m := range req.Members {
		teamEntity.Members = append(teamEntity.Members, entity.User{
			ID:       m.UserId,
			TeamName: req.TeamName,
			Username: m.Username,
			IsActive: m.IsActive,
		})
	}

	createdTeam, err := h.service.AddOrUpdateTeam(r.Context(), teamEntity)
	if err != nil {
		if err == usecase.ErrTeamExists {
			WriteError(w, http.StatusBadRequest, gen.ErrorResponse{
				Error: struct {
					Code    gen.ErrorResponseErrorCode `json:"code"`
					Message string                     `json:"message"`
				}{
					Code:    gen.TEAMEXISTS,
					Message: err.Error(),
				},
			})
			return
		}
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

	// Map back to gen.Team
	teamResp := gen.Team{
		TeamName: createdTeam.Name,
		Members:  make([]gen.TeamMember, 0, len(createdTeam.Members)),
	}

	for _, m := range createdTeam.Members {
		teamResp.Members = append(teamResp.Members, gen.TeamMember{
			UserId:   m.ID,
			Username: m.Username,
			IsActive: m.IsActive,
		})
	}

	resp := map[string]interface{}{
		"team": teamResp,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}
