package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonwhittlestone/tools-onoffapi/models"
)

func newVolumeMux() *http.ServeMux {
	h := NewMachineHandler(models.NewStore())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return mux
}

func TestGetVolume_NotFound(t *testing.T) {
	req := httptest.NewRequest("GET", "/machines/unknown/volume", nil)
	w := httptest.NewRecorder()
	newVolumeMux().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetVolume_MissingCredentials(t *testing.T) {
	store := models.NewStore()
	store.Create(models.Machine{ID: "nokeys", Name: "No Keys", IP: "192.168.0.99", MAC: "aa:bb:cc:dd:ee:ff"})
	h := NewMachineHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/machines/nokeys/volume", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestGetVolume_MissingKeyFile(t *testing.T) {
	store := models.NewStore()
	store.Create(models.Machine{
		ID: "badkey", Name: "Bad Key", IP: "192.168.0.99", MAC: "aa:bb:cc:dd:ee:ff",
		SSHUser: "jon", SSHKeyPath: "/nonexistent/id_rsa",
	})
	h := NewMachineHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/machines/badkey/volume", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestSetVolume_NotFound(t *testing.T) {
	body, _ := json.Marshal(setVolumeRequest{Volume: 50})
	req := httptest.NewRequest("POST", "/machines/unknown/volume", bytes.NewReader(body))
	w := httptest.NewRecorder()
	newVolumeMux().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestSetVolume_MissingCredentials(t *testing.T) {
	store := models.NewStore()
	store.Create(models.Machine{ID: "nokeys", Name: "No Keys", IP: "192.168.0.99", MAC: "aa:bb:cc:dd:ee:ff"})
	h := NewMachineHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body, _ := json.Marshal(setVolumeRequest{Volume: 50})
	req := httptest.NewRequest("POST", "/machines/nokeys/volume", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestSetVolume_InvalidBody(t *testing.T) {
	store := models.NewStore()
	store.Create(models.Machine{
		ID: "m1", Name: "M1", IP: "192.168.0.99", MAC: "aa:bb:cc:dd:ee:ff",
		SSHUser: "jon", SSHKeyPath: "/some/key",
	})
	h := NewMachineHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/machines/m1/volume", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSetVolume_OutOfRange(t *testing.T) {
	store := models.NewStore()
	store.Create(models.Machine{
		ID: "m2", Name: "M2", IP: "192.168.0.99", MAC: "aa:bb:cc:dd:ee:ff",
		SSHUser: "jon", SSHKeyPath: "/some/key",
	})
	h := NewMachineHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	for _, v := range []int{-1, 101} {
		body, _ := json.Marshal(setVolumeRequest{Volume: v})
		req := httptest.NewRequest("POST", "/machines/m2/volume", bytes.NewReader(body))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("volume=%d: expected 400, got %d", v, w.Code)
		}
	}
}

func TestSetVolume_MissingKeyFile(t *testing.T) {
	store := models.NewStore()
	store.Create(models.Machine{
		ID: "badkey2", Name: "Bad Key 2", IP: "192.168.0.99", MAC: "aa:bb:cc:dd:ee:ff",
		SSHUser: "jon", SSHKeyPath: "/nonexistent/id_rsa",
	})
	h := NewMachineHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body, _ := json.Marshal(setVolumeRequest{Volume: 50})
	req := httptest.NewRequest("POST", "/machines/badkey2/volume", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestSetMute_NotFound(t *testing.T) {
	body, _ := json.Marshal(setMuteRequest{Muted: true})
	req := httptest.NewRequest("POST", "/machines/unknown/mute", bytes.NewReader(body))
	w := httptest.NewRecorder()
	newVolumeMux().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestSetMute_MissingCredentials(t *testing.T) {
	store := models.NewStore()
	store.Create(models.Machine{ID: "nokeys2", Name: "No Keys 2", IP: "192.168.0.99", MAC: "aa:bb:cc:dd:ee:ff"})
	h := NewMachineHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body, _ := json.Marshal(setMuteRequest{Muted: true})
	req := httptest.NewRequest("POST", "/machines/nokeys2/mute", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestSetMute_InvalidBody(t *testing.T) {
	store := models.NewStore()
	store.Create(models.Machine{
		ID: "m3", Name: "M3", IP: "192.168.0.99", MAC: "aa:bb:cc:dd:ee:ff",
		SSHUser: "jon", SSHKeyPath: "/some/key",
	})
	h := NewMachineHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/machines/m3/mute", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSetMute_MissingKeyFile(t *testing.T) {
	store := models.NewStore()
	store.Create(models.Machine{
		ID: "badkey3", Name: "Bad Key 3", IP: "192.168.0.99", MAC: "aa:bb:cc:dd:ee:ff",
		SSHUser: "jon", SSHKeyPath: "/nonexistent/id_rsa",
	})
	h := NewMachineHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body, _ := json.Marshal(setMuteRequest{Muted: true})
	req := httptest.NewRequest("POST", "/machines/badkey3/mute", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestParseWpctlVolume(t *testing.T) {
	cases := []struct {
		input   string
		want    int
		wantOk  bool
		comment string
	}{
		{input: "Volume: 0.66\n", want: 66, wantOk: true, comment: "typical unmuted reading"},
		{input: "Volume: 0.66 [MUTED]\n", want: 66, wantOk: true, comment: "muted — volume still parses, mute detected separately"},
		{input: "Volume: 1.00\n", want: 100, wantOk: true, comment: "full volume"},
		{input: "Volume: 0.00\n", want: 0, wantOk: true, comment: "zero volume"},
		{input: "garbage, no volume here", want: 0, wantOk: false, comment: "unparseable output"},
	}
	for _, c := range cases {
		got, ok := parseWpctlVolume(c.input)
		if ok != c.wantOk || (ok && got != c.want) {
			t.Errorf("%s: parseWpctlVolume(%q) = (%d, %v), want (%d, %v)", c.comment, c.input, got, ok, c.want, c.wantOk)
		}
	}
}
