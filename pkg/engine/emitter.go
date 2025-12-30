package engine

import (
	"context"

	"github.com/google/uuid"
)

type DecisionEvent struct {
	TenantID  string
	UserID    uuid.UUID
	ObjectID  uuid.UUID
	Operation string
	Allowed   bool
	Revision  int64
	TimeUnix  int64
}

type ObligationEmitter interface {
	PublishDecision(ctx context.Context, ev DecisionEvent) error
}
