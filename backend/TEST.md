# Backend Testing Guide

## Prerequisites

- Docker & Docker Compose installed
- Go 1.21+ installed
- Confab CLI built

## Step 1: Start Backend Services

```bash
cd backend
docker-compose up -d
```

Wait for services to be healthy:
- PostgreSQL: `localhost:5432`
- MinIO API: `localhost:9000`
- MinIO Console: `localhost:9001` (minioadmin/minioadmin)

## Step 2: Start Backend Server

```bash
cd backend
go run cmd/server/main.go
```

Server will start on `http://localhost:8080`

## Step 3: Create API Key

```bash
cd backend
go run scripts/create-api-key.go my-test-key
```

Save the API key that is printed.

## Step 4: Configure Confab CLI

```bash
cd ..
./confab cloud configure \
  --backend-url http://localhost:8080 \
  --api-key <your-api-key-from-step-3> \
  --enable
```

Check configuration:
```bash
./confab cloud status
```

## Step 5: Test Session Upload

Create a test session file:
```bash
cat > /tmp/test-session.jsonl << 'EOF'
{"type":"message","id":"msg_01","content":[{"type":"text","text":"test"}]}
EOF
```

Create a test hook input:
```bash
cat > /tmp/test-hook.json << 'EOF'
{
  "session_id": "test-session-123",
  "transcript_path": "/tmp/test-session.jsonl",
  "cwd": "/tmp",
  "reason": "user_exit"
}
EOF
```

Run the save command:
```bash
./confab save < /tmp/test-hook.json
```

## Step 6: Verify Upload

Check backend logs - you should see:
- Session metadata saved to PostgreSQL
- File uploaded to MinIO

Access MinIO Console at `http://localhost:9001`:
- Username: `minioadmin`
- Password: `minioadmin`
- Browse bucket: `confab`
- You should see: `1/test-session-123/test-session.jsonl`

## Step 7: Query Database

```bash
docker exec -it confab-postgres psql -U confab -d confab
```

Run queries:
```sql
-- View all sessions
SELECT * FROM sessions;

-- View all runs
SELECT * FROM runs;

-- View all files
SELECT * FROM files;

-- Join to see complete data
SELECT
  s.session_id,
  r.reason,
  r.end_timestamp,
  f.file_type,
  f.s3_key
FROM sessions s
JOIN runs r ON s.session_id = r.session_id
JOIN files f ON r.id = f.run_id;
```

## Cleanup

```bash
# Stop backend server (Ctrl+C)

# Stop docker services
cd backend
docker-compose down

# Optional: Remove volumes (deletes all data)
docker-compose down -v
```

## Troubleshooting

### Backend won't start
- Check if PostgreSQL is running: `docker ps`
- Check logs: `docker-compose logs postgres`

### Upload fails
- Verify API key is correct: `./confab cloud status`
- Check backend logs for errors
- Verify MinIO is running: `docker ps`

### Database connection issues
- Ensure PostgreSQL is healthy: `docker-compose ps`
- Check connection string in backend logs
