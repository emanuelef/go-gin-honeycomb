package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptrace"
	"os"
	"time"

	"github.com/emanuelef/go-gin-honeycomb/otel_instrumentation"
	protos "github.com/emanuelef/go-gin-honeycomb/proto"
	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
	"golang.org/x/exp/slices"

	"github.com/go-resty/resty/v2"

	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	//"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	externalURL = "https://pokeapi.co/api/v2/pokemon/ditto"
)

var notToLogEndpoints = []string{"/health", "/metrics"}

var tracer trace.Tracer

var (
	secondaryHost     = getEnv("SECONDARY_HOST", "localhost")
	secondaryAddress  = fmt.Sprintf("http://%s:8082", secondaryHost)
	secondaryHelloUrl = fmt.Sprintf("%s/hello", secondaryAddress)
)

func init() {
	tracer = otel.Tracer("github.com/emanuelef/go-gin-honeycomb")
}

func FilterTraces(req *http.Request) bool {
	return slices.Index(notToLogEndpoints, req.URL.Path) == -1
}

func getEnv(key, fallback string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		value = fallback
	}
	return value
}

// The context will carry the traceid and span id
// so once is passed it can be used access to the current span
// or create a child one, the function below will work if placed anywhere
// even in other packages
func exampleChildSpan(ctx context.Context) {
	// Get the current span from context
	// this can be needed to add an attribute or an event
	// but not necessary if the intention is then just to create a child span
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String("stringAttr", "Ciao"))

	// Create a child span
	_, anotherSpan := tracer.Start(ctx, "child-operation")
	anotherSpan.AddEvent("ciao")
	time.Sleep(10 * time.Millisecond)
	anotherSpan.End()
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
	r.Use(otelgin.Middleware("my-server", otelgin.WithFilter(FilterTraces)))

	// Just to check health and an example of a very frequent request
	// that we might not want to generate traces
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusNoContent, gin.H{})
	})

	// Basic GET API to show the otelgin middleware is taking
	// care of creating the span when called
	r.GET("/hello", func(c *gin.Context) {
		c.JSON(http.StatusNoContent, gin.H{})
	})

	// Creates a child span
	r.GET("/hello-child", func(c *gin.Context) {
		_, childSpan := tracer.Start(c.Request.Context(), "custom-child-span")
		time.Sleep(10 * time.Millisecond) // simulate some work
		childSpan.End()
		c.JSON(http.StatusNoContent, gin.H{})
	})

	// Runs HTTP requests to a public URL and to the secondary app
	r.GET("/hello-otelhttp", func(c *gin.Context) {
		resp, err := otelhttp.Get(c.Request.Context(), externalURL)
		if err != nil {
			return
		}

		_, _ = io.ReadAll(resp.Body) // This is needed to close the span

		// make sure secondary app is running
		resp, err = otelhttp.Get(c.Request.Context(), secondaryHelloUrl)

		if err != nil {
			return
		}

		_, _ = io.ReadAll(resp.Body) // This is needed to close the span

		// Get current span and add new attributes
		span := trace.SpanFromContext(c.Request.Context())
		span.SetAttributes(attribute.Bool("isTrue", true), attribute.String("stringAttr", "Ciao"))

		// Create a child span
		ctx, childSpan := tracer.Start(c.Request.Context(), "custom-span")
		time.Sleep(10 * time.Millisecond)
		resp, _ = otelhttp.Get(ctx, externalURL)
		_, _ = io.ReadAll(resp.Body)
		defer childSpan.End()

		time.Sleep(20 * time.Millisecond)

		// Add an event to the current span
		span.AddEvent("Done Activity")
		exampleChildSpan(ctx)
		c.JSON(http.StatusNoContent, gin.H{})
	})

	r.GET("/hello-http-client", func(c *gin.Context) {
		client := http.Client{
			Transport: otelhttp.NewTransport(
				http.DefaultTransport,
				otelhttp.WithClientTrace(func(ctx context.Context) *httptrace.ClientTrace {
					return otelhttptrace.NewClientTrace(ctx)
				})),
		}

		req, err := http.NewRequestWithContext(c.Request.Context(), "GET", externalURL, nil)
		if err != nil {
			return
		}

		// Needed to propagate the traceparent remotely if not setting the otelhttp.NewTransport
		// otel.GetTextMapPropagator().Inject(c.Request.Context(), propagation.HeaderCarrier(req.Header))

		resp, _ := client.Do(req)
		_, _ = io.ReadAll(resp.Body)

		req, err = http.NewRequestWithContext(c.Request.Context(), "GET", secondaryHelloUrl, nil)
		if err != nil {
			return
		}
		resp, _ = client.Do(req)
		body, _ := io.ReadAll(resp.Body)
		result := []map[string]any{}
		_ = json.Unmarshal([]byte(body), &result)

		c.JSON(resp.StatusCode, gin.H{})
	})

	r.GET("/hello-resty", func(c *gin.Context) {
		// get current span
		span := trace.SpanFromContext(c.Request.Context())

		// add events to span
		time.Sleep(5 * time.Millisecond)
		span.AddEvent("Done first fake long running task")
		time.Sleep(5 * time.Millisecond)
		span.AddEvent("Done second fake long running task")

		span.AddEvent("log", trace.WithAttributes(
			attribute.String("log.severity", "warning"),
			attribute.String("log.message", "Example log"),
		))

		client := resty.NewWithClient(
			&http.Client{
				Transport: otelhttp.NewTransport(http.DefaultTransport,
					otelhttp.WithClientTrace(func(ctx context.Context) *httptrace.ClientTrace {
						return otelhttptrace.NewClientTrace(ctx)
					})),
			},
		)

		restyReq := client.R()
		restyReq.SetContext(c.Request.Context()) // makes it possible to use the HTTP request trace_id

		// Needed to propagate the traceparent remotely if not setting the otelhttp.NewTransport
		// otel.GetTextMapPropagator().Inject(c.Request.Context(), propagation.HeaderCarrier(restyReq.Header))

		// run HTTP request first time
		resp, _ := restyReq.Get(externalURL)

		// run second time and notice http.getconn time compared to first one
		_, _ = restyReq.Get(externalURL)

		_, _ = restyReq.Get(secondaryHelloUrl)

		// simulate some post processing
		span.AddEvent("Start post processing")
		time.Sleep(10 * time.Millisecond)

		c.JSON(resp.StatusCode(), gin.H{})
	})

	r.GET("/hello-grpc", func(c *gin.Context) {
		grpcHost := getEnv("GRPC_TARGET", "localhost")
		grpcTarget := fmt.Sprintf("%s:7070", grpcHost)

		conn, err := grpc.NewClient(grpcTarget,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		)
		if err != nil {
			log.Printf("Did not connect: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
			return
		}

		defer conn.Close()
		cli := protos.NewGreeterClient(conn)

		r, err := cli.SayHello(c.Request.Context(), &protos.HelloRequest{Greeting: "ciao"})
		if err != nil {
			log.Printf("Error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
			return
		}

		log.Printf("Greeting: %s", r.GetReply())

		c.JSON(http.StatusNoContent, gin.H{})
	})

	// This is to generate a new span that is not a descendand of an existing one
	go func() {
		// in a real app it is better to use time.NewTicker so that the tivker will be
		// recovered by garbage collector
		for range time.Tick(time.Minute) {
			ctx, span := tracer.Start(context.Background(), "timed-operation")
			resp, _ := otelhttp.Get(ctx, externalURL)
			_, _ = io.ReadAll(resp.Body)
			span.End()
		}
	}()

	host := getEnv("HOST", "localhost")
	port := getEnv("PORT", "8080")
	hostAddress := fmt.Sprintf("%s:%s", host, port)

	err = r.Run(hostAddress)
	if err != nil {
		log.Printf("Starting router failed, %v", err)
	}
}
