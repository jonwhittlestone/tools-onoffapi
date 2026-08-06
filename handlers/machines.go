package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jonwhittlestone/tools-onoffapi/models"
)

// MachineHandler holds the shared store and exposes HTTP handler methods.
type MachineHandler struct {
	store           *models.Store
	screentimeStore *screentimeStore
	networkMode     *models.NetworkModeStore
}

// NewMachineHandler creates a handler wired to the given store.
func NewMachineHandler(store *models.Store) *MachineHandler {
	return &MachineHandler{
		store:           store,
		screentimeStore: newScreentimeStore("/app/data/screentime.json"),
		networkMode:     models.NewNetworkModeStore(),
	}
}

// RegisterRoutes attaches all /machines routes to the given ServeMux.
func (h *MachineHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /machines", h.listMachines)
	mux.HandleFunc("GET /machines/{id}", h.getMachine)
	mux.HandleFunc("POST /machines", h.createMachine)
	mux.HandleFunc("PUT /machines/{id}", h.updateMachine)
	mux.HandleFunc("DELETE /machines/{id}", h.deleteMachine)
	mux.HandleFunc("POST /machines/{id}/wake", h.wake)
	mux.HandleFunc("POST /machines/{id}/shutdown", h.shutdown)
	mux.HandleFunc("POST /machines/{id}/suspend", h.suspend)
	mux.HandleFunc("GET /machines/{id}/ping", h.ping)
	mux.HandleFunc("GET /machines/{id}/screentime", h.getScreentime)
	mux.HandleFunc("POST /machines/{id}/screentime", h.startScreentime)
	mux.HandleFunc("DELETE /machines/{id}/screentime", h.stopScreentime)
	mux.HandleFunc("POST /machines/{id}/password", h.setPassword)
	mux.HandleFunc("POST /machines/{id}/unlock-screen", h.unlockScreen)
	mux.HandleFunc("POST /machines/{id}/lock-screen", h.lockScreen)
	mux.HandleFunc("GET /machines/{id}/volume", h.getVolume)
	mux.HandleFunc("POST /machines/{id}/volume", h.setVolume)
	mux.HandleFunc("POST /machines/{id}/mute", h.setMute)
	mux.HandleFunc("GET /network-mode", h.getNetworkMode)
	mux.HandleFunc("POST /network-mode", h.setNetworkMode)
}

func (h *MachineHandler) listMachines(w http.ResponseWriter, r *http.Request) {
	machines := h.store.GetAll()
	writeJSON(w, http.StatusOK, machines)
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
