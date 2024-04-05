FROM golang:1.22.2-alpine as builder
WORKDIR /app
COPY ./secondary/main.go .
COPY ./otel_instrumentation ./otel_instrumentation
COPY ./go.mod .
COPY ./go.sum .
RUN go mod download
RUN go build -o secondary ./main.go

FROM alpine:latest AS runner
WORKDIR /home/app
COPY --from=builder /app/secondary .
EXPOSE 8099
ENTRYPOINT ["./secondary"]