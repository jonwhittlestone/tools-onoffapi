# tools-onoffapi — Design Spec

## Overview

A Go REST API hosted on `doylestonex` (Raspberry Pi, `zapto.howapped.org`) that allows remote power control of home machines — starting with `doylestone02` (192.168.0.203, MAC `58:47:ca:70:62:27`).

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
curl -H "X-API-Key: your-key" https://zapto.howapped.org/onoffapi/machines

# Missing key → 401
curl https://zapto.howapped.org/onoffapi/machines
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

Base URL: `https://zapto.howapped.org/onoffapi`

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

**Files added:** `deploy/deploy.sh`

```bash
make deploy   # builds, rsyncs, restarts container on doylestonex
```

---

## Feature 2 — Simple Frontend (next)

A minimal HTML/JS single-page app served by the Go API itself (no separate server needed). Shows a list of registered machines with Wake / Shutdown buttons.

Served at: `GET /` → serves `static/index.html`

Tech: vanilla HTML + fetch() calls to the REST API. No React/Vue — keep it simple and deployable as static files embedded in the Go binary.

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
