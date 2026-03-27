package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"stash-scanner/internal/app"
	"stash-scanner/internal/config"
	"stash-scanner/internal/control"
	"stash-scanner/internal/logging"
	"stash-scanner/internal/state"
)

func main() {
	var (
		configPath   string
		runOnce      bool
		requeuePaths string
	)

	flag.StringVar(&configPath, "config", "", "Path to the JSON config file")
	flag.BoolVar(&runOnce, "once", false, "Run a single scan cycle and exit")
	flag.StringVar(&requeuePaths, "requeue-paths", "", "Comma-separated tracked paths to remove from state so they will be rediscovered on the next run")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	logging.SetDebug(cfg.Debug)
	logging.DebugEvent(
		log.Default(),
		"config_loaded",
		"config_path", configPath,
		"state_path", cfg.StatePath,
		"watch_roots_from_stash", cfg.WatchRootsFromStash,
		"watch_roots", len(cfg.WatchRoots),
		"dry_run", cfg.DryRun,
		"debug", cfg.Debug,
		"control_bind", cfg.Control.Bind,
		"control_fallback_bind", cfg.Control.FallbackBind,
	)

	if strings.TrimSpace(requeuePaths) != "" {
		store := state.NewStore(cfg.StatePath)
		removed, err := store.RequeuePaths(strings.Split(requeuePaths, ","))
		if err != nil {
			log.Fatalf("requeue paths: %v", err)
		}
		log.Printf("requeued %d tracked entries from %s", removed, cfg.StatePath)
		return
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
