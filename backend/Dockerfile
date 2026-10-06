# ---- Build stage ----
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy dependency files first to cache layer
COPY go.mod go.sum ./
RUN go mod download

# Copy full source code
COPY . .

# Build static binary for the container architecture (Railway environment)
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server

# ---- Runtime stage ----
FROM alpine:3.19

# Install base dependencies
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /server /app/server

EXPOSE 8080

# Starts the API server, which automatically executes embedded migrations first
CMD ["/app/server"]
