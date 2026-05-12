import { defineStore } from 'pinia';
import { cancelChat, loadHistory, sendChat, streamChat, triggerChat } from '@/api/chat';
import type { ChatMessage, ChatPayload, ChatRequest, HistoryPayload, ToolCall, ToolResult } from '@/types/chat';
import type { ParsedSseEvent } from '@/types/sse';
import { toUserMessage } from '@/utils/errors';

interface ChatState {
  messagesByWorkspace: Record<string, ChatMessage[]>;
  loadedHistoryKeys: Record<string, boolean>;
  /** instanceId -> { workspaceId, messageId } 路由表，用于把长 SSE 事件分发回正确消息 */
  pendingByInstance: Record<string, { workspaceId: string; messageId: string }>;
  /** requestId -> pending（兼容旧版 /api/chat/stream 等仍用 requestId 的场景） */
  pendingByRequest: Record<string, { workspaceId: string; instanceId?: string; messageId: string }>;
  activeInstanceId: string | null;
  activeRequestId: string | null;
  activeMessageId: string | null;
  abortController: AbortController | null;
  /** 记录上一个 tool 名，用于 model.tool_call_delta 时判断是否需要插入新工具条目 */
  lastToolName: string | null;
  historyLoading: boolean;
  error: string | null;
}

function chatKey(workspaceId: string, instanceId?: string): string {
  return `${workspaceId}:${instanceId ?? 'default'}`;
}

function newId(prefix: string): string {
  return `${prefix}_${Date.now()}_${Math.random().toString(16).slice(2)}`;
}

export const useChatStore = defineStore('chat', {
  state: (): ChatState => ({
    messagesByWorkspace: {},
    loadedHistoryKeys: {},
    pendingByInstance: {},
    pendingByRequest: {},
    activeInstanceId: null,
    activeRequestId: null,
    activeMessageId: null,
    abortController: null,
    lastToolName: null,
    historyLoading: false,
    error: null,
  }),
  getters: {
    isStreaming: (state) => Boolean(state.activeInstanceId || state.activeRequestId),
  },
  actions: {
    getMessages(workspaceId: string, instanceId?: string): ChatMessage[] {
      return this.messagesByWorkspace[chatKey(workspaceId, instanceId)] ?? [];
    },
    ensureMessages(workspaceId: string, instanceId?: string): ChatMessage[] {
      const key = chatKey(workspaceId, instanceId);
      if (!this.messagesByWorkspace[key]) this.messagesByWorkspace[key] = [];
      return this.messagesByWorkspace[key];
    },
    clear(workspaceId: string, instanceId?: string) {
      const key = chatKey(workspaceId, instanceId);
      this.messagesByWorkspace[key] = [];
      delete this.loadedHistoryKeys[key];
      this.lastToolName = null;
    },
    async sendNormal(request: ChatRequest) {
      this.error = null;
      const messages = this.ensureMessages(request.workspaceId, request.instanceId);
      messages.push({
        id: newId('user'),
        workspaceId: request.workspaceId,
        instanceId: request.instanceId,
        role: 'user',
        content: request.text,
        createdAt: new Date().toISOString(),
        status: 'done',
        source: 'current',
      });
      const assistantId = newId('assistant');
      messages.push({
        id: assistantId,
        workspaceId: request.workspaceId,
        instanceId: request.instanceId,
        role: 'assistant',
        content: '',
        createdAt: new Date().toISOString(),
        status: 'sending',
        source: 'current',
      });
      try {
        const response = await sendChat(request);
        const target = messages.find((item) => item.id === assistantId);
        if (!target) return;
        if (response.ok) {
          applyPayload(target, response.payload);
          target.status = 'done';
        } else {
          target.status = 'error';
          target.error = { code: response.error.code, message: response.error.message };
          this.error = toUserMessage(response.error);
        }
      } catch (error) {
        const target = messages.find((item) => item.id === assistantId);
        if (target) {
          target.status = 'error';
          target.error = { message: toUserMessage(error) };
        }
        this.error = toUserMessage(error);
      }
    },
    /**
     * 触发型流式：调 /api/chat/trigger 拿 requestId，立刻 push 占位 assistant 消息；
     * 后续 model.* 事件由工作区长 SSE 推回，通过 handleWorkspaceEvent 分发。
     */
    async sendStream(request: ChatRequest) {
      this.error = null;
      const messages = this.ensureMessages(request.workspaceId, request.instanceId);
      messages.push({
        id: newId('user'),
        workspaceId: request.workspaceId,
        instanceId: request.instanceId,
        role: 'user',
        content: request.text,
        createdAt: new Date().toISOString(),
        status: 'done',
        source: 'current',
      });
      const assistantId = newId('assistant');
      messages.push({
        id: assistantId,
        workspaceId: request.workspaceId,
        instanceId: request.instanceId,
        role: 'assistant',
        content: '',
        createdAt: new Date().toISOString(),
        status: 'sending',
        source: 'current',
        toolCalls: [],
        toolResults: [],
      });
      try {
        const response = await triggerChat(request);
        const target = messages.find((item) => item.id === assistantId);
        if (!response.ok) {
          if (target) {
            target.status = 'error';
            target.error = { code: response.error.code, message: response.error.message };
          }
          this.error = toUserMessage(response.error);
          return;
        }
        const instanceId = response.payload.instanceId;
        if (target) {
          target.status = 'streaming';
        }
        // 核心：按 instanceId 分发，SSE 事件中带 instanceId 即命中
        this.pendingByInstance[instanceId] = {
          workspaceId: request.workspaceId,
          messageId: assistantId,
        };
        this.activeInstanceId = instanceId;
        this.activeMessageId = assistantId;
        this.lastToolName = null;
      } catch (error) {
        const target = messages.find((item) => item.id === assistantId);
        if (target) {
          target.status = 'error';
          const err = error as { code?: string; message?: string } | undefined;
          target.error = { code: err?.code, message: toUserMessage(error) };
        }
        this.error = toUserMessage(error);
      }
    },
    /**
     * 工作区长 SSE 收到事件时调用：优先按 event.instanceId 路由（基于 instanceId 分发）；
     * 兼容 fallback：若无 instanceId 则按 requestId 路由（保留旧 /api/chat/stream 兼容路径）。
     */
    handleWorkspaceEvent(event: ParsedSseEvent, workspaceId: string) {
      if (event.event === 'ready') return;
      const data = (event.data && typeof event.data === 'object' ? (event.data as Record<string, unknown>) : {}) as Record<string, unknown>;

      // 优先：按 instanceId 路由（trigger 触发型路径）
      const instanceId = typeof data.instanceId === 'string' ? data.instanceId : '';
      if (instanceId) {
        const pending = this.pendingByInstance[instanceId];
        if (pending && pending.workspaceId === workspaceId) {
          const messages = this.ensureMessages(pending.workspaceId, instanceId);
          const target = messages.find((item) => item.id === pending.messageId);
          if (target) {
            this.applyEventToMessage(target, event);
            if (
              event.event === 'model.request_completed' ||
              event.event === 'model.request_cancelled' ||
              event.event === 'model.request_error' ||
              event.event === 'done' ||
              event.event === 'cancelled' ||
              event.event === 'error'
            ) {
              delete this.pendingByInstance[instanceId];
              if (this.activeInstanceId === instanceId) {
                this.activeInstanceId = null;
                this.activeMessageId = null;
              }
            }
          }
          return;
        }
      }

      // 兼容 fallback：按 requestId 路由（保留旧 /api/chat/stream 路径）
      const requestId = typeof data.requestId === 'string' ? data.requestId : '';
      if (!requestId) return;
      const pending = this.pendingByRequest[requestId];
      if (!pending) return;
      if (pending.workspaceId !== workspaceId) return;
      const messages = this.ensureMessages(pending.workspaceId, pending.instanceId);
      const target = messages.find((item) => item.id === pending.messageId);
      if (!target) return;
      this.applyEventToMessage(target, event);
      if (
        event.event === 'model.request_completed' ||
        event.event === 'model.request_cancelled' ||
        event.event === 'model.request_error' ||
        event.event === 'done' ||
        event.event === 'cancelled' ||
        event.event === 'error'
      ) {
        delete this.pendingByRequest[requestId];
        if (this.activeRequestId === requestId) {
          this.activeRequestId = null;
          this.activeMessageId = null;
        }
      }
    },
    handleStreamEvent(workspaceId: string, instanceId: string | undefined, fallbackMessageId: string, event: ParsedSseEvent) {
      const messages = this.ensureMessages(workspaceId, instanceId);
      const target = messages.find((item) => item.id === fallbackMessageId);
      if (!target) return;
      this.applyEventToMessage(target, event);
    },
    applyEventToMessage(target: ChatMessage, event: ParsedSseEvent) {
      const data = (event.data && typeof event.data === 'object' ? (event.data as Record<string, unknown>) : {}) as Record<string, unknown>;
      const payload = (data?.payload && typeof data.payload === 'object' ? (data.payload as Record<string, unknown>) : {}) as Record<string, unknown>;
      switch (event.event) {
        case 'model.request_started':
        case 'message_start': {
          if (typeof data.requestId === 'string') this.activeRequestId = data.requestId;
          target.status = 'streaming';
          break;
        }
        case 'model.text_delta':
        case 'content_delta': {
          const delta =
            typeof payload.delta === 'string'
              ? payload.delta
              : typeof data.delta === 'string'
                ? (data.delta as string)
                : typeof (data.payload as Record<string, unknown>)?.delta === 'string'
                  ? ((data.payload as Record<string, unknown>).delta as string)
                  : '';
          if (delta) target.content += delta;
          target.status = 'streaming';
          break;
        }
        case 'model.reasoning_delta':
        case 'reasoning_delta': {
          const delta =
            typeof payload.delta === 'string'
              ? payload.delta
              : typeof data.delta === 'string'
                ? (data.delta as string)
                : typeof (data.payload as Record<string, unknown>)?.delta === 'string'
                  ? ((data.payload as Record<string, unknown>).delta as string)
                  : '';
          target.reasoning = `${target.reasoning ?? ''}${delta}`;
          break;
        }
        case 'model.tool_call_started':
        case 'tool_call': {
          const toolCall = (payload.toolCall ?? data.toolCall) as ToolCall | undefined;
          if (toolCall) {
            this.lastToolName = typeof toolCall.name === 'string' ? toolCall.name : null;
            target.toolCalls = [...(target.toolCalls ?? []), toolCall];
          }
          break;
        }
        case 'model.tool_call_delta': {
          const toolCallId = typeof payload.toolCallId === 'string' ? payload.toolCallId : '';
          const toolName = typeof payload.toolName === 'string' ? payload.toolName : '';
          const argumentsDelta = typeof payload.argumentsDelta === 'string' ? payload.argumentsDelta : '';
          if (!toolName) break;
          const existing = target.toolCalls?.find((tc) => tc.id === toolCallId);
          if (existing && this.lastToolName === toolName) {
            // 同一工具的 delta，追加参数
            if (argumentsDelta) existing.arguments = `${existing.arguments ?? ''}${argumentsDelta}`;
          } else {
            // 工具名变了，插入新的 toolCall 条目，并标注"正在调用：xxx"
            this.lastToolName = toolName;
            const newToolCall: ToolCall = {
              id: toolCallId || newId('tool'),
              name: toolName,
              arguments: argumentsDelta,
            };
            target.toolCalls = [...(target.toolCalls ?? []), newToolCall];
          }
          break;
        }
        case 'model.tool_call_completed': {
          // 仅记录工具名，不额外操作；delta 已在 model.tool_call_delta 中处理
          break;
        }
        case 'model.tool_result':
        case 'tool_result': {
          target.toolResults = [
            ...(target.toolResults ?? []),
            {
              toolCallId: String(payload.toolCallId ?? data.toolCallId ?? ''),
              result: payload.result ?? data.result,
              success: Boolean(payload.success ?? data.success),
            } as ToolResult,
          ];
          break;
        }
        case 'model.assistant_final':
        case 'model.assistant_delta': {
          applyPayload(target, payload as ChatPayload);
          if (event.event === 'model.assistant_final') target.status = 'done';
          break;
        }
        case 'model.request_error':
        case 'error': {
          target.status = 'error';
          const code = String(payload.errorCode ?? data.code ?? data.errorCode ?? '');
          const message = String(payload.errorMessage ?? data.message ?? data.errorMessage ?? '流式响应错误');
          target.error = { code, message };
          this.lastToolName = null;
          break;
        }
        case 'done':
        case 'model.request_completed': {
          if (target.status !== 'error') target.status = 'done';
          this.lastToolName = null;
          break;
        }
        case 'cancelled':
        case 'model.request_cancelled': {
          if (target.status !== 'error') target.status = 'cancelled';
          this.lastToolName = null;
          break;
        }
        default:
          break;
      }
    },
    async cancelActive() {
      const instanceId = this.activeInstanceId;
      const requestId = this.activeRequestId;
      const messageId = this.activeMessageId;
      if (this.abortController) {
        this.abortController.abort();
        this.abortController = null;
      }
      if (messageId) {
        const all = Object.values(this.messagesByWorkspace).flat();
        const target = all.find((item) => item.id === messageId);
        if (target && target.status !== 'done' && target.status !== 'error') {
          target.status = 'cancelled';
        }
      }
      // 优先按 instanceId 取消（trigger 触发型）
      if (instanceId) {
        delete this.pendingByInstance[instanceId];
        this.activeInstanceId = null;
        this.activeMessageId = null;
        try {
          await cancelChat({
            instanceId,
            reason: 'user_cancelled',
          });
        } catch (error) {
          this.error = toUserMessage(error);
        }
        return;
      }
      // 兼容 fallback：按 requestId 取消（旧路径）
      if (requestId) {
        const pending = this.pendingByRequest[requestId];
        delete this.pendingByRequest[requestId];
        this.activeRequestId = null;
        this.activeMessageId = null;
        try {
          await cancelChat({
            requestId,
            workspaceId: pending?.workspaceId,
            instanceId: pending?.instanceId,
            reason: 'user_cancelled',
          });
        } catch (error) {
          this.error = toUserMessage(error);
        }
      } else {
        this.activeRequestId = null;
        this.activeMessageId = null;
      }
    },
    /**
     * 兼容旧的 /api/chat/stream 路径（保留但建议改用 sendStream 触发型 + workspace 长 SSE）。
     */
    async sendStreamLegacy(request: ChatRequest) {
      this.cancelActive();
      const controller = new AbortController();
      this.abortController = controller;
      this.error = null;
      const messages = this.ensureMessages(request.workspaceId, request.instanceId);
      messages.push({
        id: newId('user'),
        workspaceId: request.workspaceId,
        instanceId: request.instanceId,
        role: 'user',
        content: request.text,
        createdAt: new Date().toISOString(),
        status: 'done',
        source: 'current',
      });
      const assistantId = newId('assistant');
      this.activeMessageId = assistantId;
      messages.push({
        id: assistantId,
        workspaceId: request.workspaceId,
        instanceId: request.instanceId,
        role: 'assistant',
        content: '',
        createdAt: new Date().toISOString(),
        status: 'streaming',
        source: 'current',
        toolCalls: [],
        toolResults: [],
      });
      try {
        await streamChat(request, {
          signal: controller.signal,
          onEvent: (event) => this.handleStreamEvent(request.workspaceId, request.instanceId, assistantId, event),
          onError: (error) => {
            this.error = toUserMessage(error);
          },
        });
      } catch (error) {
        const target = this.ensureMessages(request.workspaceId, request.instanceId).find((item) => item.id === assistantId);
        if (target && target.status !== 'cancelled') {
          target.status = 'error';
          const err = error as { code?: string; message?: string } | undefined;
          target.error = { code: err?.code, message: toUserMessage(error) };
        }
        this.error = toUserMessage(error);
      } finally {
        this.activeRequestId = null;
        this.activeMessageId = null;
        this.abortController = null;
      }
    },
    async loadHistory(request: ChatRequest) {
      this.historyLoading = true;
      this.error = null;
      const key = chatKey(request.workspaceId, request.instanceId);
      try {
        const response = await loadHistory(request);
        if (response.ok) {
          const messages = mapHistory(response.payload, request.workspaceId, request.instanceId);
          this.messagesByWorkspace[key] = messages;
          this.loadedHistoryKeys[key] = true;
        } else {
          this.error = toUserMessage(response.error);
        }
      } catch (error) {
        this.error = toUserMessage(error);
      } finally {
        this.historyLoading = false;
      }
    },
    async ensureHistoryLoaded(request: ChatRequest) {
      const key = chatKey(request.workspaceId, request.instanceId);
      if (this.loadedHistoryKeys[key]) return;
      if (this.historyLoading) return;
      if (this.isStreaming) return;
      const existing = this.messagesByWorkspace[key];
      if (existing && existing.length > 0) {
        this.loadedHistoryKeys[key] = true;
        return;
      }
      await this.loadHistory(request);
    },
  },
});

function applyPayload(message: ChatMessage, payload: ChatPayload) {
  message.content = String(payload.text ?? payload.content ?? message.content ?? '');
  if (typeof payload.reasoning === 'string') message.reasoning = payload.reasoning;
  if (Array.isArray(payload.toolCalls)) message.toolCalls = payload.toolCalls;
  if (Array.isArray(payload.toolResults)) message.toolResults = payload.toolResults;
}

function mapHistory(payload: HistoryPayload, workspaceId: string, instanceId?: string): ChatMessage[] {
  return (payload.messages ?? []).map((item) => ({
    id: item.id ?? newId('history'),
    workspaceId,
    instanceId,
    role: item.role ?? 'assistant',
    content: String(item.content ?? item.text ?? ''),
    reasoning: item.reasoning,
    toolCalls: item.toolCalls,
    toolResults: item.toolResults,
    createdAt: item.createdAt ?? new Date().toISOString(),
    status: 'done',
    source: 'history',
  }));
}
