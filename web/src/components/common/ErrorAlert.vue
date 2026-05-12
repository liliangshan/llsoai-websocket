<template>
  <el-alert v-if="message" :title="message" type="error" show-icon :closable="false" />
</template>
<script setup lang="ts">
defineProps<{ message?: string | null }>();
</script>
