package handlers

import (
	"fmt"
	"net/http"
)

// lockScreen handles POST /machines/{id}/lock-screen.
//
// Mirror image of unlockScreen (handlers/unlockscreen.go): SSHes in as the
// machine's own login user and asks systemd-logind to lock that user's
// active graphical session via `loginctl lock-session`, which emits the
// D-Bus "Lock" signal the desktop's screen shield listens for.
func (h *MachineHandler) lockScreen(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, ok := h.store.GetByID(id)
	if !ok {
		writeError(w, http.StatusNotFound, "machine not found")
		return
	}
	if m.SSHUser == "" || m.SSHKeyPath == "" {
		writeError(w, http.StatusUnprocessableEntity, "machine has no SSH credentials")
		return
	}

	out, err := screentimeSSHOutput(m.SSHUser, m.SSHKeyPath, m.IP, "loginctl list-sessions --no-legend")
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("could not list sessions: %v", err))
		return
	}
	sessionID := parseActiveSessionID(out, m.SSHUser)
	if sessionID == "" {
		writeError(w, http.StatusNotFound, "no active graphical session found for this user")
		return
	}

	if err := screentimeSSH(m.SSHUser, m.SSHKeyPath, m.IP, "loginctl lock-session "+sessionID); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("lock failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "screen locked"})
}
