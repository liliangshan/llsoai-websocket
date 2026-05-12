<template>
  <div class="workspace-item" :class="{ active }" @click="$emit('select', item)">
    <div class="workspace-item__header">
      <div class="workspace-item__title" :title="displayName">{{ displayName }}</div>
      <el-tag size="small" :type="stateTagType">{{ stateLabel }}</el-tag>
    </div>
    <div v-if="displayPath" class="workspace-item__path" :title="displayPath">{{ displayPath }}</div>
    <div class="workspace-item__meta">
      <span class="workspace-item__meta-item" :title="item.workspaceId">ID：{{ shortWorkspaceId }}</span>
      <span v-if="item.lastSeenAt" class="workspace-item__meta-item" :title="item.lastSeenAt">
        活跃：{{ relativeLastSeen }}
      </span>
    </div>
    <div v-if="extraFolders.length" class="workspace-item__folders">
      <el-tag
        v-for="folder in extraFolders"
        :key="folder.path"
        size="small"
        type="info"
        effect="plain"
      >
        {{ folder.name }}
      </el-tag>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import type { WorkspaceInstance, WorkspaceFolder } from '@/types/workspace';

const props = defineProps<{ item: WorkspaceInstance; active: boolean }>();
defineEmits<{ select: [item: WorkspaceInstance] }>();

const primaryFolder = computed<WorkspaceFolder | null>(() => {
  const folders = props.item.metadata?.workspaceFolders ?? [];
  return folders.find((f) => f.isPrimary) ?? folders[0] ?? null;
});

const displayName = computed(() => {
  return (
    primaryFolder.value?.name ||
    props.item.metadata?.workspaceName ||
    props.item.workspaceId
  );
});

const displayPath = computed(() => {
  return (
    props.item.metadata?.activeWorkspaceFolder ||
    primaryFolder.value?.path ||
    ''
  );
});

const extraFolders = computed<WorkspaceFolder[]>(() => {
  const folders = props.item.metadata?.workspaceFolders ?? [];
  if (folders.length <= 1) return [];
  const primary = primaryFolder.value;
  return folders.filter((f) => f !== primary);
});

const shortWorkspaceId = computed(() => {
  const id = props.item.workspaceId ?? '';
  return id.length > 10 ? `${id.slice(0, 8)}…` : id;
});

const stateLabel = computed(() => {
  switch (props.item.state) {
    case 'ready':
      return '在线';
    case 'busy':
      return '繁忙';
    case 'offline':
      return '离线';
    default:
      return props.item.state || '未知';
  }
});

const stateTagType = computed<'success' | 'warning' | 'info' | 'danger'>(() => {
  switch (props.item.state) {
    case 'ready':
      return 'success';
    case 'busy':
      return 'warning';
    case 'offline':
      return 'danger';
    default:
      return 'info';
  }
});

const relativeLastSeen = computed(() => {
  const ts = props.item.lastSeenAt;
  if (!ts) return '';
  const t = new Date(ts).getTime();
  if (Number.isNaN(t)) return ts;
  const diff = Date.now() - t;
  if (diff < 0) return '刚刚';
  const sec = Math.floor(diff / 1000);
  if (sec < 60) return `${sec} 秒前`;
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min} 分钟前`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr} 小时前`;
  const day = Math.floor(hr / 24);
  return `${day} 天前`;
});
</script>

<style scoped>
.workspace-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px 12px;
  border: 1px solid var(--el-border-color-lighter, #ebeef5);
  border-radius: 10px;
  cursor: pointer;
  background: var(--el-bg-color, #fff);
  transition: border-color 0.15s ease, background 0.15s ease, box-shadow 0.15s ease;
}

.workspace-item:hover {
  border-color: var(--el-color-primary-light-5, #a0cfff);
  background: var(--el-color-primary-light-9, #ecf5ff);
}

.workspace-item.active {
  border-color: var(--el-color-primary, #2563eb);
  background: var(--el-color-primary-light-9, #ecf5ff);
  box-shadow: 0 0 0 2px rgba(37, 99, 235, 0.12);
}

.workspace-item__header {
  display: flex;
  align-items: center;
  gap: 8px;
  justify-content: space-between;
}

.workspace-item__title {
  flex: 1 1 auto;
  min-width: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.workspace-item__path {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  direction: rtl;
  text-align: left;
}

.workspace-item__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.workspace-item__folders {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 2px;
}
</style>
