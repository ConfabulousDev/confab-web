# Confab

Archive and query your Claude Code sessions locally and in the cloud.

**Monorepo containing:**
- **[cli/](cli/)** - Command-line tool for local session archiving
- **[backend/](backend/)** - Cloud backend service for session sync
- **[frontend/](frontend/)** - Web dashboard with GitHub OAuth

## Quick Start

### Install CLI

```bash
git clone https://github.com/santaclaude2025/confab.git
cd confab/cli
./install.sh
```

See [cli/README.md](cli/README.md) for detailed CLI documentation.

### Run Backend + Frontend (Optional)

For cloud sync and web dashboard:

```bash
# Terminal 1: Start databases
cd backend
docker-compose up -d

# Terminal 2: Start backend
go run cmd/server/main.go

# Terminal 3: Start frontend
cd ../frontend
npm install
npm run dev
```

- Backend: `http://localhost:8080`
- Frontend: `http://localhost:5173`

See [backend/README.md](backend/README.md) and [frontend/README.md](frontend/README.md) for details.

## Architecture

```
┌──────────────────┐
│   Confab CLI     │ ← Runs on user's machine
│                  │   - Captures sessions via hook
│  Local Storage:  │   - Stores in SQLite
│  ~/.confab/      │   - Optionally uploads to cloud
└────────┬─────────┘
         │ HTTPS (optional)
         ▼
┌──────────────────┐
│  Backend Service │ ← Cloud/self-hosted server
│                  │   - PostgreSQL database
│  - REST API      │   - MinIO object storage
│  - Multi-user    │   - API key auth
│  - S3 Storage    │
└──────────────────┘
```

## Features

### CLI
- ✅ Automatic session capture via SessionEnd hook
- ✅ Local SQLite storage
- ✅ Agent sidechain tracking
- ✅ Session resumption support
- ✅ Cloud sync (optional)
- ✅ Structured logging

### Backend
- ✅ PostgreSQL 18 database
- ✅ MinIO S3-compatible storage
- ✅ API key authentication
- ✅ Multi-user support
- ✅ Docker Compose for local dev

## Project Structure

```
confab/
├── cli/                    # CLI tool (Go)
│   ├── cmd/               # Commands (init, save, status, cloud, etc.)
│   ├── pkg/               # Packages (db, discovery, logger, upload)
│   ├── main.go
│   ├── install.sh
│   └── README.md
│
├── backend/               # Backend service (Go)
│   ├── cmd/server/       # Server entry point
│   ├── internal/         # Internal packages
│   │   ├── api/         # HTTP handlers
│   │   ├── auth/        # OAuth & sessions
│   │   ├── db/          # Database layer
│   │   ├── models/      # Data models
│   │   └── storage/     # S3/MinIO client
│   ├── docker-compose.yml
│   └── README.md
│
├── frontend/              # Web dashboard (SvelteKit)
│   ├── src/routes/       # Pages and routes
│   ├── src/app.css       # Minimal styling
│   ├── vite.config.ts    # Vite + backend proxy
│   └── README.md
│
├── BACKEND_PLAN.md       # Backend architecture docs
├── NOTES.md              # Development notes
└── LICENSE
```

## Use Cases

**Solo Developer (Local Only)**
```bash
cd cli && ./install.sh
# Sessions stored in ~/.confab/sessions.db
```

**Team / Multi-Device (Cloud Sync)**
```bash
# Install CLI on all machines
cd cli && ./install.sh

# Run backend on server or localhost
cd backend && docker-compose up -d && go run cmd/server/main.go

# Configure each CLI (or use 'confab login' for interactive auth)
confab configure --backend-url https://your-server.com --api-key <key>
```

## Development

Both CLI and backend are independent Go modules:

```bash
# CLI development
cd cli
go build
make test

# Backend development
cd backend
docker-compose up -d
go run cmd/server/main.go
```

## Roadmap

- [x] Local SQLite storage
- [x] SessionEnd hook integration
- [x] Agent sidechain discovery
- [x] Cloud backend service
- [x] API key authentication
- [ ] Full-text search
- [ ] Session analytics dashboard
- [ ] Hosted SaaS version
- [ ] Export formats (JSON, Markdown)
- [ ] Compression (tar.zstd)

## License

MIT
