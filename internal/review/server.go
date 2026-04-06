package review

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"stash-scanner/internal/logging"
)

//go:embed ui/*
var uiFS embed.FS

type Server struct {
	logger  *log.Logger
	service *Service
	http    *http.Server
}

func NewServer(bind string, service *Service, logger *log.Logger) *Server {
	mux := http.NewServeMux()
	s := &Server{
		logger:  logger,
		service: service,
		http: &http.Server{
			Addr:              bind,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}

	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/ui/app.js", s.handleAppJS)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/refresh", s.handleRefresh)
	return s
}

func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		logging.Event(s.logger, "review_server_listening", "bind", s.http.Addr)
		err := s.http.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.http.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.ServeFileFS(w, r, uiFS, "ui/index.html")
}

func (s *Server) handleAppJS(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, uiFS, "ui/app.js")
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, s.service.Status())
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.service.Refresh(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "refreshed"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
