package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type setPasswordRequest struct {
	Password string `json:"password"`
}

// setPassword handles POST /machines/{id}/password.
// It SSHes into the target machine and runs `sudo -S chpasswd`, piping the
// machine's current SSHSudoPw (for sudo auth) followed by "user:newpassword"
// on stdin — the same technique screentime-timer.py uses for its lock/unlock
// feature. On success, the machine's SSHSudoPw is updated in-memory to the
// new password so subsequent sudo-dependent actions (shutdown, screentime
// lock) keep working for the rest of this process's lifetime. This does NOT
// persist to the remote .env — update JOSEPH_SUDO_PW there too (e.g. via
// `make set-remote-env`) before the next deploy/restart, or it'll revert to
// the old password.
func (h *MachineHandler) setPassword(w http.ResponseWriter, r *http.Request) {
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
	if m.SSHSudoPw == "" {
		writeError(w, http.StatusUnprocessableEntity, "machine has no sudo password on file — cannot authenticate chpasswd")
		return
	}

	var req setPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Password) == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}

	keyBytes, err := os.ReadFile(m.SSHKeyPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("could not read SSH key: %v", err))
		return
	}
	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not parse SSH key")
		return
	}

	cfg := &ssh.ClientConfig{
		User:            m.SSHUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // acceptable on a trusted LAN
		Timeout:         5 * time.Second,
	}
	client, err := ssh.Dial("tcp", m.IP+":22", cfg)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("SSH dial failed: %v", err))
		return
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not open SSH session")
		return
	}
	defer sess.Close()

	// sudo -S consumes the first stdin line for its own password prompt, then
	// forwards the rest of stdin to chpasswd untouched.
	sess.Stdin = strings.NewReader(m.SSHSudoPw + "\n" + m.SSHUser + ":" + req.Password + "\n")

	if err := sess.Run("sudo -S chpasswd"); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("password change failed: %v", err))
		return
	}

	m.SSHSudoPw = req.Password
	h.store.Update(id, m)

	writeJSON(w, http.StatusOK, map[string]string{"status": "password updated"})
}
