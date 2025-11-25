// Package migrations provides embedded SQL migration files.
// These are used by testutil for running migrations in integration tests.
// Production deployments run migrations via the golang-migrate CLI.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
