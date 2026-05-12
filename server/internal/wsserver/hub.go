package wsserver

import (
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"llsoai-websocket/server/internal/protocol"
)

const (
	StateConnecting      = "connecting"
	StateAuthenticated   = "authenticated"
	StateHelloReceived   = "hello_received"
	StateContextReceived = "context_received"
	StateReady           = "ready"
	StateClosing         = "closing"
	StateClosed          = "closed"

	PendingModeChat    = "chat"
	PendingModeStream  = "stream"
	PendingModeHistory = "history"
)

type Client struct {
	UserID      uint64
	WorkspaceID string
	InstanceID  string
	SessionID   string
	Conn        *websocket.Conn
	Send        chan protocol.Envelope
	Metadata    map[string]any
	ConnectedAt time.Time
	LastSeenAt  time.Time
	State       string

	mu       sync.Mutex
	eventSeq int64
	closed   bool
}

type PendingRequest struct {
	RequestID   string
	UserID      uint64
	WorkspaceID string
	InstanceID  string
	SessionID   string
	Mode        string
	State       string
	Response    chan protocol.Envelope
	Stream      chan protocol.Envelope
	Final       *protocol.Envelope
	CreatedAt   time.Time
	ExpiresAt   time.Time
	Client      *Client
	mu          sync.Mutex
	done        bool
}

type ClientSnapshot struct {
	UserID      uint64
	WorkspaceID string
	InstanceID  string
	SessionID   string
	ConnectedAt time.Time
	LastSeenAt  time.Time
	State       string
}

type Hub struct {
	mu            sync.RWMutex
	pendingMu     sync.RWMutex
	byWorkspace   map[string]map[string]*Client
	byInstance    map[string]*Client
	byUser        map[uint64]map[string]*Client
	pending       map[string]*PendingRequest
	subscriptions *subscriptionState
}

func NewHub() *Hub {
	return &Hub{
		byWorkspace:   map[string]map[string]*Client{},
		byInstance:    map[string]*Client{},
		byUser:        map[uint64]map[string]*Client{},
		pending:       map[string]*PendingRequest{},
		subscriptions: newSubscriptionState(64),
	}
}

func NewClient(userID uint64, conn *websocket.Conn, queueSize int) *Client {
	if queueSize <= 0 {
		queueSize = 256
	}
	now := time.Now()
	return &Client{
		UserID:      userID,
		Conn:        conn,
		Send:        make(chan protocol.Envelope, queueSize),
		Metadata:    map[string]any{},
		ConnectedAt: now,
		LastSeenAt:  now,
		State:       StateAuthenticated,
	}
}

func (c *Client) NextSeq() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eventSeq++
	return c.eventSeq
}

func (c *Client) Snapshot() ClientSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	return ClientSnapshot{UserID: c.UserID, WorkspaceID: c.WorkspaceID, InstanceID: c.InstanceID, SessionID: c.SessionID, ConnectedAt: c.ConnectedAt, LastSeenAt: c.LastSeenAt, State: c.State}
}

func (c *Client) SetState(state string) {
	c.mu.Lock()
	c.State = state
	c.mu.Unlock()
}

func (c *Client) Touch() {
	c.mu.Lock()
	c.LastSeenAt = time.Now()
	c.mu.Unlock()
}

func (c *Client) SetContext(workspaceID, instanceID, sessionID string, metadata map[string]any) {
	c.mu.Lock()
	c.WorkspaceID = workspaceID
	c.InstanceID = instanceID
	c.SessionID = sessionID
	c.Metadata = metadata
	c.State = StateReady
	c.mu.Unlock()
}

func (c *Client) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.State = StateClosed
	close(c.Send)
	_ = c.Conn.Close()
	c.mu.Unlock()
}

func (c *Client) Enqueue(msg protocol.Envelope) error {
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return errors.New("client closed")
	}
	select {
	case c.Send <- msg:
		return nil
	default:
		c.Close()
		return errors.New("send queue full")
	}
}

func (h *Hub) RegisterClient(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := c.Snapshot()
	if s.WorkspaceID != "" && s.InstanceID != "" {
		if old := h.byInstance[s.InstanceID]; old != nil && old != c {
			old.Close()
		}
		if _, ok := h.byWorkspace[s.WorkspaceID]; !ok {
			h.byWorkspace[s.WorkspaceID] = map[string]*Client{}
		}
		h.byWorkspace[s.WorkspaceID][s.InstanceID] = c
	}
	if s.InstanceID != "" {
		h.byInstance[s.InstanceID] = c
	}
	if _, ok := h.byUser[s.UserID]; !ok {
		h.byUser[s.UserID] = map[string]*Client{}
	}
	if s.InstanceID != "" {
		h.byUser[s.UserID][s.InstanceID] = c
	}
}

func (h *Hub) UnregisterClient(c *Client) {
	s := c.Snapshot()
	h.mu.Lock()
	if s.WorkspaceID != "" {
		if m, ok := h.byWorkspace[s.WorkspaceID]; ok {
			if m[s.InstanceID] == c {
				delete(m, s.InstanceID)
			}
			if len(m) == 0 {
				delete(h.byWorkspace, s.WorkspaceID)
			}
		}
	}
	if s.InstanceID != "" {
		if h.byInstance[s.InstanceID] == c {
			delete(h.byInstance, s.InstanceID)
		}
	}
	if m, ok := h.byUser[s.UserID]; ok {
		if m[s.InstanceID] == c {
			delete(m, s.InstanceID)
		}
		if len(m) == 0 {
			delete(h.byUser, s.UserID)
		}
	}
	h.mu.Unlock()

	h.failClientPending(c, "client_disconnected")
	c.Close()
}

func (h *Hub) Route(userID uint64, workspaceID, instanceID string) (*Client, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if instanceID != "" {
		c := h.byInstance[instanceID]
		if c == nil {
			return nil, errors.New("instance_offline")
		}
		s := c.Snapshot()
		if s.UserID != userID || s.State != StateReady {
			return nil, errors.New("instance_offline")
		}
		return c, nil
	}
	workspace := h.byWorkspace[workspaceID]
	if len(workspace) == 0 {
		return nil, errors.New("workspace_offline")
	}
	var selected *Client
	for _, c := range workspace {
		s := c.Snapshot()
		if s.UserID != userID || s.State != StateReady {
			continue
		}
		if selected == nil || s.ConnectedAt.After(selected.Snapshot().ConnectedAt) {
			selected = c
		}
	}
	if selected == nil {
		return nil, errors.New("workspace_offline")
	}
	return selected, nil
}

func (h *Hub) AddPending(p *PendingRequest) {
	h.pendingMu.Lock()
	h.pending[p.RequestID] = p
	h.pendingMu.Unlock()
}

func (h *Hub) GetPending(requestID string) (*PendingRequest, bool) {
	h.pendingMu.RLock()
	p, ok := h.pending[requestID]
	h.pendingMu.RUnlock()
	return p, ok
}

func (h *Hub) DeletePending(requestID string) {
	h.pendingMu.Lock()
	delete(h.pending, requestID)
	h.pendingMu.Unlock()
}

func (h *Hub) ResolvePending(msg protocol.Envelope) bool {
	p, ok := h.GetPending(msg.RequestID)
	// 无论是否找到 pending，都尝试广播给该 workspace 的 SSE 订阅者
	h.broadcastToSubscribers(p, msg)
	if !ok {
		log.Printf("[hub][resolve] no pending for reqId=%s type=%s workspaceId=%s instanceId=%s (broadcast only)", msg.RequestID, msg.Type, msg.WorkspaceID, msg.InstanceID)
		return false
	}
	switch p.Mode {
	case PendingModeStream:
		if dump, err := json.Marshal(msg); err == nil {
			log.Printf("[hub][stream->sse] reqId=%s type=%s workspaceId=%s instanceId=%s envelope=%s", p.RequestID, msg.Type, p.WorkspaceID, p.InstanceID, string(dump))
		}
		if !p.sendStream(msg, time.Second) {
			log.Printf("[hub][stream->sse] reqId=%s type=%s drop: sse channel blocked or closed", p.RequestID, msg.Type)
			h.DeletePending(p.RequestID)
			p.finish(protocol.Envelope{Type: "sse_disconnected", RequestID: p.RequestID, WorkspaceID: p.WorkspaceID, InstanceID: p.InstanceID})
			return true
		}
		if msg.Type == protocol.TypeModelRequestCompleted || msg.Type == protocol.TypeModelRequestCancelled || msg.Type == protocol.TypeModelRequestError {
			log.Printf("[hub][stream->sse] reqId=%s terminal type=%s, closing stream", p.RequestID, msg.Type)
			h.DeletePending(p.RequestID)
			p.closeStream()
		}
	case PendingModeHistory:
		if msg.Type != protocol.TypeClientChatHistoryReply && msg.Type != protocol.TypeClientChatHistoryError {
			return true
		}
		h.DeletePending(p.RequestID)
		p.finish(msg)
	case PendingModeChat:
		if msg.Type == protocol.TypeModelAssistantFinal {
			copy := msg
			p.Final = &copy
			return true
		}
		if msg.Type == protocol.TypeModelRequestCompleted || msg.Type == protocol.TypeModelRequestCancelled || msg.Type == protocol.TypeModelRequestError {
			if p.Final != nil && msg.Type == protocol.TypeModelRequestCompleted {
				msg = *p.Final
			} else if p.Final == nil && msg.Type == protocol.TypeModelRequestCompleted {
				msg.Type = "empty_assistant_final"
				msg.Payload = map[string]any{"errorCode": "empty_assistant_final", "errorMessage": "assistant final message not received"}
			}
			h.DeletePending(p.RequestID)
			p.finish(msg)
		}
	}
	return true
}

func (h *Hub) CleanupExpired() {
	now := time.Now()
	var expired []*PendingRequest
	h.pendingMu.Lock()
	for id, p := range h.pending {
		if !p.ExpiresAt.IsZero() && now.After(p.ExpiresAt) {
			expired = append(expired, p)
			delete(h.pending, id)
		}
	}
	h.pendingMu.Unlock()
	for _, p := range expired {
		msg := protocol.Envelope{Type: "timeout", RequestID: p.RequestID, WorkspaceID: p.WorkspaceID, InstanceID: p.InstanceID}
		p.finish(msg)
	}
}

func (h *Hub) failClientPending(c *Client, code string) {
	var matched []*PendingRequest
	h.pendingMu.Lock()
	for id, p := range h.pending {
		if p.Client == c {
			matched = append(matched, p)
			delete(h.pending, id)
		}
	}
	h.pendingMu.Unlock()
	for _, p := range matched {
		msg := protocol.Envelope{Type: code, RequestID: p.RequestID, WorkspaceID: p.WorkspaceID, InstanceID: p.InstanceID}
		p.finish(msg)
	}
}

// broadcastToSubscribers 将一条扩展回报的消息按 (userId + workspaceId) 推给对应订阅者。
// 优先从 pending 取 userId/workspaceId；否则按 InstanceID 反查 client。
func (h *Hub) broadcastToSubscribers(p *PendingRequest, msg protocol.Envelope) {
	userID := uint64(0)
	workspaceID := msg.WorkspaceID
	if p != nil {
		userID = p.UserID
		if workspaceID == "" {
			workspaceID = p.WorkspaceID
		}
	}
	if userID == 0 {
		h.mu.RLock()
		c := h.byInstance[msg.InstanceID]
		h.mu.RUnlock()
		if c != nil {
			s := c.Snapshot()
			userID = s.UserID
			if workspaceID == "" {
				workspaceID = s.WorkspaceID
			}
		}
	}
	if userID == 0 || workspaceID == "" {
		return
	}
	n := h.Broadcast(userID, workspaceID, msg)
	if n > 0 {
		log.Printf("[hub][broadcast] userId=%d workspaceId=%s type=%s reqId=%s delivered=%d", userID, workspaceID, msg.Type, msg.RequestID, n)
	}
}

func (p *PendingRequest) finish(msg protocol.Envelope) {
	p.mu.Lock()
	if p.done {
		p.mu.Unlock()
		return
	}
	p.done = true
	resp := p.Response
	stream := p.Stream
	p.mu.Unlock()
	if resp != nil {
		select {
		case resp <- msg:
		default:
		}
	}
	if stream != nil {
		close(stream)
	}
}

func (p *PendingRequest) closeStream() {
	p.mu.Lock()
	if p.done {
		p.mu.Unlock()
		return
	}
	p.done = true
	stream := p.Stream
	p.mu.Unlock()
	if stream != nil {
		close(stream)
	}
}

func (p *PendingRequest) sendStream(msg protocol.Envelope, timeout time.Duration) bool {
	p.mu.Lock()
	if p.done || p.Stream == nil {
		p.mu.Unlock()
		return false
	}
	stream := p.Stream
	p.mu.Unlock()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case stream <- msg:
		return true
	case <-timer.C:
		return false
	}
}

// WorkspaceInfo workspace 信息
type WorkspaceInfo struct {
	WorkspaceID string         `json:"workspaceId"`
	InstanceID  string         `json:"instanceId"`
	SessionID   string         `json:"sessionId,omitempty"`
	ConnectedAt time.Time      `json:"connectedAt,omitempty"`
	LastSeenAt  time.Time      `json:"lastSeenAt,omitempty"`
	State       string         `json:"state"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// ListWorkspaces 返回用户的所有 workspace
func (h *Hub) ListWorkspaces(userID uint64) []WorkspaceInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()

	instances, ok := h.byUser[userID]
	if !ok {
		return []WorkspaceInfo{}
	}

	result := make([]WorkspaceInfo, 0, len(instances))
	for _, client := range instances {
		snap := client.Snapshot()
		info := WorkspaceInfo{
			WorkspaceID: snap.WorkspaceID,
			InstanceID:  snap.InstanceID,
			SessionID:   snap.SessionID,
			ConnectedAt: snap.ConnectedAt,
			LastSeenAt:  snap.LastSeenAt,
			State:       snap.State,
			Metadata:    client.Metadata,
		}
		result = append(result, info)
	}
	return result
}
