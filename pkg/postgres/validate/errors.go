package validate

// ValidationError represents a validation failure
type ValidationError struct {
	Code    string
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

// IsValidationError checks if an error is a ValidationError and returns it
func IsValidationError(err error) (*ValidationError, bool) {
	if err == nil {
		return nil, false
	}
	ve, ok := err.(ValidationError)
	if !ok {
		return nil, false
	}
	return &ve, true
}

// Validation error codes
const (
	CodeInvalidEdgeType = "INVALID_EDGE_TYPE"
	CodeCycleDetected   = "CYCLE_DETECTED"
	CodeMissingNode      = "MISSING_NODE"
	CodeDuplicateEdge    = "DUPLICATE_EDGE"
)

