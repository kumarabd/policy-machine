# Policy Machine API Documentation

This document describes how to access and use the Swagger API documentation for the Policy Machine.

## Accessing Swagger UI

Once the server is running, you can access the interactive API documentation at:

```
http://localhost:<PORT>/swagger/index.html
```

For example, if your server is configured to run on port 8500 (default):

```
http://localhost:8500/swagger/index.html
```

You can also access the raw Swagger JSON at:

```
http://localhost:<PORT>/swagger/doc.json
```

## Testing the Setup

To test if Swagger is working properly:

1. **Start the server**: `./policy-machine`
2. **Test the JSON endpoint**: `curl http://localhost:8500/swagger/doc.json` (replace 8500 with your configured port)
3. **Open Swagger UI**: Navigate to `http://localhost:8500/swagger/index.html`

If you see a "Internal Server Error" for `/swagger/doc.json`, ensure that:
- The docs package is properly imported in your main package
- The SwaggerInfo is correctly configured in `docs/docs.go`
- The server is built after generating the docs

## API Endpoints

### Health Endpoints
- `GET /healthz` - Health check endpoint
- `GET /readyz` - Readiness check endpoint  
- `GET /metrics` - Metrics endpoint

### RBAC Endpoints
- `POST /api/v1/rbac/roles` - Create a new role
- `PUT /api/v1/rbac/roles/{roleId}` - Update an existing role
- `GET /api/v1/rbac/roles` - List all roles
- `DELETE /api/v1/rbac/roles/{roleId}` - Delete a role

### Example API Usage

#### Create a Role
```bash
curl -X POST http://localhost:8500/api/v1/rbac/roles \
  -H "Content-Type: application/json" \
  -d '{
    "name": "admin",
    "description": "Administrator role with full access",
    "properties": {
      "department": "IT",
      "level": "high"
    }
  }'
```

#### Response
```json
{
  "message": "Role created successfully",
  "role": {
    "name": "admin",
    "description": "Administrator role with full access", 
    "properties": {
      "department": "IT",
      "level": "high",
      "type": "role",
      "parent": "role"
    },
    "entity_id": "role_admin_abc123"
  }
}
```

## Regenerating Documentation

To regenerate the Swagger documentation after making changes to the API:

### Using Make (Recommended)
```bash
make generate_docs
```

### Manual Command
```bash
~/go/bin/swag init -g cmd/main.go -o docs
```

The make command will automatically install `swag` if it's not already available and generate the documentation.

## API Models

The API uses the following main models:

- **CreateRoleRequest**: Request body for creating roles
- **CreateRoleResponse**: Response body for role creation  
- **RoleDetails**: Detailed role information
- **ErrorResponse**: Standard error response format

All models are automatically documented in the Swagger UI with examples and field descriptions.
