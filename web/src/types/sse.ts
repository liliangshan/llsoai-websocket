import type { ToolCall, ToolResult } from './chat';

export type ChatStreamEvent =
  | { event: 'message_start'; data: { requestId: string; messageId: string } }
  | { event: 'content_delta'; data: { messageId: string; delta: string } }
  | { event: 'reasoning_delta'; data: { messageId: string; delta: string } }
  | { event: 'tool_call'; data: { messageId: string; toolCall: ToolCall } }
  | { event: 'tool_result'; data: { messageId: string; toolCallId: string; result: unknown; success: boolean } }
  | { event: 'error'; data: { code: string; message: string; requestId?: string } }
  | { event: 'done'; data: { messageId: string } }
  | { event: 'cancelled'; data: { messageId?: string } }
  | { event: string; data: unknown };

export interface ParsedSseEvent<T = unknown> {
  event: string;
  data: T;
}
