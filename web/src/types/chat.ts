export type MessageRole = 'user' | 'assistant' | 'system' | 'tool';
export type MessageStatus = 'pending' | 'sending' | 'streaming' | 'done' | 'error' | 'cancelled';
export type MessageSource = 'current' | 'history';

export interface ToolCall {
  id: string;
  name: string;
  /** 流式累积过程中为 string（原始 JSON 文本片段），最终可解析为 Record<string, unknown> */
  arguments: string | Record<string, unknown>;
}

export interface ToolResult {
  toolCallId: string;
  result: unknown;
  success: boolean;
}

export interface ChatMessage {
  id: string;
  workspaceId: string;
  instanceId?: string;
  role: MessageRole;
  content: string;
  reasoning?: string;
  toolCalls?: ToolCall[];
  toolResults?: ToolResult[];
  /** 当前正在流式调用中的工具名，显示「正在调用工具：xxx」 */
  streamingToolName?: string;
  createdAt: string;
  status: MessageStatus;
  source: MessageSource;
  error?: {
    code?: string;
    message: string;
  };
}

export interface ChatRequest {
  workspaceId: string;
  instanceId?: string;
  sessionId?: string;
  text: string;
  autoSend?: boolean;
  dedupeKey?: string;
}

export interface ChatPayload {
  text?: string;
  content?: string;
  reasoning?: string;
  finishReason?: string;
  toolCalls?: ToolCall[];
  toolResults?: ToolResult[];
  [key: string]: unknown;
}

export interface HistoryRequest {
  workspaceId: string;
  instanceId?: string;
  sessionId?: string;
  scope?: 'project';
  order?: 'asc';
  limit?: number;
}

export interface HistoryPayload {
  messages?: Array<Partial<ChatMessage> & { content?: string; text?: string }>;
  [key: string]: unknown;
}
