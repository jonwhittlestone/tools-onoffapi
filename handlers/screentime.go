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

type screentimeStartRequest struct {
	Duration     string `json:"duration"`
	Action       string `json:"action"`
	LockAccount  bool   `json:"lock_account"`
	RestoreAfter string `json:"restore_after"`
}

// startScreentime handles POST /machines/{id}/screentime
// Starts the screentime-timer.py on the target machine over SSH.
// The Python script forks to detach from the SSH session, so this returns quickly.
func (h *MachineHandler) startScreentime(w http.ResponseWriter, r *http.Request) {
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

	var req screentimeStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Duration == "" {
		writeError(w, http.StatusBadRequest, "duration is required")
		return
	}
	if req.Action == "" {
		req.Action = "lock"
	}
	if req.Action != "lock" && req.Action != "suspend" && req.Action != "poweroff" {
		writeError(w, http.StatusBadRequest, "action must be lock, suspend, or poweroff")
		return
	}

	cmd := buildStartCmd(req)
	if err := screentimeSSH(m.SSHUser, m.SSHKeyPath, m.IP, cmd); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("command failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "timer started",
		"duration": req.Duration,
		"action":   req.Action,
	})
}

// stopScreentime handles DELETE /machines/{id}/screentime
func (h *MachineHandler) stopScreentime(w http.ResponseWriter, r *http.Request) {
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

	if err := screentimeSSH(m.SSHUser, m.SSHKeyPath, m.IP, "python3 ~/screentime-timer.py --stop"); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("command failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "timer stopped"})
}

// unlockScreentime handles POST /machines/{id}/screentime/unlock
func (h *MachineHandler) unlockScreentime(w http.ResponseWriter, r *http.Request) {
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

	if err := screentimeSSH(m.SSHUser, m.SSHKeyPath, m.IP, "python3 ~/screentime-timer.py --unlock"); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("command failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "account unlocked"})
}

func buildStartCmd(req screentimeStartRequest) string {
	parts := []string{
		"python3", "~/screentime-timer.py",
		req.Duration,
		"--action", req.Action,
	}
	if req.LockAccount {
		parts = append(parts, "--lock-account")
	}
	if req.RestoreAfter != "" {
		parts = append(parts, "--restore-after", req.RestoreAfter)
	}
	return strings.Join(parts, " ")
}

// screentimeSSH opens an SSH connection to ip:22, runs cmd, and returns.
// The screentime-timer.py forks when invoked via SSH, so start commands return
// quickly even though the timer runs for hours.
func screentimeSSH(user, keyPath, ip, cmd string) error {
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("could not read SSH key: %w", err)
	}
	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return fmt.Errorf("could not parse SSH key")
	}

	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}
	client, err := ssh.Dial("tcp", ip+":22", cfg)
	if err != nil {
		return fmt.Errorf("SSH dial failed: %w", err)
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("could not open SSH session")
	}
	defer sess.Close()

	return sess.Run(cmd)
}
