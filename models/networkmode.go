package models

import "sync"

// NetworkModeStore holds the single global "tailscale" vs "lan" switch that
// decides which address field (TailscaleIP vs IP) every SSH/dial call site
// uses. In-memory only — resets to the "tailscale" default on redeploy,
// same as every other piece of state in this project (no database).
type NetworkModeStore struct {
	mu   sync.RWMutex
	mode string
}

func NewNetworkModeStore() *NetworkModeStore {
	return &NetworkModeStore{mode: "tailscale"}
}

func (s *NetworkModeStore) Get() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mode
}

// Set returns false (no-op) for anything other than "tailscale" or "lan".
func (s *NetworkModeStore) Set(mode string) bool {
	if mode != "tailscale" && mode != "lan" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mode = mode
	return true
}
