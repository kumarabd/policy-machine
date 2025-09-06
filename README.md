# Policy Machine

A Go microservice with OPA (Open Policy Agent) integration implementing NGAC-style authorization policies. The Go app acts as a Policy Enforcement Point (PEP), OPA serves as the Policy Decision Point (PDP), and simple Policy Information Point (PIP) stubs fetch attributes.

## Architecture

- **Go App (PEP)**: Runs on port 8080, enforces authorization decisions
- **OPA (PDP)**: Runs on port 8181, evaluates NGAC-style policies
- **Policies**: Deny-overrides, condition predicates, and obligations
- **Hot Reload**: OPA watches for policy changes and reloads automatically

## Features

- **NGAC-style Policies**: Deny-overrides with condition predicates
- **Obligations**: Masking, logging, and alerting obligations
- **Fast Developer Loop**: Hot-reload Rego policies, `opa test`, `make dev`
- **Docker Compose**: Complete development environment
- **Comprehensive Testing**: Go tests + OPA policy tests

## Quick Start

### Prerequisites

- Docker and Docker Compose
- Go 1.23+ (for local development)

### Development Environment

1. **Start the complete stack**:
   ```bash
   make dev
   ```

2. **Test authorization endpoints**:
   ```bash
   make test-auth
   ```

3. **Run OPA policy tests**:
   ```bash
   make opa-test
   ```

### Manual Testing

Test the masking obligation with different user roles:

```bash
# Regular user (sensitive fields masked)
curl -H "X-User-ID: user1" \
     -H "X-User-Role: user" \
     -H "X-Resource-Owner: user1" \
     -H "X-Resource-Type: normal" \
     http://localhost:8080/api/v1/users/user123/data

# Admin user (no masking)
curl -H "X-User-ID: admin1" \
     -H "X-User-Role: admin" \
     -H "X-Resource-Owner: admin1" \
     -H "X-Resource-Type: normal" \
     http://localhost:8080/api/v1/users/user123/data

# Sensitive resource (access denied)
curl -H "X-User-ID: user1" \
     -H "X-User-Role: user" \
     -H "X-Resource-Owner: user1" \
     -H "X-Resource-Type: sensitive" \
     http://localhost:8080/api/v1/users/user123/data
```

## Available Make Targets

- `make dev` - Start development environment with Docker Compose
- `make build` - Build the Go application
- `make test` - Run Go tests
- `make opa-test` - Run OPA policy tests
- `make lint` - Run linter
- `make clean` - Clean build artifacts and stop containers
- `make deps` - Install dependencies
- `make run-local` - Run application locally (without Docker)
- `make test-auth` - Test authorization endpoints with curl
- `make help` - Show all available targets

## Project Structure

```
├── cmd/                    # Application entrypoints
├── internal/
│   ├── authz/             # OPA integration middleware
│   │   ├── client.go      # OPA HTTP client
│   │   └── middleware.go  # Gin authorization middleware
│   └── ...                # Other internal packages
├── pkg/
│   └── server/            # HTTP server with authz integration
├── opa/
│   ├── config.yaml        # OPA configuration
│   └── policies/
│       ├── authz.rego     # NGAC-style authorization policies
│       └── authz_test.rego # Policy tests
├── docker-compose.yml     # Development environment
└── Makefile              # Development commands
```

## Policy Architecture

### NGAC-style Authorization

The authorization system implements Next Generation Access Control (NGAC) principles:

1. **Deny-overrides**: Prohibitions take precedence over permissions
2. **Condition Predicates**: Named conditions evaluated by reference
3. **Obligations**: Actions returned with authorization decisions

### Policy Structure

```rego
# Main decision with deny-overrides
decision := "deny" if has_prohibition else "allow" if has_permission else "deny"

# Obligations from matching rules
obligations := [obligation | ...]

# Condition evaluation
conditions_met := {name: condition_result | ...}
```

### Example Obligations

- **Masking**: `{"type": "mask", "fields": ["ssn", "credit_card"]}`
- **Logging**: `{"type": "log", "message": "access granted"}`
- **Alerting**: `{"type": "alert", "message": "sensitive data access"}`

## Configuration

### Environment Variables

- `OPA_URL`: OPA server URL (default: `http://localhost:8181`)

### OPA Configuration

The OPA server is configured via `opa/config.yaml`:
- Decision logs enabled to console
- Policies loaded from mounted volume
- Watch mode for hot reload

## Development Workflow

1. **Edit policies** in `opa/policies/authz.rego`
2. **Test policies** with `make opa-test`
3. **Start environment** with `make dev`
4. **Test endpoints** with `make test-auth`
5. **Iterate** - OPA hot-reloads policy changes automatically

## API Endpoints

### Health & Metrics (No Auth Required)
- `GET /healthz` - Health check
- `GET /readyz` - Readiness check  
- `GET /metrics` - Prometheus metrics

### Protected Endpoints (Auth Required)
- `GET /api/v1/users/:resource_id/data` - User data with masking obligations

### Request Headers

- `X-User-ID`: User identifier
- `X-User-Role`: User role (admin, user, etc.)
- `X-Resource-Owner`: Resource owner ID
- `X-Resource-Type`: Resource type (normal, sensitive, etc.)
