package handlers

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// volumeStatus is the GET /machines/{id}/volume response shape.
type volumeStatus struct {
	Volume int  `json:"volume"`
	Muted  bool `json:"muted"`
}

type setVolumeRequest struct {
	Volume int `json:"volume"`
}

type setMuteRequest struct {
	Muted bool `json:"muted"`
}

var wpctlVolumeRe = regexp.MustCompile(`Volume:\s*([\d.]+)`)

// getVolume handles GET /machines/{id}/volume. Reads the current volume
// level and mute state via `wpctl get-volume` over SSH, as the machine's
// own SSHUser — no sudo needed, this is a normal per-session PipeWire
// operation (WirePlumber's native CLI, present by default — no extra
// package needed, unlike pactl/pulseaudio-utils which isn't installed on
// this machine). For doylestone440 that's `maker`, matching whichever OS
// user is actually sitting at the desktop and hearing the audio.
func (h *MachineHandler) getVolume(w http.ResponseWriter, r *http.Request) {
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

	out, err := volumeSSHOutput(m.SSHUser, m.SSHKeyPath, effectiveIP(m, h.networkMode.Get()), "wpctl get-volume @DEFAULT_AUDIO_SINK@")
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("command failed: %v", err))
		return
	}
	vol, ok := parseWpctlVolume(out)
	if !ok {
		writeError(w, http.StatusInternalServerError, "could not parse volume output")
		return
	}

	writeJSON(w, http.StatusOK, volumeStatus{
		Volume: vol,
		Muted:  strings.Contains(out, "[MUTED]"),
	})
}

// setVolume handles POST /machines/{id}/volume, body {"volume": 0-100}.
func (h *MachineHandler) setVolume(w http.ResponseWriter, r *http.Request) {
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

	var req setVolumeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Volume < 0 || req.Volume > 100 {
		writeError(w, http.StatusBadRequest, "volume must be between 0 and 100")
		return
	}

	cmd := fmt.Sprintf("wpctl set-volume @DEFAULT_AUDIO_SINK@ %d%%", req.Volume)
	if err := volumeSSH(m.SSHUser, m.SSHKeyPath, effectiveIP(m, h.networkMode.Get()), cmd); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("command failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "volume set", "volume": req.Volume})
}

// setMute handles POST /machines/{id}/mute, body {"muted": bool}.
func (h *MachineHandler) setMute(w http.ResponseWriter, r *http.Request) {
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

	var req setMuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	muteArg := "0"
	if req.Muted {
		muteArg = "1"
	}
	cmd := fmt.Sprintf("wpctl set-mute @DEFAULT_AUDIO_SINK@ %s", muteArg)
	if err := volumeSSH(m.SSHUser, m.SSHKeyPath, effectiveIP(m, h.networkMode.Get()), cmd); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("command failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "mute set", "muted": req.Muted})
}

// parseWpctlVolume extracts the fraction from wpctl's "Volume: 0.66" (or
// "Volume: 0.66 [MUTED]") output and converts it to a 0-100 percentage.
func parseWpctlVolume(out string) (int, bool) {
	match := wpctlVolumeRe.FindStringSubmatch(out)
	if match == nil {
		return 0, false
	}
	frac, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, false
	}
	return int(math.Round(frac * 100)), true
}

// --- SSH helpers (same pattern as screentimeSSH/screentimeSSHOutput) ---

func volumeSSH(user, keyPath, ip, cmd string) error {
	_, err := volumeSSHOutput(user, keyPath, ip, cmd)
	return err
}

func volumeSSHOutput(user, keyPath, ip, cmd string) (string, error) {
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
