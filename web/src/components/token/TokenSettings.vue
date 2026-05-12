<template>
  <div class="token-settings">
    <!-- 未登录：胶囊登录按钮 -->
    <button v-if="!auth.isLoggedIn" class="login-pill" @click="emit('login')">
      <el-icon class="login-pill__icon"><User /></el-icon>
      <span>{{ t('auth.loginOrRegister') }}</span>
    </button>

    <!-- 未登录时也提供语言切换入口 -->
    <div v-if="!auth.isLoggedIn" class="guest-language">
      <span class="guest-language__label">{{ t('common.language') }}</span>
      <LanguageSwitcher />
    </div>

    <!-- 已登录：整个用户卡片可点击 -->
    <button v-else class="user-card" type="button" @click="dialogVisible = true">
      <div class="user-card__avatar">{{ avatarText }}</div>
      <div class="user-card__info">
        <div class="user-card__name">{{ auth.username }}</div>
        <div v-if="auth.user?.email" class="user-card__email" :title="auth.user.email">
          {{ auth.user.email }}
        </div>
        <div v-else class="user-card__status">
          <span class="user-card__dot" />
          {{ t('common.online') }}
        </div>
      </div>
      <span class="user-card__action" aria-hidden="true">
        <el-icon><Setting /></el-icon>
      </span>
    </button>

    <!-- 用户设置对话框 -->
    <el-dialog
      v-model="dialogVisible"
      :title="t('account.title')"
      width="460px"
      :close-on-click-modal="true"
      append-to-body
      @open="loadWsToken"
    >
      <div class="dialog-section">
        <div class="dialog-section__label">{{ t('account.username') }}</div>
        <div class="dialog-section__value">{{ auth.username }}</div>
      </div>

      <div v-if="auth.user?.email" class="dialog-section">
        <div class="dialog-section__label">{{ t('account.email') }}</div>
        <div class="dialog-section__value">{{ auth.user.email }}</div>
      </div>

      <div class="dialog-section">
        <div class="dialog-section__label">{{ t('common.language') }}</div>
        <LanguageSwitcher />
      </div>

      <div class="dialog-section">
        <div class="dialog-section__label">{{ t('account.wsAddress') }}</div>
        <div v-if="wsTokenLoading" class="token-loading">
          <el-icon class="is-loading"><Loading /></el-icon>
          <span>{{ t('account.wsLoading') }}</span>
        </div>
        <template v-else-if="wsTokenError">
          <el-alert :title="wsTokenError" type="error" show-icon :closable="false" />
          <el-button class="token-retry" size="small" @click="loadWsToken">{{ t('common.retry') }}</el-button>
        </template>
        <template v-else>
          <div class="token-row">
            <el-input
              :model-value="displayValue"
              type="text"
              readonly
              size="default"
            />
            <el-button type="primary" plain :icon="DocumentCopy" @click="copyToken">{{ t('common.copy') }}</el-button>
          </div>
          <div class="token-actions">
            <el-button
              size="small"
              :icon="Refresh"
              :loading="wsTokenRotating"
              @click="handleRotate"
            >
              {{ t('account.regenerate') }}
            </el-button>
            <span class="token-tip">{{ t('account.regenerateTip') }}</span>
          </div>
        </template>
      </div>

      <template #footer>
        <el-button @click="dialogVisible = false">{{ t('common.close') }}</el-button>
        <el-button type="danger" :icon="SwitchButton" @click="handleLogout">{{ t('auth.logout') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { ElMessage, ElMessageBox } from 'element-plus';
import {
  User,
  Setting,
  SwitchButton,
  DocumentCopy,
  Loading,
  Refresh,
} from '@element-plus/icons-vue';
import { useAuthStore } from '@/stores/auth';
import { getWebSocketToken, rotateWebSocketToken } from '@/api/auth';
import LanguageSwitcher from '@/components/common/LanguageSwitcher.vue';

const { t } = useI18n();
const auth = useAuthStore();
const emit = defineEmits<{
  login: [];
}>();

const dialogVisible = ref(false);
const wsToken = ref('');
const wsTokenLoading = ref(false);
const wsTokenRotating = ref(false);
const wsTokenError = ref('');

const avatarText = computed(() => {
  const name = auth.username || '';
  return name ? name.charAt(0).toUpperCase() : 'U';
});

/** 根据当前页面协议/域名 + token 组装出 ws(s)://host/ws?token=xxx */
const wsUrl = computed(() => {
  if (!wsToken.value) return '';
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const host = window.location.host;
  return `${proto}//${host}/ws?token=${encodeURIComponent(wsToken.value)}`;
});

/** 输入框展示值：直接明文显示完整 URL */
const displayValue = computed(() => wsUrl.value);

async function loadWsToken() {
  wsTokenLoading.value = true;
  wsTokenError.value = '';
  try {
    const res = await getWebSocketToken();
    if (res.ok && res.payload) {
      wsToken.value = res.payload.token;
    } else {
      wsTokenError.value = res.error?.message ?? t('account.wsLoadError');
    }
  } catch {
    wsTokenError.value = t('account.wsLoadErrorNetwork');
  } finally {
    wsTokenLoading.value = false;
  }
}

async function handleRotate() {
  try {
    await ElMessageBox.confirm(
      t('account.regenerateConfirm'),
      t('account.regenerateTitle'),
      {
        confirmButtonText: t('account.regenerateConfirmBtn'),
        cancelButtonText: t('common.cancel'),
        type: 'warning',
      },
    );
  } catch {
    return;
  }
  wsTokenRotating.value = true;
  try {
    const res = await rotateWebSocketToken();
    if (res.ok && res.payload) {
      wsToken.value = res.payload.token;
      ElMessage.success(t('account.regenerateSuccess'));
    } else {
      ElMessage.error(res.error?.message ?? t('account.regenerateError'));
    }
  } catch {
    ElMessage.error(t('account.regenerateErrorNetwork'));
  } finally {
    wsTokenRotating.value = false;
  }
}

async function copyToken() {
  const value = wsUrl.value;
  if (!value) {
    ElMessage.warning(t('account.copyEmpty'));
    return;
  }
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(value);
    } else {
      const textarea = document.createElement('textarea');
      textarea.value = value;
      textarea.style.position = 'fixed';
      textarea.style.opacity = '0';
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand('copy');
      document.body.removeChild(textarea);
    }
    ElMessage.success(t('account.copySuccess'));
  } catch {
    ElMessage.error(t('account.copyError'));
  }
}

function handleLogout() {
  auth.logout();
  dialogVisible.value = false;
  ElMessage.success(t('auth.logoutSuccess'));
}
</script>

<style scoped>
.token-settings {
  width: 100%;
  box-sizing: border-box;
}

/* 未登录态：语言切换器 */
.guest-language {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
}

.guest-language__label {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  flex-shrink: 0;
}

/* 登录按钮：圆角胶囊样式 */
.login-pill {
  display: flex;
  width: 100%;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 12px 20px;
  border: none;
  border-radius: 999px;
  background: linear-gradient(135deg, #4f8cff 0%, #2563eb 100%);
  color: #fff;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  box-shadow: 0 4px 14px rgba(37, 99, 235, 0.35);
  transition: transform 0.2s ease, box-shadow 0.2s ease, filter 0.2s ease;
}

.login-pill:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 20px rgba(37, 99, 235, 0.45);
  filter: brightness(1.05);
}

.login-pill:active {
  transform: translateY(0);
  box-shadow: 0 3px 10px rgba(37, 99, 235, 0.35);
}

.login-pill__icon {
  font-size: 16px;
}

/* 已登录用户卡片（整体可点击） */
.user-card {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 12px;
  padding: 8px 14px 8px 8px;
  border-radius: 999px;
  background: var(--el-bg-color, #fff);
  border: 1px solid var(--el-border-color-lighter, #ebeef5);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
  transition: box-shadow 0.2s ease, transform 0.2s ease, border-color 0.2s ease;
  box-sizing: border-box;
  cursor: pointer;
  text-align: left;
  font: inherit;
  color: inherit;
}

.user-card:hover {
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.12);
  transform: translateY(-1px);
  border-color: var(--el-color-primary-light-5, #a0cfff);
}

.user-card:active {
  transform: translateY(0);
}

.user-card__avatar {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: linear-gradient(135deg, #4f8cff 0%, #2563eb 100%);
  color: #fff;
  font-weight: 600;
  font-size: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.user-card__info {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  line-height: 1.2;
  min-width: 0;
}

.user-card__name {
  font-size: 14px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.user-card__email {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 2px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.user-card__status {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 2px;
}

.user-card__dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #10b981;
  box-shadow: 0 0 0 2px rgba(16, 185, 129, 0.2);
}

.user-card__action {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--el-text-color-secondary);
  flex-shrink: 0;
  transition: background 0.2s ease, color 0.2s ease, transform 0.3s ease;
}

.user-card:hover .user-card__action {
  background: var(--el-color-primary-light-9, #ecf5ff);
  color: var(--el-color-primary, #2563eb);
  transform: rotate(60deg);
}

/* 弹窗内容 */
.dialog-section {
  margin-bottom: 16px;
}

.dialog-section:last-child {
  margin-bottom: 0;
}

.dialog-section__label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-bottom: 6px;
}

.dialog-section__value {
  font-size: 14px;
  color: var(--el-text-color-primary);
  word-break: break-all;
}

.token-row {
  display: flex;
  gap: 8px;
  align-items: stretch;
}

.token-row :deep(.el-input) {
  flex: 1 1 auto;
  min-width: 0;
}

.token-tip {
  margin-top: 6px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.token-loading {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
  padding: 4px 0;
}

.token-loading .el-icon {
  animation: rotate 1s linear infinite;
}

.token-retry {
  margin-top: 8px;
}

.token-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 8px;
}

.token-actions .token-tip {
  margin-top: 0;
}

@keyframes rotate {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>

