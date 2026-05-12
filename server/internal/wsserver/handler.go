package wsserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"llsoai-websocket/server/internal/config"
	"llsoai-websocket/server/internal/guard"
	"llsoai-websocket/server/internal/protocol"
	"llsoai-websocket/server/internal/store"
)

type Handler struct {
	Hub      *Hub
	Users    *store.UserStore
	Config   config.WebSocketConfig
	upgrader websocket.Upgrader
	serverID string
	deduper  *guard.Deduper
}

func NewHandler(hub *Hub, users *store.UserStore, cfg config.WebSocketConfig) *Handler {
	return &Handler{
		Hub:      hub,
		Users:    users,
		Config:   cfg,
		serverID: "go-server",
		deduper:  guard.NewDeduper(10000, 5*time.Minute),
		upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	remote := r.RemoteAddr
	log.Printf("[ws] incoming connection from %s url=%s", remote, r.URL.RequestURI())
	token := r.URL.Query().Get("token")
	if strings.TrimSpace(token) == "" {
		log.Printf("[ws] reject %s: missing token", remote)
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}
	tokenPreview := token
	if len(tokenPreview) > 12 {
		tokenPreview = tokenPreview[:12] + "..."
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	user, err := h.Users.FindByWebSocketToken(ctx, token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Printf("[ws] reject %s: invalid token (preview=%s)", remote, tokenPreview)
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		log.Printf("[ws] reject %s: auth error: %v", remote, err)
		http.Error(w, "auth error", http.StatusInternalServerError)
		return
	}
	log.Printf("[ws] auth ok remote=%s userId=%d username=%s tokenPreview=%s", remote, user.ID, user.Username, tokenPreview)
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ws] upgrade failed remote=%s userId=%d: %v", remote, user.ID, err)
		return
	}
	log.Printf("[ws] upgraded remote=%s userId=%d", remote, user.ID)
	client := NewClient(user.ID, conn, h.Config.SendQueueSize)
	conn.SetReadLimit(2 * 1024 * 1024)
	go h.writeLoop(client)
	h.readLoop(client)
	log.Printf("[ws] disconnected remote=%s userId=%d", remote, user.ID)
}

func (h *Handler) readLoop(c *Client) {
	defer h.Hub.UnregisterClient(c)
	_ = c.Conn.SetReadDeadline(time.Now().Add(h.Config.HeartbeatTimeout))
	c.Conn.SetPongHandler(func(string) error {
		c.Touch()
		return c.Conn.SetReadDeadline(time.Now().Add(h.Config.HeartbeatTimeout))
	})
	helloDeadline := time.Now().Add(5 * time.Second)
	contextDeadline := time.Time{}
	for {
		s := c.Snapshot()
		if s.State == StateAuthenticated && time.Now().After(helloDeadline) {
			log.Printf("[ws] hello deadline exceeded userId=%d", c.UserID)
			return
		}
		if !contextDeadline.IsZero() && s.State == StateHelloReceived && time.Now().After(contextDeadline) {
			log.Printf("[ws] context deadline exceeded userId=%d", c.UserID)
			return
		}
		var msg protocol.Envelope
		if err := c.Conn.ReadJSON(&msg); err != nil {
			log.Printf("[ws] read error userId=%d: %v", c.UserID, err)
			return
		}
		c.Touch()
		log.Printf("[ws] recv userId=%d type=%s msgId=%s workspaceId=%s sessionId=%s", c.UserID, msg.Type, msg.MessageID, msg.WorkspaceID, msg.SessionID)
		if !compatibleProtocol(msg.ProtocolVersion) {
			log.Printf("[ws] reject userId=%d unsupported protocol version=%q", c.UserID, msg.ProtocolVersion)
			h.sendError(c, msg, "unsupported_protocol_version", "unsupported protocol version")
			return
		}
		if msg.MessageID != "" && h.deduper.Seen("message:"+msg.MessageID) {
			log.Printf("[ws] duplicate userId=%d msgId=%s", c.UserID, msg.MessageID)
			h.sendError(c, msg, "duplicate_request", "duplicate message")
			continue
		}
		switch msg.Type {
		case protocol.TypeClientHello:
			h.handleHello(c, msg)
			contextDeadline = time.Now().Add(10 * time.Second)
		case protocol.TypeClientConnectionContext:
			h.handleContext(c, msg)
		case protocol.TypeClientHeartbeatPing:
			h.handleHeartbeat(c, msg)
		case protocol.TypeClientHeartbeatPong:
			c.Touch()
		case protocol.TypeClientChatHistoryReply, protocol.TypeClientChatHistoryError,
			protocol.TypeModelRequestStarted, protocol.TypeModelTextDelta, protocol.TypeModelReasoningDelta,
			protocol.TypeModelToolCallStarted, protocol.TypeModelToolCallDelta, protocol.TypeModelToolCallCompleted,
			protocol.TypeModelToolResult, protocol.TypeModelAssistantFinal, protocol.TypeModelRequestCompleted,
			protocol.TypeModelRequestCancelled, protocol.TypeModelRequestError:
			if dump, err := json.Marshal(msg); err == nil {
				log.Printf("[ws][stream<-ext] userId=%d reqId=%s type=%s envelope=%s", c.UserID, msg.RequestID, msg.Type, string(dump))
			} else {
				log.Printf("[ws][stream<-ext] userId=%d reqId=%s type=%s marshal_error=%v", c.UserID, msg.RequestID, msg.Type, err)
			}
			h.Hub.ResolvePending(msg)
		default:
			log.Printf("[ws] unhandled type userId=%d type=%s", c.UserID, msg.Type)
			h.ack(c, msg, false)
		}
	}
}

func (h *Handler) writeLoop(c *Client) {
	for msg := range c.Send {
		_ = c.Conn.SetWriteDeadline(time.Now().Add(h.Config.WriteTimeout))
		if err := c.Conn.WriteJSON(msg); err != nil {
			log.Printf("[ws] write error userId=%d type=%s: %v", c.UserID, msg.Type, err)
			c.Close()
			return
		}
		log.Printf("[ws] send userId=%d type=%s msgId=%s", c.UserID, msg.Type, msg.MessageID)
	}
	log.Printf("[ws] writeLoop exit userId=%d", c.UserID)
}

func (h *Handler) handleHello(c *Client, msg protocol.Envelope) {
	if msg.WorkspaceID == "" {
		log.Printf("[ws] hello missing workspaceId userId=%d", c.UserID)
		h.sendError(c, msg, "workspace_required", "workspaceId is required")
		return
	}
	log.Printf("[ws] hello userId=%d workspaceId=%s sessionId=%s", c.UserID, msg.WorkspaceID, msg.SessionID)
	c.SetState(StateHelloReceived)
	ack := protocol.Envelope{
		ProtocolVersion: protocol.ProtocolVersion,
		Type:            protocol.TypeServerHelloAck,
		MessageID:       protocol.NewID("msg_ack"),
		EventID:         protocol.NewID("evt_ack"),
		EventSeq:        c.NextSeq(),
		SessionID:       msg.SessionID,
		RequestID:       msg.RequestID,
		WorkspaceID:     msg.WorkspaceID,
		InstanceID:      h.serverID,
		Timestamp:       protocol.Now(),
		Source:          protocol.SourceServer,
		Payload: map[string]any{
			"accepted":            true,
			"serverName":          "llsoai-websocket",
			"serverVersion":       "0.1.0",
			"heartbeatIntervalMs": 30000,
			"enabledCapabilities": map[string]bool{"streamEvents": true, "toolEvents": true, "inboundChatMessage": true},
			"errorCode":           "",
			"errorMessage":        "",
		},
	}
	_ = c.Enqueue(ack)
}

func (h *Handler) handleContext(c *Client, msg protocol.Envelope) {
	if msg.WorkspaceID == "" || msg.InstanceID == "" {
		log.Printf("[ws] context missing fields userId=%d workspaceId=%s instanceId=%s", c.UserID, msg.WorkspaceID, msg.InstanceID)
		h.sendError(c, msg, "workspace_required", "workspaceId and instanceId are required")
		return
	}
	log.Printf("[ws] context userId=%d workspaceId=%s instanceId=%s sessionId=%s", c.UserID, msg.WorkspaceID, msg.InstanceID, msg.SessionID)
	c.SetContext(msg.WorkspaceID, msg.InstanceID, msg.SessionID, msg.Payload)
	h.Hub.RegisterClient(c)
	h.ack(c, msg, true)
}

func (h *Handler) handleHeartbeat(c *Client, msg protocol.Envelope) {
	pong := protocol.Envelope{
		ProtocolVersion: protocol.ProtocolVersion,
		Type:            protocol.TypeServerHeartbeatPong,
		MessageID:       protocol.NewID("msg_pong"),
		EventID:         protocol.NewID("evt_pong"),
		EventSeq:        c.NextSeq(),
		SessionID:       msg.SessionID,
		RequestID:       msg.RequestID,
		WorkspaceID:     msg.WorkspaceID,
		InstanceID:      h.serverID,
		Timestamp:       protocol.Now(),
		Source:          protocol.SourceServer,
		Payload: map[string]any{
			"nonce":      msg.Payload["nonce"],
			"receivedAt": protocol.Now(),
		},
	}
	_ = c.Enqueue(pong)
}

func (h *Handler) ack(c *Client, msg protocol.Envelope, accepted bool) {
	ack := protocol.Envelope{
		ProtocolVersion: protocol.ProtocolVersion,
		Type:            protocol.TypeServerAck,
		MessageID:       protocol.NewID("msg_ack"),
		EventID:         protocol.NewID("evt_ack"),
		EventSeq:        c.NextSeq(),
		SessionID:       msg.SessionID,
		RequestID:       msg.RequestID,
		WorkspaceID:     msg.WorkspaceID,
		InstanceID:      h.serverID,
		Timestamp:       protocol.Now(),
		Source:          protocol.SourceServer,
		Payload: map[string]any{
			"ackMessageId": msg.MessageID,
			"ackEventId":   msg.EventID,
			"accepted":     accepted,
		},
	}
	_ = c.Enqueue(ack)
}

func (h *Handler) sendError(c *Client, msg protocol.Envelope, code, message string) {
	errMsg := protocol.Envelope{
		ProtocolVersion: protocol.ProtocolVersion,
		Type:            protocol.TypeServerError,
		MessageID:       protocol.NewID("msg_error"),
		EventID:         protocol.NewID("evt_error"),
		EventSeq:        c.NextSeq(),
		SessionID:       msg.SessionID,
		RequestID:       msg.RequestID,
		WorkspaceID:     msg.WorkspaceID,
		InstanceID:      h.serverID,
		Timestamp:       protocol.Now(),
		Source:          protocol.SourceServer,
		Payload: map[string]any{
			"relatedMessageId": msg.MessageID,
			"errorCode":        code,
			"errorMessage":     message,
			"retryable":        false,
		},
	}
	_ = c.Enqueue(errMsg)
}

func compatibleProtocol(version string) bool {
	if version == "" {
		return false
	}
	major := strings.SplitN(version, ".", 2)[0]
	want := strings.SplitN(protocol.ProtocolVersion, ".", 2)[0]
	if _, err := strconv.Atoi(major); err != nil {
		return false
	}
	return major == want
}
