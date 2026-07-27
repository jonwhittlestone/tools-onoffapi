package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jonwhittlestone/tools-onoffapi/models"
)

// effectiveIP returns the address to dial for m under the given mode.
// "tailscale" mode falls back to the LAN IP for any machine that has no
// TailscaleIP set (i.e. hasn't been enrolled in the tailnet yet) — so
// enabling the global switch never breaks a machine that isn't ready for it.
func effectiveIP(m models.Machine, mode string) string {
	if mode == "tailscale" && m.TailscaleIP != "" {
		return m.TailscaleIP
	}
	return m.IP
}

func (h *MachineHandler) getNetworkMode(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"mode": h.networkMode.Get()})
}

func (h *MachineHandler) setNetworkMode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !h.networkMode.Set(body.Mode) {
		writeError(w, http.StatusBadRequest, `mode must be "tailscale" or "lan"`)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"mode": h.networkMode.Get()})
}
