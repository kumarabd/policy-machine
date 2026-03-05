package http

import (
	"encoding/json"
	"net/http"

	"github.com/kumarabd/policy-machine/internal/postgres"
	"github.com/kumarabd/policy-machine/pkg/engine"
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

// HandleDBError is a small helper to translate common DB errors into HTTP responses.
func HandleDBError(w http.ResponseWriter, err error, resource string) {
	if err == postgres.ErrNotFound {
		RespondError(w, http.StatusNotFound, "NOT_FOUND", resource+" not found")
		return
	}
	RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
}

