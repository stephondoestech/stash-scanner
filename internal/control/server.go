package control

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"syscall"
	"time"

	"stash-scanner/internal/app"
)

//go:embed ui/*
var uiFS embed.FS

type Runner interface {
	Status(context.Context) (app.Status, error)
	StartManualRun() error
}

type Server struct {
	addr         string
	fallbackAddr string
	logger       *log.Logger
	runner       Runner
	http         *http.Server
}

type listenResult struct {
	listener net.Listener
	err      error
}

func New(addr, fallbackAddr string, runner Runner, logger *log.Logger) *Server {
	mux := http.NewServeMux()
	s := &Server{
		addr:         addr,
		fallbackAddr: fallbackAddr,
		logger:       logger,
		runner:       runner,
		http: &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}

	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/ui/app.js", s.handleAppJS)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/run-now", s.handleRunNow)
	return s
}

func (s *Server) Run(ctx context.Context) error {
	listenCh := make(chan listenResult, 1)
	go func() {
		listener, addr, err := s.listen()
		if err != nil {
			listenCh <- listenResult{err: err}
			return
		}
		s.http.Addr = addr
		listenCh <- listenResult{listener: listener}
	}()

	result := <-listenCh
	if result.err != nil {
		return result.err
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Printf("control server listening on %s", s.http.Addr)
		err := s.http.Serve(result.listener)
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

func (s *Server) listen() (net.Listener, string, error) {
	listener, err := net.Listen("tcp", s.addr)
	if err == nil {
		return listener, s.addr, nil
	}

	primaryErr := formatBindError(s.addr, err)
	if s.fallbackAddr == "" {
		return nil, "", primaryErr
	}

	fallbackListener, fallbackErr := net.Listen("tcp", s.fallbackAddr)
	if fallbackErr == nil {
		s.logger.Printf("control bind %s failed, falling back to %s: %v", s.addr, s.fallbackAddr, primaryErr)
		return fallbackListener, s.fallbackAddr, nil
	}

	return nil, "", errors.Join(primaryErr, formatBindError(s.fallbackAddr, fallbackErr))
}

func formatBindError(addr string, err error) error {
	switch {
	case errors.Is(err, syscall.EADDRINUSE):
		return errors.New("control bind failed for " + addr + ": address already in use")
	case errors.Is(err, syscall.EACCES), errors.Is(err, syscall.EPERM):
		return errors.New("control bind failed for " + addr + ": permission denied")
	default:
		return errors.New("control bind failed for " + addr + ": " + err.Error())
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

	status, err := s.runner.Status(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleRunNow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := s.runner.StartManualRun()
	switch {
	case err == nil:
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "started"})
	case errors.Is(err, app.ErrRunInProgress):
		writeJSON(w, http.StatusConflict, map[string]string{"status": "running"})
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
