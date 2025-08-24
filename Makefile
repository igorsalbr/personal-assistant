# Makefile for personal-assistant (Go WhatsApp LLM Bot)

.PHONY: run test tidy migrate lint build

run:
	go run cmd/server/main.go

test:
	go test ./...

tidy:
	go mod tidy

migrate:
	# Example: run migrations (customize as needed)
	go run cmd/server/main.go --migrate

lint:
	golangci-lint run

build:
	go build -o bin/server cmd/server/main.go

help:
	@echo "Available targets:"
	@echo "  run     - Run the server"
	@echo "  test    - Run all tests"
	@echo "  tidy    - Tidy Go modules"
	@echo "  migrate - Run DB migrations (customize as needed)"
	@echo "  lint    - Run linter (requires golangci-lint)"
	@echo "  build   - Build the server binary"
