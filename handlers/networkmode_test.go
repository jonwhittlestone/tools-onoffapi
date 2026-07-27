package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jonwhittlestone/tools-onoffapi/models"
)

func TestEffectiveIP_TailscaleModeWithAddress(t *testing.T) {
	m := models.Machine{IP: "192.168.0.218", TailscaleIP: "100.105.77.123"}
	if got := effectiveIP(m, "tailscale"); got != "100.105.77.123" {
		t.Errorf("expected tailscale IP, got %q", got)
	}
}

func TestEffectiveIP_TailscaleModeWithoutAddress_FallsBackToLAN(t *testing.T) {
	m := models.Machine{IP: "192.168.0.220"} // no TailscaleIP set
	if got := effectiveIP(m, "tailscale"); got != "192.168.0.220" {
		t.Errorf("expected LAN fallback, got %q", got)
	}
}

func TestEffectiveIP_LANMode(t *testing.T) {
	m := models.Machine{IP: "192.168.0.203", TailscaleIP: "100.111.143.116"}
	if got := effectiveIP(m, "lan"); got != "192.168.0.203" {
		t.Errorf("expected LAN IP even though tailscale is set, got %q", got)
	}
}

func TestGetNetworkMode_DefaultsToTailscale(t *testing.T) {
	req := httptest.NewRequest("GET", "/network-mode", nil)
	w := httptest.NewRecorder()
	newScreentimeMux().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"mode":"tailscale"`) {
		t.Errorf("expected default mode tailscale in body, got %s", w.Body.String())
	}
}

func TestSetNetworkMode_LAN(t *testing.T) {
	req := httptest.NewRequest("POST", "/network-mode", strings.NewReader(`{"mode":"lan"}`))
	w := httptest.NewRecorder()
	newScreentimeMux().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"mode":"lan"`) {
		t.Errorf("expected mode lan in body, got %s", w.Body.String())
	}
}

func TestSetNetworkMode_RejectsInvalid(t *testing.T) {
	req := httptest.NewRequest("POST", "/network-mode", strings.NewReader(`{"mode":"bogus"}`))
	w := httptest.NewRecorder()
	newScreentimeMux().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
