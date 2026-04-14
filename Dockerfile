# Multi-stage build — this is the standard Go Docker pattern.
#
# Stage 1 (builder): full Go toolchain, compiles the binary
# Stage 2 (final):   minimal Debian image, just copies in the binary
#
# Result: a ~15MB image instead of ~800MB. Go compiles to a single static
# binary with no runtime dependencies — unlike Python which needs the
# interpreter + all pip packages in the image.

# ── Stage 1: build ──────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy module files first — Docker caches this layer if go.mod/go.sum haven't changed.
# Equivalent to: COPY requirements.txt . && pip install -r requirements.txt
COPY go.mod go.sum* ./
RUN go mod download

# Copy source and compile
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o onoffapi main.go
#   CGO_ENABLED=0  → fully static binary (no C library dependency)
#   GOOS=linux     → cross-compile for Linux if building on Mac

# ── Stage 2: run ────────────────────────────────────────────────────────────
FROM debian:bookworm-slim

WORKDIR /app

COPY --from=builder /app/onoffapi .

# API_KEY must be provided at container start — see docker-compose.yml
ENV PORT=8080

EXPOSE 8080

CMD ["./onoffapi"]
