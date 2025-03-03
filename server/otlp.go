package main

import (
	"context"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpgrpc"
	metricglobal "go.opentelemetry.io/otel/metric/global"
	metriccontroller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func createOTLPExporter(ctx context.Context) (*otlp.Exporter, error) {
	return otlp.NewExporter(ctx, otlpgrpc.NewDriver(
		otlpgrpc.WithInsecure(),                 // insecure because sending to localhost
		otlpgrpc.WithEndpoint("localhost:4317"), // otel-collector running as agent on this host (4317 is the default grpc port)
		otlpgrpc.WithHeaders(map[string]string{"ContentType": "application/grpc"}),
	))
}

func setupTraces(ctx context.Context, exporter *otlp.Exporter) (func(), error) {
	bsp := sdktrace.NewBatchSpanProcessor(exporter)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(0.5)),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(tp)
	return func() {
		if err := tp.Shutdown(ctx); err != nil {
			panic(err)
		}
	}, nil
}

func setupMetrics(ctx context.Context, exporter *otlp.Exporter) (func(), error) {
	mc := metriccontroller.New(
		processor.New(simple.NewWithInexpensiveDistribution(), exporter),
		metriccontroller.WithCollectPeriod(1*time.Minute),
		metriccontroller.WithExporter(exporter),
	)
	metricglobal.SetMeterProvider(mc.MeterProvider())

	// Capture runtime metrics
	if err := runtime.Start(runtime.WithMinimumReadMemStatsInterval(time.Second)); err != nil {
		panic(err)
	}

	// Handle this error in a sensible manner where possible
	return func() {
		if err := mc.Stop(ctx); err != nil {
			panic(err)
		}
	}, mc.Start(ctx)
}
