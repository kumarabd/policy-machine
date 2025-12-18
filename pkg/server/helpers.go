package server

import (
	"github.com/kumarabd/policy-machine/pkg/engine"
	"github.com/kumarabd/policy-machine/pkg/postgres"
)

// getDBFromEngine gets the DB handler from engine
func getDBFromEngine(eng *engine.Engine) *postgres.Handler {
	return eng.GetDB()
}

