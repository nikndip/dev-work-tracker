package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dev-work-tracker/internal/config"
	"dev-work-tracker/internal/httpserver"
	"dev-work-tracker/internal/storage"
	telegrambot "dev-work-tracker/internal/telegram"
)

const shutdownTimeout = 10 * time.Second

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("application stopped with an error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	logger.Info("starting application")
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	signalCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	pool, err := storage.NewPostgresPool(signalCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	logger.Info("connected to PostgreSQL")

	telegramBot, err := telegrambot.New(cfg.TelegramBotToken, cfg.TelegramAllowedUserID, logger)
	if err != nil {
		return err
	}
	httpServer := httpserver.New(cfg.HTTPAddress, pool, logger)
	runCtx, cancel := context.WithCancel(signalCtx)
	defer cancel()
	errCh := make(chan error, 1)

	go func() {
		logger.Info("HTTP server started", "address", cfg.HTTPAddress)
		if serveErr := httpServer.Start(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			select {
			case errCh <- serveErr:
			default:
			}
		}
	}()
	botDone := make(chan struct{})
	go func() {
		defer close(botDone)
		logger.Info("Telegram bot started")
		telegramBot.Start(runCtx)
	}()

	var runErr error
	select {
	case <-signalCtx.Done():
		logger.Info("shutdown signal received")
	case runErr = <-errCh:
		logger.Error("HTTP server failed", "error", runErr)
	case <-botDone:
		runErr = errors.New("Telegram polling stopped unexpectedly")
		logger.Error("Telegram bot failed", "error", runErr)
	}

	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()
	select {
	case <-botDone:
		logger.Info("Telegram bot stopped")
	case <-shutdownCtx.Done():
		logger.Error("Telegram bot shutdown timed out", "error", shutdownCtx.Err())
		if runErr == nil {
			runErr = shutdownCtx.Err()
		}
	}
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown failed", "error", err)
		if runErr == nil {
			runErr = err
		}
	}
	logger.Info("application stopped")
	return runErr
}
