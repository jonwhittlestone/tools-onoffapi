package handlers

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// shutdown handles POST /machines/{id}/shutdown.
// It SSHes into the target machine using the key stored at SSHKeyPath and
// runs `sudo poweroff`. The SSH connection will be severed by the shutdown
// itself, so any "process exited" error from sess.Run is expected and ignored.
// If SSHSudoPw is set on the machine, the password is piped via sudo -S so
// machines without passwordless sudo (e.g. EndlessOS) work correctly.
func (h *MachineHandler) shutdown(w http.ResponseWriter, r *http.Request) {
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

	cmd := "sudo poweroff"
	if m.SSHSudoPw != "" {
		// Pipe the sudo password via stdin so machines without passwordless sudo work.
		sess.Stdin = strings.NewReader(m.SSHSudoPw + "\n")
		cmd = "sudo -S poweroff"
	}

	if err := sess.Run(cmd); err != nil {
		// poweroff severs the SSH connection before the command exits cleanly —
		// an exit error here is expected and not a failure.
		_ = err
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"shutdown command sent"}`)) //nolint:errcheck
}
