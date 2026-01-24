.PHONY: help build run test clean deps

help:
	@echo "Minted - hledger Dashboard"
	@echo "Available commands:"
	@echo "  make deps   - Install dependencies"
	@echo "  make build  - Build the application"
	@echo "  make run    - Run the application"
	@echo "  make test   - Run tests"
	@echo "  make clean  - Clean build artifacts"

deps:
	go mod download
	go mod tidy

build: deps
	mkdir -p bin
	go build -o bin/minted ./cmd/minted

run: build
	./bin/minted

test:
	go test -v ./...

clean:
	rm -rf bin/
	go clean
