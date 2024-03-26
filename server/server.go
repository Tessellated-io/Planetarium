package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tessellated-io/pickaxe/arrays"
	"github.com/tessellated-io/pickaxe/log"
)

/** Constants */

const apiVersion = "v1"

const (
	chainsNamespace     = "chains"
	validatorsNamespace = "validators"
)

const allChainsEndpoint = "all"

const (
	healthyStatus   = "HEALTHY"
	unhealthyStatus = "UNHEALTHY"
)

/** Type and constructor */

type Server struct {
	logger                     *log.Logger
	chainRegistryDirectory     string
	validatorRegistryDirectory string
}

func NewServer(chainRegistryDirectory, validatorRegistryDirectory string, logger *log.Logger) (*Server, error) {
	// Remove trailing / on inputs, which mess up the all chains helper
	normalizedChainRegistryDirectory := chainRegistryDirectory
	if strings.HasSuffix(chainRegistryDirectory, "/") {
		normalizedChainRegistryDirectory = filepath.Dir(chainRegistryDirectory)
	}
	normalizedValidatorRegistryDirectory := validatorRegistryDirectory
	if strings.HasSuffix(validatorRegistryDirectory, "/") {
		normalizedValidatorRegistryDirectory = filepath.Dir(validatorRegistryDirectory)
	}

	return &Server{
		chainRegistryDirectory:     normalizedChainRegistryDirectory,
		validatorRegistryDirectory: normalizedValidatorRegistryDirectory,
		logger:                     logger,
	}, nil
}

/** Public API */

func (s *Server) Start(port int) error {
	s.logger.Debug().Str("chain_registry_directory", s.chainRegistryDirectory).Str("validator_registry_directory", s.validatorRegistryDirectory).Int("port", port).Msg("starting to serve static content")

	versionedChainsNamespace := fmt.Sprintf("/%s/%s", apiVersion, chainsNamespace)
	versionedValidatorsNamespace := fmt.Sprintf("/%s/%s", apiVersion, validatorsNamespace)

	// Static content hosting for chains and validator registry
	chainRegistryFileServer := http.FileServer(http.Dir(s.chainRegistryDirectory))
	http.Handle(versionedChainsNamespace, http.StripPrefix(versionedChainsNamespace, chainRegistryFileServer))
	http.Handle(fmt.Sprintf("%s/", versionedChainsNamespace), http.StripPrefix(fmt.Sprintf("%s/", versionedChainsNamespace), chainRegistryFileServer))
	s.logger.Debug().Str("endpoint", versionedChainsNamespace).Msg("hosting chain registry")

	validatorRegistryFileServer := http.FileServer(http.Dir(s.validatorRegistryDirectory))
	http.Handle(versionedValidatorsNamespace, http.StripPrefix(versionedValidatorsNamespace, validatorRegistryFileServer))
	http.Handle(fmt.Sprintf("%s/", versionedValidatorsNamespace), http.StripPrefix(fmt.Sprintf("%s/", versionedValidatorsNamespace), validatorRegistryFileServer))
	s.logger.Debug().Str("endpoint", versionedValidatorsNamespace).Msg("hosting validator registry")

	// Convenience endpoints
	versionedAllChainsEndpoint := fmt.Sprintf("/%s/%s/%s", apiVersion, chainsNamespace, allChainsEndpoint)
	http.HandleFunc(versionedAllChainsEndpoint, s.allChains)
	http.HandleFunc(fmt.Sprintf("%s/", versionedAllChainsEndpoint), s.allChains)
	s.logger.Debug().Str("endpoint", versionedAllChainsEndpoint).Msg("hosting all chains helper")

	// Health endpoint
	versionedHealthEndpoint := fmt.Sprintf("/%s/health", apiVersion)
	http.HandleFunc(versionedHealthEndpoint, s.health)
	http.HandleFunc(fmt.Sprintf("%s/", versionedHealthEndpoint), s.health)
	s.logger.Debug().Str("endpoint", versionedHealthEndpoint).Msg("hosting health helper")

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

func (s *Server) allChains(w http.ResponseWriter, req *http.Request) {
	requestMethod := "all_chains"
	s.logger.Info().Str("method", requestMethod).Msg("💻 handling request")

	// Walk through all files/folders in the root, looking for folders
	folders := []string{}
	err := filepath.WalkDir(s.chainRegistryDirectory, func(path string, directoryEntry fs.DirEntry, err error) error {
		s.logger.Debug().Str("item", directoryEntry.Name()).Msg("examining item")

		if err != nil {
			s.logger.Debug().Str("item", directoryEntry.Name()).Err(err).Msg("error examining item")
			return err
		}

		// Do not process containing directory
		if directoryEntry.IsDir() && path == s.chainRegistryDirectory {
			s.logger.Debug().Str("item", directoryEntry.Name()).Str("path", path).Msg("skipping processing of chain-registry root directory")
			return nil
		}

		// Skip subdirectories
		if directoryEntry.IsDir() && filepath.Dir(path) != s.chainRegistryDirectory {
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
		s.logger.Error().Err(err).Str("method", requestMethod).Msg("🚨 error traversing directories while handling request")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Filter out folders that are special or metadata
	chains := arrays.Filter(folders, func(input string) bool {
		isHidden := strings.HasPrefix(input, ".")
		isMetadata := strings.HasPrefix(input, "_")
		isTestnet := strings.EqualFold("testnets", input)

		return !isHidden && !isMetadata && !isTestnet
	})

	// Serialize to json
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(chains)
	if err != nil {
		s.logger.Error().Err(err).Str("method", requestMethod).Msg("🚨 error serializing json while handling request")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.logger.Info().Str("method", requestMethod).Msg("💡 successfully handled request")
}

func (s *Server) health(w http.ResponseWriter, req *http.Request) {
	requestMethod := "health"
	s.logger.Info().Str("method", requestMethod).Msg("💻 handling request")

	healthStatus := healthyStatus

	// Determine age of git commits
	chainAgeCmd := exec.Command("git", "log", "-1", "--format=%ct")
	chainAgeCmd.Dir = s.chainRegistryDirectory
	chainRegistryAge, err := chainAgeCmd.Output()
	if err != nil {
		s.logger.Error().Err(err).Msg("error retrieving git commit age for chain registry")
		chainRegistryAge = []byte("")
		healthStatus = unhealthyStatus
	}

	validatorAge := exec.Command("git", "log", "-1", "--format=%ct")
	validatorAge.Dir = s.validatorRegistryDirectory
	validatorRegistryAge, err := validatorAge.Output()
	if err != nil {
		s.logger.Error().Err(err).Msg("error retrieving git commit age for validator registry")
		validatorRegistryAge = []byte("")
		healthStatus = unhealthyStatus
	}

	// Determine git commit
	chainCommitCmd := exec.Command("git", "rev-parse", "HEAD")
	chainCommitCmd.Dir = s.chainRegistryDirectory
	chainRegistryCommit, err := chainCommitCmd.Output()
	if err != nil {
		s.logger.Error().Err(err).Msg("error retrieving git commit for chain registry")
		chainRegistryCommit = []byte("")
		healthStatus = unhealthyStatus
	}

	validatorCommitCmd := exec.Command("git", "rev-parse", "HEAD")
	validatorCommitCmd.Dir = s.validatorRegistryDirectory
	validatorRegistryCommit, err := validatorCommitCmd.Output()
	if err != nil {
		s.logger.Error().Err(err).Msg("error retrieving git commit for validator registry")
		validatorRegistryCommit = []byte("")
		healthStatus = unhealthyStatus
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")

	if healthStatus == healthyStatus {
		w.WriteHeader(http.StatusOK) // http.StatusOK equals 200
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}

	response := map[string]string{
		"status":                    healthStatus,
		"chain_registry_commit":     strings.TrimSpace(string(chainRegistryCommit)),
		"validator_registry_commit": strings.TrimSpace(string(validatorRegistryCommit)),
		"chain_registry_age":        strings.TrimSpace(string(chainRegistryAge)),
		"validator_registry_age":    strings.TrimSpace(string(validatorRegistryAge)),
	}

	// Serialize to json
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		s.logger.Error().Err(err).Str("method", requestMethod).Msg("🚨 error serializing json while handling request")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.logger.Info().Str("method", requestMethod).Msg("💡 successfully handled request")
}
