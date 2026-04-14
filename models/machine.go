package models

import "sync"

// Machine represents a network-accessible machine that can be remotely controlled.
// JSON tags control how field names appear in API responses (snake_case, matching FastAPI convention).
type Machine struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IP         string `json:"ip"`
	MAC        string `json:"mac"`
	SSHUser    string `json:"ssh_user,omitempty"`
	SSHKeyPath string `json:"ssh_key_path,omitempty"`
	Notes      string `json:"notes,omitempty"`
}

// Store is a simple in-memory data store backed by a map.
// sync.RWMutex allows safe concurrent reads (multiple goroutines) and exclusive writes.
// This is the Go equivalent of a module-level dict in Python.
type Store struct {
	mu       sync.RWMutex
	machines map[string]Machine
}

// NewStore creates a Store pre-seeded with known machines.
func NewStore() *Store {
	s := &Store{
		machines: make(map[string]Machine),
	}
	// Seed with doylestone02
	s.machines["doylestone02"] = Machine{
		ID:         "doylestone02",
		Name:       "doylestone02",
		IP:         "192.168.0.203",
		MAC:        "58:47:ca:70:62:27",
		SSHUser:    "jon",
		SSHKeyPath: "/home/admin/.ssh/id_onoffapi_shutdown_doylestone02",
		Notes:      "Gaming/media PC. Auto-shuts down at 23:59 via systemd timer.",
	}
	return s
}

// GetAll returns a slice of all machines.
// RLock allows multiple concurrent readers.
func (s *Store) GetAll() []Machine {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]Machine, 0, len(s.machines))
	for _, m := range s.machines {
		list = append(list, m)
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
	return true
}
