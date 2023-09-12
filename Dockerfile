FROM golang:1.21.1-alpine as builder
WORKDIR /app
COPY main.go .
COPY otel_instrumentation ./otel_instrumentation
COPY proto ./proto
COPY go.mod .
COPY go.sum .
RUN go mod download
RUN go build -o otel_honeycomb ./main.go

FROM alpine:latest AS runner
WORKDIR /home/app
COPY --from=builder /app/otel_honeycomb .
EXPOSE 8080
ENTRYPOINT ["./otel_honeycomb"]