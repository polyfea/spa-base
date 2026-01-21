package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

var processCtx = context.Background() // allow test coverage by processContext externalized

func main() {
	cfg := loadConfiguration()
	logger := configureLogger(cfg)
	signalChannel := make(chan os.Signal, 2)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)

	run(processCtx, cfg, logger, signalChannel)
}

func run(ctx context.Context, cfg Config, logger zerolog.Logger, signalChannel <-chan os.Signal) {
	if !cfg.TelemetryDisabled {
		shutdownTelemetry, err := initTelemetry(ctx)
		Must(err)
		defer shutdownTelemetry(ctx)
	}

	httpServer := &http.Server{
		Addr:    ":" + strconv.Itoa(cfg.Port),
		Handler: otelhttp.NewHandler(&server{cfg: cfg, logger: logger}, "serve-spa"),
	}

	go func() {
		logger.Info().Int("port", cfg.Port).Msg("Starting server")
		err := httpServer.ListenAndServe()
		logger.Info().Err(err).Msg("HTTP server stopped")
	}()

	stop := false
	for !stop {
		select {
		case sig := <-signalChannel:
			if sig == os.Interrupt || sig == syscall.SIGTERM {
				logger.Info().Str("signal", sig.String()).Msg("Shutdown signal received")
				stop = true
			}
		case <-ctx.Done():
			logger.Info().Msg("context done")
			stop = true
		}

	}

	httpServer.Shutdown(context.Background())
}
