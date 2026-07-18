.PHONY: run build test fmt lint clean docker-build docker-up docker-down deploy set-remote-env show-remote-env

BINARY=bin/onoffapi
PORT?=8080

## ── Local development ──────────────────────────────────────────────────────

# Start the server locally (requires API_KEY env var)
run:
	@echo "Starting onoffapi on :$(PORT)..."
	@API_KEY=$${API_KEY:-devkey} PORT=$(PORT) go run main.go

# Compile to a binary (Go compiles to a single static executable — no interpreter needed)
build:
	@mkdir -p bin
	go build -o $(BINARY) main.go
	@echo "Binary written to $(BINARY)"

# Run all tests
test:
	go test ./... -v

# Run tests with coverage report
test-cover:
	go test ./... -cover

# Format all Go files (gofmt is the standard — no arguments, no debate)
fmt:
	gofmt -w .

# Run staticcheck linter (install once: go install honnef.co/go/tools/cmd/staticcheck@latest)
lint:
	staticcheck ./...

# Remove compiled binary
clean:
	rm -rf bin/

## ── Smoke tests (server must be running) ───────────────────────────────────

KEY?=devkey

health:
	curl -s http://localhost:$(PORT)/health | jq .

list:
	curl -s -H "X-API-Key: $(KEY)" http://localhost:$(PORT)/machines | jq .

get:
	curl -s -H "X-API-Key: $(KEY)" http://localhost:$(PORT)/machines/doylestone02 | jq .

## ── Docker ─────────────────────────────────────────────────────────────────

docker-build:
	docker-compose build

docker-up:
	docker-compose up -d
	@echo "Waiting for container..."
	@sleep 3
	@curl -s http://localhost:8082/health | jq .

docker-down:
	docker-compose down

## ── Deploy to doylestonex ──────────────────────────────────────────────────

deploy:
	@bash deploy/deploy.sh

# .env on doylestonex is untracked and never synced by deploy.sh (it holds real
# secrets; local .env has throwaway dev values). Set/update vars there with:
#   make set-remote-env NAME=JOSEPH_SUDO_PW VALUE=secret
REMOTE_HOST=doylestonex
REMOTE_ENV_FILE=/home/admin/www/tools-onoffapi/.env

set-remote-env:
	@if [ -z "$(NAME)" ] || [ -z "$(VALUE)" ]; then \
		echo "Usage: make set-remote-env NAME=VAR_NAME VALUE=var_value"; \
		exit 1; \
	fi
	@ssh $(REMOTE_HOST) "grep -q '^$(NAME)=' $(REMOTE_ENV_FILE) 2>/dev/null \
		&& sed -i 's|^$(NAME)=.*|$(NAME)=$(VALUE)|' $(REMOTE_ENV_FILE) \
		|| echo '$(NAME)=$(VALUE)' >> $(REMOTE_ENV_FILE)"
	@echo "Set $(NAME) in $(REMOTE_HOST):$(REMOTE_ENV_FILE)"

# List keys (not values) currently set in the remote .env
show-remote-env:
	@ssh $(REMOTE_HOST) "cut -d= -f1 $(REMOTE_ENV_FILE)"
