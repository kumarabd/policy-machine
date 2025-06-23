package service

import (
	"github.com/kumarabd/gokit/logger"
	"github.com/kumarabd/policy-machine/internal/metrics"
)

type Config struct {
}

type Handler struct {
	log    *logger.Handler
	config *Config
	metric *metrics.Handler
	store  DataHandler
	cache  DataHandler
}

func New(l *logger.Handler, m *metrics.Handler, sConfig *Config) (*Handler, error) {
	return &Handler{
		log:    l,
		config: sConfig,
		metric: m,
	}, nil
}

func (h *Handler) InitStore(handler DataHandler) {
	h.store = handler
}

func (h *Handler) InitCache(handler DataHandler) {
	h.cache = handler
}

func (h *Handler) GetStore() DataHandler {
	return h.store
}

func (h *Handler) CreateRole(name string, properties map[string]string) *subjectBuild {
	return SubjectBuilder(name, properties)
}
