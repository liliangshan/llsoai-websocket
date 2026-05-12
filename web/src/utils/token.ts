import { STORAGE_KEYS } from '@/constants/storage';

const TOKEN_KEY = STORAGE_KEYS.token;

/** Read JWT token from localStorage */
export function readToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

/** Save JWT token to localStorage */
export function saveToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token);
}

/** Clear JWT token from all storage */
export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(STORAGE_KEYS.tokenMode);
  sessionStorage.removeItem(TOKEN_KEY);
  sessionStorage.removeItem(STORAGE_KEYS.tokenMode);
}
