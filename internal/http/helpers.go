package http

import (
	"encoding/json"
	"net/http"

	"github.com/kumarabd/policy-machine/pkg/engine"
	"github.com/kumarabd/policy-machine/internal/postgres"
)

// RespondError writes a JSON error response (exported for use by dataplane/controlplane)
func RespondError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

// respondError is kept for backward compatibility with existing http package code
func respondError(w http.ResponseWriter, status int, code, message string) {
	RespondError(w, status, code, message)
}

// RespondJSON writes a JSON response (exported for use by dataplane/controlplane)
func RespondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// getDBFromEngine gets the DB handler from engine
func getDBFromEngine(eng *engine.Engine) *postgres.Handler {
	return eng.GetDB()
}

