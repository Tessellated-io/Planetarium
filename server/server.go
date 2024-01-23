package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/tessellated-io/pickaxe/arrays"
	"github.com/tessellated-io/pickaxe/log"
)

/** Constants */

const chainsEndpoint = "/chains"

/** Type and constructor */

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

/** Public API */

func (s *Server) Start(port int) error {
	s.logger.Debug().Str("static_content_dir", s.staticContentDirectory).Msg("starting to serve static content")

	// Static content hosting
	fs := http.FileServer(http.Dir(s.staticContentDirectory))
	http.Handle("/", fs)

	// Convenience endpoints
	http.HandleFunc(chainsEndpoint, s.chains)

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

/** Handlers */

func (s *Server) chains(w http.ResponseWriter, req *http.Request) {
	requestPath := "chains/"
	s.logger.Info().Str("method", requestPath).Msg("💻 handling request")

	// Walk through all files/folders in the root, looking for folders
	folders := []string{}
	err := filepath.WalkDir(s.staticContentDirectory, func(path string, directoryEntry fs.DirEntry, err error) error {
		s.logger.Debug().Str("item", directoryEntry.Name()).Msg("examining item")

		if err != nil {
			s.logger.Debug().Str("item", directoryEntry.Name()).Err(err).Msg("error examining item")
			return err
		}

		// Do not process containing directory
		if directoryEntry.IsDir() && path == s.staticContentDirectory {
			s.logger.Debug().Str("item", directoryEntry.Name()).Str("path", path).Msg("skipping processing of chain-registry root directory")
			return nil
		}

		// Skip subdirectories
		if directoryEntry.IsDir() && filepath.Dir(path) != s.staticContentDirectory {
			s.logger.Debug().Str("item", directoryEntry.Name()).Str("path", path).Msg("skipping subdirectory")
			return filepath.SkipDir
		}

		if directoryEntry.IsDir() {
			s.logger.Debug().Str("item", directoryEntry.Name()).Msg("noting item as a directory")
			folders = append(folders, directoryEntry.Name())
		}
		s.logger.Debug().Str("item", directoryEntry.Name()).Msg("item is not a directory")
		return nil
	})
	if err != nil {
		s.logger.Error().Err(err).Str("method", requestPath).Msg("🚨 error traversing directories while handling request")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Filter out folders that are special or metadata
	chains := arrays.Filter(folders, func(input string) bool {
		isHidden := strings.HasPrefix(input, ".")
		isMetadata := strings.HasPrefix(input, "_")

		return !isHidden && !isMetadata
	})

	// Serialize to json
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(chains)
	if err != nil {
		s.logger.Error().Err(err).Str("method", requestPath).Msg("🚨 error serializing json while handling request")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.logger.Info().Str("method", requestPath).Msg("💡 successfully handled request")
}
