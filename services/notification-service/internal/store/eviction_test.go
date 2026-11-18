package store

import "testing"

func TestInboxEvictionDropsOldestAcknowledgedFirst(t *testing.T) {
	inbox := NewWithCap(5)
	ids := make([]string, 0, 7)
	for i := 0; i < 3; i++ {
		ids = append(ids, inbox.Add(map[string]any{"@type": "as:Announce"}).ID)
	}
	// Acknowledge the two oldest while under the cap.
	if !inbox.Acknowledge(ids[0]) || !inbox.Acknowledge(ids[1]) {
		t.Fatal("acknowledge failed")
	}
	// Add four more (total 7, cap 5) — the two acknowledged oldest entries
	// must be evicted before any unacknowledged ones.
	for i := 0; i < 4; i++ {
		ids = append(ids, inbox.Add(map[string]any{"@type": "as:Announce"}).ID)
	}
	remaining := map[string]bool{}
	for _, n := range inbox.List("") {
		remaining[n.ID] = true
	}
	if len(remaining) != 5 {
		t.Fatalf("want 5 notifications after eviction, got %d", len(remaining))
	}
	if remaining[ids[0]] || remaining[ids[1]] {
		t.Error("oldest acknowledged notifications should have been evicted first")
	}
	if !remaining[ids[2]] {
		t.Error("unacknowledged notification should have been kept")
	}
	if s := inbox.Stats(); s["total"] != 5 || s["pending"] != 5 {
		t.Errorf("stats = %v, want total=5 pending=5", s)
	}
}

func TestInboxEvictionDropsOldestWhenNothingAcknowledged(t *testing.T) {
	inbox := NewWithCap(3)
	var first string
	for i := 0; i < 5; i++ {
		n := inbox.Add(map[string]any{"@type": "as:Announce"})
		if i == 0 {
			first = n.ID
		}
	}
	list := inbox.List("")
	if len(list) != 3 {
		t.Fatalf("want 3 notifications, got %d", len(list))
	}
	if list[0].ID == first {
		t.Error("oldest unacknowledged notification should have been dropped")
	}
}

func TestInboxCapZeroDisablesEviction(t *testing.T) {
	inbox := NewWithCap(0)
	for i := 0; i < 20; i++ {
		inbox.Add(map[string]any{"@type": "as:Announce"})
	}
	if got := len(inbox.List("")); got != 20 {
		t.Fatalf("cap<=0 must disable eviction; want 20, got %d", got)
	}
}
