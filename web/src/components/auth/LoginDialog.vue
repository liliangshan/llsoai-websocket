<template>
  <el-dialog
    v-model="visible"
    :title="isLoginMode ? t('auth.login') : t('auth.register')"
    width="400px"
    :close-on-click-modal="false"
    @closed="resetForm"
  >
    <el-form label-position="top" @submit.prevent="handleSubmit">
      <!-- 登录表单 -->
      <template v-if="isLoginMode">
        <el-form-item :label="t('auth.username')">
          <el-input
            v-model="loginForm.username"
            :placeholder="t('auth.placeholderUsername')"
            :prefix-icon="User"
            @keyup.enter="handleSubmit"
          />
        </el-form-item>
        <el-form-item :label="t('auth.password')">
          <el-input
            v-model="loginForm.password"
            type="password"
            show-password
            :placeholder="t('auth.placeholderPassword')"
            :prefix-icon="Lock"
            @keyup.enter="handleSubmit"
          />
        </el-form-item>
        <div class="toggle-mode">
          {{ t('auth.noAccount') }}
          <el-button type="primary" link @click="isLoginMode = false">{{ t('auth.goRegister') }}</el-button>
        </div>
      </template>

      <!-- 注册表单 -->
      <template v-else>
        <el-form-item :label="t('auth.username')">
          <el-input
            v-model="registerForm.username"
            :placeholder="t('auth.placeholderUsername')"
            :prefix-icon="User"
          />
        </el-form-item>
        <el-form-item :label="t('auth.email')">
          <el-input
            v-model="registerForm.email"
            type="email"
            :placeholder="t('auth.placeholderEmail')"
            :prefix-icon="Message"
          />
        </el-form-item>
        <el-form-item :label="t('auth.password')">
          <el-input
            v-model="registerForm.password"
            type="password"
            show-password
            :placeholder="t('auth.placeholderPasswordWithRule')"
            :prefix-icon="Lock"
          />
        </el-form-item>
        <el-form-item :label="t('auth.confirmPassword')">
          <el-input
            v-model="confirmPassword"
            type="password"
            show-password
            :placeholder="t('auth.placeholderPasswordAgain')"
            :prefix-icon="Lock"
            @keyup.enter="handleSubmit"
          />
        </el-form-item>
        <div class="toggle-mode">
          {{ t('auth.hasAccount') }}
          <el-button type="primary" link @click="isLoginMode = true">{{ t('auth.goLogin') }}</el-button>
        </div>
      </template>

      <ErrorAlert :message="formError || auth.error" />

      <el-form-item>
        <el-button type="primary" class="submit-btn" :loading="auth.validating" @click="handleSubmit">
          {{ isLoginMode ? t('auth.login') : t('auth.register') }}
        </el-button>
      </el-form-item>
    </el-form>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { User, Lock, Message } from '@element-plus/icons-vue';
import { useAuthStore } from '@/stores/auth';
import ErrorAlert from '@/components/common/ErrorAlert.vue';

const { t } = useI18n();
const props = defineProps<{
  modelValue: boolean;
}>();

const emit = defineEmits<{
  'update:modelValue': [value: boolean];
}>();

const auth = useAuthStore();

const visible = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val),
});

const isLoginMode = ref(true);
const formError = ref('');

const loginForm = reactive({ username: '', password: '' });
const registerForm = reactive({ username: '', password: '', email: '' });
const confirmPassword = ref('');

watch(visible, (val) => {
  if (val) {
    auth.error = null;
    formError.value = '';
  }
});

function resetForm() {
  loginForm.username = '';
  loginForm.password = '';
  registerForm.username = '';
  registerForm.password = '';
  registerForm.email = '';
  confirmPassword.value = '';
  formError.value = '';
  isLoginMode.value = true;
}

async function handleSubmit() {
  formError.value = '';
  auth.error = null;

  if (isLoginMode.value) {
    if (!loginForm.username.trim() || !loginForm.password) {
      formError.value = t('auth.errEmptyAccount');
      return;
    }
    const ok = await auth.login({
      username: loginForm.username.trim(),
      password: loginForm.password,
    });
    if (ok) {
      visible.value = false;
    }
  } else {
    if (!registerForm.username.trim()) {
      formError.value = t('auth.errEmptyUsername');
      return;
    }
    if (!registerForm.email.trim()) {
      formError.value = t('auth.errEmptyEmail');
      return;
    }
    if (registerForm.password.length < 6) {
      formError.value = t('auth.errPasswordTooShort');
      return;
    }
    if (registerForm.password !== confirmPassword.value) {
      formError.value = t('auth.errPasswordMismatch');
      return;
    }
    const ok = await auth.register({
      username: registerForm.username.trim(),
      password: registerForm.password,
      email: registerForm.email.trim(),
    });
    if (ok) {
      visible.value = false;
    }
  }
}
</script>

<style scoped>
.toggle-mode {
  text-align: center;
  margin-bottom: 16px;
  color: var(--el-text-color-secondary);
  font-size: 14px;
}

.submit-btn {
  width: 100%;
}
</style>
