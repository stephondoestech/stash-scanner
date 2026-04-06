package review

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
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
	RegisterRoutes(mux, "/", service)
	s := &Server{
		logger:  logger,
		service: service,
		http: &http.Server{
			Addr:              bind,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}

	return s
}

func RegisterRoutes(mux *http.ServeMux, prefix string, service *Service) {
	base := normalizePrefix(prefix)
	mux.HandleFunc(base, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != base {
			http.NotFound(w, r)
			return
		}
		http.ServeFileFS(w, r, uiFS, "ui/index.html")
	})
	if base != "/" {
		trimmed := strings.TrimSuffix(base, "/")
		mux.HandleFunc(trimmed, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != trimmed {
				http.NotFound(w, r)
				return
			}
			http.Redirect(w, r, base, http.StatusMovedPermanently)
		})
	}
	mux.HandleFunc(base+"ui/app.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, uiFS, "ui/app.js")
	})
	mux.HandleFunc(base+"api/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, service.Status())
	})
	mux.HandleFunc(base+"api/refresh", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := service.Refresh(r.Context()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "refreshed"})
	})
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

func normalizePrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" || prefix == "/" {
		return "/"
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return prefix
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
