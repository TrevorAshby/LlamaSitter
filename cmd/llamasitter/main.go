package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/trevorashby/llamasitter/internal/cli"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	os.Exit(cli.Run(context.Background(), os.Args[1:], logger))
}
