package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/tessellated-io/pickaxe/log"
)

type Server struct {
	logger                 *log.Logger
	staticContentDirectory string
}

func NewServer(staticContentDirectory string, logger *log.Logger) (*Server, error) {
	return &Server{
		staticContentDirectory: staticContentDirectory,
		logger:                 logger,
	}, nil
}

func (s *Server) Start(port int) error {
	s.logger.Debug().Str("static_content_dir", s.staticContentDirectory).Msg("starting to serve static content")

	fs := http.FileServer(http.Dir(s.staticContentDirectory))
	http.Handle("/", fs)

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		ReadHeaderTimeout: 3 * time.Second,
	}

	err := server.ListenAndServe()
	s.logger.Info().Msg("🔌 Planetarium server terminated")
	if err != nil {
		return err
	}

	return nil
}
