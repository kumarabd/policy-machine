package server

import (
	"encoding/json"
	"net/http"

	"github.com/swaggo/swag"
)

func (h *BaseServer) MetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("# Metrics endpoint placeholder\n"))
}

// HealthHandler returns the health status of the service
// @Summary Health check
// @Description Get the health status of the service
// @Tags health
// @Produce json
// @Success 200 {object} object{status=string} "Service is healthy"
// @Router /healthz [get]
func (h *BaseServer) HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// SwaggerJSONHandler serves the swagger.json file
func (h *BaseServer) SwaggerJSONHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Get the swagger docs from the swag registry
	if spec := swag.GetSwagger("swagger"); spec != nil {
		doc := spec.ReadDoc()
		w.Write([]byte(doc))
	} else {
		http.Error(w, "Swagger docs not found", http.StatusNotFound)
	}
}
