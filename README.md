# Confab

Archive and search your Claude Code sessions in the cloud.

**This repo contains:**
- **[backend/](backend/)** - Cloud backend service (PostgreSQL + MinIO)
- **[frontend/](frontend/)** - React web dashboard with GitHub OAuth

**See also:** [confab-cli](https://github.com/ConfabulousDev/confab-cli) - Command-line tool for capturing and uploading sessions

## Quick Start

### Install CLI

```bash
git clone https://github.com/ConfabulousDev/confab-cli.git
cd confab-cli
./install.sh
```

The CLI captures sessions via hook and uploads to the cloud backend. See [confab-cli README](https://github.com/ConfabulousDev/confab-cli#readme) for details.

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
│   Confab CLI     │ ← Runs on user's machine (separate repo)
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
├── frontend/              # React web dashboard
│   ├── src/pages/        # Pages and routes
│   ├── src/services/     # API client
│   └── README.md
│
└── docs/                  # Additional documentation
```

See also: [confab-cli](https://github.com/ConfabulousDev/confab-cli) (separate repo)

## Development

```bash
# Backend development
cd backend
docker-compose up -d
go run cmd/server/main.go
go test ./...

# Frontend development
cd frontend
npm install
npm run dev
npm test
```

## License

MIT
