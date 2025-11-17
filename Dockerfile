# Multi-stage Dockerfile for Confab (Go backend + SvelteKit frontend)

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
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o confab ./cmd/server

# Stage 3: Final Runtime Image
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy backend binary from builder
COPY --from=backend-builder /app/backend/confab ./

# Copy frontend static files from builder
COPY --from=frontend-builder /app/frontend/build ./static

# Set environment variable for static files
ENV STATIC_FILES_DIR=/app/static

# Expose port
EXPOSE 8080

# Run the application
CMD ["./confab"]
