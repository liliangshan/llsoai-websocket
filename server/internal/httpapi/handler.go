package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"llsoai-websocket/server/internal/config"
	"llsoai-websocket/server/internal/guard"
	"llsoai-websocket/server/internal/protocol"
	"llsoai-websocket/server/internal/wsserver"
)

type Handler struct {
	Hub      *wsserver.Hub
	Auth     *Authenticator
	HTTP     config.HTTPConfig
	ServerID string
	httpLim  *guard.RateLimiter
	histLim  *guard.RateLimiter
	deduper  *guard.Deduper
}

func NewHandler(hub *wsserver.Hub, auth *Authenticator, cfg config.HTTPConfig) *Handler {
	return &Handler{Hub: hub, Auth: auth, HTTP: cfg, ServerID: "go-server", httpLim: guard.NewRateLimiter(5, time.Second), histLim: guard.NewRateLimiter(10, time.Minute), deduper: guard.NewDeduper(10000, 5*time.Minute)}
}

type chatRequest struct {
	WorkspaceID string `json:"workspaceId"`
	InstanceID  string `json:"instanceId"`
	SessionID   string `json:"sessionId"`
	Text        string `json:"text"`
	AutoSend    bool   `json:"autoSend"`
	DedupeKey   string `json:"dedupeKey"`
}

type historyRequest struct {
	WorkspaceID string `json:"workspaceId"`
	InstanceID  string `json:"instanceId"`
	SessionID   string `json:"sessionId"`
	Limit       int    `json:"limit"`
	Order       string `json:"order"`
	Scope       string `json:"scope"`
}

func (h *Handler) Register(mux *http.ServeMux) {
	// Auth endpoints (no auth required)
	mux.HandleFunc("/api/auth/login", h.Auth.Login)
	mux.HandleFunc("/api/auth/register", h.Auth.Register)
	// Authenticated endpoints
	mux.HandleFunc("/api/me", h.Auth.Me)
	mux.HandleFunc("/api/me/websocket-token", h.Auth.WebSocketToken)
	mux.HandleFunc("/api/me/websocket-token/rotate", h.Auth.RotateWebSocketToken)
	mux.HandleFunc("/api/workspaces", h.handleWorkspaces)
	mux.HandleFunc("/api/workspaces/stream", h.handleWorkspaceStream)
	mux.HandleFunc("/api/chat", h.handleChat)
	mux.HandleFunc("/api/chat/trigger", h.handleChatTrigger)
	mux.HandleFunc("/api/chat/stream", h.handleChatStream)
	mux.HandleFunc("/api/chat/cancel", h.handleChatCancel)
	mux.HandleFunc("/api/chat/history", h.handleHistory)
}

// handleWorkspaces 返回用户的所有 workspace
func (h *Handler) handleWorkspaces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userID, err := h.Auth.UserID(r)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized", "", "", "")
		return
	}
	workspaces := h.Hub.ListWorkspaces(userID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"workspaces": workspaces,
	})
}

func (h *Handler) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userID, err := h.Auth.UserID(r)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized", "", "", "")
		return
	}
	if !h.httpLim.Allow(fmt.Sprintf("%d", userID)) {
		h.writeError(w, http.StatusTooManyRequests, "rate_limited", "rate limited", "", "", "")
		return
	}
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.WorkspaceID == "" || req.Text == "" {
		h.writeError(w, http.StatusBadRequest, "workspace_required", "workspaceId and text are required", "", req.WorkspaceID, req.InstanceID)
		return
	}
	if req.DedupeKey != "" && h.deduper.Seen("dedupe:"+req.DedupeKey) {
		h.writeError(w, http.StatusConflict, "duplicate_request", "duplicate request", "", req.WorkspaceID, req.InstanceID)
		return
	}
	client, err := h.Hub.Route(userID, req.WorkspaceID, req.InstanceID)
	if err != nil {
		h.writeRouteError(w, err, "", req.WorkspaceID, req.InstanceID)
		return
	}
	requestID := protocol.NewID("req")
	pending := &wsserver.PendingRequest{
		RequestID:   requestID,
		UserID:      userID,
		WorkspaceID: req.WorkspaceID,
		InstanceID:  client.InstanceID,
		SessionID:   req.SessionID,
		Mode:        wsserver.PendingModeChat,
		State:       "created",
		Response:    make(chan protocol.Envelope, 1),
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(h.HTTP.RequestTimeout),
		Client:      client,
	}
	h.Hub.AddPending(pending)
	msg := h.newServerMessage(protocol.TypeServerChatMessage, requestID, req.SessionID, req.WorkspaceID, map[string]any{
		"text":                    req.Text,
		"autoSend":                req.AutoSend,
		"bypassPromptEnhancement": true,
		"dedupeKey":               req.DedupeKey,
		"expireAt":                time.Now().Add(2 * time.Minute).UTC().Format("2006-01-02T15:04:05.000Z"),
	})
	if err := client.Enqueue(msg); err != nil {
		h.Hub.DeletePending(requestID)
		h.writeError(w, http.StatusServiceUnavailable, "upstream_write_failed", "failed to write to extension", requestID, req.WorkspaceID, client.InstanceID)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.HTTP.RequestTimeout)
	defer cancel()
	select {
	case resp := <-pending.Response:
		h.writeEnvelopeResponse(w, resp, requestID, req.WorkspaceID, client.InstanceID)
	case <-ctx.Done():
		h.Hub.DeletePending(requestID)
		h.writeError(w, http.StatusGatewayTimeout, "request_timeout", "request timeout", requestID, req.WorkspaceID, client.InstanceID)
	}
}

func (h *Handler) handleChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userID, err := h.Auth.UserID(r)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized", "", "", "")
		return
	}
	if !h.httpLim.Allow(fmt.Sprintf("%d", userID)) {
		h.writeError(w, http.StatusTooManyRequests, "rate_limited", "rate limited", "", "", "")
		return
	}
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.WorkspaceID == "" || req.Text == "" {
		h.writeError(w, http.StatusBadRequest, "workspace_required", "workspaceId and text are required", "", req.WorkspaceID, req.InstanceID)
		return
	}
	if req.DedupeKey != "" && h.deduper.Seen("dedupe:"+req.DedupeKey) {
		h.writeError(w, http.StatusConflict, "duplicate_request", "duplicate request", "", req.WorkspaceID, req.InstanceID)
		return
	}
	client, err := h.Hub.Route(userID, req.WorkspaceID, req.InstanceID)
	if err != nil {
		h.writeRouteError(w, err, "", req.WorkspaceID, req.InstanceID)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "stream unsupported", "", req.WorkspaceID, client.InstanceID)
		return
	}
	if !h.histLim.Allow(fmt.Sprintf("%d", userID)) {
		h.writeError(w, http.StatusTooManyRequests, "rate_limited", "rate limited", "", "", "")
		return
	}
	requestID := protocol.NewID("req")
	log.Printf("[sse][stream] open reqId=%s userId=%d workspaceId=%s instanceId=%s text_len=%d", requestID, userID, req.WorkspaceID, client.InstanceID, len(req.Text))
	stream := make(chan protocol.Envelope, 32)
	pending := &wsserver.PendingRequest{RequestID: requestID, UserID: userID, WorkspaceID: req.WorkspaceID, InstanceID: client.InstanceID, SessionID: req.SessionID, Mode: wsserver.PendingModeStream, State: "created", Stream: stream, Response: make(chan protocol.Envelope, 1), CreatedAt: time.Now(), ExpiresAt: time.Now().Add(h.HTTP.SSEMaxLifetime), Client: client}
	h.Hub.AddPending(pending)
	msg := h.newServerMessage(protocol.TypeServerChatMessage, requestID, req.SessionID, req.WorkspaceID, map[string]any{"text": req.Text, "autoSend": req.AutoSend, "bypassPromptEnhancement": true, "dedupeKey": req.DedupeKey, "expireAt": time.Now().Add(2 * time.Minute).UTC().Format("2006-01-02T15:04:05.000Z")})
	if err := client.Enqueue(msg); err != nil {
		log.Printf("[sse][stream] enqueue failed reqId=%s err=%v", requestID, err)
		h.Hub.DeletePending(requestID)
		h.writeError(w, http.StatusServiceUnavailable, "upstream_write_failed", "failed to write to extension", requestID, req.WorkspaceID, client.InstanceID)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	ctx, cancel := context.WithTimeout(r.Context(), h.HTTP.SSEMaxLifetime)
	defer cancel()
	ping := time.NewTicker(h.HTTP.SSEPingInterval)
	defer ping.Stop()
	for {
		select {
		case msg, ok := <-stream:
			if !ok {
				log.Printf("[sse][stream] reqId=%s channel closed", requestID)
				return
			}
			event := msg.Type
			if msg.Type == protocol.TypeModelRequestCompleted {
				event = "done"
			}
			if msg.Type == protocol.TypeModelRequestCancelled {
				event = "cancelled"
			}
			if msg.Type == protocol.TypeModelRequestError {
				event = "error"
			}
			if dump, mErr := json.Marshal(msg); mErr == nil {
				log.Printf("[sse][stream->web] reqId=%s event=%s type=%s payload=%s", requestID, event, msg.Type, string(dump))
			}
			writeSSE(w, event, msg)
			flusher.Flush()
			if event == "done" || event == "cancelled" || event == "error" {
				log.Printf("[sse][stream->web] reqId=%s terminal event=%s, closing http response", requestID, event)
				return
			}
		case <-ctx.Done():
			h.Hub.DeletePending(requestID)
			writeSSE(w, "error", protocol.APIError{Code: "sse_disconnected", Message: "sse disconnected"})
			flusher.Flush()
			return
		case <-ping.C:
			_, _ = w.Write([]byte(": ping\n\n"))
			flusher.Flush()
		}
	}
}

func (h *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userID, err := h.Auth.UserID(r)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized", "", "", "")
		return
	}
	var req historyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_chat_history_request", "invalid request", "", "", "")
		return
	}
	if req.Scope == "" {
		req.Scope = "project"
	}
	if req.Order == "" {
		req.Order = "asc"
	}
	if req.Limit == 0 {
		req.Limit = 100
	}
	if req.WorkspaceID == "" || req.Scope != "project" || req.Order != "asc" || req.Limit < 1 {
		h.writeError(w, http.StatusBadRequest, "invalid_chat_history_request", "invalid history request", "", req.WorkspaceID, req.InstanceID)
		return
	}
	if req.Limit > 100 {
		req.Limit = 100
	}
	client, err := h.Hub.Route(userID, req.WorkspaceID, req.InstanceID)
	if err != nil {
		h.writeRouteError(w, err, "", req.WorkspaceID, req.InstanceID)
		return
	}
	requestID := protocol.NewID("history_request")
	pending := &wsserver.PendingRequest{RequestID: requestID, UserID: userID, WorkspaceID: req.WorkspaceID, InstanceID: client.InstanceID, SessionID: req.SessionID, Mode: wsserver.PendingModeHistory, State: "created", Response: make(chan protocol.Envelope, 1), CreatedAt: time.Now(), ExpiresAt: time.Now().Add(h.HTTP.HistoryTimeout), Client: client}
	h.Hub.AddPending(pending)
	msg := h.newServerMessage(protocol.TypeServerChatHistoryRequest, requestID, req.SessionID, req.WorkspaceID, map[string]any{"limit": req.Limit, "order": "asc", "scope": "project"})
	if err := client.Enqueue(msg); err != nil {
		h.Hub.DeletePending(requestID)
		h.writeError(w, http.StatusServiceUnavailable, "upstream_write_failed", "failed to write to extension", requestID, req.WorkspaceID, client.InstanceID)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.HTTP.HistoryTimeout)
	defer cancel()
	select {
	case resp := <-pending.Response:
		if resp.Type == protocol.TypeClientChatHistoryError {
			code, _ := resp.Payload["errorCode"].(string)
			message, _ := resp.Payload["errorMessage"].(string)
			retryable, _ := resp.Payload["retryable"].(bool)
			writeJSON(w, http.StatusOK, protocol.APIResponse{RequestID: requestID, OK: false, WorkspaceID: req.WorkspaceID, InstanceID: client.InstanceID, Error: &protocol.APIError{Code: code, Message: message, Retryable: protocol.BoolPtr(retryable)}})
			return
		}
		payload := resp.Payload
		if payload == nil {
			payload = map[string]any{}
		}
		if err := h.validateHistoryPayload(payload); err != nil {
			h.writeError(w, http.StatusOK, "CHAT_HISTORY_RESPONSE_TOO_LARGE", err.Error(), requestID, req.WorkspaceID, client.InstanceID)
			return
		}
		writeJSON(w, http.StatusOK, protocol.APIResponse{RequestID: requestID, OK: true, WorkspaceID: req.WorkspaceID, InstanceID: client.InstanceID, Payload: payload, Error: nil})
	case <-ctx.Done():
		h.Hub.DeletePending(requestID)
		h.writeError(w, http.StatusGatewayTimeout, "history_request_timeout", "history request timeout", requestID, req.WorkspaceID, client.InstanceID)
	}
}

func (h *Handler) newServerMessage(typ, requestID, sessionID, workspaceID string, payload map[string]any) protocol.Envelope {
	return protocol.Envelope{ProtocolVersion: protocol.ProtocolVersion, Type: typ, MessageID: protocol.NewID("msg"), EventID: protocol.NewID("evt"), EventSeq: 0, SessionID: sessionID, RequestID: requestID, WorkspaceID: workspaceID, InstanceID: h.ServerID, Timestamp: protocol.Now(), Source: protocol.SourceServer, Payload: payload}
}

func (h *Handler) writeEnvelopeResponse(w http.ResponseWriter, msg protocol.Envelope, requestID, workspaceID, instanceID string) {
	if msg.Type == protocol.TypeModelRequestError || msg.Type == protocol.TypeModelRequestCancelled || msg.Type == "timeout" || msg.Type == "client_disconnected" || msg.Type == "empty_assistant_final" {
		code := msg.Type
		message := msg.Type
		if value, ok := msg.Payload["errorCode"].(string); ok && value != "" {
			code = value
		}
		if value, ok := msg.Payload["errorMessage"].(string); ok && value != "" {
			message = value
		}
		h.writeError(w, http.StatusOK, code, message, requestID, workspaceID, instanceID)
		return
	}
	payload := normalizeAssistantPayload(msg.Payload)
	writeJSON(w, http.StatusOK, protocol.APIResponse{RequestID: requestID, OK: true, WorkspaceID: workspaceID, InstanceID: instanceID, Payload: payload, Error: nil})
}

func (h *Handler) writeRouteError(w http.ResponseWriter, err error, requestID, workspaceID, instanceID string) {
	code := err.Error()
	status := http.StatusServiceUnavailable
	if code == "ambiguous_workspace" {
		status = http.StatusBadRequest
	}
	h.writeError(w, status, code, code, requestID, workspaceID, instanceID)
}

func (h *Handler) writeError(w http.ResponseWriter, status int, code, message, requestID, workspaceID, instanceID string) {
	writeJSON(w, status, protocol.APIResponse{RequestID: requestID, OK: false, WorkspaceID: workspaceID, InstanceID: instanceID, Error: &protocol.APIError{Code: code, Message: message}})
}

func writeSSE(w http.ResponseWriter, event string, value any) {
	data, _ := json.Marshal(value)
	_, _ = w.Write([]byte("event: " + event + "\n"))
	_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
}

func normalizeAssistantPayload(payload map[string]any) map[string]any {
	if payload == nil {
		return map[string]any{"text": "", "finishReason": ""}
	}
	result := map[string]any{}
	if text, ok := payload["text"]; ok {
		result["text"] = text
	} else {
		result["text"] = ""
	}
	if finish, ok := payload["finishReason"]; ok {
		result["finishReason"] = finish
	} else {
		result["finishReason"] = ""
	}
	for k, v := range payload {
		if _, exists := result[k]; !exists {
			result[k] = v
		}
	}
	return result
}

func (h *Handler) validateHistoryPayload(payload map[string]any) error {
	data, _ := json.Marshal(payload)
	if int64(len(data)) > h.HTTP.MaxHistoryResponseSize {
		return fmt.Errorf("history response too large")
	}
	messages, ok := payload["messages"].([]any)
	if !ok {
		return nil
	}
	for _, item := range messages {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		content, _ := m["content"].(string)
		if int64(len([]byte(content))) > h.HTTP.MaxHistoryMessageSize {
			m["content"] = string([]byte(content)[:h.HTTP.MaxHistoryMessageSize])
			m["truncated"] = true
		}
	}
	return nil
}

// handleWorkspaceStream 一个 workspace 一条长 SSE：订阅该 workspace 的所有模型事件。
// GET /api/workspaces/stream?workspaceId=xxx&instanceId=yyy(可选)
func (h *Handler) handleWorkspaceStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userID, err := h.Auth.UserID(r)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized", "", "", "")
		return
	}
	workspaceID := r.URL.Query().Get("workspaceId")
	if workspaceID == "" {
		h.writeError(w, http.StatusBadRequest, "workspace_required", "workspaceId is required", "", "", "")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "stream unsupported", "", workspaceID, "")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	sub := h.Hub.Subscribe(userID, workspaceID)
	defer h.Hub.Unsubscribe(sub)

	// 立即推一条 ready 事件，便于前端确认连接建立
	writeSSE(w, "ready", map[string]any{
		"workspaceId": workspaceID,
		"subId":       sub.ID,
		"serverTime":  protocol.Now(),
	})
	flusher.Flush()
	log.Printf("[sse][workspace] open userId=%d workspaceId=%s subId=%d", userID, workspaceID, sub.ID)

	ping := time.NewTicker(h.HTTP.SSEPingInterval)
	defer ping.Stop()

	ctx := r.Context()
	for {
		select {
		case msg, ok := <-sub.Ch:
			if !ok {
				log.Printf("[sse][workspace] subscription channel closed userId=%d workspaceId=%s subId=%d", userID, workspaceID, sub.ID)
				return
			}
			event := msg.Type
			if dump, mErr := json.Marshal(msg); mErr == nil {
				log.Printf("[sse][workspace->web] userId=%d workspaceId=%s subId=%d event=%s payload=%s", userID, workspaceID, sub.ID, event, string(dump))
			}
			writeSSE(w, event, msg)
			flusher.Flush()
		case <-ctx.Done():
			log.Printf("[sse][workspace] client disconnected userId=%d workspaceId=%s subId=%d", userID, workspaceID, sub.ID)
			return
		case <-ping.C:
			_, _ = w.Write([]byte(": ping\n\n"))
			flusher.Flush()
		}
	}
}

// handleChatTrigger 触发型聊天：仅做"路由 + 投递"，不注册 pending、不依赖 requestId。
//
// 设计要点：
//   - 后端只生成一个 requestId 透传给扩展（便于扩展端日志关联），但不再用它做服务端路由。
//   - 扩展回报任何事件时，只要带上 instanceId，wsserver.Hub.ResolvePending → broadcastToSubscribers
//     会按 instanceId 反查 (userId, workspaceId)，把消息广播给所有订阅该 workspace 的 SSE。
//   - 因此前端在 /api/workspaces/stream 这一条长连接上即可收到所有事件，按 instanceId 关联即可，
//     不再需要在客户端按 requestId 做匹配；前端可直接用 trigger 返回的 instanceId 去监听。
func (h *Handler) handleChatTrigger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userID, err := h.Auth.UserID(r)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized", "", "", "")
		return
	}
	if !h.httpLim.Allow(fmt.Sprintf("%d", userID)) {
		h.writeError(w, http.StatusTooManyRequests, "rate_limited", "rate limited", "", "", "")
		return
	}
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.WorkspaceID == "" || req.Text == "" {
		h.writeError(w, http.StatusBadRequest, "workspace_required", "workspaceId and text are required", "", req.WorkspaceID, req.InstanceID)
		return
	}
	if req.DedupeKey != "" && h.deduper.Seen("dedupe:"+req.DedupeKey) {
		h.writeError(w, http.StatusConflict, "duplicate_request", "duplicate request", "", req.WorkspaceID, req.InstanceID)
		return
	}
	client, err := h.Hub.Route(userID, req.WorkspaceID, req.InstanceID)
	if err != nil {
		h.writeRouteError(w, err, "", req.WorkspaceID, req.InstanceID)
		return
	}
	// requestId 仅用于扩展端的日志关联，服务端不再注册 pending
	requestID := protocol.NewID("req")
	msg := h.newServerMessage(protocol.TypeServerChatMessage, requestID, req.SessionID, req.WorkspaceID, map[string]any{
		"text":                    req.Text,
		"autoSend":                req.AutoSend,
		"bypassPromptEnhancement": true,
		"dedupeKey":               req.DedupeKey,
		"expireAt":                time.Now().Add(2 * time.Minute).UTC().Format("2006-01-02T15:04:05.000Z"),
	})
	if err := client.Enqueue(msg); err != nil {
		h.writeError(w, http.StatusServiceUnavailable, "upstream_write_failed", "failed to write to extension", requestID, req.WorkspaceID, client.InstanceID)
		return
	}
	log.Printf("[chat][trigger] reqId=%s userId=%d workspaceId=%s instanceId=%s text_len=%d (no pending, route_by=instanceId)",
		requestID, userID, req.WorkspaceID, client.InstanceID, len(req.Text))
	writeJSON(w, http.StatusOK, protocol.APIResponse{
		RequestID:   requestID,
		OK:          true,
		WorkspaceID: req.WorkspaceID,
		InstanceID:  client.InstanceID,
		Payload: map[string]any{
			"requestId":  requestID,
			"instanceId": client.InstanceID,
			"accepted":   true,
		},
	})
}

// handleChatCancel 取消正在进行的请求。
type chatCancelRequest struct {
	RequestID   string `json:"requestId"`
	WorkspaceID string `json:"workspaceId"`
	InstanceID  string `json:"instanceId"`
	Reason      string `json:"reason"`
}

func (h *Handler) handleChatCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userID, err := h.Auth.UserID(r)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized", "", "", "")
		return
	}
	var req chatCancelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_cancel_request", "invalid request body", "", req.WorkspaceID, req.InstanceID)
		return
	}
	workspaceID := req.WorkspaceID
	instanceID := req.InstanceID

	// Trigger 模式下服务端不再保存 pending，因此优先按请求体里的 instanceId 路由；
	// 仍兼容旧的 /api/chat、/api/chat/stream 这类带 pending 的请求：如果客户端
	// 只给了 requestId，就回退到 pending 表里取出对应的 workspaceId / instanceId。
	if req.RequestID != "" {
		if p, ok := h.Hub.GetPending(req.RequestID); ok {
			if p.UserID != userID {
				h.writeError(w, http.StatusForbidden, "forbidden", "not your request", req.RequestID, workspaceID, instanceID)
				return
			}
			if workspaceID == "" {
				workspaceID = p.WorkspaceID
			}
			if instanceID == "" {
				instanceID = p.InstanceID
			}
		}
	}

	if instanceID == "" && workspaceID == "" {
		h.writeError(w, http.StatusBadRequest, "instance_required", "instanceId or workspaceId is required", req.RequestID, "", "")
		return
	}

	client, err := h.Hub.Route(userID, workspaceID, instanceID)
	if err != nil {
		h.writeRouteError(w, err, req.RequestID, workspaceID, instanceID)
		return
	}
	// 路由成功后，workspaceID 用 client 实际所属，避免请求体里给的不一致
	if workspaceID == "" {
		workspaceID = client.WorkspaceID
	}
	cancelMsg := h.newServerMessage("server.cancel_request", req.RequestID, "", workspaceID, map[string]any{
		"requestId":  req.RequestID,
		"instanceId": client.InstanceID,
		"reason":     req.Reason,
	})
	if err := client.Enqueue(cancelMsg); err != nil {
		h.writeError(w, http.StatusServiceUnavailable, "upstream_write_failed", "failed to write to extension", req.RequestID, workspaceID, client.InstanceID)
		return
	}
	log.Printf("[chat][cancel] reqId=%s userId=%d workspaceId=%s instanceId=%s reason=%q (route_by=instanceId)",
		req.RequestID, userID, workspaceID, client.InstanceID, req.Reason)
	writeJSON(w, http.StatusOK, protocol.APIResponse{
		RequestID:   req.RequestID,
		OK:          true,
		WorkspaceID: workspaceID,
		InstanceID:  client.InstanceID,
		Payload:     map[string]any{"requestId": req.RequestID, "instanceId": client.InstanceID, "cancelled": true},
	})
}
