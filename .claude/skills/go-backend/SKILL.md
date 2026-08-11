---
name: go-backend
description: Implement and maintain the Go Gin backend for the LMS.
---

# Go Backend

Use Go + Gin + pgxpool.

## Module structure
Each substantial domain should prefer:

internal/<domain>/
- handler.go
- service.go
- repository.go
- model.go

## Rules
- Keep SQL out of handlers.
- Keep HTTP status codes out of repositories.
- Pass `context.Context` where appropriate.
- Use parameterized SQL.
- Return meaningful errors.
- Use transactions for multi-step writes.
- Read config from environment variables.
- Never hardcode secrets.

## HTTP conventions
- JSON in/out
- REST resource naming
- `/api/v1/*`
- consistent error shape

Example:
```json
{
  "error": {
    "code": "COURSE_NOT_FOUND",
    "message": "Course not found"
  }
}
```

## Startup
Typical flow:
1. Load config.
2. Connect pgxpool.
3. Build repositories.
4. Build services.
5. Build handlers.
6. Register routes.
7. Start Gin.

## Verification
After backend changes:
- `go fmt ./...`
- `go test ./...`
- `go vet ./...` where applicable
- start the API and call the affected endpoint
