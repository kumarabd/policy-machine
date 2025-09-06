package service

import (
	"github.com/kumarabd/gokit/logger"
	"github.com/kumarabd/policy-machine/internal/metrics"
)

type Config struct {
}

type Handler struct {
	log       *logger.Handler
	config    *Config
	datalayer DataLayer
	metric    *metrics.Handler
}

func New(l *logger.Handler, m *metrics.Handler, datalayer DataLayer, sConfig *Config) (*Handler, error) {
	return &Handler{
		log:       l,
		config:    sConfig,
		datalayer: datalayer,
		metric:    m,
	}, nil
}
