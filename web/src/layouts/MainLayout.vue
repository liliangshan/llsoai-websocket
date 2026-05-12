<template>
  <div v-if="ready" class="main-layout">
    <aside class="sidebar">
      <WorkspaceList />
      <TokenSettings @login="showLoginDialog = true" />
    </aside>
    <main class="content">
      <slot />
    </main>
    <LoginDialog v-model="showLoginDialog" />
  </div>
  <div v-else class="main-layout-loading">{{ t('app.initializing') }}</div>
</template>
<script setup lang="ts">
import { onMounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import WorkspaceList from '@/components/workspace/WorkspaceList.vue';
import TokenSettings from '@/components/token/TokenSettings.vue';
import LoginDialog from '@/components/auth/LoginDialog.vue';
import { useAuthStore } from '@/stores/auth';
import { useWorkspaceStore } from '@/stores/workspace';

const { t } = useI18n();
const auth = useAuthStore();
const workspace = useWorkspaceStore();
const ready = ref(false);
const showLoginDialog = ref(false);

onMounted(async () => {
  ready.value = false;
  await auth.restore();
  await auth.validateToken();
  
  if (auth.isLoggedIn) {
    await workspace.refresh();
  } else {
    workspace.clear();
  }
  ready.value = true;
});

watch(
  () => auth.isLoggedIn,
  async (isLoggedIn) => {
    if (isLoggedIn) {
      showLoginDialog.value = false;
      await workspace.refresh();
    } else {
      workspace.clear();
    }
  },
);
</script>

<style scoped>
.main-layout-loading {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--el-text-color-secondary);
}
</style>
