package handlers

import (
	"fmt"
	"net/http"
	"strings"
)

// unlockScreen handles POST /machines/{id}/unlock-screen.
//
// It SSHes in as the machine's own login user (no sudo, no password needed)
// and asks systemd-logind to unlock that user's active graphical session.
// logind's Session.Unlock() emits an "Unlock" D-Bus signal that GNOME Shell's
// screen shield listens for and responds to by dismissing the lock screen —
// the same mechanism biometric/smartcard unlock plugins use. It works
// without ever knowing the account's actual login password, and stays
// correct even after that password changes (e.g. via the SET PASSWORD
// feature), since it never touches the password at all.
func (h *MachineHandler) unlockScreen(w http.ResponseWriter, r *http.Request) {
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

	if err := screentimeSSH(m.SSHUser, m.SSHKeyPath, m.IP, "loginctl unlock-session "+sessionID); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("unlock failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "screen unlocked"})
}

// parseActiveSessionID scans `loginctl list-sessions --no-legend` output for
// the given user's session on seat0 — the physical/graphical session, as
// opposed to a headless SSH session, which has no seat.
func parseActiveSessionID(output, user string) string {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		if fields[2] == user && fields[3] == "seat0" {
			return fields[0]
		}
	}
	return ""
}
