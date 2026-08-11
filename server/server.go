package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
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
	chainByIDEndpoint = "id"
	chainFileName     = "chain.json"
)

const (
	healthyStatus   = "HEALTHY"
	unhealthyStatus = "UNHEALTHY"
)

/** Type and constructor */

type Server struct {
	logger                     *log.Logger
	chainRegistryDirectory     string
	validatorRegistryDirectory string

	// Maps chain ids (ex. "cosmoshub-4") to chain registry directory names (ex. "cosmoshub")
	chainIDCache map[string]string
}

// chainInfo models the fields of a chain registry chain.json that the server reads.
type chainInfo struct {
	ChainID string `json:"chain_id"`
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

	// Registry data only changes across server restarts, so the cache is built once and never invalidated
	chainIDCache, err := buildChainIDCache(normalizedChainRegistryDirectory, logger)
	if err != nil {
		return nil, err
	}

	return &Server{
		chainRegistryDirectory:     normalizedChainRegistryDirectory,
		validatorRegistryDirectory: normalizedValidatorRegistryDirectory,
		chainIDCache:               chainIDCache,
		logger:                     logger,
	}, nil
}

// buildChainIDCache maps chain ids to chain registry directory names. Testnet chains are excluded, matching
// the behavior of the all chains endpoint.
func buildChainIDCache(chainRegistryDirectory string, logger *log.Logger) (map[string]string, error) {
	cache := make(map[string]string)

	directoryEntries, err := os.ReadDir(chainRegistryDirectory)
	if err != nil {
		return nil, err
	}

	for _, directoryEntry := range directoryEntries {
		if !directoryEntry.IsDir() {
			continue
		}

		chainName := directoryEntry.Name()
		isHidden := strings.HasPrefix(chainName, ".")
		isMetadata := strings.HasPrefix(chainName, "_")
		isTestnet := strings.EqualFold("testnets", chainName)
		if isHidden || isMetadata || isTestnet {
			continue
		}

		chainFile := filepath.Join(chainRegistryDirectory, chainName, chainFileName)
		chainFileContents, err := os.ReadFile(chainFile)
		if err != nil {
			logger.Warn().Str("chain", chainName).Err(err).Msg("excluding chain with unreadable chain.json from chain id cache")
			continue
		}

		chainData := &chainInfo{}
		err = json.Unmarshal(chainFileContents, chainData)
		if err != nil {
			logger.Warn().Str("chain", chainName).Err(err).Msg("excluding chain with malformed chain.json from chain id cache")
			continue
		}

		if chainData.ChainID == "" {
			logger.Warn().Str("chain", chainName).Msg("excluding chain with missing chain_id from chain id cache")
			continue
		}

		cache[chainData.ChainID] = chainName
	}

	logger.Debug().Int("num_chains", len(cache)).Msg("built chain id cache")
	return cache, nil
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

	versionedChainByIDEndpoint := fmt.Sprintf("/%s/%s/%s", apiVersion, chainsNamespace, chainByIDEndpoint)
	http.HandleFunc(versionedChainByIDEndpoint, s.chainByID)
	http.HandleFunc(fmt.Sprintf("%s/", versionedChainByIDEndpoint), s.chainByID)
	s.logger.Debug().Str("endpoint", versionedChainByIDEndpoint).Msg("hosting chain by id helper")

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

func (s *Server) chainByID(w http.ResponseWriter, req *http.Request) {
	requestMethod := "chain_by_id"
	s.logger.Info().Str("method", requestMethod).Msg("💻 handling request")

	// Requests are formatted as /v1/chains/id/<chain-id>[/<file>], where <file> defaults to chain.json
	prefix := fmt.Sprintf("/%s/%s/%s", apiVersion, chainsNamespace, chainByIDEndpoint)
	requestPath := strings.Trim(strings.TrimPrefix(req.URL.Path, prefix), "/")
	chainID, filePath, hasFilePath := strings.Cut(requestPath, "/")
	if !hasFilePath || filePath == "" {
		filePath = chainFileName
	}

	if chainID == "" {
		s.logger.Info().Str("method", requestMethod).Msg("request is missing a chain id")
		http.Error(w, "missing chain id", http.StatusBadRequest)
		return
	}

	chainName, found := s.chainIDCache[chainID]
	if !found {
		s.logger.Info().Str("method", requestMethod).Str("chain_id", chainID).Msg("no chain found for requested chain id")
		http.NotFound(w, req)
		return
	}

	redirectPath := fmt.Sprintf("/%s/%s/%s/%s", apiVersion, chainsNamespace, chainName, filePath)
	http.Redirect(w, req, redirectPath, http.StatusFound)

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
