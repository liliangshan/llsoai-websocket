import { defineStore } from 'pinia';
import { getWorkspaces } from '@/api/workspace';
import type { WorkspaceInstance } from '@/types/workspace';
import { toUserMessage } from '@/utils/errors';

interface WorkspaceStateValue {
  workspaces: WorkspaceInstance[];
  selectedWorkspaceId: string | null;
  selectedInstanceId: string | null;
  loading: boolean;
  polling: boolean;
  lastRefreshAt: string | null;
  error: string | null;
}

export const useWorkspaceStore = defineStore('workspace', {
  state: (): WorkspaceStateValue => ({
    workspaces: [],
    selectedWorkspaceId: null,
    selectedInstanceId: null,
    loading: false,
    polling: false,
    lastRefreshAt: null,
    error: null,
  }),
  getters: {
    selectedWorkspace: (state) =>
      state.workspaces.find(
        (item) => item.workspaceId === state.selectedWorkspaceId && item.instanceId === state.selectedInstanceId,
      ) ?? null,
    selectedKey: (state) =>
      state.selectedWorkspaceId ? `${state.selectedWorkspaceId}:${state.selectedInstanceId ?? 'default'}` : null,
  },
  actions: {
    select(item: WorkspaceInstance) {
      this.selectedWorkspaceId = item.workspaceId;
      this.selectedInstanceId = item.instanceId;
    },
    clear() {
      this.workspaces = [];
      this.selectedWorkspaceId = null;
      this.selectedInstanceId = null;
      this.lastRefreshAt = null;
      this.error = null;
    },
    async refresh() {
      this.loading = true;
      this.error = null;
      try {
        const response = await getWorkspaces();
        if (response.ok) {
          this.workspaces = response.payload.workspaces ?? [];
          this.lastRefreshAt = new Date().toISOString();
          return;
        }
        this.error = toUserMessage(response.error);
      } catch (error) {
        this.error = toUserMessage(error);
      } finally {
        this.loading = false;
      }
    },
  },
});
