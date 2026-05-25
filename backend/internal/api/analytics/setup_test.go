package analytics_test

import (
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/api/apitest"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// setupTestServerWithEnv brings up the standard analytics test server via
// apitest. Same signature as the helper that used to live in
// sync_http_integration_test.go so moved tests don't need call-site changes.
func setupTestServerWithEnv(t *testing.T, env *testutil.TestEnvironment) *testutil.TestServer {
	t.Helper()
	return apitest.NewServer(t, env, apitest.Options{})
}
