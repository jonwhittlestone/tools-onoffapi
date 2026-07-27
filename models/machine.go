package models

import (
	"os"
	"sync"
)

// ScreentimeUser is an additional OS-level account on a machine that can be
// independently targeted for screen-time countdown control (its own SSH
// login, its own timer state). Only IsAdmin accounts have sudo on the box —
// non-admin accounts get suspend/poweroff/screen-lock but not the hard
// "lock account" password-swap trick, since that requires sudo.
type ScreentimeUser struct {
	ID         string `json:"id"`
	SSHUser    string `json:"ssh_user"`
	SSHKeyPath string `json:"ssh_key_path"`
	IsAdmin    bool   `json:"is_admin,omitempty"`
}

// Machine represents a network-accessible machine that can be remotely controlled.
// JSON tags control how field names appear in API responses (snake_case, matching FastAPI convention).
type Machine struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	IP            string `json:"ip"`
	TailscaleIP   string `json:"tailscale_ip,omitempty"` // stable 100.x.y.z address; empty if not enrolled on the tailnet
	MAC           string `json:"mac"`
	SSHUser       string `json:"ssh_user,omitempty"`
	SSHKeyPath    string `json:"ssh_key_path,omitempty"`
	SSHSudoPw     string `json:"ssh_sudo_pw,omitempty"`
	Notes         string `json:"notes,omitempty"`
	HideWake      bool   `json:"hide_wake,omitempty"`
	HideSuspend   bool   `json:"hide_suspend,omitempty"`
	HasScreentime bool   `json:"has_screentime,omitempty"`
	// IsAdmin says whether SSHUser has sudo on this machine — determines
	// whether the account-lockout trick (self-chpasswd) is available for the
	// default screentime profile (used when ScreentimeUsers is empty/unset).
	IsAdmin         bool             `json:"is_admin,omitempty"`
	ScreentimeUsers []ScreentimeUser `json:"screentime_users,omitempty"`
}

// Store is a simple in-memory data store backed by a map.
// sync.RWMutex allows safe concurrent reads (multiple goroutines) and exclusive writes.
// This is the Go equivalent of a module-level dict in Python.
type Store struct {
	mu       sync.RWMutex
	machines map[string]Machine
	order    []string // insertion order of IDs — map iteration order is randomised, so GetAll uses this for a stable dashboard listing
}

// NewStore creates a Store pre-seeded with known machines.
func NewStore() *Store {
	s := &Store{
		machines: make(map[string]Machine),
	}
	// Seed with doylestone02
	s.Create(Machine{
		ID:          "doylestone02",
		Name:        "doylestone02",
		IP:          "192.168.0.97", // drifted from the originally documented .203 — confirmed live via `ip addr` on the box itself (2026-07-27)
		TailscaleIP: "100.111.143.116",
		MAC:         "58:47:ca:70:62:27",
		SSHUser:     "jon",
		SSHKeyPath:  "/home/admin/.ssh/id_onoffapi_shutdown_doylestone02",
		Notes:       "Gaming/media PC. Auto-shuts down at 23:59 via systemd timer.",
	})
	// blackpants: handheld wifi cyberdeck — WoL unreliable over WiFi, shutdown only
	s.Create(Machine{
		ID:          "blackpants",
		Name:        "blackpants",
		IP:          "192.168.0.246",
		SSHUser:     "root",
		SSHKeyPath:  "/home/admin/.ssh/id_bh",
		Notes:       "Handheld wifi cyberdeck (doylestone02).",
		HideWake:    true,
		HideSuspend: true,
	})
	// joseph-laptop: EndlessOS laptop. WoL not viable (WiFi). Screentime managed via screentime-timer.py.
	// One-time setup: generate SSH key on this server, copy public key to joseph's authorized_keys.
	// ssh-keygen -t ed25519 -f /home/admin/.ssh/id_joseph_screentime -N ""
	// ssh-copy-id -i /home/admin/.ssh/id_joseph_screentime.pub joseph@192.168.0.102
	s.Create(Machine{
		ID:            "joseph-laptop",
		Name:          "joseph-laptop",
		IP:            "192.168.0.102",
		TailscaleIP:   "100.71.164.12",
		SSHUser:       "joseph",
		SSHKeyPath:    "/home/admin/.ssh/id_joseph_screentime",
		SSHSudoPw:     os.Getenv("JOSEPH_SUDO_PW"),
		Notes:         "Joseph's EndlessOS laptop. Screen time managed via screentime-timer.py.",
		HideWake:      true,
		HideSuspend:   true,
		HasScreentime: true,
		IsAdmin:       true, // joseph has sudo on his own laptop
	})
	// doylestone440: Ubuntu desktop (Lenovo 440), kid dev machine. SSHUser is
	// `maker` — unlike joseph, maker is NOT in the sudo group by design (see
	// the doylestone440 setup doc, Part 4), so SSHSudoPw here can't actually
	// authenticate chpasswd. Screentime timer/suspend/poweroff/screen-lock
	// (none need sudo) work the same as joseph-laptop; SET PASSWORD and
	// UNLOCK ACCOUNT will fail at the SSH layer for this machine.
	// One-time setup: generate SSH key on this server, copy public key to maker's authorized_keys.
	// ssh-keygen -t ed25519 -f /home/admin/.ssh/id_maker440_screentime -N ""
	// ssh-copy-id -i /home/admin/.ssh/id_maker440_screentime.pub maker@192.168.0.220
	s.Create(Machine{
		ID:            "doylestone440",
		Name:          "doylestone440",
		IP:            "192.168.0.220",
		TailscaleIP:   "100.82.116.98",
		SSHUser:       "maker",
		SSHKeyPath:    "/home/admin/.ssh/id_maker440_screentime",
		SSHSudoPw:     os.Getenv("MAKER440_SUDO_PW"),
		Notes:         "maker's Ubuntu dev/Minecraft machine (Lenovo 440). Screen time managed via screentime-timer.py.",
		HideWake:      true,
		HideSuspend:   true,
		HasScreentime: true,
		IsAdmin:       false, // maker has no sudo on doylestone440 — explicit, not just the zero value
	})
	// madebyjon: Jon's dev laptop (first Lenovo, T440s). WiFi-only, WoL not viable.
	s.Create(Machine{
		ID:          "madebyjon",
		Name:        "madebyjon",
		IP:          "192.168.0.218",
		TailscaleIP: "100.105.77.123",
		SSHUser:     "jon",
		SSHKeyPath:  "/home/admin/.ssh/id_onoffapi_madebyjon",
		Notes:       "Jon's development machine. First Lenovo! T440s.",
		HideWake:    true,
	})
	return s
}

// GetAll returns a slice of all machines in insertion order — map iteration
// order is randomised in Go, so this uses the Store's order slice to keep
// the dashboard listing stable across requests.
func (s *Store) GetAll() []Machine {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]Machine, 0, len(s.order))
	for _, id := range s.order {
		list = append(list, s.machines[id])
	}
	return list
}

// GetByID returns a machine by ID and a boolean indicating whether it was found.
// The (value, ok) pattern is idiomatic Go — same as Python's dict.get() with a default.
func (s *Store) GetByID(id string) (Machine, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, ok := s.machines[id]
	return m, ok
}

// Create adds a new machine. Returns false if the ID already exists.
func (s *Store) Create(m Machine) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.machines[m.ID]; exists {
		return false
	}
	s.machines[m.ID] = m
	s.order = append(s.order, m.ID)
	return true
}

// Update replaces an existing machine. Returns false if the ID does not exist.
func (s *Store) Update(id string, m Machine) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.machines[id]; !exists {
		return false
	}
	m.ID = id // ensure ID cannot be changed via update body
	s.machines[id] = m
	return true
}

// Delete removes a machine by ID. Returns false if not found.
func (s *Store) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.machines[id]; !exists {
		return false
	}
	delete(s.machines, id)
	for i, oid := range s.order {
		if oid == id {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
	return true
}
