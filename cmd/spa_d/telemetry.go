package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"

	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type instruments struct {
	tracer           trace.Tracer
	meters           metric.Meter
	resources_served metric.Int64Counter
	fallbacks        metric.Int64Counter
	brotli_encrypted metric.Int64Counter
	gzip_encrypted   metric.Int64Counter
	not_found        metric.Int64Counter
}

// initialize OpenTelemetry instrumentations
func initTelemetry(ctx context.Context, logger *zerolog.Logger) (shutdown func(context.Context) error, err error) {
	metricReader, err := autoexport.NewMetricReader(ctx)
	if err != nil {
		return nil, err
	}

	metricProvider :=
		metricsdk.NewMeterProvider(metricsdk.WithReader(metricReader))
	otel.SetMeterProvider(metricProvider)

	traceExporter, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		return nil, err
	}

	traceProvider := tracesdk.NewTracerProvider(
		tracesdk.WithSyncer(traceExporter))

	otel.SetTracerProvider(traceProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	shutdown = func(context.Context) error {
		errMetric := metricProvider.Shutdown(ctx)
		errTrace := traceProvider.Shutdown(ctx)

		if errMetric != nil || errTrace != nil {
			return fmt.Errorf("error shutting down telemetry: %v, %v", errMetric, errTrace)
		}
		return nil
	}

	return shutdown, nil
}

var telemetry = sync.OnceValue[instruments](func() instruments {
	var err error
	instruments := instruments{}

	instruments.tracer = otel.Tracer("spa_d")
	instruments.meters = otel.Meter("spa_d")
	instruments.resources_served, err = instruments.meters.Int64Counter(
		"resources_served",
		metric.WithDescription("Count of resources served with success"),
		metric.WithUnit("{resources}"),
	)
	if err != nil {
		panic(err)
	}

	instruments.fallbacks, err = instruments.meters.Int64Counter(
		"fallbacks",
		metric.WithDescription("Count of served resources as fallback to index.html"),
		metric.WithUnit("{resources}"),
	)
	if err != nil {
		panic(err)
	}

	instruments.brotli_encrypted, err = instruments.meters.Int64Counter(
		"brotli",
		metric.WithDescription("Count of served resourcesencoded with brotli encoding"),
		metric.WithUnit("{resources}"),
	)
	if err != nil {
		panic(err)
	}

	instruments.gzip_encrypted, err = instruments.meters.Int64Counter(
		"gzip",
		metric.WithDescription("Count of served resources encoded with gzip encoding"),
		metric.WithUnit("{resources}"),
	)
	if err != nil {
		panic(err)
	}

	instruments.not_found, err = instruments.meters.Int64Counter(
		"not_found",
		metric.WithDescription("Count of requests with not found resources"),
		metric.WithUnit("{resources}"),
	)

	if err != nil {
		panic(err)
	}

	return instruments

})
