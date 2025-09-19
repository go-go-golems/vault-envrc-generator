package webserver

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-go-golems/vault-envrc-generator/pkg/vaultlayer"
	"github.com/rs/zerolog/log"
)

//go:embed web/dist
var embeddedDist embed.FS

type Config struct {
	Host    string
	Port    string
	DevMode bool
	Vault   *vaultlayer.VaultSettings
}

type Server struct {
	config *Config
	mux    *http.ServeMux
	assets fs.FS
}

func New(config *Config) *Server {
	s := &Server{
		config: config,
		mux:    http.NewServeMux(),
	}

	// Initialize embedded assets (may be empty if web is not built yet)
	var sub fs.FS
	var err error
	if sub, err = fs.Sub(embeddedDist, "web/dist"); err != nil {
		// If subdir doesn't exist, fall back to empty FS
		log.Warn().Err(err).Msg("web/dist not found in embedded assets; serving empty SPA shell")
		s.assets = &emptyFS{}
	} else {
		s.assets = sub
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	// Health endpoint
	s.mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Vault endpoints (Stage 1)
	s.mux.HandleFunc("/api/v1/vault/list/", s.handleVaultList)
	s.mux.HandleFunc("/api/v1/vault/tree", s.handleVaultTree)
	s.mux.HandleFunc("/api/v1/vault/secrets/", s.handleVaultSecrets)
	s.mux.HandleFunc("/api/v1/vault/status", s.handleVaultStatus)

	// Generation endpoints
	s.mux.HandleFunc("/api/v1/generate/", s.handleGenerate)
	// Batch processing (stub for now)
	s.mux.HandleFunc("/api/v1/batch/process", s.handleBatchProcess)

	// Static files with SPA fallback
	fileServer := http.FileServer(http.FS(s.assets))
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Only GET and HEAD supported for static assets
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		requestPath := strings.TrimPrefix(r.URL.Path, "/")
		if requestPath == "" {
			requestPath = "index.html"
		}

		// Try to open the requested asset; if not found or is a directory, fall back to index.html
		if f, err := s.assets.Open(requestPath); err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// If the path has an extension, it's likely a real asset that we didn't find → 404
		if ext := path.Ext(requestPath); ext != "" && ext != "." {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Fallback to index.html for SPA routes
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/index.html"
		fileServer.ServeHTTP(w, r2)
	})
}

func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%s", s.config.Host, s.config.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Info().Str("addr", addr).Msg("starting web server")

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return ctx.Err()
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// emptyFS implements an empty filesystem
type emptyFS struct{}

func (e *emptyFS) Open(name string) (fs.File, error) {
	return nil, fs.ErrNotExist
}
