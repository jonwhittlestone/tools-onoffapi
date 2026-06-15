package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonwhittlestone/tools-onoffapi/models"
)

func newScreentimeMux() *http.ServeMux {
	h := NewMachineHandler(models.NewStore())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return mux
}

func TestStartScreentime_NotFound(t *testing.T) {
	body, _ := json.Marshal(screentimeStartRequest{Duration: "30m"})
	req := httptest.NewRequest("POST", "/machines/unknown/screentime", bytes.NewReader(body))
	w := httptest.NewRecorder()
	newScreentimeMux().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestStartScreentime_MissingCredentials(t *testing.T) {
	store := models.NewStore()
	store.Create(models.Machine{ID: "nokeys", Name: "No Keys", IP: "192.168.0.99", MAC: "aa:bb:cc:dd:ee:ff"})
	h := NewMachineHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body, _ := json.Marshal(screentimeStartRequest{Duration: "30m"})
	req := httptest.NewRequest("POST", "/machines/nokeys/screentime", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestStartScreentime_MissingDuration(t *testing.T) {
	store := models.NewStore()
	store.Create(models.Machine{
		ID: "nodur", Name: "No Duration", IP: "192.168.0.99", MAC: "aa:bb:cc:dd:ee:ff",
		SSHUser: "jon", SSHKeyPath: "/some/key",
	})
	h := NewMachineHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest("POST", "/machines/nodur/screentime", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestStartScreentime_InvalidAction(t *testing.T) {
	store := models.NewStore()
	store.Create(models.Machine{
		ID: "m1", Name: "M1", IP: "192.168.0.99", MAC: "aa:bb:cc:dd:ee:ff",
		SSHUser: "jon", SSHKeyPath: "/some/key",
	})
	h := NewMachineHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body, _ := json.Marshal(screentimeStartRequest{Duration: "30m", Action: "reboot"})
	req := httptest.NewRequest("POST", "/machines/m1/screentime", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestStartScreentime_MissingKeyFile(t *testing.T) {
	store := models.NewStore()
	store.Create(models.Machine{
		ID: "badkey", Name: "Bad Key", IP: "192.168.0.99", MAC: "aa:bb:cc:dd:ee:ff",
		SSHUser: "jon", SSHKeyPath: "/nonexistent/id_rsa",
	})
	h := NewMachineHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body, _ := json.Marshal(screentimeStartRequest{Duration: "30m"})
	req := httptest.NewRequest("POST", "/machines/badkey/screentime", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestStopScreentime_NotFound(t *testing.T) {
	req := httptest.NewRequest("DELETE", "/machines/unknown/screentime", nil)
	w := httptest.NewRecorder()
	newScreentimeMux().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestUnlockScreentime_NotFound(t *testing.T) {
	req := httptest.NewRequest("POST", "/machines/unknown/screentime/unlock", nil)
	w := httptest.NewRecorder()
	newScreentimeMux().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
