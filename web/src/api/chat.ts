import { apiClient, normalizeResponse } from './client';
import type { ApiResponse, LegacyApiEnvelope } from '@/types/api';
import type { ChatPayload, ChatRequest, HistoryPayload, HistoryRequest } from '@/types/chat';
import type { ParsedSseEvent } from '@/types/sse';
import { parseSseChunk } from '@/utils/sse';
import { useAuthStore } from '@/stores/auth';

export async function sendChat(request: ChatRequest): Promise<ApiResponse<ChatPayload>> {
  const { data } = await apiClient.post<LegacyApiEnvelope<ChatPayload>>('/chat', request);
  return normalizeResponse(data);
}

export interface TriggerChatPayload {
  requestId: string;
  instanceId: string;
  accepted: boolean;
}

export async function triggerChat(request: ChatRequest): Promise<ApiResponse<TriggerChatPayload>> {
  const { data } = await apiClient.post<LegacyApiEnvelope<TriggerChatPayload>>('/chat/trigger', request);
  return normalizeResponse(data);
}

export interface CancelChatRequest {
  requestId?: string;
  workspaceId?: string;
  instanceId?: string;
  reason?: string;
}

export async function cancelChat(request: CancelChatRequest): Promise<ApiResponse<{ requestId: string; cancelled: boolean }>> {
  const { data } = await apiClient.post<LegacyApiEnvelope<{ requestId: string; cancelled: boolean }>>(
    '/chat/cancel',
    request,
  );
  return normalizeResponse(data);
}

export async function loadHistory(request: HistoryRequest): Promise<ApiResponse<HistoryPayload>> {
  const { data } = await apiClient.post<LegacyApiEnvelope<HistoryPayload>>('/chat/history', {
    scope: 'project',
    order: 'asc',
    limit: 100,
    ...request,
  });
  return normalizeResponse(data);
}

export interface WorkspaceStreamHandlers {
  onEvent: (event: ParsedSseEvent) => void;
  onOpen?: () => void;
  onError?: (error: Error) => void;
  signal?: AbortSignal;
}

/**
 * 打开一个 workspace 级别的长 SSE 连接。
 * - 自动带 Authorization header（使用 fetch + ReadableStream，而非 EventSource）。
 * - 通过 signal.abort() 主动关闭。
 * - 连接到达后调用 onOpen；每条 SSE 事件触发 onEvent。
 */
export async function openWorkspaceStream(
  workspaceId: string,
  handlers: WorkspaceStreamHandlers,
  instanceId?: string,
): Promise<void> {
  const auth = useAuthStore();
  const base = import.meta.env.VITE_API_BASE_URL || '/api';
  const params = new URLSearchParams();
  params.set('workspaceId', workspaceId);
  if (instanceId) params.set('instanceId', instanceId);
  const url = `${base}/workspaces/stream?${params.toString()}`;
  const response = await fetch(url, {
    method: 'GET',
    headers: {
      Accept: 'text/event-stream',
      Authorization: `Bearer ${auth.token}`,
    },
    signal: handlers.signal,
  });
  if (!response.ok || !response.body) {
    throw await buildStreamHttpError(response);
  }
  handlers.onOpen?.();
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const parsed = parseSseChunk(buffer);
      buffer = parsed.rest;
      parsed.events.forEach(handlers.onEvent);
    }
  } catch (error) {
    if ((error as Error).name !== 'AbortError') {
      handlers.onError?.(error as Error);
      throw error;
    }
  }
}

export async function streamChat(
  request: ChatRequest,
  handlers: {
    onEvent: (event: ParsedSseEvent) => void;
    onError?: (error: Error) => void;
    signal?: AbortSignal;
  },
): Promise<void> {
  const auth = useAuthStore();
  const response = await fetch(`${import.meta.env.VITE_API_BASE_URL || '/api'}/chat/stream`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${auth.token}`,
    },
    body: JSON.stringify(request),
    signal: handlers.signal,
  });

  if (!response.ok || !response.body) {
    throw await buildStreamHttpError(response);
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const parsed = parseSseChunk(buffer);
      buffer = parsed.rest;
      parsed.events.forEach(handlers.onEvent);
    }
  } catch (error) {
    if ((error as Error).name !== 'AbortError') {
      handlers.onError?.(error as Error);
      throw error;
    }
  }
}

interface StreamHttpError extends Error {
  code?: string;
  status?: number;
  requestId?: string;
  workspaceId?: string;
  instanceId?: string;
}

async function buildStreamHttpError(response: Response): Promise<StreamHttpError> {
  let bodyText = '';
  try {
    bodyText = await response.text();
  } catch {
    bodyText = '';
  }
  let code = '';
  let message = '';
  let requestId = '';
  let workspaceId = '';
  let instanceId = '';
  if (bodyText) {
    try {
      const parsed = JSON.parse(bodyText) as {
        error?: { code?: string; message?: string };
        requestId?: string;
        workspaceId?: string;
        instanceId?: string;
      };
      code = parsed.error?.code ?? '';
      message = parsed.error?.message ?? '';
      requestId = parsed.requestId ?? '';
      workspaceId = parsed.workspaceId ?? '';
      instanceId = parsed.instanceId ?? '';
    } catch {
      message = bodyText;
    }
  }
  if (!message) {
    message = `流式请求失败：${response.status}`;
  }
  const err = new Error(message) as StreamHttpError;
  if (code) err.code = code;
  err.status = response.status;
  if (requestId) err.requestId = requestId;
  if (workspaceId) err.workspaceId = workspaceId;
  if (instanceId) err.instanceId = instanceId;
  return err;
}
