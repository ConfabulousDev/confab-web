# Integration Test Infrastructure

This package provides infrastructure for running integration tests with **real PostgreSQL and MinIO (S3-compatible) containers** using Docker, and **real HTTP servers** using the production router.

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

### Run only HTTP integration tests
```bash
go test ./internal/api/... -v -run "_HTTP_Integration"
```

### Run specific integration test
```bash
go test ./internal/api/... -v -run "TestSyncInit_HTTP_Integration/creates_new"
```

## Architecture

### Test Infrastructure Components

1. **TestEnvironment** - Manages Docker containers and connections
   - PostgreSQL 16 container
   - MinIO S3-compatible storage container
   - Database connection with migrations applied
   - S3 storage client configured

2. **TestServer** - Real HTTP server with production router
   - Starts server on random available port
   - Full middleware chain (auth, CSRF, rate limiting, compression)
   - Automatic cleanup on test completion

3. **TestClient** - HTTP client with authentication support
   - API key authentication (Bearer token)
   - Session cookie authentication
   - Automatic CSRF token handling
   - JSON request/response helpers

4. **Helper Functions** - Create test data and make HTTP requests
   - `CreateTestUser()` - Insert user into database
   - `CreateTestSession()` - Insert session into database
   - `CreateTestAPIKeyWithToken()` - Create API key and return raw token
   - `CreateTestWebSessionWithToken()` - Create web session and return token
   - `CreateTestSyncFile()` - Insert sync file into database
   - `ParseJSON()` - Decode JSON response
   - `RequireStatus()` - Check HTTP status code

## Test Patterns

### HTTP Integration Tests (Recommended)

HTTP integration tests exercise the full stack including:
- Real HTTP routing
- Middleware (auth, CSRF, rate limiting, compression)
- Request validation
- Response serialization

```go
func TestMyEndpoint_HTTP_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping HTTP integration test in short mode")
    }

    env := testutil.SetupTestEnvironment(t)

    t.Run("test case name", func(t *testing.T) {
        env.CleanDB(t)

        // Create test data
        user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
        apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")

        // Start server with production router
        ts := setupTestServerWithEnv(t, env)

        // Create authenticated client
        client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

        // Make request
        resp, err := client.Post("/api/v1/endpoint", requestBody)
        if err != nil {
            t.Fatalf("request failed: %v", err)
        }
        defer resp.Body.Close()

        // Assert response
        testutil.RequireStatus(t, resp, http.StatusOK)

        var result ResponseType
        testutil.ParseJSON(t, resp, &result)

        // Verify response data
        if result.Field != expected {
            t.Errorf("expected %v, got %v", expected, result.Field)
        }

        // Verify database state
        var count int
        row := env.DB.QueryRow(env.Ctx, "SELECT COUNT(*) FROM table WHERE ...")
        // ...
    })
}

// Helper to set up test server with required environment
func setupTestServerWithEnv(t *testing.T, env *testutil.TestEnvironment) *testutil.TestServer {
    t.Helper()

    testutil.SetEnvForTest(t, "CSRF_SECRET_KEY", "test-csrf-secret-key-32-bytes!!")
    testutil.SetEnvForTest(t, "ALLOWED_ORIGINS", "http://localhost:3000")
    testutil.SetEnvForTest(t, "FRONTEND_URL", "http://localhost:3000")
    testutil.SetEnvForTest(t, "INSECURE_DEV_MODE", "true")

    oauthConfig := auth.OAuthConfig{
        GitHubClientID:     "test-github-client-id",
        GitHubClientSecret: "test-github-client-secret",
        GitHubRedirectURL:  "http://localhost:3000/auth/github/callback",
        GoogleClientID:     "test-google-client-id",
        GoogleClientSecret: "test-google-client-secret",
        GoogleRedirectURL:  "http://localhost:3000/auth/google/callback",
    }

    apiServer := api.NewServer(env.DB, env.Storage, oauthConfig, nil)
    handler := apiServer.SetupRoutes()

    return testutil.StartTestServer(t, env, handler)
}
```

### Authentication Methods

#### API Key Authentication (CLI endpoints)
```go
apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "Test Key")
client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

resp, err := client.Post("/api/v1/sync/init", body)
```

#### Session Cookie Authentication (Web dashboard endpoints)
```go
sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

// CSRF tokens are automatically handled for state-changing requests
resp, err := client.Patch("/api/v1/sessions/"+sessionID+"/title", body)
```

### Lifecycle

```
1. Test starts
   ↓
2. SetupTestEnvironment()
   - Starts PostgreSQL container (~2 seconds)
   - Runs database migrations
   - Starts MinIO container (~1 second)
   - Creates S3 bucket
   ↓
3. For each test case:
   - CleanDB() truncates all tables
   - Create test data
   - Start test server (random port)
   - Make HTTP requests
   - Verify response and database state
   - Server auto-cleanup on test completion
   ↓
4. Test ends
   - Cleanup() stops containers
   - Removes volumes
```

## Best Practices

1. **Always call `env.CleanDB(t)` at the start of each test case** for isolation
2. **Use HTTP tests for endpoint testing** - they exercise the full middleware chain
3. **Verify both HTTP response AND database state** to ensure correctness
4. **Test error cases** (404s, validation failures, auth failures, etc.)
5. **Keep tests focused** - one test case per scenario
6. **Use appropriate auth for each endpoint**:
   - API key for `/api/v1/sync/*` (CLI endpoints)
   - Session cookie for web dashboard endpoints
   - Session + CSRF for state-changing web endpoints

## Performance

- **Container startup**: ~3 seconds (one-time per test suite)
- **Server startup**: ~10ms per test (random port selection + ready check)
- **Test execution**: ~30-50ms per test case
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

### "CSRF validation failed" error

Ensure your test uses `WithSession()` which automatically fetches CSRF tokens:
```go
client := testutil.NewTestClient(t, ts).WithSession(sessionToken)
```

### Slow test startup

First run downloads Docker images (~100MB). Subsequent runs are fast.
