package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonwhittlestone/tools-onoffapi/models"
)

func TestShutdownNotFound(t *testing.T) {
	store := models.NewStore()
	h := NewMachineHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/machines/unknown/shutdown", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestShutdownMissingSSHCredentials(t *testing.T) {
	store := models.NewStore()
	// Add a machine with no SSH credentials
	store.Create(models.Machine{
		ID:   "nokeys",
		Name: "No Keys",
		IP:   "192.168.0.99",
		MAC:  "aa:bb:cc:dd:ee:ff",
	})
	h := NewMachineHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/machines/nokeys/shutdown", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestShutdownMissingKeyFile(t *testing.T) {
	store := models.NewStore()
	store.Create(models.Machine{
		ID:         "badkey",
		Name:       "Bad Key Path",
		IP:         "192.168.0.99",
		MAC:        "aa:bb:cc:dd:ee:ff",
		SSHUser:    "jon",
		SSHKeyPath: "/nonexistent/path/id_rsa",
	})
	h := NewMachineHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/machines/badkey/shutdown", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}
