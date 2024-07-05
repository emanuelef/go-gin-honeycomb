FROM golang:1.22.4-alpine as builder
WORKDIR /app
COPY ./grpc-server/main.go .
COPY ./otel_instrumentation ./otel_instrumentation
COPY ./proto ./proto
COPY ./go.mod .
COPY ./go.sum .
RUN go mod download
RUN go build -o grpcServer ./main.go

FROM alpine:latest AS runner
WORKDIR /home/app
COPY --from=builder /app/grpcServer .
EXPOSE 8099
ENTRYPOINT ["./grpcServer"]