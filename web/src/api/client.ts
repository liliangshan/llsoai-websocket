import axios from 'axios';
import { useAuthStore } from '@/stores/auth';
import { readToken } from '@/utils/token';
import { resolveApiBaseUrl } from '@/utils/platform';
import type { ApiFailure, ApiResponse, LegacyApiEnvelope } from '@/types/api';

export const apiClient = axios.create({
  baseURL: resolveApiBaseUrl(),
  timeout: 120000,
});

apiClient.interceptors.request.use((config) => {
  const auth = useAuthStore();
  const url = String(config.url ?? '');
  const token = auth.token || readToken();
  if (token && !/^https?:\/\//.test(url)) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    const status = error.response?.status;
    if (status === 401) {
      const auth = useAuthStore();
      auth.logout();
    }
    return Promise.reject(error);
  },
);

export function normalizeResponse<T>(raw: LegacyApiEnvelope<T>): ApiResponse<T> {
  if (raw.ok) {
    return {
      ok: true,
      payload: (raw.payload ?? null) as T,
      error: null,
      requestId: raw.requestId,
      workspaceId: raw.workspaceId,
      instanceId: raw.instanceId,
    };
  }
  return {
    ok: false,
    payload: null,
    requestId: raw.requestId,
    workspaceId: raw.workspaceId,
    instanceId: raw.instanceId,
    error: {
      code: raw.error?.code ?? 'UNKNOWN_ERROR',
      message: raw.error?.message ?? '请求失败',
      detail: raw.error?.detail,
      requestId: raw.error?.requestId ?? raw.requestId,
      retryable: raw.error?.retryable,
    },
  } satisfies ApiFailure;
}
