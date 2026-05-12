<template>
  <div v-loading="loading" class="loading-block"><slot /></div>
</template>
<script setup lang="ts">
defineProps<{ loading: boolean }>();
</script>
