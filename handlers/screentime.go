package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// screentimeStatus holds the server-side record of a running timer.
// Stored when a timer starts; cleared when it stops or expires.
type screentimeStatus struct {
	startedAt   time.Time
	totalSecs   int
	action      string
	lockAccount bool
}

// screentimeStore is a thread-safe in-memory map of machine ID → active timer.
type screentimeStore struct {
	mu     sync.RWMutex
	timers map[string]screentimeStatus
}

func newScreentimeStore() *screentimeStore {
	return &screentimeStore{timers: make(map[string]screentimeStatus)}
}

func (s *screentimeStore) set(id string, st screentimeStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.timers[id] = st
}

func (s *screentimeStore) get(id string) (screentimeStatus, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st, ok := s.timers[id]
	return st, ok
}

func (s *screentimeStore) clear(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.timers, id)
}

type screentimeStartRequest struct {
	Duration     string `json:"duration"`
	Action       string `json:"action"`
	LockAccount  bool   `json:"lock_account"`
	RestoreAfter string `json:"restore_after"`
}

// getScreentime handles GET /machines/{id}/screentime
// Returns current timer state. Multiple clients can poll this to show a
// synchronised countdown — no WebSocket needed.
func (h *MachineHandler) getScreentime(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := h.store.GetByID(id); !ok {
		writeError(w, http.StatusNotFound, "machine not found")
		return
	}

	st, active := h.screentimeStore.get(id)
	if !active {
		writeJSON(w, http.StatusOK, map[string]bool{"active": false})
		return
	}

	remainingSecs := st.totalSecs - int(time.Since(st.startedAt).Seconds())
	if remainingSecs <= 0 {
		h.screentimeStore.clear(id)
		writeJSON(w, http.StatusOK, map[string]bool{"active": false})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"active":         true,
		"remaining_secs": remainingSecs,
		"remaining":      fmtSecs(remainingSecs),
		"action":         st.action,
		"lock_account":   st.lockAccount,
	})
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

	totalSecs, err := parseDurationSecs(req.Duration)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid duration: %v", err))
		return
	}

	cmd := buildStartCmd(req)
	startedAt := time.Now()
	if err := screentimeSSH(m.SSHUser, m.SSHKeyPath, m.IP, cmd); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("command failed: %v", err))
		return
	}

	h.screentimeStore.set(id, screentimeStatus{
		startedAt:   startedAt,
		totalSecs:   totalSecs,
		action:      req.Action,
		lockAccount: req.LockAccount,
	})

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

	h.screentimeStore.clear(id)
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

// --- helpers ---

var durationRe = regexp.MustCompile(`(\d+)([hms])`)

func parseDurationSecs(s string) (int, error) {
	s = strings.TrimSpace(s)
	matches := durationRe.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		n, err := strconv.Atoi(s)
		if err != nil {
			return 0, fmt.Errorf("expected format like 30m, 1h30m, 90s")
		}
		return n, nil
	}
	units := map[string]int{"h": 3600, "m": 60, "s": 1}
	total := 0
	for _, m := range matches {
		n, _ := strconv.Atoi(m[1])
		total += n * units[m[2]]
	}
	return total, nil
}

func fmtSecs(s int) string {
	h := s / 3600
	m := (s % 3600) / 60
	sec := s % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, sec)
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
