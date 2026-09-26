package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/votatdat/challenge_appointment_scheduler/internal/appointments"
	"github.com/votatdat/challenge_appointment_scheduler/internal/config"
	"github.com/votatdat/challenge_appointment_scheduler/internal/httpapi"
	"github.com/votatdat/challenge_appointment_scheduler/internal/postgres"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("application stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.Open(rootCtx, cfg.DatabaseURL, cfg.DatabaseMaxConnections, cfg.DatabaseConnectTimeout)
	if err != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	defer pool.Close()

	service := appointments.NewService(postgres.NewAppointmentStore(pool))
	server := &http.Server{
		Addr:              cfg.HTTPAddress,
		Handler:           httpapi.NewHandler(pool, cfg.DatabasePingTimeout, service, logger),
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("HTTP server started", "address", cfg.HTTPAddress)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case <-rootCtx.Done():
		logger.Info("shutdown signal received")
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTPShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	logger.Info("HTTP server stopped")
	return nil
}
