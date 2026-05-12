/**
 * PWA Service Worker 注册与版本更新提示
 *
 * 使用 vite-plugin-pwa 的虚拟模块 `virtual:pwa-register`：
 *  - 自动注入 SW 注册逻辑
 *  - 在检测到新版本时回调 onNeedRefresh，由前端弹窗让用户选择是否立即刷新
 *  - 在首次离线可用时回调 onOfflineReady
 *
 * 注意：仅在生产构建（vite build）中生效，dev 模式默认关闭。
 */
import { ElMessageBox, ElNotification } from 'element-plus';

export async function registerPWA(): Promise<void> {
  // 非浏览器环境（理论上不会发生，做一次保护）
  if (typeof window === 'undefined' || !('serviceWorker' in navigator)) {
    return;
  }

  try {
    // 该虚拟模块由 vite-plugin-pwa 在构建时提供
    const { registerSW } = await import('virtual:pwa-register');

    const updateSW = registerSW({
      immediate: true,
      onNeedRefresh() {
        ElMessageBox.confirm('检测到新版本，是否立即刷新以使用最新版本？', '版本更新', {
          confirmButtonText: '立即刷新',
          cancelButtonText: '稍后',
          type: 'info',
        })
          .then(() => {
            // true 表示在更新后自动 reload
            void updateSW(true);
          })
          .catch(() => {
            // 用户选择稍后，不做任何动作
          });
      },
      onOfflineReady() {
        ElNotification({
          title: '离线可用',
          message: '应用已缓存，下次可离线打开',
          type: 'success',
          duration: 3000,
        });
      },
      onRegisterError(error: unknown) {
        // 仅记录日志，不影响主流程
        console.warn('[PWA] Service Worker 注册失败：', error);
      },
    });
  } catch (err) {
    // 例如 dev 模式没有该虚拟模块时会进到这里，可以安全忽略
    console.warn('[PWA] 跳过 Service Worker 注册：', err);
  }
}
