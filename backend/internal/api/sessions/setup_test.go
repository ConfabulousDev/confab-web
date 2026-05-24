package sessions_test

import (
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/api/apitest"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// setupTestServerWithEnv is the sessions sub-package's thin wrapper around
// apitest.NewServer. Kept under the original name so the moved tests don't
// need to change their call sites.
func setupTestServerWithEnv(t *testing.T, env *testutil.TestEnvironment) *testutil.TestServer {
	t.Helper()
	return apitest.NewServer(t, env, apitest.Options{})
}
