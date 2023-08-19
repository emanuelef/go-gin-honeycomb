package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/emanuelef/go-gin-honeycomb/otel_instrumentation"
	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"

	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const externalURL = "https://pokeapi.co/api/v2/pokemon/ditto"

var tracer trace.Tracer

func init() {
	tracer = otel.Tracer("github.com/emanuelef/go-gin-honeycomb/secondary")
}

func getEnv(key, fallback string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		value = fallback
	}
	return value
}

func main() {
	ctx := context.Background()
	tp, exp, err := otel_instrumentation.InitializeGlobalTracerProvider(ctx)

	// Handle shutdown to ensure all sub processes are closed correctly and telemetry is exported
	defer func() {
		_ = exp.Shutdown(ctx)
		_ = tp.Shutdown(ctx)
	}()

	if err != nil {
		log.Fatalf("failed to initialize OpenTelemetry: %e", err)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(otelgin.Middleware("secondary-server"))

	r.GET("/hello", func(c *gin.Context) {
		_, err := otelhttp.Get(c.Request.Context(), externalURL)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
			return
		}

		resp, err := otelhttp.Get(c.Request.Context(), externalURL)
		_, _ = io.ReadAll(resp.Body)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
			return
		}

		// Get current span and add new attributes
		span := trace.SpanFromContext(c.Request.Context())
		span.SetAttributes(attribute.Bool("isTrue", true), attribute.String("stringAttr", "Ciao"))

		// Create a child span
		ctx, childSpan := tracer.Start(c.Request.Context(), "custom-span-secondary")
		time.Sleep(10 * time.Millisecond)
		resp, _ = otelhttp.Get(ctx, externalURL)
		_, _ = io.ReadAll(resp.Body)
		childSpan.End()
		time.Sleep(20 * time.Millisecond)

		c.JSON(resp.StatusCode, gin.H{})
	})

	host := getEnv("HOST", "localhost")
	port := getEnv("PORT", "8082")
	hostAddress := fmt.Sprintf("%s:%s", host, port)

	err = r.Run(hostAddress)
	if err != nil {
		log.Printf("Starting router failed, %v", err)
	}
}
