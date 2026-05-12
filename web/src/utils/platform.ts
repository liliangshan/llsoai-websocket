/**
 * 平台检测与运行时配置工具
 *
 * 用途：
 * - 区分当前运行环境（浏览器 / iOS / Android）
 * - 在原生 App 中，由于不存在同源代理（Vite 的 /api 反代），需要使用后端的绝对地址
 * - 通过 localStorage 让用户在 App 内可动态切换后端地址，便于联调与发布
 */

const NATIVE_API_BASE_KEY = 'llsoai.native.apiBase';

/**
 * 判断是否运行在 Capacitor 原生容器中（iOS / Android App）
 */
export function isNativePlatform(): boolean {
  // Capacitor 在原生容器中会注入 window.Capacitor 对象
  const w = window as unknown as { Capacitor?: { isNativePlatform?: () => boolean; getPlatform?: () => string } };
  if (w.Capacitor?.isNativePlatform) {
    try {
      return w.Capacitor.isNativePlatform();
    } catch {
      return false;
    }
  }
  return false;
}

/**
 * 获取当前平台标识：web | ios | android
 */
export function getPlatform(): 'web' | 'ios' | 'android' {
  const w = window as unknown as { Capacitor?: { getPlatform?: () => string } };
  const p = w.Capacitor?.getPlatform?.() ?? 'web';
  if (p === 'ios' || p === 'android') {
    return p;
  }
  return 'web';
}

/**
 * 读取原生 App 内设置的后端基础地址（可被用户在设置页修改）
 */
export function readNativeApiBase(): string | null {
  try {
    return localStorage.getItem(NATIVE_API_BASE_KEY);
  } catch {
    return null;
  }
}

/**
 * 写入原生 App 内的后端基础地址
 */
export function writeNativeApiBase(base: string): void {
  try {
    localStorage.setItem(NATIVE_API_BASE_KEY, base);
  } catch {
    // 忽略写入失败
  }
}

/**
 * 解析最终用于 axios 的 baseURL
 *
 * 优先级：
 * 1. 原生 App 内由用户保存的地址（localStorage）
 * 2. 构建时注入的环境变量 VITE_API_BASE_URL
 * 3. 原生 App 默认地址 VITE_NATIVE_API_BASE_URL（构建时注入）
 * 4. Web 默认走 /api（被 Vite 反代）
 */
export function resolveApiBaseUrl(): string {
  if (isNativePlatform()) {
    const userSet = readNativeApiBase();
    if (userSet && userSet.trim()) {
      return userSet.trim();
    }
    const nativeDefault = import.meta.env.VITE_NATIVE_API_BASE_URL as string | undefined;
    if (nativeDefault && nativeDefault.trim()) {
      return nativeDefault.trim();
    }
    const generalEnv = import.meta.env.VITE_API_BASE_URL as string | undefined;
    if (generalEnv && /^https?:\/\//.test(generalEnv)) {
      return generalEnv;
    }
    // 原生环境兜底：必须返回绝对地址，否则请求会发到 capacitor://localhost
    return 'http://localhost:29081/api';
  }

  const webEnv = import.meta.env.VITE_API_BASE_URL as string | undefined;
  return webEnv && webEnv.trim() ? webEnv : '/api';
}
