# Confabulous.dev

Archive and search your Claude Code sessions in the cloud.

**This repo contains:**
- **[backend/](backend/)** - Cloud backend service (PostgreSQL + MinIO)
- **[frontend/](frontend/)** - React web dashboard

**See also:** [confab](https://github.com/ConfabulousDev/confab) - Command-line tool for capturing and uploading sessions

## Running Locally

### Prerequisites

- **Docker & Docker Compose**

### Step 1: Start the Stack

```bash
# Start everything (PostgreSQL, MinIO, backend, worker)
docker-compose up -d

# View logs
docker-compose logs -f app
```

The app runs at `http://localhost:8080`

**Default credentials:**
- Email: `admin@local.dev`
- Password: `localdevpassword`

### Step 2: Install and Configure CLI

```bash
# Install the CLI (latest version)
curl -fsSL http://localhost:8080/install | bash

# Or install a specific version
curl -fsSL http://localhost:8080/install | CONFAB_VERSION=1.2.3 bash

# Configure to use your local backend
confab setup --backend-url http://localhost:8080
```

This will:
1. Download and install the Confab CLI
2. Open a browser to authenticate with your local backend
3. Create an API key and install the Claude Code hook

### Step 3: Verify Setup

1. **Log in** at http://localhost:8080 using the admin credentials
2. **Start a Claude Code session** - it will be automatically captured
3. **View your session** in the dashboard

### Configuration

Configuration is set in `docker-compose.yml`. Key settings:

| Variable | Default | Description |
|----------|---------|-------------|
| `ADMIN_BOOTSTRAP_EMAIL` | `admin@local.dev` | Initial admin email |
| `ADMIN_BOOTSTRAP_PASSWORD` | `localdevpassword` | Initial admin password |
| `FRONTEND_URL` | `http://localhost:8080` | Frontend URL for redirects |

See [`backend/.env.example`](backend/.env.example) for all available environment variables including OAuth, email, smart recap, worker, and deployment options.

### Admin Panel

Super admins can access the admin panel at `/admin/users` to:
- **View all users** with session counts and storage usage
- **Create new users** (when password authentication is enabled)
- **Activate/deactivate users**
- **Delete users** and all their data

To grant admin access, add the user's email to `SUPER_ADMIN_EMAILS` in `docker-compose.yml`:

```yaml
SUPER_ADMIN_EMAILS: admin@local.dev,another-admin@example.com
```

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
confab-web/
├── docker-compose.yml     # Local development stack
├── backend/               # Backend service (Go)
│   ├── cmd/server/       # Server entry point
│   ├── internal/         # Internal packages
│   │   ├── api/         # HTTP handlers
│   │   ├── auth/        # OAuth & API keys
│   │   ├── db/          # PostgreSQL layer
│   │   ├── storage/     # MinIO/S3 client
│   │   └── testutil/    # Test infrastructure
│   └── README.md
│
└── frontend/              # React web dashboard
    ├── src/pages/        # Pages and routes
    ├── src/services/     # API client
    └── README.md
```

See also: [confab](https://github.com/ConfabulousDev/confab) (separate repo)

## Development

For local development with hot-reload:

```bash
# Start databases only
docker-compose up -d postgres minio minio-setup migrate

# Backend (requires Go 1.21+)
cp backend/.env.example backend/.env
cd backend && go run cmd/server/main.go

# Frontend (requires Node.js 18+)
cd frontend && npm install && npm run dev
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
