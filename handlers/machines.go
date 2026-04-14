package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jonwhittlestone/tools-onoffapi/models"
)

// MachineHandler holds the shared store and exposes HTTP handler methods.
// This is the Go equivalent of a FastAPI router class or Django class-based view.
type MachineHandler struct {
	store *models.Store
}

// NewMachineHandler creates a handler wired to the given store.
func NewMachineHandler(store *models.Store) *MachineHandler {
	return &MachineHandler{store: store}
}

// RegisterRoutes attaches all /machines routes to the given ServeMux.
// Go 1.22 supports "METHOD /path" patterns directly — no external router needed.
func (h *MachineHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /machines", h.listMachines)
	mux.HandleFunc("GET /machines/{id}", h.getMachine)
	mux.HandleFunc("POST /machines", h.createMachine)
	mux.HandleFunc("PUT /machines/{id}", h.updateMachine)
	mux.HandleFunc("DELETE /machines/{id}", h.deleteMachine)
	mux.HandleFunc("POST /machines/{id}/wake", h.wake)
	mux.HandleFunc("POST /machines/{id}/shutdown", h.shutdown)
}

// listMachines handles GET /machines
// Equivalent to: @app.get("/machines") in FastAPI
func (h *MachineHandler) listMachines(w http.ResponseWriter, r *http.Request) {
	machines := h.store.GetAll()
	writeJSON(w, http.StatusOK, machines)
}

// getMachine handles GET /machines/{id}
func (h *MachineHandler) getMachine(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id") // Go 1.22: extracts {id} from the URL pattern
	machine, ok := h.store.GetByID(id)
	if !ok {
		writeError(w, http.StatusNotFound, "machine not found")
		return
	}
	writeJSON(w, http.StatusOK, machine)
}

// createMachine handles POST /machines
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

// updateMachine handles PUT /machines/{id}
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

// deleteMachine handles DELETE /machines/{id}
func (h *MachineHandler) deleteMachine(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !h.store.Delete(id) {
		writeError(w, http.StatusNotFound, "machine not found")
		return
	}
	w.WriteHeader(http.StatusNoContent) // 204 — success, no body
}

// --- helpers ---

// writeJSON sets Content-Type, status code, and encodes v as JSON.
// Equivalent to FastAPI's JSONResponse or Django's JsonResponse.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

// writeError sends a standard error envelope: {"error": "message"}
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
