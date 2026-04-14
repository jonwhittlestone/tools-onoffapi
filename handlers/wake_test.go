package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonwhittlestone/tools-onoffapi/models"
)

func TestBuildMagicPacket(t *testing.T) {
	pkt, err := buildMagicPacket("58:47:ca:70:62:27")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkt) != 102 {
		t.Fatalf("expected 102 bytes, got %d", len(pkt))
	}
	// First 6 bytes must be 0xFF
	for i := range 6 {
		if pkt[i] != 0xFF {
			t.Fatalf("byte %d: expected 0xFF, got %02x", i, pkt[i])
		}
	}
	// Bytes 6-11 must be the MAC (first repetition)
	mac := []byte{0x58, 0x47, 0xca, 0x70, 0x62, 0x27}
	for i, b := range mac {
		if pkt[6+i] != b {
			t.Fatalf("MAC byte %d: expected %02x, got %02x", i, b, pkt[6+i])
		}
	}
}

func TestBuildMagicPacketHyphenSeparator(t *testing.T) {
	pkt, err := buildMagicPacket("58-47-CA-70-62-27")
	if err != nil {
		t.Fatalf("unexpected error for hyphen-separated MAC: %v", err)
	}
	if len(pkt) != 102 {
		t.Fatalf("expected 102 bytes, got %d", len(pkt))
	}
}

func TestBuildMagicPacketInvalidMAC(t *testing.T) {
	_, err := buildMagicPacket("not-a-mac")
	if err == nil {
		t.Fatal("expected error for invalid MAC, got nil")
	}
}

func TestWakeNotFound(t *testing.T) {
	store := models.NewStore()
	h := NewMachineHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/machines/unknown/wake", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestWakeSeededMachine(t *testing.T) {
	store := models.NewStore()
	h := NewMachineHandler(store)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/machines/doylestone02/wake", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// 200 means the packet was built and sent — we can't assert the NIC woke up
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
