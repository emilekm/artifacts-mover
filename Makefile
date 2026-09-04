BINARY_NAME=artifacts-mover

generate:
	go generate ./...

build: generate
	go build -o bin/$(BINARY_NAME) cmd/main.go

test: generate
	go test ./... -count=1 -v
