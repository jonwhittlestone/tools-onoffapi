package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
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

// screentimeStatusJSON is used solely for file persistence.
type screentimeStatusJSON struct {
	StartedAt   time.Time `json:"started_at"`
	TotalSecs   int       `json:"total_secs"`
	Action      string    `json:"action"`
	LockAccount bool      `json:"lock_account"`
}

// remoteStateCache holds the last SSH-read result from /tmp/screentime-state on the target machine.
type remoteStateCache struct {
	checkedAt     time.Time
	remainingSecs int
	running       bool
}

const (
	// liveCheckInterval is how often we SSH-verify an active timer is still running.
	// Detects CLI stops (ssh joseph python3 ~/screentime-timer.py --stop).
	liveCheckInterval = 30 * time.Second
	// liveCheckGrace skips the liveness check for this long after a timer starts.
	// The Python child takes ~200ms to write /tmp/screentime-state after forking;
	// without a grace period the first GET after POST can catch that window and
	// falsely conclude the timer died.
	liveCheckGrace = 10 * time.Second
	// remoteStateTTL is how long we cache an SSH read of /tmp/screentime-state
	// when the API has no recorded timer (e.g. timer started via CLI/SSH).
	remoteStateTTL = 10 * time.Second
)

// screentimeStore is a thread-safe in-memory map of machine ID → active timer.
// State is persisted to stateFile on every write so it survives container restarts.
type screentimeStore struct {
	mu          sync.RWMutex
	timers      map[string]screentimeStatus
	stateFile   string
	checkMu     sync.Mutex
	lastCheck   map[string]time.Time      // last liveness check per machine
	remoteCache map[string]remoteStateCache // last direct SSH read per machine
}

func newScreentimeStore(stateFile string) *screentimeStore {
	s := &screentimeStore{
		timers:      make(map[string]screentimeStatus),
		stateFile:   stateFile,
		lastCheck:   make(map[string]time.Time),
		remoteCache: make(map[string]remoteStateCache),
	}
	s.loadFromFile()
	return s
}

func (s *screentimeStore) set(id string, st screentimeStatus) {
	s.mu.Lock()
	s.timers[id] = st
	s.mu.Unlock()
	s.saveToFile()
}

func (s *screentimeStore) get(id string) (screentimeStatus, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st, ok := s.timers[id]
	return st, ok
}

func (s *screentimeStore) clear(id string) {
	s.mu.Lock()
	delete(s.timers, id)
	s.mu.Unlock()
	s.saveToFile()
}

// checkAndMark returns true if a liveness check is due, updating the timestamp atomically.
func (s *screentimeStore) checkAndMark(id string) bool {
	s.checkMu.Lock()
	defer s.checkMu.Unlock()
	last, ok := s.lastCheck[id]
	if !ok || time.Since(last) >= liveCheckInterval {
		s.lastCheck[id] = time.Now()
		return true
	}
	return false
}

// getCachedRemote returns (entry, isFresh). Used for SSH-only timers.
func (s *screentimeStore) getCachedRemote(id string) (remoteStateCache, bool) {
	s.checkMu.Lock()
	defer s.checkMu.Unlock()
	e, ok := s.remoteCache[id]
	if !ok {
		return remoteStateCache{}, false
	}
	return e, time.Since(e.checkedAt) < remoteStateTTL
}

func (s *screentimeStore) setCachedRemote(id string, e remoteStateCache) {
	s.checkMu.Lock()
	defer s.checkMu.Unlock()
	s.remoteCache[id] = e
}

func (s *screentimeStore) saveToFile() {
	if s.stateFile == "" {
		return
	}
	s.mu.RLock()
	data := make(map[string]screentimeStatusJSON, len(s.timers))
	for id, st := range s.timers {
		data[id] = screentimeStatusJSON{
			StartedAt:   st.startedAt,
			TotalSecs:   st.totalSecs,
			Action:      st.action,
			LockAccount: st.lockAccount,
		}
	}
	s.mu.RUnlock()

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Printf("screentime: failed to marshal state: %v", err)
		return
	}
	tmp := s.stateFile + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		log.Printf("screentime: failed to write state file: %v", err)
		return
	}
	if err := os.Rename(tmp, s.stateFile); err != nil {
		log.Printf("screentime: failed to rename state file: %v", err)
	}
}

func (s *screentimeStore) loadFromFile() {
	if s.stateFile == "" {
		return
	}
	b, err := os.ReadFile(s.stateFile)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		log.Printf("screentime: failed to read state file: %v", err)
		return
	}
	var data map[string]screentimeStatusJSON
	if err := json.Unmarshal(b, &data); err != nil {
		log.Printf("screentime: failed to parse state file: %v", err)
		return
	}
	now := time.Now()
	for id, j := range data {
		remaining := j.TotalSecs - int(now.Sub(j.StartedAt).Seconds())
		if remaining > 0 {
			s.timers[id] = screentimeStatus{
				startedAt:   j.StartedAt,
				totalSecs:   j.TotalSecs,
				action:      j.Action,
				lockAccount: j.LockAccount,
			}
			log.Printf("screentime: restored timer for %s (%ds remaining)", id, remaining)
		}
	}
}

type screentimeStartRequest struct {
	Duration     string `json:"duration"`
	Action       string `json:"action"`
	LockAccount  *bool  `json:"lock_account"` // nil → default true; explicit false opts out
	RestoreAfter string `json:"restore_after"`
}

// getScreentime handles GET /machines/{id}/screentime.
// When the API has no recorded timer, it falls back to reading /tmp/screentime-state
// directly from the target machine (10s cache). This shows timers started via CLI/SSH.
// When the API does have a timer, it periodically SSH-verifies it's still alive (every
// 30s, with a 10s grace period after start) to detect CLI stops.
func (h *MachineHandler) getScreentime(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, ok := h.store.GetByID(id)
	if !ok {
		writeError(w, http.StatusNotFound, "machine not found")
		return
	}

	st, active := h.screentimeStore.get(id)

	if !active {
		// No API-recorded timer. Try reading the state file on the target machine
		// so timers started via SSH are also visible in the frontend.
		if m.SSHUser == "" || m.SSHKeyPath == "" {
			writeJSON(w, http.StatusOK, map[string]bool{"active": false})
			return
		}
		entry, fresh := h.screentimeStore.getCachedRemote(id)
		if !fresh {
			remaining, running, err := screentimeReadRemoteState(m.SSHUser, m.SSHKeyPath, m.IP)
			entry = remoteStateCache{checkedAt: time.Now(), remainingSecs: remaining, running: running}
			if err != nil {
				// SSH failed — can't determine state, assume inactive
				entry.running = false
			}
			h.screentimeStore.setCachedRemote(id, entry)
		}
		if entry.running && entry.remainingSecs > 0 {
			// Adjust for time elapsed since the cache was populated so the
			// client doesn't reset its ticker to a stale value.
			adjusted := entry.remainingSecs - int(time.Since(entry.checkedAt).Seconds())
			if adjusted > 0 {
				writeJSON(w, http.StatusOK, map[string]any{
					"active":         true,
					"remaining_secs": adjusted,
					"remaining":      fmtSecs(adjusted),
					"action":         "lock",
					"lock_account":   true,
				})
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]bool{"active": false})
		return
	}

	remainingSecs := st.totalSecs - int(time.Since(st.startedAt).Seconds())
	if remainingSecs <= 0 {
		h.screentimeStore.clear(id)
		writeJSON(w, http.StatusOK, map[string]bool{"active": false})
		return
	}

	// Periodically SSH-verify the timer process is still alive.
	// Detects timers stopped via CLI without going through the API.
	// Skip during the grace period after start — the Python child takes ~200ms
	// to write /tmp/screentime-state after forking, so checking too early gives
	// false "not running" results.
	pastGrace := time.Since(st.startedAt) > liveCheckGrace
	if pastGrace && m.SSHUser != "" && m.SSHKeyPath != "" && h.screentimeStore.checkAndMark(id) {
		running, err := screentimeIsRunning(m.SSHUser, m.SSHKeyPath, m.IP)
		if err == nil && !running {
			h.screentimeStore.clear(id)
			writeJSON(w, http.StatusOK, map[string]bool{"active": false})
			return
		}
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

	// Default lock_account to true — omitting it from the request still locks.
	lockAccount := true
	if req.LockAccount != nil {
		lockAccount = *req.LockAccount
	}

	cmd := buildStartCmd(req.Duration, req.Action, lockAccount, req.RestoreAfter)
	startedAt := time.Now()
	if err := screentimeSSH(m.SSHUser, m.SSHKeyPath, m.IP, cmd); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("command failed: %v", err))
		return
	}

	h.screentimeStore.set(id, screentimeStatus{
		startedAt:   startedAt,
		totalSecs:   totalSecs,
		action:      req.Action,
		lockAccount: lockAccount,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "timer started",
		"duration":     req.Duration,
		"total_secs":   totalSecs,
		"action":       req.Action,
		"lock_account": lockAccount,
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

func buildStartCmd(duration, action string, lockAccount bool, restoreAfter string) string {
	parts := []string{
		"python3", "~/screentime-timer.py",
		duration,
		"--action", action,
	}
	if lockAccount {
		parts = append(parts, "--lock-account")
	}
	if restoreAfter != "" {
		parts = append(parts, "--restore-after", restoreAfter)
	}
	return strings.Join(parts, " ")
}

// screentimeIsRunning checks whether the timer process is still active by testing
// for its state file. Returns (true, nil) if running, (false, nil) if cleanly
// stopped, (false, err) if SSH itself failed (unknown state — don't clear).
func screentimeIsRunning(user, keyPath, ip string) (bool, error) {
	err := screentimeSSH(user, keyPath, ip, "test -f /tmp/screentime-state")
	if err == nil {
		return true, nil
	}
	var exitErr *ssh.ExitError
	if errors.As(err, &exitErr) {
		return false, nil // test exited 1 — file gone, timer stopped
	}
	return false, err // SSH failed — unknown state
}

// screentimeReadRemoteState SSHes to the target machine, reads /tmp/screentime-state,
// and parses the remaining seconds from "display HH:MM:SS ..." lines.
// Returns (remainingSecs, running, err).
func screentimeReadRemoteState(user, keyPath, ip string) (int, bool, error) {
	out, err := screentimeSSHOutput(user, keyPath, ip, "cat /tmp/screentime-state 2>/dev/null || true")
	if err != nil {
		return 0, false, err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return 0, false, nil
	}
	parts := strings.Fields(out)
	if len(parts) < 2 {
		return 0, false, nil
	}
	if parts[0] == "FADE" {
		return 0, false, nil // screen is fading — effectively done
	}
	if parts[0] != "display" {
		return 0, false, nil
	}
	// parts[1] = "HH:MM:SS"
	timeParts := strings.Split(parts[1], ":")
	if len(timeParts) != 3 {
		return 0, false, nil
	}
	h, _ := strconv.Atoi(timeParts[0])
	m, _ := strconv.Atoi(timeParts[1])
	s, _ := strconv.Atoi(timeParts[2])
	remaining := h*3600 + m*60 + s
	return remaining, remaining > 0, nil
}

// screentimeSSHOutput is like screentimeSSH but captures and returns stdout.
func screentimeSSHOutput(user, keyPath, ip, cmd string) (string, error) {
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return "", fmt.Errorf("could not read SSH key: %w", err)
	}
	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return "", fmt.Errorf("could not parse SSH key")
	}

	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}
	client, err := ssh.Dial("tcp", ip+":22", cfg)
	if err != nil {
		return "", fmt.Errorf("SSH dial failed: %w", err)
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("could not open SSH session")
	}
	defer sess.Close()

	out, err := sess.Output(cmd)
	return string(out), err
}

// screentimeSSH opens an SSH connection to ip:22 and runs cmd, discarding output.
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
