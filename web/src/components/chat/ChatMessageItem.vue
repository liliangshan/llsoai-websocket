<template>
  <div class="message-item" :class="message.role">
    <div class="message-bubble">
      <div class="message-meta">
        <span>{{ message.role }}</span>
        <el-tag v-if="message.source === 'history'" size="small">{{ t('common.history') }}</el-tag>
        <el-tag v-if="message.status !== 'done'" size="small" type="warning">{{ message.status }}</el-tag>
      </div>
      <pre v-if="message.content" class="message-content">{{ message.content }}</pre>
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
      <el-collapse v-if="message.reasoning || message.toolCalls?.length || message.toolResults?.length">
        <el-collapse-item :title="t('common.details')">
          <pre v-if="message.reasoning">{{ message.reasoning }}</pre>
          <pre v-if="message.toolCalls?.length">{{ message.toolCalls }}</pre>
          <pre v-if="message.toolResults?.length">{{ message.toolResults }}</pre>
        </el-collapse-item>
      </el-collapse>
    </div>
  </div>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import type { ChatMessage } from '@/types/chat';
const { t } = useI18n();
defineProps<{ message: ChatMessage }>();
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
