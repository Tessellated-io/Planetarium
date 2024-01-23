package server

import (
	"fmt"
	"net/http"

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

func (s *Server) Start(port uint32) error {
	s.logger.Debug().Str("static_content_dir", s.staticContentDirectory).Msg("starting to serve static content")

	fs := http.FileServer(http.Dir(s.staticContentDirectory))
	http.Handle("/", fs)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		return err
	}
	s.logger.Info().Msg("🔌 Planetarium server terminated")
	return nil
}
