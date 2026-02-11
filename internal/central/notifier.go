package central

import (
	"log"
	"sync"
)

// Notification is a message sent to operators or thin clients.
type Notification struct {
	Type     string // e.g. "catastrophic_rating", "dispute_opened", "center_suspended"
	Severity string // "info", "warning", "critical"
	Title    string
	Message  string
	AlertID  string
}

// Notifier dispatches notifications to registered subscribers.
// In production this would integrate with email, push, or WebSocket.
// For now it fans out to an in-memory channel and logs.
type Notifier struct {
	mu          sync.RWMutex
	subscribers []chan Notification
}

// NewNotifier creates a new notifier.
func NewNotifier() *Notifier {
	return &Notifier{}
}

// Subscribe returns a channel that receives notifications.
func (n *Notifier) Subscribe() <-chan Notification {
	ch := make(chan Notification, 100)
	n.mu.Lock()
	n.subscribers = append(n.subscribers, ch)
	n.mu.Unlock()
	return ch
}

// Send broadcasts a notification to all subscribers and logs it.
func (n *Notifier) Send(notif Notification) {
	if notif.Severity == "" {
		notif.Severity = "info"
	}
	log.Printf("[notifier] %s (%s): %s", notif.Type, notif.Severity, notif.Message)

	n.mu.RLock()
	defer n.mu.RUnlock()

	for _, ch := range n.subscribers {
		select {
		case ch <- notif:
		default:
			// Channel full, drop notification
		}
	}
}
