package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Satan1an/webtermin/internal/config"
	"github.com/Satan1an/webtermin/internal/server"
	"github.com/Satan1an/webtermin/internal/store"
	webui "github.com/Satan1an/webtermin/web"
)

// Populated at build time via -ldflags (see .goreleaser.yaml).
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config.yaml")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("webtermin %s (commit %s, built %s)\n", version, commit, date)
		return
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("config load failed", "err", err)
		os.Exit(1)
	}
	// Reset log level from config.
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: parseLevel(cfg.Logging.Level),
	}))

	st, err := store.Open(cfg.DataDir)
	if err != nil {
		logger.Error("store open failed", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	webFS, err := webui.FS()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		logger.Warn("embedded web bundle unavailable", "err", err)
	}

	srv := server.New(cfg, st, logger, webFS)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := srv.Run(ctx); err != nil {
		logger.Error("server exited with error", "err", err)
		os.Exit(1)
	}
	logger.Info("bye")
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
