# tools-onoffapi — Design Spec

## Overview

A Go REST API hosted on `doylestonex` (Raspberry Pi, `howapped.zapto.org`) that allows remote power control of home machines — starting with `doylestone02` (192.168.0.203, MAC `58:47:ca:70:62:27`).

The project is built in three features, each shipped as a series of incremental commits to aid learning Go.

---

## Target Machine Context

| Machine | IP | MAC | Notes |
|---------|----|-----|-------|
| doylestone02 | 192.168.0.203 | 58:47:ca:70:62:27 | Auto-shuts down 23:59 via systemd timer. WoL enabled via nmcli. |

**doylestone02 SSH:**
```bash
ssh -i ~/.ssh/id_doylestone02 jon@192.168.0.203
```

**doylestone02 shutdown (remote):**
```bash
ssh -i ~/.ssh/id_doylestone02 jon@192.168.0.203 "sudo poweroff"
```

---

## Prerequisites

### 1. Install Go

Download the latest stable Go from https://go.dev/dl/

**On Linux/Pi:**
```bash
wget https://go.dev/dl/go1.22.4.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.22.4.linux-amd64.tar.gz

# Add to ~/.bashrc or ~/.zshrc:
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin

source ~/.bashrc
go version  # should print go1.22.x
```

**On Mac:**
```bash
brew install go
go version
```

### 2. Install VS Code Go extension

In VS Code: Extensions → search **"Go"** → install the official Go extension by the Go Team at Google.

On first open of a `.go` file, VS Code will prompt to install Go tools — accept all. This installs:
- `gopls` — language server (autocomplete, go to definition)
- `dlv` — debugger
- `staticcheck` — linter
- `goimports` — auto-format + auto-import

**Disable format-on-save for Markdown files**

`gopls` analyses Go code blocks inside `.md` files as if they were real Go code. It will flag illustrative snippets (e.g. an import block shown in isolation) as errors and `goimports` will silently strip those lines on save.

Add the following to `.vscode/settings.json` to prevent this:

```json
"[markdown]": {
    "editor.formatOnSave": false
}
```

### 3. Install Docker (for deployment)

```bash
# Ubuntu/Pi
sudo apt update && sudo apt install -y docker.io docker-compose
sudo usermod -aG docker $USER
# Log out and back in
docker --version
```

**Mac:** Install Docker Desktop from https://docker.com

### 4. Install make

```bash
# Ubuntu
sudo apt install make

# Mac (already present via Xcode tools)
make --version
```

### 5. Install wakeonlan (for Feature 3, on doylestonex)

```bash
sudo apt install wakeonlan
```

---

## Dependencies philosophy

This project uses **zero external Go packages** — only the standard library.

Rationale: Go's standard library is unusually complete. `net/http` (Go 1.22) has built-in method+path routing, `encoding/json` handles all serialisation, and `net/http/httptest` provides a full test harness. Adding a framework like Gin or Echo would reduce boilerplate slightly but obscure what Go itself is doing — which defeats the learning goal.

When you're comfortable with standard library patterns, Gin/Chi are worth evaluating for larger projects.

---

## Security

All routes except `GET /health` require an `X-API-Key` header. The server refuses to start if `API_KEY` is not set in the environment.

```bash
# Correct call
curl -H "X-API-Key: your-key" https://howapped.zapto.org/onoffapi/machines

# Missing key → 401
curl https://howapped.zapto.org/onoffapi/machines
```

**Generating a strong key:**
```bash
openssl rand -hex 32
```

Set this value as `API_KEY` in `.env` on doylestonex and in the GitHub Actions secret (`Settings → Secrets`) if you ever need it in CI.

---

## Go Concepts Cheatsheet (for FastAPI/Django developers)

| Python / FastAPI concept | Go equivalent |
|--------------------------|---------------|
| `pip install` | `go get` |
| `requirements.txt` | `go.mod` + `go.sum` |
| virtual environment | Go modules handle isolation |
| `@app.get("/path")` | `mux.HandleFunc("GET /path", handler)` |
| `def handler(req, res)` | `func handler(w http.ResponseWriter, r *http.Request)` |
| `pydantic` model | `struct` with json tags |
| `pytest` | `go test ./...` |
| `uvicorn main:app` | `go run main.go` |
| `docker build` | same |

**Key Go rules for newcomers:**
- Variables declared but not used → **compile error**
- Imports not used → **compile error**
- `gofmt` / `goimports` handles formatting — never argue with it
- Error handling is explicit: `if err != nil { ... }` — no exceptions
- Exported names (public) start with **capital letter**: `Machine`, `GetAll`
- Unexported (private) start lowercase: `store`, `findByID`

---

## Project Structure

```
tools-onoffapi/
├── main.go                    # Entry point — starts HTTP server, registers routes
├── go.mod                     # Module definition (like package.json)
├── go.sum                     # Dependency checksums (auto-managed, commit this)
├── Makefile                   # build / run / test shortcuts
├── Dockerfile                 # Multi-stage build → small production image
├── docker-compose.yml         # Local + production container config
├── .github/
│   └── workflows/
│       └── ci.yml             # GitHub Actions — runs tests on push
├── deploy/
│   └── deploy.sh              # rsync + SSH deploy to doylestonex
├── design/
│   └── spec.md                # This file
├── handlers/
│   ├── machines.go            # HTTP handler functions for /machines routes
│   └── machines_test.go       # Unit tests using net/http/httptest
└── models/
    └── machine.go             # Machine struct + in-memory store
```

---

## API Design

Base URL: `https://howapped.zapto.org/onoffapi`

### Health

| Method | Path | Response |
|--------|------|----------|
| GET | `/health` | `{"status": "ok"}` |

### Machines (Feature 1 — CRUD)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/machines` | List all machines |
| GET | `/machines/{id}` | Get one machine |
| POST | `/machines` | Register a new machine |
| PUT | `/machines/{id}` | Update a machine |
| DELETE | `/machines/{id}` | Remove a machine |

### Power Control (Feature 3)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/machines/{id}/wake` | Send WoL magic packet |
| POST | `/machines/{id}/shutdown` | SSH shutdown command |

---

## Machine Model

```json
{
  "id": "doylestone02",
  "name": "doylestone02",
  "ip": "192.168.0.203",
  "mac": "58:47:ca:70:62:27",
  "ssh_user": "jon",
  "ssh_key_path": "/home/jon/.ssh/id_doylestone02",
  "notes": "Gaming/media PC. Auto-shuts down at 23:59."
}
```

---

## Incremental Commit Plan

Each commit is a self-contained, working step. Build and test after each one.

---

### Commit 1 — Hello Go: module init + health endpoint

**What you learn:** Go module system, basic HTTP server, JSON response, `http.HandleFunc`

**Files added:** `go.mod`, `main.go`

`main.go` at this commit imports **only standard library** packages — no local packages exist yet.

```go
// main.go
package main

import (
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	log.Println("onoffapi listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
```

```bash
go mod tidy          # nothing to fetch — stdlib only
go run main.go
curl localhost:8080/health
```

Expected: `{"status":"ok"}`

---

#### Relevant Docs

- [`net/http` — ServeMux](https://pkg.go.dev/net/http#ServeMux)
- [`net/http` — ServeMux.HandleFunc](https://pkg.go.dev/net/http#ServeMux.HandleFunc)
- [`net/http` — HandlerFunc](https://pkg.go.dev/net/http#HandlerFunc)

**What is a mux?**
`ServeMux` is an HTTP request *multiplexer* — it holds a table of URL patterns and their associated handlers. When a request arrives, it finds the most specific matching pattern and calls its handler. Think of it as Go's equivalent of FastAPI's `APIRouter` or Django's `urlpatterns`. You create one with `http.NewServeMux()` rather than using the struct's zero value directly.

**What is `HandleFunc`?**
`mux.HandleFunc(pattern, fn)` registers a plain function as the handler for a URL pattern. The function must have the signature `func(http.ResponseWriter, *http.Request)`. So yes — the second argument is a callback: Go calls it each time a request matches the pattern. In Commit 1 we pass an anonymous function inline; in later commits we pass named methods instead.

**`HandleFunc` vs `HandlerFunc` — easily confused**
| | What it is | What it does |
|---|---|---|
| `mux.HandleFunc(pattern, fn)` | A method on ServeMux | Registers a function as a route handler |
| `http.HandlerFunc(fn)` | A type adapter | Converts a plain function into an `http.Handler` interface value |

You use `HandleFunc` (the method) to register routes. You use `HandlerFunc` (the type) when a function expects an `http.Handler` but you only have a plain function — which comes up in Commit 5 when writing middleware.

#### Troubleshooting

**`go mod tidy` fails with "no matching versions for query latest" for `handlers` and `models`**

_Symptom:_ After checking out at Commit 1 and running `go mod tidy`, Go tries to resolve the `handlers` and `models` packages as external modules and fails:

```
go: github.com/jonwhittlestone/tools-onoffapi imports
        github.com/jonwhittlestone/tools-onoffapi/handlers: no matching versions for query "latest"
```

_Cause:_ The `main.go` committed at this step already imports `handlers` and `models` (the full, final version of the file was committed rather than the Commit 1-only health-endpoint version). `go mod tidy` sees those import paths and, because the directories don't exist yet, tries to fetch them as external modules — which don't exist on the module proxy.

_Fix:_ Commit 1's `main.go` should only contain the health endpoint, with no imports of local packages. The full `main.go` (with handlers/models wired up) belongs in Commit 3 or later, once those packages exist. If you've already committed the wrong file, either amend the commit or accept that `go mod tidy` must be run after Commits 2–3 are in place (i.e. once `handlers/` and `models/` directories exist on disk).

---

### Commit 2 — Machine model + in-memory store

**What you learn:** Go structs, JSON struct tags, maps as a simple in-memory data store, `sync.RWMutex` for concurrency safety, package organisation

**Files added:** `models/machine.go`

`main.go` is **unchanged** from Commit 1 — there are no HTTP routes for machines yet. The data layer exists but nothing wires it in.

```go
// models/machine.go
package models

import "sync"

type Machine struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IP         string `json:"ip"`
	MAC        string `json:"mac"`
	SSHUser    string `json:"ssh_user,omitempty"`
	SSHKeyPath string `json:"ssh_key_path,omitempty"`
	Notes      string `json:"notes,omitempty"`
}

type Store struct {
	mu       sync.RWMutex
	machines map[string]Machine
}

func NewStore() *Store {
	s := &Store{machines: make(map[string]Machine)}
	s.machines["doylestone02"] = Machine{
		ID:         "doylestone02",
		Name:       "doylestone02",
		IP:         "192.168.0.203",
		MAC:        "58:47:ca:70:62:27",
		SSHUser:    "jon",
		SSHKeyPath: "/home/jon/.ssh/id_doylestone02",
		Notes:      "Gaming/media PC. Auto-shuts down at 23:59 via systemd timer.",
	}
	return s
}

func (s *Store) GetAll() []Machine {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]Machine, 0, len(s.machines))
	for _, m := range s.machines {
		list = append(list, m)
	}
	return list
}

func (s *Store) GetByID(id string) (Machine, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.machines[id]
	return m, ok
}
```

```bash
go build ./...   # verifies models package compiles — no server changes yet
```

---

### Commit 3 — GET /machines and GET /machines/{id}

**What you learn:** Handler functions, path parameters (Go 1.22 `{id}` syntax), JSON encoding, 404 handling

**Files added:** `handlers/machines.go`
**Files modified:** `main.go`

`handlers/machines.go` at this commit has **read-only** handlers. Write operations come in Commit 4.

```go
// handlers/machines.go (Commit 3 — GET only)
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jonwhittlestone/tools-onoffapi/models"
)

type MachineHandler struct {
	store *models.Store
}

func NewMachineHandler(store *models.Store) *MachineHandler {
	return &MachineHandler{store: store}
}

// RegisterRoutes — GET routes only at this commit
func (h *MachineHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /machines", h.listMachines)
	mux.HandleFunc("GET /machines/{id}", h.getMachine)
}

func (h *MachineHandler) listMachines(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.store.GetAll())
}

func (h *MachineHandler) getMachine(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	machine, ok := h.store.GetByID(id)
	if !ok {
		writeError(w, http.StatusNotFound, "machine not found")
		return
	}
	writeJSON(w, http.StatusOK, machine)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
```

`main.go` is updated to import `handlers` and `models` — no middleware yet, so no `API_KEY` requirement.

```go
// main.go (Commit 3)
package main

import (
	"log"
	"net/http"

	"github.com/jonwhittlestone/tools-onoffapi/handlers"
	"github.com/jonwhittlestone/tools-onoffapi/models"
)

func main() {
	store := models.NewStore()
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	machineHandler := handlers.NewMachineHandler(store)
	machineHandler.RegisterRoutes(mux)

	log.Println("onoffapi listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
```

```bash
go run main.go
curl localhost:8080/health
curl localhost:8080/machines
curl localhost:8080/machines/doylestone02
curl localhost:8080/machines/unknown   # 404
```


### Troubleshooting

**Go code snippets in spec.md show red underlines and lines are removed on save**

_Symptom:_ Editing a Go code block in this file causes red underlines on import lines, and saving strips those lines silently.

_Cause:_ `gopls` analyses Go code blocks inside `.md` files as real Go. Illustrative snippets (e.g. an import block shown in isolation without the rest of the file) are flagged as errors, and `goimports` removes the "unused" imports on save — exactly as it would in a `.go` file.

_Fix:_ Add the following to `.vscode/settings.json` to disable format-on-save for Markdown files (see Prerequisites §2):

```json
"[markdown]": {
    "editor.formatOnSave": false
}
```

---

### Commit 4 — POST, PUT, DELETE handlers

**What you learn:** Reading request body (`json.NewDecoder`), HTTP status codes, mutating the store

**Files modified:** `handlers/machines.go`, `models/machine.go` (add `Create`, `Update`, `Delete` methods)

`main.go` is **unchanged** — `RegisterRoutes` is extended to cover all routes and the new store methods are called internally.

Add to `models/machine.go`:

```go
func (s *Store) Create(m Machine) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.machines[m.ID]; exists {
		return false
	}
	s.machines[m.ID] = m
	return true
}

func (s *Store) Update(id string, m Machine) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.machines[id]; !exists {
		return false
	}
	m.ID = id
	s.machines[id] = m
	return true
}

func (s *Store) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.machines[id]; !exists {
		return false
	}
	delete(s.machines, id)
	return true
}
```

Update `RegisterRoutes` and add write handlers to `handlers/machines.go`:

```go
func (h *MachineHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /machines", h.listMachines)
	mux.HandleFunc("GET /machines/{id}", h.getMachine)
	mux.HandleFunc("POST /machines", h.createMachine)
	mux.HandleFunc("PUT /machines/{id}", h.updateMachine)
	mux.HandleFunc("DELETE /machines/{id}", h.deleteMachine)
}

func (h *MachineHandler) createMachine(w http.ResponseWriter, r *http.Request) {
	var m models.Machine
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if m.ID == "" || m.Name == "" || m.IP == "" || m.MAC == "" {
		writeError(w, http.StatusBadRequest, "id, name, ip and mac are required")
		return
	}
	if !h.store.Create(m) {
		writeError(w, http.StatusConflict, "machine with that id already exists")
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (h *MachineHandler) updateMachine(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var m models.Machine
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !h.store.Update(id, m) {
		writeError(w, http.StatusNotFound, "machine not found")
		return
	}
	updated, _ := h.store.GetByID(id)
	writeJSON(w, http.StatusOK, updated)
}

func (h *MachineHandler) deleteMachine(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !h.store.Delete(id) {
		writeError(w, http.StatusNotFound, "machine not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

```bash
go run main.go

# Create
curl -X POST localhost:8080/machines \
  -H 'Content-Type: application/json' \
  -d '{"id":"test","name":"test","ip":"192.168.0.99","mac":"aa:bb:cc:dd:ee:ff"}'

# Update
curl -X PUT localhost:8080/machines/test \
  -H 'Content-Type: application/json' \
  -d '{"name":"test-updated","ip":"192.168.0.99","mac":"aa:bb:cc:dd:ee:ff"}'

# Delete
curl -X DELETE localhost:8080/machines/test
```

---

### Commit 5 — API key middleware

**What you learn:** Go middleware pattern — wrapping `http.Handler` to add cross-cutting behaviour, reading environment variables

**Files added:** `handlers/middleware.go`
**Files modified:** `main.go`

```go
// handlers/middleware.go
package handlers

import "net/http"

func RequireAPIKey(apiKey string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		key := r.Header.Get("X-API-Key")
		if key == "" || key != apiKey {
			writeError(w, http.StatusUnauthorized, "missing or invalid API key")
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

`main.go` gains the `API_KEY` env-var guard and wraps the mux with the middleware:

```go
// main.go (Commit 5 — final state)
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/jonwhittlestone/tools-onoffapi/handlers"
	"github.com/jonwhittlestone/tools-onoffapi/models"
)

func main() {
	store := models.NewStore()
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	machineHandler := handlers.NewMachineHandler(store)
	machineHandler.RegisterRoutes(mux)

	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		log.Fatal("API_KEY environment variable is required")
	}

	protected := handlers.RequireAPIKey(apiKey, mux)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("onoffapi listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, protected))
}
```

```bash
export API_KEY=devkey
go run main.go

curl localhost:8080/health                            # no auth required
curl localhost:8080/machines                          # 401 — missing key
curl -H 'X-API-Key: devkey' localhost:8080/machines  # 200
```

---

### Commit 6 — Unit tests

**What you learn:** Go's built-in `testing` package, `net/http/httptest` for handler testing, table-driven tests

**Files added:** `handlers/machines_test.go`, `handlers/middleware_test.go`

```bash
go test ./...         # run all tests
go test -v ./...      # verbose output
go test -cover ./...  # with coverage %
```

#### Using Delve for step debugging

Go's equivalent of Python's `ipdb` is [Delve](https://github.com/go-delve/delve) (`dlv`). The VS Code Go extension bundles it — no separate install needed.

Add `.vscode/launch.json` to the project root:

```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Run onoffapi",
            "type": "go",
            "request": "launch",
            "mode": "auto",
            "program": "${workspaceFolder}",
            "env": {
                "API_KEY": "devkey",
                "PORT": "8080"
            }
        }
    ]
}
```

Then:
1. Click in the gutter next to any line to set a breakpoint (red dot)
2. Press **F5** — VS Code compiles and launches with Delve attached
3. Hit the endpoint with `curl` — execution pauses at your breakpoint
4. Use the debug toolbar to step through code

| `ipdb` | VS Code + Delve |
|--------|----------------|
| `import ipdb; ipdb.set_trace()` | Click gutter to set breakpoint |
| `n` next line | F10 step over |
| `s` step into | F11 step into |
| `c` continue | F5 continue |
| `p variable` | Hover over variable, or the Variables panel |

---

### Commit 7 — Makefile

**What you learn:** Make targets as a project CLI — same pattern as the kaizen project

**Files added:** `Makefile`

```bash
make run      # go run main.go
make build    # compile binary to ./bin/onoffapi
make test     # go test ./...
make fmt      # gofmt + goimports
```

---

### Commit 8 — Dockerfile + docker-compose.yml

**What you learn:** Multi-stage Go Docker build (builder → minimal final image), how Go compiles to a single static binary

**Files added:** `Dockerfile`, `docker-compose.yml`

```bash
make docker-build
make docker-up
curl localhost:8082/health
make docker-down
```

Port `8082` chosen to avoid clashing with kaizen (3001) and browsernotes.

---

### Commit 9 — GitHub Actions CI

**What you learn:** CI pipeline for Go — install Go, run tests, report pass/fail on every push

**Files added:** `.github/workflows/ci.yml`

Push to GitHub — check the Actions tab. Tests run automatically on every PR and push to main.

---

### Commit 10 — Deploy script

**What you learn:** How to ship a Go binary to a remote server via rsync + SSH, then restart via docker-compose

**Files added:** `deploy/deploy.sh`, `deploy/onoffapi-traefik.yml`
**Files modified:** `docker-compose.yml`

```bash
make deploy   # rsyncs project, installs Traefik config, restarts container on doylestonex
```

#### Traefik configuration (doylestonex)

doylestonex runs Traefik v3 as a reverse proxy with the **file provider** — each app drops a `.yml` file into `~/traefik/config/dynamic/` and Traefik picks it up immediately via its file watcher (no restart needed).

**`deploy/onoffapi-traefik.yml`** defines two routers:

| Router | Entrypoint | Rule | Middlewares |
|--------|-----------|------|-------------|
| `onoffapi` | `web` (port 80) | `Host + PathPrefix(/onoffapi)` | `https-redirect` |
| `onoffapi-secure` | `websecure` (port 443) | `Host + PathPrefix(/onoffapi)` | `onoffapi-strip-prefix` + TLS |

The `stripPrefix` middleware rewrites `/onoffapi/health` → `/health` before forwarding to the container, so the Go router sees clean paths.

The service URL uses the Docker container name: `http://onoffapi:8080`. This works because all app containers join the shared `proxy` Docker network, which Traefik is also on.

**`docker-compose.yml`** already declares:
```yaml
networks:
  proxy:
    external: true   # shared Traefik proxy network
```

**`deploy/deploy.sh`** copies the Traefik config on every deploy:
```bash
scp deploy/onoffapi-traefik.yml "$REMOTE_USER@$REMOTE_HOST:$HOME/traefik/config/dynamic/onoffapi.yml"
```

After a successful deploy, the API is reachable at:
```
https://howapped.zapto.org/onoffapi/health
```

---

## Feature 2 — Simple Frontend

A minimal HTML/JS single-page app served by the Go API itself (no separate server needed). Shows a list of registered machines with Wake / Shutdown buttons.

Served at: `GET /` → serves `static/index.html`

Tech: vanilla HTML + `fetch()` calls to the REST API. No React/Vue — keep it simple and deployable as static files embedded in the Go binary.

---

## Feature 2 — Incremental Commit Plan

---

### Commit 11 — Embed static files + serve index.html

**What you learn:** Go's `//go:embed` directive, `embed.FS`, serving embedded static files with `http.FileServer` — the whole frontend ships inside the compiled binary, no separate file server needed.

**Files added:** `static/index.html`
**Files modified:** `main.go`, `handlers/middleware.go`, `deploy/deploy.sh`

```html
<!-- static/index.html — minimal shell, just proves the route works -->
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>onoffapi</title>
</head>
<body>
    <h1>onoffapi</h1>
    <p>Machine control panel — loading…</p>
</body>
</html>
```

Add to `main.go` (the `//go:embed` directive must appear immediately above the `var`):

```go
import (
    "embed"
    "io/fs"
    // ... existing imports ...
)

//go:embed static
var staticFiles embed.FS

func main() {
    // ... existing store / mux setup ...

    // Serve embedded static files at GET /
    staticFS, _ := fs.Sub(staticFiles, "static")
    mux.Handle("GET /", http.FileServer(http.FS(staticFS)))

    // ... existing route registration and server start ...
}
```

`handlers/middleware.go` — only `/machines` routes require an API key; the frontend and `/health` are public:

```go
import (
    "net/http"
    "strings"
)

func RequireAPIKey(apiKey string, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Public routes — no API key required
        if r.URL.Path == "/health" || !strings.HasPrefix(r.URL.Path, "/machines") {
            next.ServeHTTP(w, r)
            return
        }
        key := r.Header.Get("X-API-Key")
        if key == "" || key != apiKey {
            writeError(w, http.StatusUnauthorized, "missing or invalid API key")
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

`deploy/deploy.sh` — use `--build` to force `podman-compose` to rebuild the image from source on every deploy:

```bash
podman-compose down || true && podman-compose up -d --build
```

```bash
go run main.go
open http://localhost:8080        # shows the page, no API key needed
curl localhost:8080/machines      # 401 — still protected
```

#### Relevant Docs

- [`embed` package](https://pkg.go.dev/embed) — `//go:embed` directive and `embed.FS`
- [`io/fs.Sub`](https://pkg.go.dev/io/fs#Sub) — strips the `static/` prefix so `/index.html` is served at `/`, not `/static/index.html`
- [`http.FileServer`](https://pkg.go.dev/net/http#FileServer) — serves an `fs.FS` over HTTP

#### Troubleshooting

**`pattern static: no matching files found` at build time**

The `//go:embed static` directive requires the `static/` directory to exist and contain at least one file at compile time. Create `static/index.html` before running `go build` or `go run`.

**Deployed container serves stale binary after `make deploy`**

_Symptom:_ Source files are correct on doylestonex but the running container behaves as if built from old code.

_Cause:_ `podman-compose up -d` without `--build` reuses the cached image (`tools-onoffapi_onoffapi:latest`) from the previous build, even if source files have changed on disk.

_Fix:_ Always pass `--build` to force a rebuild: `podman-compose up -d --build`. This is already set in `deploy/deploy.sh`.

---

### Commit 12 — Machine list UI

**What you learn:** `fetch()` with custom headers, rendering JSON into the DOM, handling the API key from the browser.

**Files modified:** `static/index.html`
**Files added:** `static/style.css`

```html
<!-- static/index.html (Commit 12) -->
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>onoffapi</title>
    <link rel="stylesheet" href="style.css">
</head>
<body>
    <h1>Machines</h1>
    <div id="machines"></div>

    <script>
        const apiKey = localStorage.getItem('apiKey') || prompt('Enter API key:');
        localStorage.setItem('apiKey', apiKey);

        async function loadMachines() {
            const res = await fetch('/machines', {
                headers: { 'X-API-Key': apiKey }
            });
            if (!res.ok) { document.getElementById('machines').textContent = 'Error: ' + res.status; return; }
            const machines = await res.json();
            document.getElementById('machines').innerHTML = machines.map(m => `
                <div class="machine">
                    <h2>${m.name}</h2>
                    <p><strong>IP:</strong> ${m.ip}</p>
                    <p><strong>MAC:</strong> ${m.mac}</p>
                    ${m.notes ? `<p>${m.notes}</p>` : ''}
                </div>
            `).join('');
        }

        loadMachines();
    </script>
</body>
</html>
```

```css
/* static/style.css */
body { font-family: sans-serif; max-width: 800px; margin: 2rem auto; padding: 0 1rem; }
.machine { border: 1px solid #ddd; border-radius: 6px; padding: 1rem; margin: 1rem 0; }
.machine h2 { margin: 0 0 0.5rem; }
button { margin-right: 0.5rem; padding: 0.4rem 1rem; cursor: pointer; }
```

```bash
go run main.go
# Open http://localhost:8080 — enter API key when prompted, machine list renders
```

---

### Commit 12b — Login page

**What you learn:** Gating UI behind a localStorage-persisted session, replacing `prompt()` with a real HTML form, showing/hiding sections with `display` toggling.

**Files modified:** `static/index.html`, `static/style.css`

The API key doubles as the password. On first visit the login form is shown; on submit the key is stored in `localStorage` and the machine list loads. On subsequent visits the stored key is used directly — no login required. A logout button clears the key and returns to the login form.

```html
<!-- static/index.html (Commit 12b) -->
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>onoffapi</title>
    <link rel="stylesheet" href="./style.css">
</head>
<body>
    <div id="login">
        <h1>onoffapi</h1>
        <form id="login-form">
            <label for="password">API key</label>
            <input type="password" id="password" placeholder="Enter API key" autofocus>
            <button type="submit">Login</button>
        </form>
        <p id="login-error" class="error"></p>
    </div>

    <div id="app" style="display:none">
        <h1>Machines <button id="logout">Logout</button></h1>
        <div id="machines"></div>
    </div>

    <script>
        let apiKey = localStorage.getItem('apiKey');

        async function loadMachines() {
            const res = await fetch('./machines', {
                headers: { 'X-API-Key': apiKey }
            });
            if (!res.ok) {
                if (res.status === 401) { logout(); return; }
                document.getElementById('machines').textContent = 'Error: ' + res.status;
                return;
            }
            const machines = await res.json();
            document.getElementById('machines').innerHTML = machines.map(m => `
                <div class="machine">
                    <h2>${m.name}</h2>
                    <p><strong>IP:</strong> ${m.ip}</p>
                    <p><strong>MAC:</strong> ${m.mac}</p>
                    ${m.notes ? `<p>${m.notes}</p>` : ''}
                </div>
            `).join('');
        }

        function showApp() {
            document.getElementById('login').style.display = 'none';
            document.getElementById('app').style.display = '';
            loadMachines();
        }

        function logout() {
            localStorage.removeItem('apiKey');
            apiKey = null;
            document.getElementById('app').style.display = 'none';
            document.getElementById('login').style.display = '';
            document.getElementById('password').value = '';
        }

        document.getElementById('login-form').addEventListener('submit', async e => {
            e.preventDefault();
            apiKey = document.getElementById('password').value.trim();
            const res = await fetch('./machines', { headers: { 'X-API-Key': apiKey } });
            if (!res.ok) {
                document.getElementById('login-error').textContent = 'Invalid API key';
                return;
            }
            localStorage.setItem('apiKey', apiKey);
            showApp();
        });

        document.getElementById('logout').addEventListener('click', logout);

        if (apiKey) { showApp(); } else { document.getElementById('login').style.display = ''; }
    </script>
</body>
</html>
```

```css
/* additions to static/style.css */
#login { max-width: 360px; margin: 4rem auto; padding: 2rem; border: 1px solid #ddd; border-radius: 8px; }
#login h1 { margin-top: 0; }
#login input { display: block; width: 100%; padding: 0.5rem; margin: 0.5rem 0 1rem; box-sizing: border-box; }
.error { color: #c00; min-height: 1.2em; }
#logout { float: right; font-size: 0.8rem; }
```

```bash
go run main.go
# Open http://localhost:8080 — login form shown on first visit
# Enter API key → machine list renders
# Refresh — machine list shown directly (key persisted in localStorage)
```

---

### Commit 13 — Wake / Shutdown action buttons

**What you learn:** `fetch()` POST requests from the browser, button loading state, graceful error display. The endpoints themselves come in Feature 3 — buttons return 404 until then.

**Files modified:** `static/index.html`

Add Wake and Shutdown buttons to each machine card:

```javascript
// Replace the machine card innerHTML in loadMachines() with:
`<div class="machine">
    <h2>${m.name}</h2>
    <p><strong>IP:</strong> ${m.ip}</p>
    <p><strong>MAC:</strong> ${m.mac}</p>
    ${m.notes ? `<p>${m.notes}</p>` : ''}
    <button onclick="action('${m.id}', 'wake', this)">Wake</button>
    <button onclick="action('${m.id}', 'shutdown', this)">Shutdown</button>
    <span class="status" id="status-${m.id}"></span>
</div>`

// Add the action() function:
async function action(id, cmd, btn) {
    btn.disabled = true;
    const statusEl = document.getElementById('status-' + id);
    statusEl.textContent = cmd + '…';
    try {
        const res = await fetch('/machines/' + id + '/' + cmd, {
            method: 'POST',
            headers: { 'X-API-Key': apiKey }
        });
        statusEl.textContent = res.ok ? 'OK' : 'Error ' + res.status;
    } catch (e) {
        statusEl.textContent = 'Network error';
    } finally {
        btn.disabled = false;
    }
}
```

```css
/* Add to style.css */
.status { margin-left: 0.5rem; font-style: italic; color: #555; }
button:disabled { opacity: 0.5; cursor: not-allowed; }
```

```bash
go run main.go
# Wake/Shutdown buttons appear — return 404 until Feature 3 adds the routes
```

---

## Feature 3 — Wake and Shutdown (next)

### Wake (POST /machines/{id}/wake)

Sends a WoL magic packet to the machine's MAC address from doylestonex (which is on the same LAN via the router).

Implementation: Go `net` package constructs the 102-byte magic packet (6×`0xFF` + 16× MAC bytes) and sends as UDP broadcast to port 9.

No external tool needed — pure Go.

### Shutdown (POST /machines/{id}/shutdown)

SSHes to the target machine and runs `sudo poweroff`.

Implementation: `golang.org/x/crypto/ssh` package. Reads the SSH private key from `ssh_key_path` stored in the machine record and executes the shutdown command.

**Security note:** The API should be protected by a shared secret header (`X-API-Key`) before Feature 3 is deployed. Anyone who can reach the API endpoint would otherwise be able to shut down machines.

---

## Deployment on doylestonex

### Nginx reverse proxy config (matches kaizen pattern)

On doylestonex, the nginx config proxies `/onoffapi/` to the container on port 8082:

```nginx
location /onoffapi/ {
    proxy_pass http://localhost:8082/;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
}
```

### Container port allocation

| Project | Internal port | Host port |
|---------|--------------|-----------|
| kaizen | 3001 | 3001 |
| onoffapi | 8080 | 8082 |

---

## References

- doylestone02 SSH/WoL docs: `jw-mind/home/networking/doylestone02/main.md`
- doylestonex infra: https://github.com/jonwhittlestone/infra-wwwpi
- kaizen deploy pattern: `/home/jon/code/www/tools-kaizen/`
- Go tour (start here): https://go.dev/tour/
- Go standard library net/http: https://pkg.go.dev/net/http
