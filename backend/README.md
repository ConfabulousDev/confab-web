# Confab Backend

Cloud backend service for Confab - Claude Code session archiver.

## Features

- PostgreSQL 18 database for session metadata
- MinIO (S3-compatible) object storage for session files
- RESTful API for session upload
- API key authentication
- Multi-user support

## Architecture

```
┌─────────────┐
│  Confab CLI │
└──────┬──────┘
       │ POST /api/v1/sessions/save
       ▼
┌─────────────┐       ┌──────────────┐
│   Backend   ├──────▶│  PostgreSQL  │
│   (Go)      │       │      18      │
└──────┬──────┘       └──────────────┘
       │
       ▼
┌─────────────┐
│    MinIO    │
│  (S3 API)   │
└─────────────┘
```

## Local Development

### Prerequisites

- Docker & Docker Compose
- Go 1.21+

### Quick Start

```bash
# Start PostgreSQL and MinIO
docker-compose up -d

# Install dependencies
go mod download

# Run server
go run cmd/server/main.go
```

The server will start on `http://localhost:8080`

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server port |
| `DATABASE_URL` | `postgres://confab:confab@localhost:5432/confab?sslmode=disable` | PostgreSQL connection string |
| `S3_ENDPOINT` | `localhost:9000` | MinIO/S3 endpoint |
| `AWS_ACCESS_KEY_ID` | `minioadmin` | S3 access key |
| `AWS_SECRET_ACCESS_KEY` | `minioadmin` | S3 secret key |
| `BUCKET_NAME` | `confab` | S3 bucket name |
| `S3_USE_SSL` | `false` | Use SSL for S3 |

## API Endpoints

### Health Check
```
GET /health
```

### Save Session
```
POST /api/v1/sessions/save
Authorization: Bearer <api-key>

{
  "session_id": "abc123",
  "transcript_path": "/path/to/transcript.jsonl",
  "cwd": "/working/dir",
  "reason": "user_exit",
  "files": [
    {
      "path": "/path/to/file.jsonl",
      "type": "transcript",
      "size_bytes": 1024,
      "content": "<base64>"
    }
  ]
}
```

## Database Schema

See `internal/db/db.go` for the complete schema.

## Development

```bash
# Run tests
go test ./...

# Build binary
go build -o bin/confab-backend cmd/server/main.go

# Format code
go fmt ./...
```

## License

MIT
