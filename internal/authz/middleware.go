package authz

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Middleware creates a Gin middleware for authorization
func Middleware(client *Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract subject from request (in real app, this would come from JWT/auth)
		subject := Subject{
			ID: c.GetHeader("X-User-ID"),
			Attributes: map[string]interface{}{
				"role":            c.GetHeader("X-User-Role"),
				"user_id":         c.GetHeader("X-User-ID"),
				"clearance_level": 2, // Default clearance level
			},
		}

		// Extract resource from request
		resource := Resource{
			ID: c.Param("resource_id"),
			Attributes: map[string]interface{}{
				"owner_id": c.GetHeader("X-Resource-Owner"),
				"type":     c.GetHeader("X-Resource-Type"),
			},
		}

		// Determine action from HTTP method
		action := getActionFromMethod(c.Request.Method)

		// Get policies from PIP (Policy Information Point)
		permissions, prohibitions, conditions := getPolicies(subject, resource, action)

		// Create decision request
		req := DecisionRequest{
			Input: DecisionInput{
				Subject:      subject,
				Resource:     resource,
				Action:       action,
				Permissions:  permissions,
				Prohibitions: prohibitions,
				Conditions:   conditions,
			},
		}

		// Evaluate with OPA
		decision, err := client.Evaluate(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "authorization failed"})
			c.Abort()
			return
		}

		// Check decision
		if decision.Decision != "allow" {
			c.JSON(http.StatusForbidden, gin.H{
				"error":       "access denied",
				"decision":    decision.Decision,
				"obligations": decision.Obligations,
			})
			c.Abort()
			return
		}

		// Store obligations in context for later processing
		c.Set("obligations", decision.Obligations)
		c.Set("conditions", decision.Conditions)

		c.Next()
	}
}

// getActionFromMethod maps HTTP methods to actions
func getActionFromMethod(method string) string {
	switch method {
	case "GET":
		return "read"
	case "POST", "PUT", "PATCH":
		return "write"
	case "DELETE":
		return "delete"
	default:
		return "read"
	}
}

// getPolicies is a simple PIP stub that returns policies
// In a real system, this would query a database or external service
func getPolicies(subject Subject, resource Resource, action string) ([]Rule, []Rule, map[string]bool) {
	// Simple policy examples
	permissions := []Rule{
		{
			Subject:   subject.ID,
			Resource:  resource.ID,
			Action:    action,
			Condition: true,
			Obligation: map[string]interface{}{
				"type":    "log",
				"message": "access granted",
			},
		},
	}

	prohibitions := []Rule{}

	// Add prohibition for sensitive data access
	if resource.Attributes["type"] == "sensitive" {
		prohibitions = append(prohibitions, Rule{
			Subject:   subject.ID,
			Resource:  resource.ID,
			Action:    action,
			Condition: true,
			Obligation: map[string]interface{}{
				"type":    "alert",
				"message": "sensitive data access attempted",
			},
		})
	}

	// Add masking obligation for non-admin users
	if subject.Attributes["role"] != "admin" {
		permissions[0].Obligation = map[string]interface{}{
			"type":   "mask",
			"fields": []string{"ssn", "credit_card", "salary"},
		}
	}

	// Evaluate conditions
	conditions := map[string]bool{
		"is_admin":          subject.Attributes["role"] == "admin",
		"is_owner":          subject.Attributes["user_id"] == resource.Attributes["owner_id"],
		"is_business_hours": isBusinessHours(),
	}

	return permissions, prohibitions, conditions
}

// isBusinessHours is a simple condition predicate
func isBusinessHours() bool {
	// This is a stub - in real implementation, check actual time
	return true
}
