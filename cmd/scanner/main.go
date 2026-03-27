package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"stash-scanner/internal/app"
	"stash-scanner/internal/config"
	"stash-scanner/internal/control"
	"stash-scanner/internal/logging"
)

func main() {
	var (
		configPath string
		runOnce    bool
	)

	flag.StringVar(&configPath, "config", "", "Path to the JSON config file")
	flag.BoolVar(&runOnce, "once", false, "Run a single scan cycle and exit")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	runner, err := app.NewRunner(cfg, log.New(os.Stdout, "scanner: ", log.LstdFlags|log.Lmsgprefix))
	if err != nil {
		log.Fatalf("build runner: %v", err)
	}

	var controlErrCh chan error
	if cfg.Control.Bind != "" && !runOnce {
		server := control.New(cfg.Control.Bind, cfg.Control.FallbackBind, runner, log.New(os.Stdout, "control: ", log.LstdFlags|log.Lmsgprefix))
		controlErrCh = make(chan error, 1)
		go func() {
			controlErrCh <- server.Run(ctx)
		}()
	}

	if runOnce {
		if err := runner.RunOnce(ctx); err != nil {
			log.Fatalf("run once: %v", err)
		}
		return
	}

	runErrCh := make(chan error, 1)
	go func() {
		runErrCh <- runner.Run(ctx)
	}()

	select {
	case err := <-runErrCh:
		logging.Event(log.Default(), "service_exit", "component", "scanner", "error", err)
		log.Fatalf("run: %v", err)
	case err := <-controlErrCh:
		if err != nil {
			cancel()
			logging.Event(log.Default(), "service_exit", "component", "control", "error", err)
			log.Fatalf("control server stopped: %v", err)
		}
		cancel()
		logging.Event(log.Default(), "service_exit", "component", "control")
		log.Fatalf("control server stopped")
	}
}
