.PHONY: help build run test clean deps kill

help:
	@echo "Minted - hledger Dashboard"
	@echo "Available commands:"
	@echo "  make deps   - Install dependencies"
	@echo "  make build  - Build the application"
	@echo "  make run    - Run the application"
	@echo "  make test   - Run tests"
	@echo "  make clean  - Clean build artifacts"
	@echo "  make kill   - Kill running go processes"

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

kill:
	pkill -f "bin/minted" || true
	pkill -f "go run" || true
	lsof -ti:9999 | xargs kill -9 2>/dev/null || true
