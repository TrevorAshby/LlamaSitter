package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/trevorashby/llamasitter/internal/api"
	"github.com/trevorashby/llamasitter/internal/config"
	"github.com/trevorashby/llamasitter/internal/proxy"
	"github.com/trevorashby/llamasitter/internal/storage"
)

func Run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	store, err := storage.NewSQLiteStore(cfg.Storage.SQLitePath)
	if err != nil {
		return err
	}
	defer store.Close()

	if err := store.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate sqlite: %w", err)
	}

	proxyService, err := proxy.NewService(cfg.Listeners, store, logger)
	if err != nil {
		return err
	}

	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 2)
	expected := 1

	go func() {
		errCh <- proxyService.Serve(childCtx)
	}()

	if cfg.UI.Enabled {
		expected++
		server := api.NewServer(cfg.UI.ListenAddr, store, logger)
		go func() {
			errCh <- api.Serve(childCtx, server)
		}()
	}

	for i := 0; i < expected; i++ {
		err := <-errCh
		if err == nil {
			continue
		}
		cancel()
		return err
	}

	return nil
}

func OpenStore(ctx context.Context, cfg config.Config) (*storage.SQLiteStore, error) {
	store, err := storage.NewSQLiteStore(cfg.Storage.SQLitePath)
	if err != nil {
		return nil, err
	}
	if err := store.Migrate(ctx); err != nil {
		_ = store.Close()
		return nil, err
	}
	return store, nil
}
