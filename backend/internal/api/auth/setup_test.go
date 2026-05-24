package auth_test

import (
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/api/apitest"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// All five test files (keys, device_code, github_links, shares, user)
// previously had their own near-identical setupXxxTestServer helper. They now
// share these thin wrappers around apitest.NewServer.

func setupKeysTestServer(t *testing.T, env *testutil.TestEnvironment) *testutil.TestServer {
	t.Helper()
	return apitest.NewServer(t, env, apitest.Options{})
}

func setupDeviceCodeTestServer(t *testing.T, env *testutil.TestEnvironment) *testutil.TestServer {
	t.Helper()
	return apitest.NewServer(t, env, apitest.Options{})
}

func setupDeviceCodeTestServerWithDomains(t *testing.T, env *testutil.TestEnvironment, allowedDomains []string) *testutil.TestServer {
	t.Helper()
	return apitest.NewServer(t, env, apitest.Options{
		AllowedEmailDomains: allowedDomains,
		SkipOAuthClientIDs:  true,
	})
}

func setupGitHubLinksTestServer(t *testing.T, env *testutil.TestEnvironment) *testutil.TestServer {
	t.Helper()
	return apitest.NewServer(t, env, apitest.Options{})
}

func setupSharesTestServer(t *testing.T, env *testutil.TestEnvironment) *testutil.TestServer {
	t.Helper()
	return apitest.NewServer(t, env, apitest.Options{EnableShareCreation: true})
}

func setupUserTestServer(t *testing.T, env *testutil.TestEnvironment) *testutil.TestServer {
	t.Helper()
	return apitest.NewServer(t, env, apitest.Options{})
}
