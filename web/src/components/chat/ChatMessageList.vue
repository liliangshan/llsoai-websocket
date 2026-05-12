<template>
  <div ref="container" class="message-list">
    <EmptyState v-if="messages.length === 0" :text="t('chat.emptyHint')" />
    <ChatMessageItem v-for="message in messages" :key="message.id" :message="message" />
  </div>
</template>
<script setup lang="ts">
import { nextTick, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import type { ChatMessage } from '@/types/chat';
import ChatMessageItem from './ChatMessageItem.vue';
import EmptyState from '@/components/common/EmptyState.vue';
const { t } = useI18n();
const props = defineProps<{ messages: ChatMessage[] }>();
const container = ref<HTMLElement | null>(null);
watch(
  () => props.messages.map((item) => `${item.id}:${item.content.length}:${item.status}`).join('|'),
  async () => {
    await nextTick();
    if (container.value) container.value.scrollTop = container.value.scrollHeight;
  },
);
</script>
