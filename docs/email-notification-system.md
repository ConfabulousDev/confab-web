# Email Notification System Design

Simple email notification system for confab using Postgres + River.

## Architecture Overview

```
┌─────────────────────────────────────────────────────┐
│  Fly.io (3 replicas)                                │
│                                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────┐  │
│  │ Backend #1   │  │ Backend #2   │  │ Backend #3│  │
│  │              │  │              │  │           │  │
│  │ HTTP Server  │  │ HTTP Server  │  │ HTTP     │  │
│  │ River Workers│  │ River Workers│  │ Workers  │  │
│  └──────┬───────┘  └──────┬───────┘  └─────┬────┘  │
│         │                 │                 │       │
│         └─────────────────┴─────────────────┘       │
│                           │                         │
└───────────────────────────┼─────────────────────────┘
                            │
                    ┌───────▼────────┐
                    │   PostgreSQL   │
                    │                │
                    │  river_jobs    │
                    │  river_leaders │
                    └───────┬────────┘
                            │
                    ┌───────▼────────┐
                    │  ESP (SES/etc) │
                    └────────────────┘
```

## Key Design Decisions

1. **Combined HTTP + Workers**: Each backend replica runs both HTTP server and email workers
2. **Postgres as Queue**: Use River + Postgres instead of Redis/SQS for simplicity
3. **20 Workers per Replica**: Configurable concurrency per backend instance
4. **Automatic Retries**: Up to 5 attempts with exponential backoff
5. **Job Priority**: Critical emails (password reset) get higher priority

## Database Schema

### River Tables (created automatically)

```sql
-- Jobs table (created by River)
CREATE TABLE river_job (
  id bigserial PRIMARY KEY,
  args jsonb NOT NULL,
  attempt smallint NOT NULL DEFAULT 0,
  attempted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT NOW(),
  errors jsonb[],
  finalized_at timestamptz,
  kind text NOT NULL,
  max_attempts smallint NOT NULL,
  metadata jsonb NOT NULL DEFAULT '{}',
  priority smallint NOT NULL DEFAULT 1,
  queue text NOT NULL DEFAULT 'default',
  state text NOT NULL DEFAULT 'available',
  scheduled_at timestamptz NOT NULL DEFAULT NOW(),
  attempted_by text[]
);
```

### Application Tables

```sql
-- Email templates
CREATE TABLE email_templates (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  subject TEXT NOT NULL,
  html_body TEXT NOT NULL,
  text_body TEXT,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Track email delivery status
CREATE TABLE email_logs (
  id BIGSERIAL PRIMARY KEY,
  recipient TEXT NOT NULL,
  template_name TEXT NOT NULL,
  esp_message_id TEXT,
  status TEXT NOT NULL, -- queued, sent, delivered, bounced, failed
  error TEXT,
  metadata JSONB,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_email_logs_recipient ON email_logs(recipient);
CREATE INDEX idx_email_logs_status ON email_logs(status);
```

## Project Structure

```
backend/
├── cmd/
│   └── server/
│       └── main.go              # App entrypoint
├── internal/
│   ├── api/
│   │   └── server.go            # HTTP handlers
│   ├── email/
│   │   ├── jobs.go              # River job definitions
│   │   ├── worker.go            # Email worker implementation
│   │   ├── templates.go         # Template rendering
│   │   └── sender.go            # ESP integration
│   ├── db/
│   │   └── db.go                # Database connection
│   └── models/
│       └── models.go            # Data models
└── go.mod
```

## Implementation

### 1. Main Application (cmd/server/main.go)

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/riverqueue/river"
    "github.com/riverqueue/river/riverdriver/riverpgxv5"

    "confab/internal/api"
    "confab/internal/db"
    "confab/internal/email"
)

func main() {
    ctx := context.Background()

    // Database connection
    dbPool, err := db.Connect(ctx)
    if err != nil {
        log.Fatalf("Failed to connect to database: %v", err)
    }
    defer dbPool.Close()

    // Initialize River workers
    workers := river.NewWorkers()
    river.AddWorker(workers, &email.SendEmailWorker{
        DB: dbPool,
    })

    // Create River client
    riverClient, err := river.NewClient(riverpgxv5.New(dbPool), &river.Config{
        Queues: map[string]river.QueueConfig{
            river.QueueDefault: {MaxWorkers: 20}, // 20 concurrent email workers
        },
        Workers: workers,
    })
    if err != nil {
        log.Fatalf("Failed to create River client: %v", err)
    }

    // Start River workers
    if err := riverClient.Start(ctx); err != nil {
        log.Fatalf("Failed to start River client: %v", err)
    }

    // HTTP server
    server := &http.Server{
        Addr:    ":8080",
        Handler: api.NewServer(dbPool, riverClient),
    }

    // Graceful shutdown
    go func() {
        sigChan := make(chan os.Signal, 1)
        signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
        <-sigChan

        log.Println("Shutting down gracefully...")

        // Stop accepting new jobs
        riverClient.Stop(ctx)

        // Shutdown HTTP server
        shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
        defer cancel()
        server.Shutdown(shutdownCtx)
    }()

    log.Printf("Server starting on :8080, River workers running...")
    if err := server.ListenAndServe(); err != http.ErrServerClosed {
        log.Fatalf("Server error: %v", err)
    }
}
```

### 2. Job Definitions (internal/email/jobs.go)

```go
package email

import (
    "context"

    "github.com/riverqueue/river"
)

// SendEmailArgs defines the job arguments for sending emails
type SendEmailArgs struct {
    Recipient    string            `json:"recipient"`
    TemplateName string            `json:"template_name"`
    Data         map[string]string `json:"data"`
    Priority     int               `json:"priority,omitempty"`
}

// Kind returns the unique job type identifier
func (SendEmailArgs) Kind() string { return "send_email" }

// EnqueueEmail adds an email job to the queue
func EnqueueEmail(ctx context.Context, client *river.Client[pgx.Tx], args SendEmailArgs) error {
    priority := args.Priority
    if priority == 0 {
        priority = 1 // Default priority
    }

    _, err := client.Insert(ctx, args, &river.InsertOpts{
        Priority: priority,
        MaxAttempts: 5,
    })
    return err
}
```

### 3. Worker Implementation (internal/email/worker.go)

```go
package email

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "strings"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/riverqueue/river"
)

// SendEmailWorker processes email jobs
type SendEmailWorker struct {
    river.WorkerDefaults[SendEmailArgs]
    DB *pgxpool.Pool
}

// Work executes the email sending job
func (w *SendEmailWorker) Work(ctx context.Context, job *river.Job[SendEmailArgs]) error {
    args := job.Args

    log.Printf("Processing email job %d: recipient=%s, template=%s",
        job.ID, args.Recipient, args.TemplateName)

    // Create email log entry
    var logID int64
    err := w.DB.QueryRow(ctx, `
        INSERT INTO email_logs (recipient, template_name, status, metadata)
        VALUES ($1, $2, 'queued', $3)
        RETURNING id
    `, args.Recipient, args.TemplateName, mustJSON(args.Data)).Scan(&logID)
    if err != nil {
        return fmt.Errorf("failed to create email log: %w", err)
    }

    // Get template
    template, err := w.getTemplate(ctx, args.TemplateName)
    if err != nil {
        w.updateEmailLog(ctx, logID, "failed", err.Error(), "")
        return fmt.Errorf("failed to get template: %w", err)
    }

    // Render template
    rendered, err := renderTemplate(template, args.Data)
    if err != nil {
        w.updateEmailLog(ctx, logID, "failed", err.Error(), "")
        return fmt.Errorf("failed to render template: %w", err)
    }

    // Send via ESP
    messageID, err := w.sendEmail(ctx, args.Recipient, rendered)
    if err != nil {
        w.updateEmailLog(ctx, logID, "failed", err.Error(), "")
        return fmt.Errorf("failed to send email: %w", err)
    }

    // Update log
    w.updateEmailLog(ctx, logID, "sent", "", messageID)

    log.Printf("Email sent successfully: job=%d, log=%d, messageID=%s",
        job.ID, logID, messageID)

    return nil
}

func (w *SendEmailWorker) getTemplate(ctx context.Context, name string) (*EmailTemplate, error) {
    var tmpl EmailTemplate
    err := w.DB.QueryRow(ctx, `
        SELECT name, subject, html_body, text_body
        FROM email_templates
        WHERE name = $1
    `, name).Scan(&tmpl.Name, &tmpl.Subject, &tmpl.HTMLBody, &tmpl.TextBody)
    if err != nil {
        return nil, err
    }
    return &tmpl, nil
}

func (w *SendEmailWorker) updateEmailLog(ctx context.Context, id int64, status, errorMsg, messageID string) {
    _, err := w.DB.Exec(ctx, `
        UPDATE email_logs
        SET status = $1, error = $2, esp_message_id = $3, updated_at = NOW()
        WHERE id = $4
    `, status, errorMsg, messageID, id)
    if err != nil {
        log.Printf("Failed to update email log: %v", err)
    }
}

func (w *SendEmailWorker) sendEmail(ctx context.Context, recipient string, rendered *RenderedEmail) (string, error) {
    // TODO: Integrate with ESP (SES, SendGrid, etc.)
    // For now, just log it
    log.Printf("Would send email to %s: subject=%s", recipient, rendered.Subject)

    // Return fake message ID
    return fmt.Sprintf("msg_%d", time.Now().Unix()), nil
}

type EmailTemplate struct {
    Name     string
    Subject  string
    HTMLBody string
    TextBody string
}

type RenderedEmail struct {
    Subject  string
    HTMLBody string
    TextBody string
}

func renderTemplate(tmpl *EmailTemplate, data map[string]string) (*RenderedEmail, error) {
    // Simple string replacement for now
    // TODO: Use proper template engine (html/template or similar)
    subject := tmpl.Subject
    htmlBody := tmpl.HTMLBody
    textBody := tmpl.TextBody

    for key, value := range data {
        placeholder := "{{" + key + "}}"
        subject = strings.ReplaceAll(subject, placeholder, value)
        htmlBody = strings.ReplaceAll(htmlBody, placeholder, value)
        textBody = strings.ReplaceAll(textBody, placeholder, value)
    }

    return &RenderedEmail{
        Subject:  subject,
        HTMLBody: htmlBody,
        TextBody: textBody,
    }, nil
}

func mustJSON(v interface{}) []byte {
    b, _ := json.Marshal(v)
    return b
}
```

### 4. API Server (internal/api/server.go)

```go
package api

import (
    "encoding/json"
    "net/http"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/riverqueue/river"

    "confab/internal/email"
)

type Server struct {
    DB    *pgxpool.Pool
    River *river.Client[pgx.Tx]
}

func NewServer(db *pgxpool.Pool, riverClient *river.Client[pgx.Tx]) http.Handler {
    s := &Server{
        DB:    db,
        River: riverClient,
    }

    mux := http.NewServeMux()
    mux.HandleFunc("/api/send-test-email", s.handleSendTestEmail)

    return mux
}

func (s *Server) handleSendTestEmail(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var req struct {
        Recipient string `json:"recipient"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }

    // Enqueue email job
    err := email.EnqueueEmail(r.Context(), s.River, email.SendEmailArgs{
        Recipient:    req.Recipient,
        TemplateName: "welcome",
        Data: map[string]string{
            "name": "User",
        },
        Priority: 1,
    })
    if err != nil {
        http.Error(w, "Failed to enqueue email", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusAccepted)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "queued",
    })
}
```

## Dependencies

```go
// go.mod
module confab

go 1.21

require (
    github.com/jackc/pgx/v5 v5.5.0
    github.com/riverqueue/river v0.0.25
    github.com/riverqueue/river/riverdriver/riverpgxv5 v0.0.25
)
```

## Configuration

```bash
# Environment variables
DATABASE_URL=postgresql://user:pass@host:5432/confab

# Or Fly.io secrets
fly secrets set DATABASE_URL=postgresql://...
```

## Running Migrations

```go
// Run River migrations on startup (add to main.go)
import "github.com/riverqueue/river/rivermigrate"

// After creating riverClient
_, err = rivermigrate.New(riverpgxv5.New(dbPool), nil).Migrate(
    ctx,
    rivermigrate.DirectionUp,
    &rivermigrate.MigrateOpts{},
)
if err != nil {
    log.Fatalf("Failed to run migrations: %v", err)
}
```

## Email Types and Priorities

| Type | Priority | Max Attempts | Example |
|------|----------|--------------|---------|
| Critical | 4 | 5 | Password reset, email verification |
| High | 3 | 5 | Order confirmation, payment receipt |
| Normal | 2 | 5 | Comment notifications, mentions |
| Low | 1 | 3 | Weekly digests, marketing |

## Usage Examples

### Enqueue a Password Reset Email

```go
err := email.EnqueueEmail(ctx, riverClient, email.SendEmailArgs{
    Recipient:    "user@example.com",
    TemplateName: "password_reset",
    Data: map[string]string{
        "reset_link": "https://confab.app/reset/abc123",
        "expires_in": "1 hour",
    },
    Priority: 4, // Critical priority
})
```

### Enqueue a Welcome Email

```go
err := email.EnqueueEmail(ctx, riverClient, email.SendEmailArgs{
    Recipient:    "newuser@example.com",
    TemplateName: "welcome",
    Data: map[string]string{
        "name": "John Doe",
        "login_link": "https://confab.app/login",
    },
    Priority: 2, // Normal priority
})
```

## Features

✅ **Combined HTTP + Workers**: Simplifies deployment
✅ **20 Concurrent Workers**: Per replica (configurable)
✅ **Automatic Retries**: Up to 5 attempts with exponential backoff
✅ **Job Priority**: Critical emails processed first
✅ **Graceful Shutdown**: Finishes in-flight jobs before stopping
✅ **Email Logging**: Track every email sent with status
✅ **Template System**: Reusable email templates in database
✅ **Postgres Queue**: No Redis/SQS needed

## Monitoring

### Check Queue Depth

```sql
SELECT state, COUNT(*)
FROM river_job
WHERE kind = 'send_email'
GROUP BY state;
```

### Check Failed Jobs

```sql
SELECT id, args, errors
FROM river_job
WHERE kind = 'send_email'
  AND state = 'discarded'
ORDER BY created_at DESC
LIMIT 10;
```

### Email Delivery Metrics

```sql
SELECT
    status,
    COUNT(*) as count,
    DATE_TRUNC('hour', created_at) as hour
FROM email_logs
WHERE created_at > NOW() - INTERVAL '24 hours'
GROUP BY status, hour
ORDER BY hour DESC;
```

## Scaling Considerations

### Current Capacity

- 3 replicas × 20 workers = 60 concurrent emails
- Can handle ~3,600 emails/minute (assuming 1 second per email)
- ~216,000 emails/hour
- ~5M emails/day

### When to Scale

| Scenario | Action |
|----------|--------|
| Queue depth consistently > 100 | Add more replicas |
| Workers idle most of the time | Reduce worker count |
| Jobs timing out | Increase timeout or worker count |
| Database CPU high | Optimize queries or add read replicas |

### Future Optimizations

1. **ESP Integration**: Add AWS SES, SendGrid, or Postmark
2. **Template Engine**: Use html/template for better rendering
3. **Webhook Handlers**: Track delivery, opens, clicks
4. **Unsubscribe Management**: Handle opt-outs and preferences
5. **Rate Limiting**: Per-user and per-tenant limits
6. **Separate Worker Pools**: Different queues for different email types

## Next Steps

1. [ ] Add ESP integration (AWS SES recommended for cost)
2. [ ] Implement proper HTML template rendering
3. [ ] Add webhook handlers for delivery tracking
4. [ ] Create email preference management
5. [ ] Set up monitoring and alerts
6. [ ] Add SPF/DKIM/DMARC authentication
7. [ ] Create email templates for all notification types

## References

- [River Documentation](https://riverqueue.com/docs)
- [River GitHub](https://github.com/riverqueue/river)
- [AWS SES Documentation](https://docs.aws.amazon.com/ses/)
- [Email Best Practices](../email-notification-research.md)
