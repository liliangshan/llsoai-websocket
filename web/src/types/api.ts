export interface ApiError {
  code: string;
  message: string;
  detail?: string;
  requestId?: string;
  retryable?: boolean;
}

export interface LegacyApiEnvelope<T = unknown> {
  requestId?: string;
  ok: boolean;
  workspaceId?: string;
  instanceId?: string;
  payload?: T;
  error?: ApiError | null;
}

export interface ApiSuccess<T> {
  ok: true;
  payload: T;
  error: null;
  requestId?: string;
  workspaceId?: string;
  instanceId?: string;
}

export interface ApiFailure {
  ok: false;
  payload: null;
  error: ApiError;
  requestId?: string;
  workspaceId?: string;
  instanceId?: string;
}

export type ApiResponse<T> = ApiSuccess<T> | ApiFailure;
