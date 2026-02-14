# Multi-stage Dockerfile for Confab (Go backend + React frontend)

# Stage 1: Build Frontend
FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend

# Copy package files
COPY frontend/package*.json ./

# Install dependencies
RUN npm ci

# Copy frontend source
COPY frontend/ ./

# Build static files
RUN npm run build

# Stage 2: Build Backend
FROM golang:1.25-alpine AS backend-builder

WORKDIR /app/backend

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY backend/go.mod backend/go.sum ./

# Download dependencies
RUN go mod download

# Copy backend source
COPY backend/ ./

# Build binary
ARG TARGETARCH
ARG VERSION
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -a -installsuffix cgo \
    -ldflags "-X main.version=${VERSION}" -o confab ./cmd/server

# Stage 3: Migrate CLI
FROM migrate/migrate:v4.19.1 AS migrate-cli

# Stage 4: Final Runtime Image
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN adduser -D -h /app appuser

WORKDIR /app

# Copy backend binary from builder
COPY --from=backend-builder /app/backend/confab ./

# Copy frontend static files from builder
COPY --from=frontend-builder /app/frontend/dist ./static

# Copy migrate CLI and migration files
COPY --from=migrate-cli /migrate /usr/local/bin/migrate
COPY backend/internal/db/migrations/*.sql /app/migrations/
COPY migrate_db.sh /app/migrate_db.sh
RUN chmod +x /app/migrate_db.sh

# Set environment variable for static files
ENV STATIC_FILES_DIR=/app/static

USER appuser

# Expose port
EXPOSE 8080

# Run the application
CMD ["./confab"]
