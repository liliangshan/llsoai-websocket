export type WorkspaceState = 'ready' | 'busy' | 'offline' | 'unknown';

export interface WorkspaceFolder {
  isPrimary?: boolean;
  name: string;
  path: string;
}

export interface WorkspaceMetadata {
  activeWorkspaceFolder?: string;
  workspaceFolders?: WorkspaceFolder[];
  workspaceName?: string;
  [key: string]: unknown;
}

export interface WorkspaceInstance {
  workspaceId: string;
  instanceId: string;
  sessionId?: string;
  connectedAt?: string;
  lastSeenAt?: string;
  state: WorkspaceState;
  metadata?: WorkspaceMetadata;
}

export interface WorkspacesPayload {
  workspaces: WorkspaceInstance[];
}
