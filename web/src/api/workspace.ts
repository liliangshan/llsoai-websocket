import { apiClient } from './client';
import type { ApiResponse } from '@/types/api';
import type { WorkspacesPayload } from '@/types/workspace';

export async function getWorkspaces(): Promise<ApiResponse<WorkspacesPayload>> {
  try {
    const { data } = await apiClient.get<WorkspacesPayload>('/workspaces');
    return {
      ok: true,
      payload: { workspaces: data?.workspaces ?? [] },
      error: null,
    };
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : '获取工作区失败';
    return {
      ok: false,
      payload: null,
      error: {
        code: 'FETCH_WORKSPACES_FAILED',
        message,
        detail: undefined,
        requestId: '',
        retryable: true,
      },
    };
  }
}
