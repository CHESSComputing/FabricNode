package store_test

import (
	"sync"
	"testing"

	"github.com/CHESSComputing/FabricNode/services/notification-service/internal/store"
)

// ── New / type consistency ────────────────────────────────────────────────────

// TestNew_TypeConsistency verifies that store.New() returns *store.Inbox,
// matching the type declared in handlers/config.go (Config.Inbox *store.Inbox).
// This is the regression guard for TODO item #2.
func TestNew_TypeConsistency(t *testing.T) {
	inbox := store.New()
	if inbox == nil {
		t.Fatal("store.New() returned nil")
	}
	// Compile-time proof: assign to a named *store.Inbox variable.
	var _ *store.Inbox = inbox
}

// ── Add ───────────────────────────────────────────────────────────────────────

func TestAdd_ReturnsNotificationWithID(t *testing.T) {
	inbox := store.New()
	n := inbox.Add(map[string]any{"@type": "as:Announce"})
	if n == nil {
		t.Fatal("Add returned nil")
	}
	if n.ID == "" {
		t.Error("notification ID should not be empty")
	}
	if !isURNUUID(n.ID) {
		t.Errorf("ID should be a urn:uuid: string, got %q", n.ID)
	}
}

func TestAdd_ExtractsTypeFromStringValue(t *testing.T) {
	inbox := store.New()
	n := inbox.Add(map[string]any{"@type": "chess:NewRun"})
	if len(n.Type) != 1 || n.Type[0] != "chess:NewRun" {
		t.Errorf("expected Type=[\"chess:NewRun\"], got %v", n.Type)
	}
}

func TestAdd_ExtractsTypeFromSlice(t *testing.T) {
	inbox := store.New()
	n := inbox.Add(map[string]any{
		"@type": []any{"chess:DataReady", "as:Announce"},
	})
	if len(n.Type) != 2 {
		t.Fatalf("expected 2 types, got %v", n.Type)
	}
}

func TestAdd_ExtractsActor(t *testing.T) {
	inbox := store.New()
	n := inbox.Add(map[string]any{
		"@type": "as:Announce",
		"actor": "https://peer-node.example.org",
	})
	if n.Actor != "https://peer-node.example.org" {
		t.Errorf("expected actor URL, got %q", n.Actor)
	}
}

func TestAdd_ExtractsTarget(t *testing.T) {
	inbox := store.New()
	n := inbox.Add(map[string]any{
		"@type":  "as:Announce",
		"target": "https://local-node.example.org/inbox",
	})
	if n.Target != "https://local-node.example.org/inbox" {
		t.Errorf("expected target, got %q", n.Target)
	}
}

func TestAdd_ExtractsObject(t *testing.T) {
	obj := map[string]any{"@id": "https://example.org/dataset/1", "@type": "dcat:Dataset"}
	inbox := store.New()
	n := inbox.Add(map[string]any{"@type": "as:Announce", "object": obj})
	if n.Object == nil {
		t.Fatal("expected Object to be set")
	}
	if n.Object["@id"] != "https://example.org/dataset/1" {
		t.Errorf("Object['@id'] = %v", n.Object["@id"])
	}
}

func TestAdd_RawBodyPreserved(t *testing.T) {
	raw := map[string]any{"@type": "chess:NewRun", "extra": "value"}
	inbox := store.New()
	n := inbox.Add(raw)
	if n.RawBody["extra"] != "value" {
		t.Errorf("RawBody not preserved, got %v", n.RawBody)
	}
}

func TestAdd_NotAcknowledgedByDefault(t *testing.T) {
	inbox := store.New()
	n := inbox.Add(map[string]any{"@type": "as:Announce"})
	if n.Acknowledged {
		t.Error("new notification should not be acknowledged")
	}
}

// ── List ──────────────────────────────────────────────────────────────────────

func TestList_EmptyInbox(t *testing.T) {
	inbox := store.New()
	if got := inbox.List(""); len(got) != 0 {
		t.Errorf("empty inbox should return 0 notifications, got %d", len(got))
	}
}

func TestList_NoFilter_ReturnsAll(t *testing.T) {
	inbox := store.New()
	inbox.Add(map[string]any{"@type": "chess:NewRun"})
	inbox.Add(map[string]any{"@type": "chess:DataReady"})
	inbox.Add(map[string]any{"@type": "as:Announce"})
	got := inbox.List("")
	if len(got) != 3 {
		t.Errorf("expected 3, got %d", len(got))
	}
}

func TestList_TypeFilter_MatchesOnly(t *testing.T) {
	inbox := store.New()
	inbox.Add(map[string]any{"@type": "chess:NewRun"})
	inbox.Add(map[string]any{"@type": "chess:DataReady"})
	inbox.Add(map[string]any{"@type": "chess:NewRun"})
	got := inbox.List("chess:NewRun")
	if len(got) != 2 {
		t.Errorf("filter by chess:NewRun: expected 2, got %d", len(got))
	}
}

func TestList_TypeFilter_NoMatch(t *testing.T) {
	inbox := store.New()
	inbox.Add(map[string]any{"@type": "chess:NewRun"})
	got := inbox.List("chess:DataReady")
	if len(got) != 0 {
		t.Errorf("non-matching filter should return 0, got %d", len(got))
	}
}

// ── Get ───────────────────────────────────────────────────────────────────────

func TestGet_ExistingID(t *testing.T) {
	inbox := store.New()
	added := inbox.Add(map[string]any{"@type": "as:Announce"})
	got := inbox.Get(added.ID)
	if got == nil {
		t.Fatal("Get returned nil for existing ID")
	}
	if got.ID != added.ID {
		t.Errorf("ID mismatch: want %q, got %q", added.ID, got.ID)
	}
}

func TestGet_UnknownID_ReturnsNil(t *testing.T) {
	inbox := store.New()
	inbox.Add(map[string]any{"@type": "as:Announce"})
	if got := inbox.Get("urn:uuid:does-not-exist"); got != nil {
		t.Errorf("unknown ID should return nil, got %v", got)
	}
}

// ── Acknowledge ───────────────────────────────────────────────────────────────

func TestAcknowledge_ExistingID(t *testing.T) {
	inbox := store.New()
	n := inbox.Add(map[string]any{"@type": "as:Announce"})
	if !inbox.Acknowledge(n.ID) {
		t.Fatal("Acknowledge returned false for existing ID")
	}
	got := inbox.Get(n.ID)
	if !got.Acknowledged {
		t.Error("notification should be marked acknowledged")
	}
}

func TestAcknowledge_UnknownID_ReturnsFalse(t *testing.T) {
	inbox := store.New()
	if inbox.Acknowledge("urn:uuid:no-such-id") {
		t.Error("Acknowledge should return false for unknown ID")
	}
}

func TestAcknowledge_Idempotent(t *testing.T) {
	inbox := store.New()
	n := inbox.Add(map[string]any{"@type": "as:Announce"})
	inbox.Acknowledge(n.ID)
	if !inbox.Acknowledge(n.ID) {
		t.Error("second Acknowledge should also return true")
	}
}

// ── Stats ─────────────────────────────────────────────────────────────────────

func TestStats_Empty(t *testing.T) {
	inbox := store.New()
	s := inbox.Stats()
	if s["total"] != 0 || s["acknowledged"] != 0 || s["pending"] != 0 {
		t.Errorf("empty inbox stats should be all zero, got %v", s)
	}
}

func TestStats_MixedAcknowledgement(t *testing.T) {
	inbox := store.New()
	n1 := inbox.Add(map[string]any{"@type": "as:Announce"})
	inbox.Add(map[string]any{"@type": "as:Announce"})
	inbox.Add(map[string]any{"@type": "as:Announce"})
	inbox.Acknowledge(n1.ID)

	s := inbox.Stats()
	if s["total"] != 3 {
		t.Errorf("total: want 3, got %d", s["total"])
	}
	if s["acknowledged"] != 1 {
		t.Errorf("acknowledged: want 1, got %d", s["acknowledged"])
	}
	if s["pending"] != 2 {
		t.Errorf("pending: want 2, got %d", s["pending"])
	}
}

// ── Concurrency ───────────────────────────────────────────────────────────────

func TestInbox_ConcurrentAddAndList(t *testing.T) {
	inbox := store.New()
	const goroutines = 20
	const perGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				inbox.Add(map[string]any{"@type": "as:Announce"})
			}
		}()
	}
	wg.Wait()

	got := inbox.List("")
	if len(got) != goroutines*perGoroutine {
		t.Errorf("expected %d notifications after concurrent adds, got %d",
			goroutines*perGoroutine, len(got))
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func isURNUUID(s string) bool {
	return len(s) > 9 && s[:9] == "urn:uuid:"
}
