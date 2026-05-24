package org_test

import (
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/api/apitest"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

func setupTestServerWithEnv(t *testing.T, env *testutil.TestEnvironment) *testutil.TestServer {
	t.Helper()
	return apitest.NewServer(t, env, apitest.Options{})
}
