.PHONY: build install test vet fmt

build:
	go build -o ccux ./cmd/ccux

install:
	go install ./cmd/ccux

test:
	go test ./...

vet:
	go vet ./...

fmt:
	go fmt ./...
