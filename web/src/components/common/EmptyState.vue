<template>
  <div class="empty-state">{{ text }}</div>
</template>
<script setup lang="ts">
defineProps<{ text: string }>();
</script>
