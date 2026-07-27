package models

import "testing"

// TestGetAll_StableInsertionOrder guards against the dashboard reshuffling
// machine cards on every page load — Go map iteration order is randomised,
// so GetAll must return machines in the order they were created, not map order.
func TestGetAll_StableInsertionOrder(t *testing.T) {
	s := &Store{machines: make(map[string]Machine)}
	s.Create(Machine{ID: "c", Name: "c"})
	s.Create(Machine{ID: "a", Name: "a"})
	s.Create(Machine{ID: "b", Name: "b"})

	want := []string{"c", "a", "b"}
	for i := 0; i < 5; i++ {
		got := s.GetAll()
		if len(got) != len(want) {
			t.Fatalf("expected %d machines, got %d", len(want), len(got))
		}
		for j, id := range want {
			if got[j].ID != id {
				t.Fatalf("run %d: position %d: expected %q, got %q", i, j, id, got[j].ID)
			}
		}
	}
}

func TestGetAll_OrderSurvivesDelete(t *testing.T) {
	s := &Store{machines: make(map[string]Machine)}
	s.Create(Machine{ID: "a", Name: "a"})
	s.Create(Machine{ID: "b", Name: "b"})
	s.Create(Machine{ID: "c", Name: "c"})
	s.Delete("b")

	got := s.GetAll()
	want := []string{"a", "c"}
	if len(got) != len(want) {
		t.Fatalf("expected %d machines, got %d", len(want), len(got))
	}
	for j, id := range want {
		if got[j].ID != id {
			t.Fatalf("position %d: expected %q, got %q", j, id, got[j].ID)
		}
	}
}

func TestNewStore_SeedOrder(t *testing.T) {
	s := NewStore()
	got := s.GetAll()
	want := []string{"doylestone02", "blackpants", "joseph-laptop", "doylestone440", "madebyjon"}
	if len(got) != len(want) {
		t.Fatalf("expected %d seeded machines, got %d", len(want), len(got))
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Errorf("position %d: expected %q, got %q", i, id, got[i].ID)
		}
	}
}
