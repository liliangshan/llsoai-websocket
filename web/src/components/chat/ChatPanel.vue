<template>
  <section class="chat-panel">
    <div class="chat-header">
      <div v-if="!workspace">{{ t('workspace.needSelect') }}</div>
      <div class="chat-version">1.0.3</div>
    </div>
    <ErrorAlert :message="chat.error" />
    <ChatMessageList :messages="messages" />
    <ChatInput
      :disabled="!workspace || workspace.state !== 'ready'"
      :streaming="chat.isStreaming"
      @send="handleSend"
      @history="handleHistory"
      @clear="handleClear"
      @cancel="chat.cancelActive()"
    />
  </section>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useChatStore } from '@/stores/chat';
import { useWorkspaceStore } from '@/stores/workspace';
import { useSettingsStore } from '@/stores/settings';
import { openWorkspaceStream } from '@/api/chat';
import { toUserMessage } from '@/utils/errors';
import ChatMessageList from './ChatMessageList.vue';
import ChatInput from './ChatInput.vue';
import ErrorAlert from '@/components/common/ErrorAlert.vue';
const { t } = useI18n();
const chat = useChatStore();
const workspaceStore = useWorkspaceStore();
const settings = useSettingsStore();
const workspace = computed(() => workspaceStore.selectedWorkspace);
const messages = computed(() => (workspace.value ? chat.getMessages(workspace.value.workspaceId, workspace.value.instanceId) : []));

let streamController: AbortController | null = null;
let streamingWorkspaceId: string | null = null;
let reconnectTimer: number | null = null;

function clearReconnectTimer() {
  if (reconnectTimer !== null) {
    window.clearTimeout(reconnectTimer);
    reconnectTimer = null;
  }
}

function closeStream() {
  clearReconnectTimer();
  if (streamController) {
    streamController.abort();
    streamController = null;
  }
  streamingWorkspaceId = null;
}

async function openStream(workspaceId: string, instanceId?: string, retry = 0) {
  clearReconnectTimer();
  if (streamController) {
    streamController.abort();
    streamController = null;
  }
  const controller = new AbortController();
  streamController = controller;
  streamingWorkspaceId = workspaceId;
  try {
    await openWorkspaceStream(
      workspaceId,
      {
        signal: controller.signal,
        onOpen: () => {
          // 连接建立后清空错误，重置重试次数
          retry = 0;
        },
        onEvent: (event) => {
          chat.handleWorkspaceEvent(event, workspaceId, instanceId);
        },
        onError: (error) => {
          chat.error = toUserMessage(error);
        },
      },
      instanceId,
    );
    // 自然结束：服务端关闭或网络中断
    if (streamingWorkspaceId === workspaceId && !controller.signal.aborted) {
      scheduleReconnect(workspaceId, instanceId, retry);
    }
  } catch (error) {
    if (controller.signal.aborted) return;
    chat.error = toUserMessage(error);
    if (streamingWorkspaceId === workspaceId) {
      scheduleReconnect(workspaceId, instanceId, retry);
    }
  }
}

function scheduleReconnect(workspaceId: string, instanceId: string | undefined, retry: number) {
  const next = Math.min(retry + 1, 6);
  const delay = Math.min(1000 * 2 ** retry, 15000);
  clearReconnectTimer();
  reconnectTimer = window.setTimeout(() => {
    if (streamingWorkspaceId === workspaceId) {
      void openStream(workspaceId, instanceId, next);
    }
  }, delay);
}

watch(
  workspace,
  (current, prev) => {
    // 工作区变化（包括首次）：关旧 SSE
    if (!current) {
      closeStream();
      return;
    }
    const sameWorkspace = prev && prev.workspaceId === current.workspaceId && prev.instanceId === current.instanceId;
    if (current.state !== 'ready' && current.state !== 'busy') {
      // 当前工作区离线，不建 SSE，也不拉历史
      closeStream();
      return;
    }
    if (!sameWorkspace) {
      closeStream();
      void openStream(current.workspaceId, current.instanceId);
    } else if (!streamController) {
      // 同一工作区但 SSE 还没建立（首次 immediate 触发）
      void openStream(current.workspaceId, current.instanceId);
    }
    void chat.ensureHistoryLoaded({
      workspaceId: current.workspaceId,
      instanceId: current.instanceId,
      sessionId: current.sessionId,
      text: '',
    });
  },
  { immediate: true },
);

onBeforeUnmount(() => {
  closeStream();
});

function buildRequest(text: string) {
  if (!workspace.value) throw new Error(t('workspace.needSelect'));
  return {
    workspaceId: workspace.value.workspaceId,
    instanceId: workspace.value.instanceId,
    sessionId: workspace.value.sessionId,
    text,
    autoSend: settings.autoSend,
    dedupeKey: `${Date.now()}-${Math.random().toString(16).slice(2)}`,
  };
}
function handleSend(text: string, stream: boolean) {
  const request = buildRequest(text);
  if (stream) void chat.sendStream(request);
  else void chat.sendNormal(request);
}
function handleHistory() {
  if (!workspace.value) return;
  void chat.loadHistory({ workspaceId: workspace.value.workspaceId, instanceId: workspace.value.instanceId, sessionId: workspace.value.sessionId, text: '' });
}
function handleClear() {
  if (!workspace.value) return;
  chat.clear(workspace.value.workspaceId, workspace.value.instanceId);
}
</script>
