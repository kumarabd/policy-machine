package authz

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client handles communication with OPA
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new OPA client
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// DecisionRequest represents the input to OPA
type DecisionRequest struct {
	Input DecisionInput `json:"input"`
}

// DecisionInput contains the authorization context
type DecisionInput struct {
	Subject      Subject         `json:"subject"`
	Resource     Resource        `json:"resource"`
	Action       string          `json:"action"`
	Permissions  []Rule          `json:"permissions"`
	Prohibitions []Rule          `json:"prohibitions"`
	Conditions   map[string]bool `json:"conditions"`
}

// Subject represents the requesting entity
type Subject struct {
	ID         string                 `json:"id"`
	Attributes map[string]interface{} `json:"attributes"`
}

// Resource represents the target resource
type Resource struct {
	ID         string                 `json:"id"`
	Attributes map[string]interface{} `json:"attributes"`
}

// Rule represents a permission or prohibition
type Rule struct {
	Subject    string                 `json:"subject"`
	Resource   string                 `json:"resource"`
	Action     string                 `json:"action"`
	Condition  bool                   `json:"condition"`
	Obligation map[string]interface{} `json:"obligation"`
}

// DecisionResponse represents OPA's response
type DecisionResponse struct {
	Result DecisionResult `json:"result"`
}

// DecisionResult contains the authorization decision
type DecisionResult struct {
	Decision    string                   `json:"decision"`
	Obligations []map[string]interface{} `json:"obligations"`
	Conditions  map[string]bool          `json:"conditions"`
}

// Evaluate makes an authorization decision via OPA
func (c *Client) Evaluate(ctx context.Context, req DecisionRequest) (*DecisionResult, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/data/authz/result", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OPA returned status %d", resp.StatusCode)
	}

	var decisionResp DecisionResponse
	if err := json.NewDecoder(resp.Body).Decode(&decisionResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &decisionResp.Result, nil
}
