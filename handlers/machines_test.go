package handlers

// Go test files live alongside the code they test, in the same package.
// The testing package and net/http/httptest are part of the standard library —
// no pytest install, no fixtures file, no conftest.py equivalent needed.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonwhittlestone/tools-onoffapi/models"
)

// newTestHandler is a test helper that returns a handler wired to a fresh store.
// Equivalent to a pytest fixture that returns a test client.
func newTestHandler() *MachineHandler {
	return NewMachineHandler(models.NewStore())
}

// TestListMachines verifies GET /machines returns 200 and the seeded machine.
func TestListMachines(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()

	h.RegisterRoutes(mux)

	// httptest.NewRecorder() is the Go equivalent of FastAPI's TestClient.
	req := httptest.NewRequest(http.MethodGet, "/machines", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var machines []models.Machine
	if err := json.NewDecoder(w.Body).Decode(&machines); err != nil {
		t.Fatalf("could not decode response: %v", err)
	}
	if len(machines) == 0 {
		t.Error("expected at least one machine in seed data")
	}
}

// TestGetMachine_Found verifies GET /machines/{id} returns the correct machine.
func TestGetMachine_Found(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/machines/doylestone02", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var m models.Machine
	json.NewDecoder(w.Body).Decode(&m)
	if m.ID != "doylestone02" {
		t.Errorf("expected id doylestone02, got %q", m.ID)
	}
}

// TestGetMachine_NotFound verifies GET /machines/{id} returns 404 for unknown IDs.
func TestGetMachine_NotFound(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/machines/doesnotexist", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// TestCreateMachine verifies POST /machines creates and returns the new machine.
func TestCreateMachine(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := models.Machine{
		ID:   "testmachine",
		Name: "Test Machine",
		IP:   "192.168.0.99",
		MAC:  "aa:bb:cc:dd:ee:ff",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/machines", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}

	var created models.Machine
	json.NewDecoder(w.Body).Decode(&created)
	if created.ID != "testmachine" {
		t.Errorf("expected id testmachine, got %q", created.ID)
	}
}

// TestCreateMachine_Duplicate verifies 409 is returned when ID already exists.
func TestCreateMachine_Duplicate(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := models.Machine{ID: "doylestone02", Name: "dup", IP: "1.1.1.1", MAC: "aa:bb:cc:dd:ee:ff"}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/machines", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 conflict, got %d", w.Code)
	}
}

// TestCreateMachine_MissingFields verifies 400 when required fields are absent.
func TestCreateMachine_MissingFields(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := map[string]string{"name": "incomplete"} // no id, ip, mac
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/machines", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestDeleteMachine verifies DELETE /machines/{id} returns 204 on success.
func TestDeleteMachine(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/machines/doylestone02", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}

	// Verify it's gone
	req2 := httptest.NewRequest(http.MethodGet, "/machines/doylestone02", nil)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)
	if w2.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", w2.Code)
	}
}

// TestDeleteMachine_NotFound verifies 404 when deleting a non-existent machine.
func TestDeleteMachine_NotFound(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/machines/ghost", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
