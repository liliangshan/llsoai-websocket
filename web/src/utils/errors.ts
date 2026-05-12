import { ERROR_CODE_MESSAGES } from '@/constants/errors';
import type { ApiError } from '@/types/api';

export function toUserMessage(error: unknown): string {
  if (typeof error === 'string') return ERROR_CODE_MESSAGES[error] ?? error;
  const apiError = error as Partial<ApiError> | undefined;
  if (apiError?.code) return ERROR_CODE_MESSAGES[apiError.code] ?? apiError.message ?? apiError.code;
  if (apiError?.message) return apiError.message;
  return '请求失败，请稍后再试';
}
