.PHONY: dev test lint build clean opa-test

# Development targets
dev: build
	@echo "Starting development environment..."
	docker-compose up --build

# Build the Go application
build:
	@echo "Building Go application..."
	go build -o bin/policy-machine ./cmd

# Run tests
test:
	@echo "Running Go tests..."
	go test -v ./...

# Run OPA tests
opa-test:
	@echo "Running OPA tests..."
	docker run --rm -v $(PWD)/opa:/opa openpolicyagent/opa:latest test /opa/policies -v

# Lint code
lint:
	@echo "Running linter..."
	golangci-lint run

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	docker-compose down -v

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Run the application locally (without Docker)
run-local:
	@echo "Running application locally..."
	OPA_URL=http://localhost:8181 go run ./cmd

# Test the authorization endpoint
test-auth:
	@echo "Testing authorization endpoint..."
	@echo "Testing as regular user (should mask sensitive fields):"
	curl -H "X-User-ID: user1" -H "X-User-Role: user" -H "X-Resource-Owner: user1" -H "X-Resource-Type: normal" http://localhost:8080/api/v1/users/user123/data
	@echo "\n\nTesting as admin (should not mask fields):"
	curl -H "X-User-ID: admin1" -H "X-User-Role: admin" -H "X-Resource-Owner: admin1" -H "X-Resource-Type: normal" http://localhost:8080/api/v1/users/user123/data
	@echo "\n\nTesting sensitive resource (should be denied):"
	curl -H "X-User-ID: user1" -H "X-User-Role: user" -H "X-Resource-Owner: user1" -H "X-Resource-Type: sensitive" http://localhost:8080/api/v1/users/user123/data

# Help target
help:
	@echo "Available targets:"
	@echo "  dev        - Start development environment with Docker Compose"
	@echo "  build      - Build the Go application"
	@echo "  test       - Run Go tests"
	@echo "  opa-test   - Run OPA policy tests"
	@echo "  lint       - Run linter"
	@echo "  clean      - Clean build artifacts and stop containers"
	@echo "  deps       - Install dependencies"
	@echo "  run-local  - Run application locally (without Docker)"
	@echo "  test-auth  - Test authorization endpoints with curl"
	@echo "  help       - Show this help message"