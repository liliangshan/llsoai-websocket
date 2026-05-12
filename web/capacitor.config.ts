import type { CapacitorConfig } from '@capacitor/cli';

/**
 * Capacitor 配置文件
 *
 * 说明：
 * - appId：应用唯一标识符，按反向域名格式书写，发布前请按你自己的域名修改
 * - appName：应用显示名称
 * - webDir：Vite 构建产物输出目录（默认 dist）
 * - server.androidScheme：安卓内置 WebView 使用的协议，建议保持 https 以便 cookie 与安全策略一致
 * - server.cleartext：是否允许明文 http（仅本地开发联调用，发布前请置为 false）
 * - server.url：开发模式下可指向本机 dev server 实现热更新，发布前请删除或注释
 */
const config: CapacitorConfig = {
  appId: 'com.llsoai.websocket',
  appName: 'llsoai',
  webDir: 'dist',
  bundledWebRuntime: false,

  server: {
    androidScheme: 'https',
    // 开发联调时，把下面的 url 改为电脑局域网 IP，例如：
    // url: 'http://192.168.1.10:5173',
    // cleartext: true,
  },

  ios: {
    // iOS 内容内边距策略，'always' 可让内容自动避开刘海/状态栏
    contentInset: 'always',
    // 允许在 WKWebView 中混合内容（仅开发联调用）
    // limitsNavigationsToAppBoundDomains: false,
  },

  android: {
    // 安卓允许打开 mixed content（仅本地联调，发布前关闭）
    allowMixedContent: false,
    captureInput: true,
    webContentsDebuggingEnabled: true,
  },

  plugins: {
    SplashScreen: {
      launchShowDuration: 1500,
      launchAutoHide: true,
      backgroundColor: '#ffffff',
      androidSplashResourceName: 'splash',
      androidScaleType: 'CENTER_CROP',
      showSpinner: false,
      splashFullScreen: true,
      splashImmersive: true,
    },
    Keyboard: {
      resize: 'body',
      style: 'light',
      resizeOnFullScreen: true,
    },
    StatusBar: {
      style: 'default',
      backgroundColor: '#ffffff',
      overlaysWebView: false,
    },
  },
};

export default config;
