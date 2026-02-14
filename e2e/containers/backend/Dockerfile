# Backend-only Dockerfile for E2E tests (no frontend needed)

FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

# Copy go mod files from backend directory
COPY backend/go.mod backend/go.sum ./

RUN go mod download

# Copy backend source
COPY backend/ ./

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -o confab ./cmd/server

# Runtime image
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata curl

# Create non-root user
RUN adduser -D -h /app appuser

WORKDIR /app

COPY --from=builder /app/confab ./

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=5s --timeout=3s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

CMD ["./confab"]
