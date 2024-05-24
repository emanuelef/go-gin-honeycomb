package main

import (
	"context"
	"io"
	"log"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"go.opentelemetry.io/otel/trace"
)

const externalURL = "https://pokeapi.co/api/v2/pokemon/ditto"

var tracer trace.Tracer

func init() {
	tracer = otel.Tracer("github.com/emanuelef/go-gin-honeycomb/sample")
}

func InitializeGlobalTracerProvider() (*sdktrace.TracerProvider, error) {
	// OpenTelemetry exporter for tracing telemetry to be written to an output destination as JSON.
	exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("example"),
			semconv.ServiceVersion("0.0.1"),
		)),
	)
	otel.SetTracerProvider(tp)

	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	return tp, nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tp, err := InitializeGlobalTracerProvider()
	if err != nil {
		log.Fatalln("Unable to create a global trace provider", err)
	}

	defer func() {
		_ = tp.Shutdown(ctx)
	}()

	ctx, childSpan := tracer.Start(ctx, "custom-span")
	time.Sleep(1 * time.Second)
	respClient, _ := otelhttp.Get(ctx, externalURL)
	_, _ = io.ReadAll(respClient.Body)
	childSpan.End()
	time.Sleep(1 * time.Second)
}
