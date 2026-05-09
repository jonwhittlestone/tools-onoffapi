package handlers

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

// suspend handles POST /machines/{id}/suspend.
// SSHes into the target and runs `sudo systemctl suspend`. Unlike poweroff,
// suspend returns cleanly before the session drops, so errors are real failures.
func (h *MachineHandler) suspend(w http.ResponseWriter, r *http.Request) {
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
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
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

	if err := sess.Run("sudo systemctl suspend"); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("suspend command failed: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"suspend command sent"}`)) //nolint:errcheck
}
