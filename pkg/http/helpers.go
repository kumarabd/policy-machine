package http

import (
	"encoding/json"
	"net/http"

	"github.com/kumarabd/policy-machine/pkg/engine"
	"github.com/kumarabd/policy-machine/pkg/postgres"
)

// respondError writes a JSON error response
func respondError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

// getDBFromEngine gets the DB handler from engine
func getDBFromEngine(eng *engine.Engine) *postgres.Handler {
	return eng.GetDB()
}

