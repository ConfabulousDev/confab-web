# Confab

Archive and search your Claude Code sessions in the cloud.

**This repo contains:**
- **[backend/](backend/)** - Cloud backend service (PostgreSQL + MinIO)
- **[frontend/](frontend/)** - React web dashboard

**See also:** [confab-cli](https://github.com/ConfabulousDev/confab-cli) - Command-line tool for capturing and uploading sessions

## Running Locally

This guide walks you through running the full Confab stack on your local machine.

### Prerequisites

- **Docker & Docker Compose** - For PostgreSQL and MinIO
- **Go 1.21+** - For the backend
- **Node.js 18+** - For the frontend
- **Confab CLI** - For capturing sessions

### Step 1: Start Backend Services

**Option A: Docker Compose (easiest)**

```bash
# Start everything (PostgreSQL, MinIO, backend, worker)
docker-compose up -d

# View logs
docker-compose logs -f app
```

The backend runs at `http://localhost:8080`

**Default credentials:**
- Email: `admin@local.dev`
- Password: `localdevpassword`

**Option B: Run backend locally (for development)**

```bash
# Copy environment template
cp backend/.env.example backend/.env

# Start only databases
docker-compose up -d postgres minio minio-setup migrate

# Start the backend server
cd backend
go run cmd/server/main.go
```

**Default credentials** (from `backend/.env`):
- Email: `admin@example.com`
- Password: `change-me-immediately`

### Step 2: Start Frontend

If using Docker Compose Option A, the frontend is served from the backend at `http://localhost:8080`.

For frontend development with hot-reload:

```bash
cd frontend

# Install dependencies
npm install

# Start development server
npm run dev
```

The frontend dev server runs at `http://localhost:5173`

### Step 3: Install and Configure CLI

```bash
# Install the Confab CLI
git clone https://github.com/ConfabulousDev/confab-cli.git
cd confab-cli
./install.sh

# Configure CLI to use your local backend
confab setup --backend-url http://localhost:8080
```

The setup command will:
1. Open a browser to authenticate with your local backend
2. Create an API key for the CLI
3. Install the Claude Code hook to capture sessions

### Step 4: Verify Setup

1. **Log in to the frontend** at http://localhost:8080 (or http://localhost:5173 if running frontend separately) using the admin credentials
2. **Start a Claude Code session** - it will be automatically captured
3. **View your session** in the frontend dashboard

### Configuration

The backend is configured via environment variables in `backend/.env`:

| Variable | Default | Description |
|----------|---------|-------------|
| `AUTH_PASSWORD_ENABLED` | `true` | Enable username/password authentication |
| `ADMIN_BOOTSTRAP_EMAIL` | `admin@example.com` | Initial admin email |
| `ADMIN_BOOTSTRAP_PASSWORD` | `change-me-immediately` | Initial admin password |
| `FRONTEND_URL` | `http://localhost:5173` | Frontend URL for redirects |
| `ALLOWED_ORIGINS` | `http://localhost:5173` | CORS allowed origins |

See `backend/.env.example` for all available options including OAuth, email, and more.

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

### Backend

```bash
cd backend
docker-compose up -d      # Start databases
go run cmd/server/main.go # Start server
go test ./...             # Run tests
```

### Frontend

```bash
cd frontend
npm install       # Install dependencies
npm run dev       # Start dev server
npm run build     # Build for production
npm run lint      # Run linter
npm test          # Run tests
```

### Running Tests

```bash
# Backend unit tests (fast)
cd backend && go test -short ./...

# Backend integration tests (requires Docker)
cd backend && go test ./...

# Frontend tests
cd frontend && npm test
```

## Deployment

See [backend/README.md](backend/README.md) for production deployment options including Fly.io.

## License

MIT
