package protocol

import "time"

const (
	ProtocolVersion = "1.0"
	SourceServer    = "remote-server"
	SourceExtension = "vscode-extension"

	TypeClientHello              = "client.hello"
	TypeClientConnectionContext  = "client.connection_context"
	TypeClientHeartbeatPing      = "client.heartbeat_ping"
	TypeClientHeartbeatPong      = "client.heartbeat_pong"
	TypeClientChatHistoryReply   = "client.chat_history_response"
	TypeClientChatHistoryError   = "client.chat_history_error"
	TypeServerHelloAck           = "server.hello_ack"
	TypeServerHeartbeatPong      = "server.heartbeat_pong"
	TypeServerAck                = "server.ack"
	TypeServerError              = "server.error"
	TypeServerChatMessage        = "server.chat_message"
	TypeServerChatHistoryRequest = "server.chat_history_request"
	TypeModelRequestStarted      = "model.request_started"
	TypeModelTextDelta           = "model.text_delta"
	TypeModelReasoningDelta      = "model.reasoning_delta"
	TypeModelToolCallStarted     = "model.tool_call_started"
	TypeModelToolCallDelta       = "model.tool_call_delta"
	TypeModelToolCallCompleted   = "model.tool_call_completed"
	TypeModelToolResult          = "model.tool_result"
	TypeModelAssistantFinal      = "model.assistant_final"
	TypeModelRequestCompleted    = "model.request_completed"
	TypeModelRequestCancelled    = "model.request_cancelled"
	TypeModelRequestError        = "model.request_error"
)

type Envelope struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Type            string         `json:"type"`
	MessageID       string         `json:"messageId"`
	EventID         string         `json:"eventId"`
	EventSeq        int64          `json:"eventSeq"`
	SessionID       string         `json:"sessionId"`
	RequestID       string         `json:"requestId"`
	WorkspaceID     string         `json:"workspaceId"`
	InstanceID      string         `json:"instanceId"`
	Timestamp       string         `json:"timestamp"`
	Source          string         `json:"source"`
	Payload         map[string]any `json:"payload,omitempty"`
}

type APIError struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Retryable *bool          `json:"retryable,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

type APIResponse struct {
	RequestID   string         `json:"requestId"`
	OK          bool           `json:"ok"`
	WorkspaceID string         `json:"workspaceId,omitempty"`
	InstanceID  string         `json:"instanceId,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
	Error       *APIError      `json:"error"`
}

func Now() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}

func BoolPtr(value bool) *bool {
	return &value
}
