package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/mark47B/be-internship/internal/infra/transport/rest/gen"
)

func WriteError(w http.ResponseWriter, code int, err gen.ErrorResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(err)
}
