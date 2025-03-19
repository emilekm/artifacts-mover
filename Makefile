BINARY_NAME=artifacts-mover

generate:
	go generate ./...

build: generate
	go build -o bin/$(BINARY_NAME) main.go

