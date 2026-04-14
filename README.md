# tools-onoffapi

A Go REST API for remote power control of home machines. Hosted on `doylestonex` (Raspberry Pi) behind nginx, accessible at `zapto.howapped.org`.

**Zero external dependencies** — uses only Go's standard library (`net/http`, `encoding/json`, `testing`).

---

## Quick start

### Prerequisites

1. **Install Go 1.22+**
   ```bash
   # Mac
   brew install go

   # Linux
   wget https://go.dev/dl/go1.22.4.linux-amd64.tar.gz
   sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.22.4.linux-amd64.tar.gz
   echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc && source ~/.bashrc
   go version
   ```

2. **Install VS Code Go extension**
   VS Code → Extensions → search `Go` → install the Go Team at Google extension.
   On first `.go` file open, accept all prompted tool installs (`gopls`, `dlv`, etc.)

3. **Install make** (Linux: `sudo apt install make` — Mac: already present)

4. **Install Docker** (for containerised deployment)

---

## Running locally

```bash
# 1. Copy env file and set an API key
cp .env.example .env
# Edit .env — set API_KEY to any string for local dev

# 2. Run
export API_KEY=devkey
make run

# 3. Test
make health                          # GET /health (no auth)
make list KEY=devkey                 # GET /machines
make get KEY=devkey                  # GET /machines/doylestone02
```

### Commit 1 — Hello Go: module init + health endpoint

**What you learn:** Go module system, basic HTTP server, JSON response, `http.HandleFunc`

**Files:** `go.mod`, `main.go`

```bash
# After writing files:
go mod tidy          # downloads dependencies (none yet)
go run main.go       # starts server on :8080
curl localhost:8080/health
```

Expected: `{"status":"ok"}`


---

## Go commands reference (for FastAPI/Django developers)

| Task | Go command | Equivalent |
|------|-----------|-----------|
| Start server | `go run main.go` | `uvicorn main:app` |
| Compile binary | `go build -o bin/onoffapi main.go` | n/a (Python is interpreted) |
| Run tests | `go test ./...` | `pytest` |
| Run tests (verbose) | `go test -v ./...` | `pytest -v` |
| Test with coverage | `go test -cover ./...` | `pytest --cov` |
| Format code | `gofmt -w .` | `black .` |
| Add a dependency | `go get github.com/foo/bar` | `pip install foo` |
| Tidy unused deps | `go mod tidy` | `pip-autoremove` |

---

## Project structure

```
tools-onoffapi/
├── main.go                    # Entry point — HTTP server setup
├── go.mod                     # Module definition (like package.json / pyproject.toml)
├── go.sum                     # Dependency checksums — commit this file
├── Makefile                   # Shortcuts for common tasks
├── Dockerfile                 # Multi-stage build → small production image
├── docker-compose.yml         # Container config (port 8082 on host)
├── .env.example               # Template for local env vars
├── .gitignore
├── .github/
│   └── workflows/ci.yml       # GitHub Actions — tests on every push
├── deploy/
│   └── deploy.sh              # rsync + SSH deploy to doylestonex
├── design/
│   └── spec.md                # Full feature spec and incremental commit plan
├── handlers/
│   ├── machines.go            # HTTP handlers for /machines routes
│   ├── machines_test.go       # Unit tests
│   ├── middleware.go          # X-API-Key auth middleware
│   └── middleware_test.go     # Middleware tests
└── models/
    └── machine.go             # Machine struct + in-memory store
```

---

## Security

All routes except `GET /health` require an `X-API-Key` header:

```bash
curl -H "X-API-Key: your-key" https://zapto.howapped.org/onoffapi/machines
```

Set `API_KEY` in the environment (or `.env` on the Pi) before starting the container. The server **refuses to start** if `API_KEY` is not set.

---

## Provisioning secrets (DR / fresh Pi setup)

If doylestonex is replaced or rebuilt, two secrets must be reprovisioned before the shutdown feature works. The API key is in `.env`; the SSH key must be created and authorised separately.

### 1 — API key

```bash
# On doylestonex — create or restore .env
echo "API_KEY=<your-key>" > /home/admin/www/tools-onoffapi/.env
```

### 2 — SSH shutdown key (`id_onoffapi_shutdown_doylestone02`)

This key is used by the running container to SSH into target machines and run `sudo poweroff`. It is mounted read-only into the container at deploy time — it is **never baked into the image**.

**On doylestonex** — generate the keypair:
```bash
ssh-keygen -t ed25519 -f /home/admin/.ssh/id_onoffapi_shutdown_doylestone02 -N ""
```

**On each target machine** (e.g. doylestone02) — authorise the public key:
```bash
ssh-copy-id -i /home/admin/.ssh/id_onoffapi_shutdown_doylestone02.pub jon@192.168.0.203
# verify:
ssh -i /home/admin/.ssh/id_onoffapi_shutdown_doylestone02 jon@192.168.0.203 "echo ok"
```

**On each target machine** — allow passwordless `sudo poweroff`. Must be run in an interactive terminal on the machine (cannot be done over a non-interactive SSH session):
```bash
echo 'jon ALL=(ALL) NOPASSWD: /sbin/poweroff, /usr/sbin/poweroff' | sudo tee /etc/sudoers.d/onoffapi-poweroff
sudo chmod 440 /etc/sudoers.d/onoffapi-poweroff
```

Confirm from doylestonex that it works without a password prompt:
```bash
ssh -i /home/admin/.ssh/id_onoffapi_shutdown_doylestone02 jon@192.168.0.203 'sudo -n poweroff'
```

The `deploy.sh` script mounts the key into the container automatically:
```bash
-v /home/admin/.ssh/id_onoffapi_shutdown_doylestone02:/home/admin/.ssh/id_onoffapi_shutdown_doylestone02:ro
```

No redeploy is needed after provisioning the key — the volume mount picks it up on the next container restart.

---

## Incremental commit sequence

Each commit is a working, testable step. See `design/spec.md` for full detail.

| # | What | Learn |
|---|------|-------|
| 1 | `go.mod` + health endpoint | Module init, basic HTTP server |
| 2 | `models/machine.go` | Structs, maps, RWMutex |
| 3 | GET /machines, GET /machines/{id} | Handlers, path params, JSON encoding |
| 4 | POST, PUT, DELETE | Body decoding, HTTP status codes |
| 5 | `middleware.go` — API key auth | Middleware pattern, env vars |
| 6 | Unit tests | `testing`, `httptest`, table-driven tests |
| 7 | Makefile | Build automation |
| 8 | Dockerfile + docker-compose | Multi-stage Go build, containers |
| 9 | GitHub Actions | CI for Go |
| 10 | `deploy/deploy.sh` | rsync, SSH, remote restart |

---

## Deploying to doylestonex

```bash
# First time on doylestonex — create the app directory and .env
ssh doylestonex
mkdir -p ~/apps/tools-onoffapi
echo "API_KEY=your-strong-key" > ~/apps/tools-onoffapi/.env

# Every subsequent deploy from your local machine:
make deploy
```

The nginx reverse proxy on doylestonex should be configured to forward `/onoffapi/` to `localhost:8082`. See `design/spec.md` for the nginx config block.

---

## Roadmap

- **Feature 1** (this) — CRUD REST API, auth, tests, deployment
- **Feature 2** — Simple HTML frontend served by the Go binary
- **Feature 3** — Wake-on-LAN magic packet + SSH shutdown via API
