package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonwhittlestone/tools-onoffapi/models"
)

func TestSetPassword_NotFound(t *testing.T) {
	body, _ := json.Marshal(setPasswordRequest{Password: "newpass"})
	req := httptest.NewRequest("POST", "/machines/unknown/password", bytes.NewReader(body))
	w := httptest.NewRecorder()
	newScreentimeMux().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestSetPassword_MissingSSHCredentials(t *testing.T) {
	store := models.NewStore()
	store.Create(models.Machine{ID: "nokeys", Name: "No Keys", IP: "192.168.0.99", MAC: "aa:bb:cc:dd:ee:ff"})
	h := NewMachineHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body, _ := json.Marshal(setPasswordRequest{Password: "newpass"})
	req := httptest.NewRequest("POST", "/machines/nokeys/password", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestSetPassword_MissingSudoPassword(t *testing.T) {
	store := models.NewStore()
	store.Create(models.Machine{
		ID:         "nosudo",
		Name:       "No Sudo Pw",
		IP:         "192.168.0.99",
		MAC:        "aa:bb:cc:dd:ee:ff",
		SSHUser:    "joseph",
		SSHKeyPath: "/nonexistent/path/id_rsa",
	})
	h := NewMachineHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body, _ := json.Marshal(setPasswordRequest{Password: "newpass"})
	req := httptest.NewRequest("POST", "/machines/nosudo/password", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestSetPassword_EmptyPassword(t *testing.T) {
	store := models.NewStore()
	store.Create(models.Machine{
		ID:         "hascreds",
		Name:       "Has Creds",
		IP:         "192.168.0.99",
		MAC:        "aa:bb:cc:dd:ee:ff",
		SSHUser:    "joseph",
		SSHKeyPath: "/nonexistent/path/id_rsa",
		SSHSudoPw:  "sudopw",
	})
	h := NewMachineHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body, _ := json.Marshal(setPasswordRequest{Password: ""})
	req := httptest.NewRequest("POST", "/machines/hascreds/password", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSetPassword_InvalidBody(t *testing.T) {
	store := models.NewStore()
	store.Create(models.Machine{
		ID:         "hascreds2",
		Name:       "Has Creds",
		IP:         "192.168.0.99",
		MAC:        "aa:bb:cc:dd:ee:ff",
		SSHUser:    "joseph",
		SSHKeyPath: "/nonexistent/path/id_rsa",
		SSHSudoPw:  "sudopw",
	})
	h := NewMachineHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/machines/hascreds2/password", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSetPassword_MissingKeyFile(t *testing.T) {
	store := models.NewStore()
	store.Create(models.Machine{
		ID:         "badkey",
		Name:       "Bad Key Path",
		IP:         "192.168.0.99",
		MAC:        "aa:bb:cc:dd:ee:ff",
		SSHUser:    "joseph",
		SSHKeyPath: "/nonexistent/path/id_rsa",
		SSHSudoPw:  "sudopw",
	})
	h := NewMachineHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body, _ := json.Marshal(setPasswordRequest{Password: "newpass"})
	req := httptest.NewRequest("POST", "/machines/badkey/password", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}
