package httpapi

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/orchestration"
)

const (
	// heartbeatTimeout is the maximum time the hub waits between heartbeats
	// before considering a mobile node unavailable.
	heartbeatTimeout = 90 * time.Second

	// heartbeatInterval is how often the hub sends a ping frame to each client.
	heartbeatInterval = 30 * time.Second

	// writeWait is the maximum time allowed to write a message to a client.
	writeWait = 10 * time.Second

	// maxMessageBytes caps inbound WebSocket message size to 1 MiB.
	maxMessageBytes = 1 << 20
)

// wsUpgrader upgrades HTTP connections to WebSocket connections.
// CheckOrigin is permissive because mobile nodes authenticate via DID/token,
// not browser same-origin policy.
var wsUpgrader = websocket.Upgrader{
	HandshakeTimeout: 10 * time.Second,
	ReadBufferSize:   4096,
	WriteBufferSize:  4096,
	CheckOrigin:      func(r *http.Request) bool { return true },
}

// ---------------------------------------------------------------------------
// MobileClient — one connected mobile node
// ---------------------------------------------------------------------------

// MobileClient represents a single mobile node connected via WebSocket.
type MobileClient struct {
	hub         *MobileHub
	conn        *websocket.Conn
	send        chan []byte                   // outbound task descriptors
	closeOnce   sync.Once                    // ensures send is closed exactly once (H1)
	seenMu      sync.Mutex                   // guards LastSeen only; avoids hub write lock (H6)
	NodeInfo    orchestration.MobileNodeInfo // registration metadata
	ConnectedAt time.Time
	LastSeen    time.Time
}

// writePump delivers queued messages and pings from the hub to the client.
// It runs in its own goroutine and returns when the connection closes.
func (c *MobileClient) writePump() {
	ticker := time.NewTicker(heartbeatInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait)) //nolint:errcheck
			if !ok {
				// Hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{}) //nolint:errcheck
				return
			}
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(msg) //nolint:errcheck
			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait)) //nolint:errcheck
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump reads messages (results, heartbeats) from the client.
// It runs in its own goroutine and returns when the connection closes.
func (c *MobileClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageBytes)
	c.conn.SetReadDeadline(time.Now().Add(heartbeatTimeout)) //nolint:errcheck
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(heartbeatTimeout)) //nolint:errcheck
		c.hub.refreshLastSeen(c.NodeInfo.NodeDID)
		return nil
	})

	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure) {
				log.Printf("[mobilehub] client %s read error: %v", c.NodeInfo.NodeDID, err)
			}
			break
		}

		// Update liveness timestamp on every inbound message.
		c.conn.SetReadDeadline(time.Now().Add(heartbeatTimeout)) //nolint:errcheck
		c.hub.refreshLastSeen(c.NodeInfo.NodeDID)

		// Dispatch message to hub for processing (results, heartbeats, etc.)
		c.hub.inbound <- inboundMessage{client: c, data: msg}
	}
}

// ---------------------------------------------------------------------------
// MobileHub — manages all connected mobile node WebSocket clients
// ---------------------------------------------------------------------------

// inboundMessage wraps a raw message from a client for hub processing.
type inboundMessage struct {
	client *MobileClient
	data   []byte
}

// hubMessage is the union envelope used on the WebSocket wire.
type hubMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// MobileHub manages the lifecycle of connected mobile node WebSocket clients.
// It is safe for concurrent use.
type MobileHub struct {
	mu      sync.RWMutex
	clients map[string]*MobileClient // keyed by NodeDID

	register   chan *MobileClient
	unregister chan *MobileClient
	inbound    chan inboundMessage

	// ResultHandler is called when a mobile node delivers a task result.
	// Callers should set this after creating the hub.
	ResultHandler func(result orchestration.MobileTaskResult)

	// UnregisterHook is called (outside of any hub lock) after a node is
	// removed from the client map.  Wire this to bandit.RemoveArm when using
	// the ML bandit so that stale arm matrices are freed. (B3)
	UnregisterHook func(nodeDID string)
}

// NewMobileHub creates and returns a MobileHub ready to be started.
func NewMobileHub() *MobileHub {
	return &MobileHub{
		clients:    make(map[string]*MobileClient),
		register:   make(chan *MobileClient, 64),
		unregister: make(chan *MobileClient, 64),
		inbound:    make(chan inboundMessage, 256),
	}
}

// Run starts the hub event loop.  It blocks until ctx is cancelled;
// call it in its own goroutine.  Cancelling ctx is the only way to stop
// the loop — without it the goroutine would leak on server shutdown (H3).
func (h *MobileHub) Run(ctx context.Context) {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.NodeInfo.NodeDID] = client
			h.mu.Unlock()
			log.Printf("[mobilehub] registered node %s (%s)",
				client.NodeInfo.NodeDID, client.NodeInfo.NodeClass)

		case client := <-h.unregister:
			h.mu.Lock()
			if existing, ok := h.clients[client.NodeInfo.NodeDID]; ok && existing == client {
				delete(h.clients, client.NodeInfo.NodeDID)
				// closeOnce ensures we never double-close the send channel (H1).
				client.closeOnce.Do(func() { close(client.send) })
			}
			h.mu.Unlock()
			// Call the unregister hook outside the hub lock to avoid
			// deadlock if the hook itself acquires locks (B3).
			if h.UnregisterHook != nil {
				h.UnregisterHook(client.NodeInfo.NodeDID)
			}
			log.Printf("[mobilehub] unregistered node %s", client.NodeInfo.NodeDID)

		case msg := <-h.inbound:
			h.handleInbound(msg)

		case <-ctx.Done():
			return
		}
	}
}

// PushTask sends a task descriptor to a specific mobile node.
// Returns false if the node is not currently connected.
func (h *MobileHub) PushTask(nodeDID string, task orchestration.MobileTaskDescriptor) bool {
	h.mu.RLock()
	client, ok := h.clients[nodeDID]
	h.mu.RUnlock()
	if !ok {
		return false
	}

	b, err := json.Marshal(hubMessage{
		Type:    "task",
		Payload: mustMarshal(task),
	})
	if err != nil {
		log.Printf("[mobilehub] PushTask marshal error: %v", err)
		return false
	}

	select {
	case client.send <- b:
		return true
	default:
		// Send buffer full — node is likely congested; unregister it.
		h.mu.Lock()
		if existing, ok := h.clients[nodeDID]; ok && existing == client {
			delete(h.clients, nodeDID)
			// closeOnce prevents a double-close panic if Run's unregister
			// handler races with this path (H1).
			client.closeOnce.Do(func() { close(client.send) })
		}
		h.mu.Unlock()
		return false
	}
}

// ActiveNodes returns a snapshot of all currently connected mobile nodes.
func (h *MobileHub) ActiveNodes() []orchestration.MobileNodeInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]orchestration.MobileNodeInfo, 0, len(h.clients))
	for _, c := range h.clients {
		out = append(out, c.NodeInfo)
	}
	return out
}

// NodeCount returns the number of currently connected mobile nodes.
func (h *MobileHub) NodeCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// refreshLastSeen updates the last-seen timestamp for a node (called on
// heartbeat/pong and every inbound message).
//
// H6 fix: the previous implementation took a hub-wide write lock, blocking
// all concurrent readers (e.g. ActiveNodes) during frequent heartbeat
// processing.  We now take only a hub read lock to locate the client, then
// update LastSeen under the client's own seenMu — allowing concurrent reads
// of the hub map to proceed unimpeded.
func (h *MobileHub) refreshLastSeen(nodeDID string) {
	h.mu.RLock()
	c, ok := h.clients[nodeDID]
	h.mu.RUnlock()
	if ok {
		c.seenMu.Lock()
		c.LastSeen = time.Now()
		c.seenMu.Unlock()
	}
}

// handleInbound dispatches an inbound message from a mobile client to the
// appropriate handler based on the message type field.
func (h *MobileHub) handleInbound(msg inboundMessage) {
	var env hubMessage
	if err := json.Unmarshal(msg.data, &env); err != nil {
		log.Printf("[mobilehub] bad message from %s: %v", msg.client.NodeInfo.NodeDID, err)
		return
	}

	switch env.Type {
	case "heartbeat":
		var hb orchestration.MobileHeartbeat
		if err := json.Unmarshal(env.Payload, &hb); err == nil {
			h.refreshLastSeen(hb.NodeDID)
		}

	case "result":
		var result orchestration.MobileTaskResult
		if err := json.Unmarshal(env.Payload, &result); err != nil {
			log.Printf("[mobilehub] bad result payload from %s: %v",
				msg.client.NodeInfo.NodeDID, err)
			return
		}
		log.Printf("[mobilehub] result received from %s task=%s err=%q",
			msg.client.NodeInfo.NodeDID, result.TaskID, result.Error)
		if h.ResultHandler != nil {
			h.ResultHandler(result)
		}

	default:
		log.Printf("[mobilehub] unknown message type %q from %s", env.Type, msg.client.NodeInfo.NodeDID)
	}
}

// ---------------------------------------------------------------------------
// HTTP handler — ServeWS upgrades the connection and starts pump goroutines.
// ---------------------------------------------------------------------------

// ServeWS upgrades an HTTP connection to WebSocket and registers the client
// with the hub.  The client must supply its registration payload as the first
// WebSocket message (type "register").
func (h *MobileHub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[mobilehub] upgrade error: %v", err)
		return
	}

	// Read registration message (first frame only, 5-second deadline).
	conn.SetReadDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck
	_, raw, err := conn.ReadMessage()
	if err != nil {
		log.Printf("[mobilehub] no registration frame: %v", err)
		conn.Close()
		return
	}
	conn.SetReadDeadline(time.Time{}) //nolint:errcheck — reset; readPump sets proper deadline

	var env hubMessage
	if err := json.Unmarshal(raw, &env); err != nil || env.Type != "register" {
		log.Printf("[mobilehub] expected register frame, got: %s", string(raw))
		conn.Close()
		return
	}

	var info orchestration.MobileNodeInfo
	if err := json.Unmarshal(env.Payload, &info); err != nil {
		log.Printf("[mobilehub] bad register payload: %v", err)
		conn.Close()
		return
	}
	if info.NodeDID == "" {
		log.Printf("[mobilehub] register missing node_did")
		conn.Close()
		return
	}

	client := &MobileClient{
		hub:         h,
		conn:        conn,
		send:        make(chan []byte, 64),
		NodeInfo:    info,
		ConnectedAt: time.Now(),
		LastSeen:    time.Now(),
	}

	// H5 fix: use a non-blocking send so that a temporarily-full register
	// channel (capacity 64) doesn't stall the HTTP handler goroutine.
	// In practice the channel fills only if Run() is far behind.
	select {
	case h.register <- client:
	default:
		log.Printf("[mobilehub] register queue full — rejecting connection from %s", info.NodeDID)
		conn.Close()
		return
	}

	// Start read/write pumps in separate goroutines.
	go client.writePump()
	go client.readPump()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// mustMarshal marshals v to JSON, panicking on error (used for internal types
// that are always marshalable).
func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic("mobilehub: mustMarshal: " + err.Error())
	}
	return b
}
