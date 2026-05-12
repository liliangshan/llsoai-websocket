export const ERROR_CODE_MESSAGES: Record<string, string> = {
  unauthorized: '登录已过期，请重新登录',
  UNAUTHORIZED: '登录已过期，请重新登录',
  TOKEN_INVALID: '登录已过期，请重新登录',
  workspace_offline: '工作区已离线，请选择其他工作区',
  WORKSPACE_OFFLINE: '工作区已离线，请选择其他工作区',
  instance_offline: '实例已离线，请刷新后重试',
  request_timeout: '请求超时，请检查网络后重试',
  REQUEST_TIMEOUT: '请求超时，请检查网络后重试',
  rate_limited: '请求过于频繁，请稍后再试',
  RATE_LIMITED: '请求过于频繁，请稍后再试',
  HISTORY_DISABLED: '当前项目未开启日志保存',
};
