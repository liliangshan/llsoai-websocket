import { fileURLToPath, URL } from 'node:url';
import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import { VitePWA } from 'vite-plugin-pwa';

export default defineConfig({
  plugins: [
    vue(),
    VitePWA({
      // 自动注册 Service Worker，并在新版本可用时由前端弹窗提示刷新
      registerType: 'prompt',
      injectRegister: false,
      // 开发环境也启用 SW，方便联调（生产无影响）
      devOptions: {
        enabled: false,
      },
      includeAssets: [
        'favicon.ico',
        'apple-touch-icon.png',
        'pwa-192x192.png',
        'pwa-512x512.png',
        'maskable-icon-512x512.png',
      ],
      manifest: {
        name: 'llsoai WebSocket',
        short_name: 'llsoai',
        description: 'llsoai WebSocket 客户端，支持安装到桌面与移动设备主屏幕',
        lang: 'zh-CN',
        theme_color: '#409EFF',
        background_color: '#ffffff',
        display: 'standalone',
        orientation: 'portrait',
        scope: '/',
        start_url: '/',
        icons: [
          {
            src: 'pwa-192x192.png',
            sizes: '192x192',
            type: 'image/png',
          },
          {
            src: 'pwa-512x512.png',
            sizes: '512x512',
            type: 'image/png',
          },
          {
            src: 'maskable-icon-512x512.png',
            sizes: '512x512',
            type: 'image/png',
            purpose: 'maskable',
          },
        ],
      },
      workbox: {
        // 预缓存所有打包产物（JS / CSS / HTML / 图片 / 字体）
        globPatterns: ['**/*.{js,css,html,ico,png,svg,webp,woff,woff2,ttf}'],
        // SPA 路由回退到 index.html，确保离线也能进入应用
        navigateFallback: '/index.html',
        // 后端接口与 WebSocket 不缓存
        navigateFallbackDenylist: [/^\/api\//, /^\/ws/, /^\/health/],
        cleanupOutdatedCaches: true,
        clientsClaim: true,
        skipWaiting: false,
        runtimeCaching: [
          {
            // 不缓存任何后端 API，始终走网络
            urlPattern: ({ url }) =>
              url.pathname.startsWith('/api') ||
              url.pathname.startsWith('/ws') ||
              url.pathname.startsWith('/health'),
            handler: 'NetworkOnly',
            method: 'GET',
          },
          {
            // 跨域字体走 CacheFirst
            urlPattern: ({ url }) =>
              url.origin === 'https://fonts.googleapis.com' ||
              url.origin === 'https://fonts.gstatic.com',
            handler: 'CacheFirst',
            options: {
              cacheName: 'google-fonts-cache',
              expiration: {
                maxEntries: 20,
                maxAgeSeconds: 60 * 60 * 24 * 365,
              },
            },
          },
        ],
      },
    }),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:29081',
        changeOrigin: true,
      },
      '/health': {
        target: 'http://localhost:29081',
        changeOrigin: true,
      },
    },
  },
});
