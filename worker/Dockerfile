# ---- Build stage ----
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files first to leverage build cache
COPY backend/go.mod backend/go.sum /app/backend/
COPY worker/go.mod worker/go.sum /app/worker/

WORKDIR /app/worker
RUN go mod download

# Copy source directories
COPY backend /app/backend
COPY worker /app/worker

# Build static binary for the container architecture
RUN CGO_ENABLED=0 GOOS=linux go build -o /worker ./main.go

# ---- Runtime stage ----
FROM alpine:3.19

# Install base packages
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /worker /app/worker

CMD ["/app/worker"]
