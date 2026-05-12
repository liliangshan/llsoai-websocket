<template>
  <div class="login-page">
    <el-card class="login-card">
      <template #header>
        <div class="card-header">
          <el-radio-group v-model="mode" size="default">
            <el-radio-button value="login">{{ t('auth.login') }}</el-radio-button>
            <el-radio-button value="register">{{ t('auth.register') }}</el-radio-button>
          </el-radio-group>
        </div>
      </template>

      <!-- 登录表单 -->
      <el-form v-if="mode === 'login'" label-position="top" class="login-form" @submit.prevent="handleLogin">
        <el-form-item :label="t('auth.username')">
          <el-input v-model="loginForm.username" :placeholder="t('auth.placeholderUsername')" :prefix-icon="User" />
        </el-form-item>
        <el-form-item :label="t('auth.password')">
          <el-input v-model="loginForm.password" type="password" show-password :placeholder="t('auth.placeholderPassword')" :prefix-icon="Lock" />
        </el-form-item>
        <ErrorAlert :message="auth.error" />
        <el-button
          type="primary"
          :loading="auth.validating"
          :disabled="!loginForm.username.trim() || !loginForm.password"
          @click="handleLogin"
        >
          {{ t('auth.login') }}
        </el-button>
      </el-form>

      <!-- 注册表单 -->
      <el-form v-else label-position="top" class="login-form" @submit.prevent="handleRegister">
        <el-form-item :label="t('auth.username')">
          <el-input v-model="registerForm.username" :placeholder="t('auth.placeholderUsername')" :prefix-icon="User" />
        </el-form-item>
        <el-form-item :label="t('auth.email')">
          <el-input v-model="registerForm.email" type="email" :placeholder="t('auth.placeholderEmail')" :prefix-icon="Message" />
        </el-form-item>
        <el-form-item :label="t('auth.password')">
          <el-input v-model="registerForm.password" type="password" show-password :placeholder="t('auth.placeholderPassword')" :prefix-icon="Lock" />
        </el-form-item>
        <el-form-item :label="t('auth.confirmPassword')">
          <el-input v-model="confirmPassword" type="password" show-password :placeholder="t('auth.placeholderPasswordAgain')" :prefix-icon="Lock" />
        </el-form-item>
        <ErrorAlert :message="registerError || auth.error" />
        <el-button
          type="primary"
          :loading="auth.validating"
          :disabled="!registerFormValid"
          @click="handleRegister"
        >
          {{ t('auth.register') }}
        </el-button>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue';
import { useRouter } from 'vue-router';
import { useI18n } from 'vue-i18n';
import { User, Lock, Message } from '@element-plus/icons-vue';
import { useAuthStore } from '@/stores/auth';
import { ROUTES } from '@/constants/routes';
import ErrorAlert from '@/components/common/ErrorAlert.vue';

const { t } = useI18n();
const mode = ref<'login' | 'register'>('login');
const auth = useAuthStore();
const router = useRouter();

// 登录表单
const loginForm = reactive({ username: '', password: '' });

// 注册表单
const registerForm = reactive({ username: '', password: '', email: '' });
const confirmPassword = ref('');
const registerError = ref('');

const registerFormValid = computed(() => {
  return (
    registerForm.username.trim() &&
    registerForm.email.trim() &&
    registerForm.password &&
    confirmPassword.value
  );
});

async function handleLogin() {
  const ok = await auth.login({
    username: loginForm.username.trim(),
    password: loginForm.password,
  });
  if (ok) await router.push(ROUTES.chat);
}

async function handleRegister() {
  registerError.value = '';
  if (registerForm.password !== confirmPassword.value) {
    registerError.value = t('auth.errPasswordMismatch');
    return;
  }
  if (registerForm.password.length < 6) {
    registerError.value = t('auth.errPasswordTooShort');
    return;
  }
  const ok = await auth.register({
    username: registerForm.username.trim(),
    password: registerForm.password,
    email: registerForm.email.trim(),
  });
  if (ok) await router.push(ROUTES.chat);
}
</script>
