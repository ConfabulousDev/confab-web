# Database Migration Strategy for Confab

## Current State

The Confab backend currently initializes the PostgreSQL schema on startup using a `CREATE TABLE IF NOT EXISTS` approach in `backend/internal/db/db.go:59-170`. While this works for initial development, it has significant limitations for production deployments.

### Schema Overview

- **8 tables**: users, web_sessions, api_keys, sessions, runs, files, session_shares, session_share_invites
- **12 indexes**: Covering common query patterns
- **Foreign keys**: Proper referential integrity with CASCADE deletes
- **JSONB column**: git_info in runs table for flexible metadata

### Current Limitations

1. **No version tracking** - Cannot determine what schema version is deployed in production
2. **No rollback capability** - Cannot safely undo problematic schema changes
3. **Limited evolution** - Difficult to safely ALTER existing tables without downtime risk
4. **No audit trail** - No record of when/why schema changes were made
5. **Team coordination issues** - Risk of conflicting schema changes across branches
6. **Testing gaps** - Cannot test migrations in isolation before production deployment
7. **Zero-downtime challenges** - Cannot perform multi-step migrations (e.g., add column, backfill, add constraint)

## Production Requirements

A production-grade migration system must provide:

- **Version control**: Track current schema version in database
- **Ordering**: Apply migrations in correct sequence
- **Idempotency**: Safe to run multiple times
- **Rollback**: Ability to revert problematic migrations
- **Locking**: Prevent concurrent migration execution
- **Dirty state detection**: Identify partially-applied migrations
- **Testing**: Validate migrations before production
- **CI/CD integration**: Automated migration checks in pipelines
- **Audit trail**: Record of all applied migrations with timestamps

## Migration Tool Comparison

### Option 1: golang-migrate/migrate ⭐ **RECOMMENDED**

**Repository**: https://github.com/golang-migrate/migrate
**Stars**: ~13k | **Maturity**: Production-proven since 2014

#### Pros

- Most widely adopted Go migration tool
- Both CLI and library support
- Built-in version control and database locking
- Dirty state detection and recovery
- Can embed migrations in binary (`embed.FS` support)
- Extensive database support (PostgreSQL, MySQL, etc.)
- Active maintenance and community
- Simple migration file format: `{version}_{name}.up.sql` and `{version}_{name}.down.sql`
- No external dependencies

#### Cons

- SQL-only migrations (no programmatic Go code)
- Requires learning file naming conventions
- Error messages can be cryptic for beginners

#### Use Cases

- Standard CRUD applications
- Teams comfortable with SQL
- Production systems requiring stability
- Multi-database support needed

#### Example Migration

```sql
-- 001_initial_schema.up.sql
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- 001_initial_schema.down.sql
DROP TABLE users;
```

#### Integration Pattern

```go
import (
    "embed"
    "github.com/golang-migrate/migrate/v4"
    "github.com/golang-migrate/migrate/v4/database/postgres"
    "github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func RunMigrations(db *sql.DB) error {
    driver, err := postgres.WithInstance(db, &postgres.Config{})
    if err != nil {
        return err
    }

    source, err := iofs.New(migrationsFS, "migrations")
    if err != nil {
        return err
    }

    m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
    if err != nil {
        return err
    }

    return m.Up()
}
```

---

### Option 2: pressly/goose

**Repository**: https://github.com/pressly/goose
**Stars**: ~6k | **Maturity**: Production-ready since 2016

#### Pros

- Supports both SQL and Go migrations
- Simpler API than golang-migrate
- Go migrations enable complex data transformations
- Good for migrations requiring application logic
- Embedded migrations support
- Provider-specific features (e.g., PostgreSQL advisory locks)

#### Cons

- Less popular than golang-migrate (~50% fewer users)
- Go migrations can be harder to review/audit
- Mixing SQL and Go migrations can be confusing
- Slightly more opinionated

#### Use Cases

- Complex data transformations during migration
- Need to call external APIs during migration
- Require application business logic in migrations
- Teams wanting flexibility

#### Example Migration

```go
// 001_initial_schema.go
package migrations

import (
    "database/sql"
    "github.com/pressly/goose/v3"
)

func init() {
    goose.AddMigration(upInitialSchema, downInitialSchema)
}

func upInitialSchema(tx *sql.Tx) error {
    _, err := tx.Exec(`
        CREATE TABLE users (
            id BIGSERIAL PRIMARY KEY,
            email VARCHAR(255) NOT NULL UNIQUE
        )
    `)
    return err
}

func downInitialSchema(tx *sql.Tx) error {
    _, err := tx.Exec("DROP TABLE users")
    return err
}
```

---

### Option 3: Ariga Atlas

**Repository**: https://github.com/ariga/atlas
**Stars**: ~5k | **Maturity**: Modern (2021), rapidly evolving

#### Pros

- **Declarative approach**: Define desired state, not steps
- Automatic migration generation from schema diff
- Schema visualization and inspection
- Built-in migration linting and testing
- Excellent PostgreSQL support
- Modern CLI experience
- Supports versioned and declarative workflows

#### Cons

- Newer tool (less battle-tested than alternatives)
- Different mental model from traditional imperative migrations
- Requires HCL or SQL schema definitions
- More complex setup initially
- Smaller community

#### Use Cases

- Teams wanting schema-as-code approach
- Projects with complex schema evolution
- Need automatic migration generation
- Want built-in testing/validation

#### Example Workflow

```hcl
// schema.hcl
table "users" {
  schema = schema.public
  column "id" {
    type = bigserial
  }
  column "email" {
    type = varchar(255)
    null = false
  }
  primary_key {
    columns = [column.id]
  }
  index "idx_users_email" {
    unique  = true
    columns = [column.email]
  }
}
```

```bash
# Generate migration from schema diff
atlas migrate diff initial --to file://schema.hcl
```

---

### Option 4: GORM Auto-Migrate

**Not recommended for production**

#### Cons

- No version control or rollback
- Dangerous AUTO migrations in production
- Limited control over migration order
- Cannot handle complex schema changes
- Poor audit trail
- Risk of data loss

**Use only for**: Local development, prototyping

---

## Recommendation: golang-migrate/migrate

For Confab's production deployment, I recommend **golang-migrate/migrate** for these reasons:

### Why golang-migrate?

1. **Battle-tested stability**: Used by thousands of production systems
2. **Simple mental model**: Up/down SQL files are easy to understand and review
3. **Team collaboration**: SQL migrations are universally reviewable (no Go knowledge needed)
4. **CI/CD friendly**: Easy to integrate into deployment pipelines
5. **PostgreSQL-perfect**: Excellent support for your single-database architecture
6. **Low risk**: Conservative, proven approach for mission-critical systems
7. **Migration embeddability**: Can bundle migrations in your binary for simplified deployment

### When to Consider Alternatives

- **Choose goose if**: You need complex data transformations requiring Go code
- **Choose Atlas if**: You have a complex schema and want automatic migration generation
- **Stay with current approach if**: Still in very early development (not production)

## Implementation Plan

### Phase 1: Setup (1-2 hours)

1. Install golang-migrate CLI and library
2. Create `backend/migrations/` directory
3. Convert current schema to initial migration file
4. Add migration version table

### Phase 2: Integration (2-3 hours)

1. Replace `RunMigrations()` in `db.go` with golang-migrate
2. Embed migrations in binary using `embed.FS`
3. Add migration status check to server startup
4. Handle dirty state scenarios

### Phase 3: Deployment (1-2 hours)

1. Test migrations against production snapshot
2. Update deployment scripts
3. Add rollback procedures to runbook
4. Document migration workflow for team

### Phase 4: CI/CD (1-2 hours)

1. Add migration linting to CI
2. Validate migrations don't break tests
3. Add migration dry-run checks
4. Document migration review process

## Migration Best Practices

### Naming Convention

```
{version}_{description}.up.sql
{version}_{description}.down.sql
```

Example:
```
20250121001_initial_schema.up.sql
20250121001_initial_schema.down.sql
20250121002_add_session_title.up.sql
20250121002_add_session_title.down.sql
```

### Safe Migration Patterns

#### Adding a Column (Non-Breaking)

```sql
-- up
ALTER TABLE sessions ADD COLUMN title TEXT;

-- down
ALTER TABLE sessions DROP COLUMN title;
```

#### Adding a NOT NULL Column (Multi-Step)

```sql
-- Migration 1: Add column as nullable
ALTER TABLE users ADD COLUMN phone VARCHAR(20);

-- Migration 2: Backfill data
UPDATE users SET phone = '' WHERE phone IS NULL;

-- Migration 3: Add constraint
ALTER TABLE users ALTER COLUMN phone SET NOT NULL;
```

#### Renaming a Column (Zero-Downtime)

```sql
-- Migration 1: Add new column
ALTER TABLE users ADD COLUMN full_name VARCHAR(255);

-- Migration 2: Backfill + dual-write in app code
-- (Deploy app version that writes to both columns)

-- Migration 3: Switch reads to new column
-- (Deploy app version that reads from full_name)

-- Migration 4: Drop old column
ALTER TABLE users DROP COLUMN name;
```

### Testing Migrations

1. **Local testing**: Apply up/down/up cycle on local database
2. **Staging validation**: Test on production-like dataset
3. **Performance testing**: Measure migration duration on large tables
4. **Rollback testing**: Verify down migrations work correctly
5. **CI validation**: Automated checks in pull requests

### Deployment Workflow

The `migrate_db.sh` script supports a dedicated `MIGRATE_DATABASE_URL` for running
migrations with an elevated/admin DB user. If unset, it falls back to `DATABASE_URL`.

```bash
# Using the migration script (preferred — handles MIGRATE_DATABASE_URL fallback)
./migrate_db.sh

# Or manually:
# 1. Check current version
migrate -database "$MIGRATE_DATABASE_URL" -path migrations version

# 2. Apply migrations
migrate -database "$MIGRATE_DATABASE_URL" -path migrations up

# 3. Verify migration success
psql "$MIGRATE_DATABASE_URL" -c "SELECT * FROM schema_migrations"

# 4. Rollback if needed (emergency only)
migrate -database "$MIGRATE_DATABASE_URL" -path migrations down 1
```

## Next Steps

1. **Decision**: Confirm golang-migrate as the chosen solution
2. **Setup**: Install tools and create initial migration
3. **Testing**: Validate migration on local/staging environments
4. **Documentation**: Update deployment docs with migration procedures
5. **Training**: Brief team on migration workflow

## References

- [golang-migrate documentation](https://github.com/golang-migrate/migrate/tree/master/database/postgres)
- [PostgreSQL migration best practices](https://www.postgresql.org/docs/current/ddl-alter.html)
- [Zero-downtime migrations guide](https://stripe.com/blog/online-migrations)
