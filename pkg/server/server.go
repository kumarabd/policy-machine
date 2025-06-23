package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/kumarabd/gokit/logger"
	"github.com/kumarabd/policy-machine/internal/metrics"
	"github.com/kumarabd/policy-machine/pkg/service"
	"github.com/kumarabd/policy-machine/docs"
)

type Config struct {
	Base *BaseServerConfig `json:"base" yaml:"base"`
}

type Handler struct {
	BaseServer *BaseServer
	config     *Config
	log        *logger.Handler
}

// formatTitle converts "abc-def" format to "Abc Def" format
// Examples: "policy-machine" -> "Policy Machine", "auth-service" -> "Auth Service"
func formatTitle(name string) string {
	parts := strings.Split(name, "-")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(string(part[0])) + strings.ToLower(part[1:])
		}
	}
	return strings.Join(parts, " ")
}

func New(name string, l *logger.Handler, m *metrics.Handler, config *Config, service *service.Handler) (*Handler, error) {
	// Update Swagger info with actual config values
	docs.SwaggerInfo.Host = fmt.Sprintf("localhost:%d", config.Base.Port)
	docs.SwaggerInfo.Title = fmt.Sprintf("%s API", formatTitle(name))
	
	l.Info().Msgf("Swagger UI will be available at: http://localhost:%d/swagger/index.html", config.Base.Port)
	
	// Initiate Base Server object
	httpObj := &BaseServer{
		log:     l,
		service: service,
		metric:  m,
	}
	httpObj.handler = chi.NewRouter()
	// Register all routes
	httpObj.RegisterRoutes()

	return &Handler{
		BaseServer: httpObj,
		config:     config,
		log:        l,
	}, nil
}

func (h *Handler) Start(ch chan struct{}) {
	// Start the Base server
	go func() {
		h.log.Info().Msgf("started http server on port: %d", h.config.Base.Port)
		err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", h.config.Base.Port), h.BaseServer.handler)
		h.log.Error().Err(err).Msg("server stopped")
		ch <- struct{}{}
	}()

	//// Start the GRPC server
	//go func() {
	//	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%s", h.config.GRPC.Port))
	//	if err != nil {
	//		h.log.Error().Err(err).Msg("server stopped")
	//	}

	//	h.log.Info().Msgf("started grpc server on port: %s", h.config.GRPC.Port)
	//	err = h.GRPCServer.handler.Serve(listener)
	//	if err != nil {
	//		h.log.Error().Err(err).Msg("server stopped")
	//	}
	//	ch <- struct{}{}
	//}()
}
