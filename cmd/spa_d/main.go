package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	cfg := loadConfiguration()
	logger := configureLogger(cfg)
	ctx := context.Background()

	if !cfg.TelemetryDisabled {
		shutdownTelemetry, err := initTelemetry(ctx, &logger)
		if err != nil {
			logger.Fatal().Err(err).Msg("Cannot initialize telemetry")
		}
		defer shutdownTelemetry(ctx)
	}

	httpServer := &http.Server{
		Addr:    ":" + strconv.Itoa(cfg.Port),
		Handler: otelhttp.NewHandler(&server{cfg: cfg, logger: logger}, "serve-spa"),
	}

	func() {
		logger.Info().Int("port", cfg.Port).Msg("Starting server")
		err := httpServer.ListenAndServe()
		if err != nil {
			logger.Fatal().Err(err).Msg("Server failed")
		}
	}()

	signalChannel := make(chan os.Signal, 2)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	for {
		sig := <-signalChannel
		switch sig {
		case os.Interrupt:
			logger.Info().Msg("interrupt")
		case syscall.SIGTERM:
			logger.Info().Msg("SIGTERM")
			httpServer.Shutdown(ctx)
			return
		}
	}

}
