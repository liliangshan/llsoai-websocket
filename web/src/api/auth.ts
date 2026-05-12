import { apiClient } from './client';
import type { ApiResponse } from '@/types/api';
import type { CurrentUserPayload, LoginRequest, RegisterRequest, AuthResponse } from '@/types/auth';

export async function login(credentials: LoginRequest): Promise<ApiResponse<AuthResponse>> {
  const { data } = await apiClient.post<AuthResponse>('/auth/login', credentials);
  if (data && data.token) {
    return { ok: true, payload: data, error: null };
  }
  return { ok: false, payload: null, error: { code: 'AUTH_FAILED', message: '登录失败', detail: undefined, requestId: '', retryable: false } };
}

export async function register(info: RegisterRequest): Promise<ApiResponse<AuthResponse>> {
  const { data } = await apiClient.post<AuthResponse>('/auth/register', info);
  if (data && data.token) {
    return { ok: true, payload: data, error: null };
  }
  return { ok: false, payload: null, error: { code: 'REGISTER_FAILED', message: '注册失败', detail: undefined, requestId: '', retryable: false } };
}

export async function getCurrentUser(): Promise<ApiResponse<CurrentUserPayload>> {
  const { data } = await apiClient.get<CurrentUserPayload>('/me');
  console.log('getCurrentUser response:', data);
  if (data && data.userId !== undefined) {
    return { ok: true, payload: data, error: null };
  }
  return { ok: false, payload: null, error: { code: 'FETCH_USER_FAILED', message: '获取用户信息失败', detail: undefined, requestId: '', retryable: false } };
}

export interface WebSocketTokenPayload {
  token: string;
}

export async function getWebSocketToken(): Promise<ApiResponse<WebSocketTokenPayload>> {
  const { data } = await apiClient.get<WebSocketTokenPayload>('/me/websocket-token');
  if (data && typeof data.token === 'string') {
    return { ok: true, payload: data, error: null };
  }
  return { ok: false, payload: null, error: { code: 'FETCH_WS_TOKEN_FAILED', message: '获取 WebSocket 令牌失败', detail: undefined, requestId: '', retryable: false } };
}

export async function rotateWebSocketToken(): Promise<ApiResponse<WebSocketTokenPayload>> {
  const { data } = await apiClient.post<WebSocketTokenPayload>('/me/websocket-token/rotate');
  if (data && typeof data.token === 'string') {
    return { ok: true, payload: data, error: null };
  }
  return { ok: false, payload: null, error: { code: 'ROTATE_WS_TOKEN_FAILED', message: '重置 WebSocket 令牌失败', detail: undefined, requestId: '', retryable: false } };
}
