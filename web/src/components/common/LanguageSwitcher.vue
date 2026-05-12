<template>
  <el-select
    :model-value="current"
    size="small"
    class="language-switcher"
    @change="onChange"
  >
    <el-option
      v-for="code in SUPPORTED_LOCALES"
      :key="code"
      :value="code"
      :label="LOCALE_LABELS[code]"
    />
  </el-select>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { LOCALE_LABELS, SUPPORTED_LOCALES, setLocale, type SupportedLocale } from '@/i18n';

const { locale } = useI18n();
const current = computed(() => locale.value as SupportedLocale);

function onChange(value: SupportedLocale) {
  setLocale(value);
}
</script>

<style scoped>
.language-switcher {
  width: 100%;
}
</style>
