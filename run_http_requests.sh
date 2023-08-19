#!/bin/bash -x

curl http://localhost:8080/hello
sleep 2
curl http://localhost:8080/hello-child
sleep 2
curl http://localhost:8080/hello-otelhttp
sleep 2
curl http://localhost:8080/hello-http-client
sleep 2
curl http://localhost:8080/hello-resty
sleep 2
curl http://localhost:8080/hello-grpc


