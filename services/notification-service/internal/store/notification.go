// Package store holds in-memory notification inbox state.
package store

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// EventType classifies LDN notification payloads.
type EventType string

const (
	EventNewRun       EventType = "chess:NewRun"
	EventDataReady    EventType = "chess:DataReady"
	EventSchemaChange EventType = "fabric:SchemaChange"
	EventNodeAdmit    EventType = "fabric:NodeAdmission"
	EventTrustGap     EventType = "fabric:PendingTask"
	EventGeneric      EventType = "as:Announce"
)

// Notification is a stored LDN notification.
type Notification struct {
	ID          string                 `json:"id"`
	ReceivedAt  time.Time              `json:"receivedAt"`
	Actor       string                 `json:"actor,omitempty"`
	Type        []string               `json:"type"`
	Object      map[string]interface{} `json:"object,omitempty"`
	Target      string                 `json:"target,omitempty"`
	RawBody     map[string]interface{} `json:"rawBody"`
	Acknowledged bool                  `json:"acknowledged"`
}

// Inbox is the in-memory notification store.
type Inbox struct {
	mu            sync.RWMutex
	notifications []*Notification
}

// New returns an empty Inbox.
func New() *Inbox { return &Inbox{} }

// Add stores a notification and returns it with an assigned ID.
func (i *Inbox) Add(raw map[string]interface{}) *Notification {
	n := &Notification{
		ID:         "urn:uuid:" + uuid.NewString(),
		ReceivedAt: time.Now().UTC(),
		RawBody:    raw,
	}
	// Extract well-known fields from the JSON-LD body
	if v, ok := raw["@type"]; ok {
		switch t := v.(type) {
		case string:
			n.Type = []string{t}
		case []interface{}:
			for _, s := range t {
				if str, ok := s.(string); ok {
					n.Type = append(n.Type, str)
				}
			}
		}
	}
	if v, ok := raw["actor"]; ok {
		if s, ok := v.(string); ok {
			n.Actor = s
		}
	}
	if v, ok := raw["target"]; ok {
		if s, ok := v.(string); ok {
			n.Target = s
		}
	}
	if v, ok := raw["object"]; ok {
		if m, ok := v.(map[string]interface{}); ok {
			n.Object = m
		}
	}

	i.mu.Lock()
	i.notifications = append(i.notifications, n)
	i.mu.Unlock()
	return n
}

// List returns all notifications, optionally filtered by type.
func (i *Inbox) List(typeFilter string) []*Notification {
	i.mu.RLock()
	defer i.mu.RUnlock()
	if typeFilter == "" {
		cp := make([]*Notification, len(i.notifications))
		copy(cp, i.notifications)
		return cp
	}
	var out []*Notification
	for _, n := range i.notifications {
		for _, t := range n.Type {
			if t == typeFilter {
				out = append(out, n)
				break
			}
		}
	}
	return out
}

// Get returns a notification by ID.
func (i *Inbox) Get(id string) *Notification {
	i.mu.RLock()
	defer i.mu.RUnlock()
	for _, n := range i.notifications {
		if n.ID == id {
			return n
		}
	}
	return nil
}

// Acknowledge marks a notification as handled.
func (i *Inbox) Acknowledge(id string) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	for _, n := range i.notifications {
		if n.ID == id {
			n.Acknowledged = true
			return true
		}
	}
	return false
}

// Stats returns summary counts.
func (i *Inbox) Stats() map[string]int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	total := len(i.notifications)
	acked := 0
	for _, n := range i.notifications {
		if n.Acknowledged {
			acked++
		}
	}
	return map[string]int{"total": total, "acknowledged": acked, "pending": total - acked}
}
