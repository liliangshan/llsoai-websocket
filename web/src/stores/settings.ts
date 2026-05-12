import { defineStore } from 'pinia';
import { STORAGE_KEYS } from '@/constants/storage';

type Language = 'zh-CN' | 'en-US';

interface SettingsState {
  language: Language;
  showReasoning: boolean;
  autoSend: boolean;
  theme: 'light';
}

export const useSettingsStore = defineStore('settings', {
  state: (): SettingsState => ({
    language: (localStorage.getItem(STORAGE_KEYS.language) as Language) || 'zh-CN',
    showReasoning: false,
    autoSend: false,
    theme: 'light',
  }),
  actions: {
    setLanguage(language: Language) {
      this.language = language;
      localStorage.setItem(STORAGE_KEYS.language, language);
    },
  },
});
