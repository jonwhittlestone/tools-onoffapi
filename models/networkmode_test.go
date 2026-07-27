package models

import "testing"

func TestNetworkModeStore_DefaultsToTailscale(t *testing.T) {
	s := NewNetworkModeStore()
	if got := s.Get(); got != "tailscale" {
		t.Errorf("expected default mode tailscale, got %q", got)
	}
}

func TestNetworkModeStore_SetLAN(t *testing.T) {
	s := NewNetworkModeStore()
	if !s.Set("lan") {
		t.Fatal("expected Set(lan) to succeed")
	}
	if got := s.Get(); got != "lan" {
		t.Errorf("expected lan, got %q", got)
	}
}

func TestNetworkModeStore_RejectsInvalidMode(t *testing.T) {
	s := NewNetworkModeStore()
	if s.Set("bogus") {
		t.Fatal("expected Set(bogus) to fail")
	}
	if got := s.Get(); got != "tailscale" {
		t.Errorf("invalid Set must not change mode, got %q", got)
	}
}
