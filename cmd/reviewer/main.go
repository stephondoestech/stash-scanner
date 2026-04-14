package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"stash-scanner/internal/logging"
	"stash-scanner/internal/review"
	"stash-scanner/internal/stash"
	"stash-scanner/internal/version"
)

func main() {
	cfg, err := review.LoadConfig()
	if err != nil {
		log.Fatalf("load reviewer config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger := log.New(os.Stdout, "reviewer: ", log.LstdFlags|log.Lmsgprefix)
	logging.Event(logger, "build_info", "version", version.Current(), "commit", version.Commit())
	service, err := review.NewService(
		review.NewStore(cfg.QueuePath),
		stash.NewClient(cfg.StashURL, cfg.APIKey, false),
		logger,
	)
	if err != nil {
		log.Fatalf("build reviewer service: %v", err)
	}
	service.SetMatchConfig(review.MatchConfigFromConfig(cfg))

	server := review.NewServer(cfg.Bind, service, logger)
	go func() {
		if err := service.Run(ctx, cfg.RefreshInterval); err != nil && !errors.Is(err, context.Canceled) {
			logging.Event(logger, "review_service_exit", "error", err)
			cancel()
		}
	}()

	if err := server.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("run reviewer server: %v", err)
	}
}
