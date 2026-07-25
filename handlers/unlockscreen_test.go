package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonwhittlestone/tools-onoffapi/models"
)

func TestParseActiveSessionID(t *testing.T) {
	out := "SESSION  UID USER    SEAT  TTY  STATE  IDLE SINCE\n" +
		"     58 1001 joseph seat0 tty2 active no   \n" +
		"     82 1001 joseph       n/a  active no   \n"
	got := parseActiveSessionID(out, "joseph")
	if got != "58" {
		t.Errorf("expected session 58, got %q", got)
	}
}

func TestParseActiveSessionID_NoSeatSession(t *testing.T) {
	// Only a headless/background session exists (no seat) — should not match.
	out := "SESSION  UID USER    SEAT  TTY  STATE  IDLE SINCE\n" +
		"     82 1001 joseph       n/a  active no   \n"
	got := parseActiveSessionID(out, "joseph")
	if got != "" {
		t.Errorf("expected no match, got %q", got)
	}
}

func TestParseActiveSessionID_WrongUser(t *testing.T) {
	out := "SESSION  UID USER    SEAT  TTY  STATE  IDLE SINCE\n" +
		"     58 1002 maker  seat0 tty2 active no   \n"
	got := parseActiveSessionID(out, "joseph")
	if got != "" {
		t.Errorf("expected no match, got %q", got)
	}
}

func TestUnlockScreen_NotFound(t *testing.T) {
	req := httptest.NewRequest("POST", "/machines/unknown/unlock-screen", nil)
	w := httptest.NewRecorder()
	newScreentimeMux().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestUnlockScreen_MissingSSHCredentials(t *testing.T) {
	store := models.NewStore()
	store.Create(models.Machine{ID: "nokeys", Name: "No Keys", IP: "192.168.0.99", MAC: "aa:bb:cc:dd:ee:ff"})
	h := NewMachineHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/machines/nokeys/unlock-screen", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestUnlockScreen_MissingKeyFile(t *testing.T) {
	store := models.NewStore()
	store.Create(models.Machine{
		ID: "badkey", Name: "Bad Key", IP: "192.168.0.99", MAC: "aa:bb:cc:dd:ee:ff",
		SSHUser: "jon", SSHKeyPath: "/nonexistent/id_rsa",
	})
	h := NewMachineHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/machines/badkey/unlock-screen", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}
