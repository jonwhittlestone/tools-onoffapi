package handlers

import (
	"net"
	"net/http"
	"time"
)

// ping handles GET /machines/{id}/ping.
// It attempts a TCP dial to the machine's SSH port (22) with a 2-second timeout.
// Returns {"reachable":true} if the port answers, {"reachable":false} otherwise.
// Used by the frontend to enable/disable Wake and Shutdown buttons.
func (h *MachineHandler) ping(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, ok := h.store.GetByID(id)
	if !ok {
		writeError(w, http.StatusNotFound, "machine not found")
		return
	}

	conn, err := net.DialTimeout("tcp", m.IP+":22", 2*time.Second)
	reachable := err == nil
	if conn != nil {
		conn.Close()
	}

	w.Header().Set("Content-Type", "application/json")
	if reachable {
		w.Write([]byte(`{"reachable":true}`)) //nolint:errcheck
	} else {
		w.Write([]byte(`{"reachable":false}`)) //nolint:errcheck
	}
}
