# Confab

Archive and search your Claude Code sessions in the cloud.

**Monorepo containing:**
- **[cli/](cli/)** - Command-line tool for capturing and uploading sessions
- **[backend/](backend/)** - Cloud backend service (PostgreSQL + MinIO)
- **[frontend-new/](frontend-new/)** - React web dashboard with GitHub OAuth

## Quick Start

### Install CLI

```bash
git clone https://github.com/santaclaude2025/confab.git
cd confab/cli
./install.sh
```

The CLI captures sessions via hook and uploads to the cloud backend. See [cli/README.md](cli/README.md) for details.

### Run Backend + Frontend (Development)

```bash
# Start databases (PostgreSQL + MinIO)
cd backend
docker-compose up -d

# Start backend (Terminal 1)
go run cmd/server/main.go

# Start frontend (Terminal 2)
cd ../frontend-new
npm install
npm run dev
```

- Backend: `http://localhost:8080`
- Frontend: `http://localhost:5173`

See [backend/README.md](backend/README.md) for deployment.

## Architecture

```
┌──────────────────┐
│   Confab CLI     │ ← Runs on user's machine
│                  │   - Captures sessions via SessionEnd hook
│  ~/.confab/      │   - Uploads to cloud backend
└────────┬─────────┘
         │ HTTPS
         ▼
┌──────────────────┐
│  Backend Service │ ← Cloud/self-hosted server
│                  │   - PostgreSQL database
│  - REST API      │   - MinIO object storage (S3-compatible)
│  - Multi-user    │   - GitHub OAuth authentication
│  - Rate limiting │   - API key support
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  React Frontend  │ ← Web dashboard
│                  │   - Browse sessions
│  - GitHub OAuth  │   - View transcripts
│  - Search        │   - Share links
│  - Share Links   │   - Delete sessions
└──────────────────┘
```

## Features

### CLI
- Automatic session capture via SessionEnd hook
- Agent sidechain tracking
- Session resumption support
- Cloud upload with compression (zstd)
- Redaction support for sensitive data
- Structured logging

### Backend
- PostgreSQL 18 database
- MinIO S3-compatible storage
- GitHub OAuth authentication
- API key authentication
- Weekly upload rate limiting (200 runs/week)
- Session sharing with public/private links
- Delete sessions and individual runs

### Frontend
- Browse all sessions with search
- View session details and transcripts
- Create and manage share links
- Delete sessions and versions
- Responsive React UI with React Query

## Project Structure

```
confab/
├── cli/                    # CLI tool (Go)
│   ├── cmd/               # Commands (configure, login, save, etc.)
│   ├── pkg/               # Packages (config, discovery, upload, redactor)
│   └── README.md
│
├── backend/               # Backend service (Go)
│   ├── cmd/server/       # Server entry point
│   ├── internal/         # Internal packages
│   │   ├── api/         # HTTP handlers
│   │   ├── auth/        # OAuth & API keys
│   │   ├── db/          # PostgreSQL layer
│   │   ├── storage/     # MinIO/S3 client
│   │   └── testutil/    # Test infrastructure
│   ├── migrations/       # Database migrations
│   ├── docker-compose.yml
│   └── README.md
│
├── frontend-new/          # React web dashboard
│   ├── src/pages/        # Pages and routes
│   ├── src/services/     # API client
│   └── README.md
│
└── docs/                  # Additional documentation
```

## Development

```bash
# CLI development
cd cli
go build
make test

# Backend development
cd backend
docker-compose up -d
go run cmd/server/main.go
go test ./...

# Frontend development
cd frontend-new
npm install
npm run dev
npm test
```

## License

MIT
