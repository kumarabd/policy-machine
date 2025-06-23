SRC_DIR=pkg

help: ## Show this help message
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ { printf "  %-15s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

check: ## Run linting checks
	golangci-lint run

generate_tests: ## Generate test files for all Go files
	# install and setup https://github.com/cweill/gotests
	@find $(SRC_DIR) -type f -name '*.go' -exec bash -c ' \
        for file do \
			gotests -all -w $$file; \
        done' _ {} +
	
generate_proto: ## Generate protobuf files
	protoc -I ./api \
	--go_out ./api --go_opt paths=source_relative \
	--go-grpc_out ./api --go-grpc_opt paths=source_relative \
	--openapiv2_out ./api \
	--openapiv2_opt logtostderr=true \
	--openapiv2_opt generate_unbound_methods=true \
	./api/sample/sample.proto

generate_gql: ## Generate GraphQL files
	cd api/graphql && \
	go run github.com/99designs/gqlgen generate && \
	cd ../..

generate_docs: ## Generate Swagger API documentation
	@echo "Generating Swagger documentation..."
	@which swag > /dev/null || (echo "Installing swag..." && go install github.com/swaggo/swag/cmd/swag@latest)
	@if command -v swag >/dev/null 2>&1; then \
		swag init -g cmd/main.go -o docs; \
	else \
		$(HOME)/go/bin/swag init -g cmd/main.go -o docs; \
	fi
	@echo "Swagger docs generated in ./docs directory"

test: ## Run tests with coverage
	go test ./... -cover

run: ## Run the application in debug mode
	DEBUG="true" go run cmd/main.go --config internal/config/config.yaml

build: ## Build the application binary
	go build -ldflags "-w -s -X github.com/kumarabd/policy-machine/internal/config.ApplicationName=test -X github.com/kumarabd/policy-machine/internal/config.ApplicationVersion=test" -a -o service cmd/main.go

.PHONY: help check generate_tests generate_proto generate_gql generate_docs test run build