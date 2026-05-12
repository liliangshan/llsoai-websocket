import { createI18n } from 'vue-i18n';
import zhCN from './zh-CN';
import zhTW from './zh-TW';
import enUS from './en-US';
import jaJP from './ja-JP';
import koKR from './ko-KR';
import deDE from './de-DE';
import frFR from './fr-FR';

/** 支持的标准 BCP 47 语言代码。 */
export const SUPPORTED_LOCALES = ['en-US', 'zh-CN', 'zh-TW', 'ja-JP', 'ko-KR', 'de-DE', 'fr-FR'] as const;
export type SupportedLocale = (typeof SUPPORTED_LOCALES)[number];

/** 默认语言：英文。 */
export const DEFAULT_LOCALE: SupportedLocale = 'en-US';

/** 语言展示名（在语言切换器里显示自身名称）。 */
export const LOCALE_LABELS: Record<SupportedLocale, string> = {
  'en-US': 'English',
  'zh-CN': '简体中文',
  'zh-TW': '繁體中文',
  'ja-JP': '日本語',
  'ko-KR': '한국어',
  'de-DE': 'Deutsch',
  'fr-FR': 'Français',
};

/** 常见非标准/短代码到标准 BCP 47 的别名映射。 */
const LOCALE_ALIAS: Record<string, SupportedLocale> = {
  en: 'en-US',
  'en-us': 'en-US',
  'en-gb': 'en-US',
  zh: 'zh-CN',
  'zh-cn': 'zh-CN',
  'zh-hans': 'zh-CN',
  'zh-hans-cn': 'zh-CN',
  'zh-sg': 'zh-CN',
  'zh-tw': 'zh-TW',
  'zh-hant': 'zh-TW',
  'zh-hant-tw': 'zh-TW',
  'zh-hk': 'zh-TW',
  'zh-mo': 'zh-TW',
  ja: 'ja-JP',
  'ja-jp': 'ja-JP',
  ko: 'ko-KR',
  'ko-kr': 'ko-KR',
  de: 'de-DE',
  'de-de': 'de-DE',
  'de-at': 'de-DE',
  'de-ch': 'de-DE',
  fr: 'fr-FR',
  'fr-fr': 'fr-FR',
  'fr-ca': 'fr-FR',
};

const STORAGE_KEY = 'app.locale';

/**
 * 把任意输入归一化为受支持的标准 BCP 47 语言代码；无法识别时返回 null。
 */
export function normalizeLocale(input: string | null | undefined): SupportedLocale | null {
  if (!input) return null;
  const lower = input.trim().toLowerCase();
  if (!lower) return null;
  if (LOCALE_ALIAS[lower]) return LOCALE_ALIAS[lower];
  // 直接匹配（大小写不敏感）
  const direct = SUPPORTED_LOCALES.find((code) => code.toLowerCase() === lower);
  if (direct) return direct;
  // 只取主语言段再尝试一次
  const primary = lower.split('-')[0];
  if (primary && LOCALE_ALIAS[primary]) return LOCALE_ALIAS[primary];
  return null;
}

function readQueryLocale(): SupportedLocale | null {
  if (typeof window === 'undefined') return null;
  try {
    const params = new URLSearchParams(window.location.search);
    return normalizeLocale(params.get('lang') ?? params.get('locale'));
  } catch {
    return null;
  }
}

function readStorageLocale(): SupportedLocale | null {
  if (typeof window === 'undefined' || !window.localStorage) return null;
  try {
    return normalizeLocale(window.localStorage.getItem(STORAGE_KEY));
  } catch {
    return null;
  }
}

/**
 * 决定初始语言：优先级 URL ?lang= > localStorage > 默认 en-US。
 * 注意：本地没有缓存且 URL 没有指定时，固定回退到英文，不再读取浏览器系统语言偏好。
 * URL 中显式指定的语言会写回 localStorage。
 */
export function resolveInitialLocale(): SupportedLocale {
  const fromQuery = readQueryLocale();
  if (fromQuery) {
    persistLocale(fromQuery);
    return fromQuery;
  }
  const fromStorage = readStorageLocale();
  if (fromStorage) return fromStorage;
  return DEFAULT_LOCALE;
}

function persistLocale(locale: SupportedLocale) {
  if (typeof window === 'undefined' || !window.localStorage) return;
  try {
    window.localStorage.setItem(STORAGE_KEY, locale);
  } catch {
    /* ignore */
  }
}

export const i18n = createI18n({
  legacy: false,
  locale: resolveInitialLocale(),
  fallbackLocale: DEFAULT_LOCALE,
  messages: {
    'en-US': enUS,
    'zh-CN': zhCN,
    'zh-TW': zhTW,
    'ja-JP': jaJP,
    'ko-KR': koKR,
    'de-DE': deDE,
    'fr-FR': frFR,
  },
});

/**
 * 切换语言：更新 vue-i18n、写 localStorage、同步 <html lang>、把 URL 的 ?lang= 也改掉（不刷新页面）。
 */
export function setLocale(input: string): SupportedLocale {
  const next = normalizeLocale(input) ?? DEFAULT_LOCALE;
  i18n.global.locale.value = next;
  persistLocale(next);
  if (typeof document !== 'undefined') {
    document.documentElement.setAttribute('lang', next);
  }
  if (typeof window !== 'undefined') {
    try {
      const url = new URL(window.location.href);
      url.searchParams.set('lang', next);
      window.history.replaceState(null, '', url.toString());
    } catch {
      /* ignore */
    }
  }
  return next;
}

/** 当前激活语言。 */
export function getLocale(): SupportedLocale {
  return i18n.global.locale.value as SupportedLocale;
}

// 启动时把 <html lang> 同步到当前语言
if (typeof document !== 'undefined') {
  document.documentElement.setAttribute('lang', getLocale());
}
