import { defineStore } from 'pinia';
import { login as apiLogin, register as apiRegister, getCurrentUser } from '@/api/auth';
import { clearToken, readToken, saveToken } from '@/utils/token';
import type { CurrentUserPayload, LoginRequest, RegisterRequest } from '@/types/auth';

interface AuthState {
  token: string | null;
  user: CurrentUserPayload | null;
  initialized: boolean;
  validating: boolean;
  error: string | null;
}

export const useAuthStore = defineStore('auth', {
  state: (): AuthState => ({
    token: null,
    user: null,
    initialized: false,
    validating: false,
    error: null,
  }),
  getters: {
    isLoggedIn: (state) => Boolean(state.token && state.user),
    username: (state) => state.user?.username ?? '',
  },
  actions: {
    /** Restore token from storage on app init, then validate via /me */
    async restore() {
      const saved = readToken();
      if (saved) {
        this.token = saved;
        console.debug('Found saved token, validating...');
      }
      this.initialized = true;
    },

    /** Login with username + password */
    async login(credentials: LoginRequest) {
      this.validating = true;
      this.error = null;
      try {
        const response = await apiLogin(credentials);
        if (response.ok && response.payload) {
          this.token = response.payload.token;
          this.user = response.payload.user;
          saveToken(response.payload.token);
          return true;
        }
        this.error = response.error?.message ?? '登录失败';
        return false;
      } catch {
        this.error = '登录请求失败，请检查网络';
        return false;
      } finally {
        this.validating = false;
      }
    },

    /** Register a new account */
    async register(info: RegisterRequest) {
      this.validating = true;
      this.error = null;
      try {
        const response = await apiRegister(info);
        if (response.ok && response.payload) {
          this.token = response.payload.token;
          this.user = response.payload.user;
          saveToken(response.payload.token);
          return true;
        }
        this.error = response.error?.message ?? '注册失败';
        return false;
      } catch {
        this.error = '注册请求失败，请检查网络';
        return false;
      } finally {
        this.validating = false;
      }
    },

    /** Validate current token by calling /me */
    async validateToken() {
      if (!this.token) return false;
      this.validating = true;
      this.error = null;
      try {
        const response = await getCurrentUser();
        if (response.ok && response.payload) {
          this.user = response.payload;
          return true;
        }
        this.error = response.error?.message ?? '令牌校验失败';
        this.logout();
        return false;
      } catch {
        this.error = '令牌校验失败';
        this.logout();
        return false;
      } finally {
        this.validating = false;
      }
    },

    logout() {
      clearToken();
      this.token = null;
      this.user = null;
      this.error = null;
    },
  },
});
