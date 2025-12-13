# Integration Test Infrastructure

This package provides infrastructure for running integration tests with **real PostgreSQL and MinIO (S3-compatible) containers** using Docker.

## Prerequisites

- **Docker** must be installed and running
- On macOS with OrbStack, set `DOCKER_HOST` environment variable:
  ```bash
  export DOCKER_HOST=unix://$HOME/.orbstack/run/docker.sock
  ```

## Running Tests

### Run all tests (unit + integration)
```bash
go test ./internal/api/... -v
```

### Run only unit tests (skip integration)
```bash
go test ./internal/api/... -short
```

### Run only integration tests
```bash
go test ./internal/api/... -v -run Integration
```

### Run specific integration test
```bash
go test ./internal/api/... -v -run TestHandleCreateShare_Integration/creates_public
```

## Architecture

### Test Infrastructure Components

1. **TestEnvironment** - Manages Docker containers and connections
   - PostgreSQL 16 container
   - MinIO S3-compatible storage container
   - Database connection with migrations applied
   - S3 storage client configured

2. **Helper Functions** - Create test data and make HTTP requests
   - `CreateTestUser()` - Insert user into database
   - `CreateTestSession()` - Insert session into database
   - `CreateTestRun()` - Insert run into database
   - `CreateTestFile()` - Insert file metadata into database
   - `CreateTestShare()` - Insert share into database
   - `AuthenticatedRequest()` - Create HTTP request with user context
   - `ParseJSONResponse()` - Decode JSON response
   - `AssertStatus()` - Check HTTP status code
   - `AssertErrorResponse()` - Verify error response

3. **Storage Helpers** - Verify files in S3
   - `VerifyFileInS3()` - Download and verify file exists

### Lifecycle

```
1. Test starts
   ↓
2. SetupTestEnvironment()
   - Starts PostgreSQL container (~3 seconds)
   - Runs database migrations
   - Starts MinIO container (~1 second)
   - Creates S3 bucket
   ↓
3. For each test case:
   - CleanDB() truncates all tables
   - Create test data
   - Execute HTTP handler
   - Verify response and database state
   ↓
4. Test ends
   - Cleanup() stops containers
   - Removes volumes
```

## Writing Integration Tests

### Example Test

```go
func TestHandleCreateShare_Integration(t *testing.T) {
    // Skip in short mode (unit tests only)
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }

    // Setup test environment (PostgreSQL + MinIO)
    env := testutil.SetupTestEnvironment(t)

    t.Run("creates public share successfully", func(t *testing.T) {
        // Clean database for test isolation
        env.CleanDB(t)

        // Create test data
        user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
        sessionID := "test-session-123"
        testutil.CreateTestSession(t, env, user.ID, sessionID)

        // Create HTTP request
        reqBody := CreateShareRequest{
            Visibility: "public",
        }
        req := testutil.AuthenticatedRequest(t, "POST",
            "/api/v1/sessions/"+sessionID+"/share", reqBody, user.ID)

        // Add chi URL parameters
        rctx := chi.NewRouteContext()
        rctx.URLParams.Add("sessionId", sessionID)
        req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

        // Execute handler
        w := httptest.NewRecorder()
        handler := HandleCreateShare(env.DB, "https://confab.dev")
        handler(w, req)

        // Assert response
        testutil.AssertStatus(t, w, http.StatusOK)

        var resp CreateShareResponse
        testutil.ParseJSONResponse(t, w, &resp)

        if resp.Visibility != "public" {
            t.Errorf("expected visibility 'public', got %s", resp.Visibility)
        }

        // Verify database state
        var count int
        row := env.DB.QueryRow(env.Ctx,
            "SELECT COUNT(*) FROM session_shares WHERE session_id = $1",
            sessionID)
        if err := row.Scan(&count); err != nil {
            t.Fatalf("failed to query shares: %v", err)
        }
        if count != 1 {
            t.Errorf("expected 1 share in database, got %d", count)
        }
    })
}
```

### Best Practices

1. **Always call `env.CleanDB(t)` at the start of each test case** for isolation
2. **Use chi.RouteContext** for handlers that read URL parameters
3. **Verify both HTTP response AND database state** to ensure correctness
4. **Test error cases** (404s, validation failures, etc.)
5. **Keep tests focused** - one test case per scenario

## Performance

- **Container startup**: ~4 seconds (one-time per test suite)
- **Test execution**: ~50-100ms per test case
- **Cleanup**: ~2 seconds (automatic on test completion)

Container startup is cached - subsequent test runs reuse pulled images.

## Troubleshooting

### "Docker not found" error

Ensure Docker is running. For OrbStack users:
```bash
export DOCKER_HOST=unix://$HOME/.orbstack/run/docker.sock
```

### "Container failed to start" error

Check Docker logs:
```bash
docker ps -a
docker logs <container_id>
```

### Slow test startup

First run downloads Docker images (~100MB). Subsequent runs are fast.

## Future Enhancements

- Add CI/CD configuration (GitHub Actions with service containers)
- Add test coverage reporting
- Add more integration tests for remaining handlers
- Add performance benchmarks
