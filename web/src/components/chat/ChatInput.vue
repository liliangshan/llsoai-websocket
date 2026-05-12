<template>
  <div class="chat-input">
    <el-input
      v-model="text"
      type="textarea"
      :rows="4"
      :disabled="disabled"
      :placeholder="t('chat.inputPlaceholder')"
      @keydown.enter.exact.prevent="emitSend(true)"
    />
    <div class="input-actions">
      <el-button @click="$emit('history')" :disabled="disabled">{{ t('chat.history') }}</el-button>
      <el-button @click="$emit('clear')" :disabled="disabled">{{ t('chat.clear') }}</el-button>
      <el-button v-if="streaming" type="warning" @click="$emit('cancel')">{{ t('chat.cancel') }}</el-button>
      <el-button type="primary" :disabled="disabled || !text.trim()" @click="emitSend(true)">{{ t('chat.send') }}</el-button>
    </div>
  </div>
</template>
<script setup lang="ts">
import { ref } from 'vue';
import { useI18n } from 'vue-i18n';
const { t } = useI18n();
defineProps<{ disabled: boolean; streaming: boolean }>();
const emit = defineEmits<{ send: [text: string, stream: boolean]; history: []; clear: []; cancel: [] }>();
const text = ref('');
function emitSend(stream: boolean) {
  const value = text.value.trim();
  if (!value) return;
  emit('send', value, stream);
  text.value = '';
}
</script>
