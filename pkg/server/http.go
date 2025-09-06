package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kumarabd/policy-machine/internal/authz"
	"github.com/kumarabd/policy-machine/pkg/service"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type HTTPServerConfig struct {
	Port string `json:"port" yaml:"port"`
}
type HTTPServer struct {
	handler     *gin.Engine
	service     *service.Handler
	authzClient *authz.Client
}

func (h *HTTPServer) MetricsHandler(c *gin.Context) {
	promhttp.Handler().ServeHTTP(c.Writer, c.Request)
}

func (h *HTTPServer) HealthHandler(c *gin.Context) {
	c.JSON(200, http.StatusText(http.StatusOK))
}

// UserDataHandler demonstrates masking obligation
func (h *HTTPServer) UserDataHandler(c *gin.Context) {
	// Sample user data
	userData := map[string]interface{}{
		"id":          "user123",
		"name":        "John Doe",
		"email":       "john@example.com",
		"ssn":         "123-45-6789",
		"credit_card": "4111-1111-1111-1111",
		"salary":      75000,
		"department":  "Engineering",
	}

	// Get obligations from authorization middleware
	obligations, exists := c.Get("obligations")
	if !exists {
		c.JSON(http.StatusOK, userData)
		return
	}

	// Apply masking obligations
	maskedData := applyMaskingObligations(userData, obligations.([]map[string]interface{}))

	c.JSON(http.StatusOK, gin.H{
		"data":        maskedData,
		"obligations": obligations,
	})
}

// applyMaskingObligations applies masking based on obligations
func applyMaskingObligations(data map[string]interface{}, obligations []map[string]interface{}) map[string]interface{} {
	maskedData := make(map[string]interface{})

	// Copy original data
	for k, v := range data {
		maskedData[k] = v
	}

	// Apply masking obligations
	for _, obligation := range obligations {
		if obligation["type"] == "mask" {
			if fields, ok := obligation["fields"].([]interface{}); ok {
				for _, field := range fields {
					if fieldStr, ok := field.(string); ok {
						if _, exists := maskedData[fieldStr]; exists {
							maskedData[fieldStr] = "***MASKED***"
						}
					}
				}
			}
		}
	}

	return maskedData
}
