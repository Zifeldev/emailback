# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files (from project root)
COPY go.mod go.sum ./
RUN go mod download

# Copy Email Service source code
COPY service/EmailParse/ ./service/EmailParse/

# Build the application and aiworker
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o email-service ./service/EmailParse/cmd/main.go && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o aiworker ./service/EmailParse/cmd/aiworker/main.go

# Runtime stage
FROM alpine:3.20

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata wget

# Create non-root user
RUN addgroup -S app && adduser -S -G app appuser

WORKDIR /app

# Copy binaries from builder
COPY --from=builder /build/email-service /app/email-service
COPY --from=builder /build/aiworker /app/aiworker

# Copy static files
COPY service/EmailParse/static/ /app/static/

# Change ownership
RUN chown -R appuser:app /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

# Run the application
ENTRYPOINT ["/app/email-service"]
