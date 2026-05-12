<template>
  <section class="workspace-list">
    <div class="section-header">
      <span class="section-header__title">{{ t('workspace.title') }}</span>
      <el-button size="small" :loading="store.loading" @click="store.refresh()">{{ t('workspace.refresh') }}</el-button>
    </div>
    <ErrorAlert :message="store.error" />

    <!-- 移动端：下拉选择 -->
    <div class="workspace-list__mobile">
      <el-select
        v-if="store.workspaces.length > 0"
        :model-value="selectedKey"
        :placeholder="t('workspace.selectPlaceholder')"
        size="default"
        class="workspace-select"
        @change="onSelectChange"
      >
        <el-option
          v-for="item in store.workspaces"
          :key="`${item.workspaceId}:${item.instanceId}`"
          :label="optionLabel(item)"
          :value="`${item.workspaceId}:${item.instanceId}`"
        >
          <div class="workspace-option">
            <span class="workspace-option__name">{{ optionLabel(item) }}</span>
            <el-tag :type="stateTagType(item.state)" size="small" effect="light">
              {{ stateLabel(item.state) }}
            </el-tag>
          </div>
        </el-option>
      </el-select>
      <EmptyState v-else-if="!store.loading" :text="t('workspace.empty')" />
    </div>

    <!-- 桌面端：卡片列表 -->
    <div class="workspace-list__desktop">
      <EmptyState v-if="!store.loading && store.workspaces.length === 0" :text="t('workspace.empty')" />
      <div class="workspace-list__items">
        <WorkspaceItem
          v-for="item in store.workspaces"
          :key="`${item.workspaceId}:${item.instanceId}`"
          :item="item"
          :active="store.selectedWorkspaceId === item.workspaceId && store.selectedInstanceId === item.instanceId"
          @select="store.select"
        />
      </div>
    </div>
  </section>
</template>
<script setup lang="ts">
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { useWorkspaceStore } from '@/stores/workspace';
import type { WorkspaceInstance, WorkspaceState } from '@/types/workspace';
import WorkspaceItem from './WorkspaceItem.vue';
import EmptyState from '@/components/common/EmptyState.vue';
import ErrorAlert from '@/components/common/ErrorAlert.vue';

const { t } = useI18n();
const store = useWorkspaceStore();

const selectedKey = computed(() => {
  if (!store.selectedWorkspaceId || !store.selectedInstanceId) return undefined;
  return `${store.selectedWorkspaceId}:${store.selectedInstanceId}`;
});

function optionLabel(item: WorkspaceInstance): string {
  const folders = item.metadata?.workspaceFolders ?? [];
  const primary = folders.find((f) => f.isPrimary) ?? folders[0];
  return primary?.name || item.metadata?.workspaceName || item.workspaceId;
}

function stateLabel(state: WorkspaceState): string {
  switch (state) {
    case 'ready':
      return t('workspace.stateReady');
    case 'busy':
      return t('workspace.stateBusy');
    case 'offline':
      return t('workspace.stateOffline');
    default:
      return t('workspace.stateUnknown');
  }
}

function stateTagType(state: WorkspaceState): 'success' | 'warning' | 'danger' | 'info' {
  switch (state) {
    case 'ready':
      return 'success';
    case 'busy':
      return 'warning';
    case 'offline':
      return 'danger';
    default:
      return 'info';
  }
}

function onSelectChange(value: string) {
  const found = store.workspaces.find((item) => `${item.workspaceId}:${item.instanceId}` === value);
  if (found) store.select(found);
}
</script>

<style scoped>
.workspace-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.section-header__title {
  font-size: 14px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.workspace-list__items {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

/* 默认桌面端显示卡片，隐藏下拉 */
.workspace-list__mobile {
  display: none;
}

.workspace-list__desktop {
  display: block;
}

.workspace-select {
  width: 100%;
}

.workspace-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.workspace-option__name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 移动端：切换为下拉选择 */
@media (max-width: 800px) {
  .workspace-list__mobile {
    display: block;
  }

  .workspace-list__desktop {
    display: none;
  }
}
</style>
