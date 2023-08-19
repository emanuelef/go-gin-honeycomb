protoc --go_out=. --go_opt=paths=source_relative \
 --go-grpc_out=. --go-grpc_opt=paths=source_relative \
 simple.proto

grpcurl -plaintext localhost:7070 list
grpcurl -plaintext localhost:7070 list protos.Greeter
grpcurl -plaintext localhost:7070 describe protos.Greeter

grpcurl -plaintext -format json -d '{"greeting": "ciao"}' \
 localhost:7070 protos.Greeter.SayHello
