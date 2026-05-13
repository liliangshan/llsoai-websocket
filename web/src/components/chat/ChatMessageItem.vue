<template>
  <div class="message-item" :class="message.role">
    <div class="message-bubble">
      <div class="message-meta">
        <span>{{ message.role }}</span>
        <el-tag v-if="message.source === 'history'" size="small">{{ t('common.history') }}</el-tag>
        <el-tag v-if="message.status !== 'done'" size="small" type="warning">{{ message.status }}</el-tag>
      </div>
      <pre v-if="message.content" class="message-content">{{ message.content }}</pre>
      <div v-else-if="message.role === 'assistant' && (message.status === 'pending' || message.status === 'sending' || message.status === 'streaming')" class="message-placeholder">
        <span class="message-placeholder-dot" />
        <span class="message-placeholder-dot" />
        <span class="message-placeholder-dot" />
      </div>
      <div v-if="message.streamingToolName" class="message-tool-calling">
        <span class="tool-calling-dot" />
        {{ t('common.callingTool') }}{{ message.streamingToolName }}
      </div>
      <div v-if="message.status === 'error' && message.error" class="message-error">
        <el-tag v-if="message.error.code" size="small" type="danger" effect="plain" class="message-error__code">
          {{ message.error.code }}
        </el-tag>
        <span class="message-error__text">{{ message.error.message }}</span>
      </div>
      <div v-if="visibleToolNames.length" class="message-tool-list">
        <div v-for="name in visibleToolNames" :key="name" class="message-tool-name">{{ name }}</div>
      </div>
      <el-collapse v-if="message.reasoning">
        <el-collapse-item :title="t('common.details')">
          <pre v-if="message.reasoning">{{ message.reasoning }}</pre>
        </el-collapse-item>
      </el-collapse>
    </div>
  </div>
</template>
<script setup lang="ts">
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import type { ChatMessage } from '@/types/chat';
const { t } = useI18n();
const props = defineProps<{ message: ChatMessage }>();
const visibleToolNames = computed(() => Array.from(new Set((props.message.toolCalls ?? []).map((item) => item.name).filter(Boolean))));
</script>

<style scoped>
.message-error {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 6px;
  padding: 8px 10px;
  background: var(--el-color-danger-light-9);
  border: 1px solid var(--el-color-danger-light-7);
  border-radius: 6px;
  color: var(--el-color-danger);
  font-size: 13px;
  line-height: 1.4;
  word-break: break-all;
}

.message-error__text {
  flex: 1;
}

.message-placeholder {
  display: flex;
  align-items: center;
  gap: 4px;
  min-height: 20px;
  padding: 4px 0;
}

.message-placeholder-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--el-text-color-secondary);
  animation: message-placeholder-bounce 1.2s ease-in-out infinite;
}

.message-placeholder-dot:nth-child(2) {
  animation-delay: 0.15s;
}

.message-placeholder-dot:nth-child(3) {
  animation-delay: 0.3s;
}

@keyframes message-placeholder-bounce {
  0%, 80%, 100% { opacity: 0.35; transform: translateY(0); }
  40% { opacity: 1; transform: translateY(-3px); }
}

.message-tool-calling {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 8px;
  padding: 6px 10px;
  background: var(--el-color-warning-light-9);
  border: 1px solid var(--el-color-warning-light-7);
  border-radius: 6px;
  color: var(--el-color-warning-dark-2);
  font-size: 13px;
  line-height: 1.4;
}

.message-tool-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 8px;
}

.message-tool-name {
  padding: 3px 8px;
  border-radius: 999px;
  background: var(--el-fill-color-light);
  color: var(--el-text-color-regular);
  font-size: 12px;
  line-height: 1.4;
}

.tool-calling-dot {
  display: inline-block;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--el-color-warning);
  animation: tool-calling-pulse 1s ease-in-out infinite;
}

@keyframes tool-calling-pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.4; transform: scale(0.8); }
}
</style>
