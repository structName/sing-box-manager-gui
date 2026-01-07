import axios from 'axios';

const api = axios.create({
  baseURL: '/api',
  timeout: 30000,
});

// 订阅 API
export const subscriptionApi = {
  getAll: () => api.get('/subscriptions'),
  add: (name: string, url: string) => api.post('/subscriptions', { name, url }),
  update: (id: string, data: any) => api.put(`/subscriptions/${id}`, data),
  delete: (id: string) => api.delete(`/subscriptions/${id}`),
  refresh: (id: string) => api.post(`/subscriptions/${id}/refresh`),
  refreshAll: () => api.post('/subscriptions/refresh-all'),
};

// 过滤器 API
export const filterApi = {
  getAll: () => api.get('/filters'),
  add: (data: any) => api.post('/filters', data),
  update: (id: string, data: any) => api.put(`/filters/${id}`, data),
  delete: (id: string) => api.delete(`/filters/${id}`),
};

// 规则 API
export const ruleApi = {
  getAll: () => api.get('/rules'),
  add: (data: any) => api.post('/rules', data),
  update: (id: string, data: any) => api.put(`/rules/${id}`, data),
  delete: (id: string) => api.delete(`/rules/${id}`),
};

// 规则组 API
export const ruleGroupApi = {
  getAll: () => api.get('/rule-groups'),
  update: (id: string, data: any) => api.put(`/rule-groups/${id}`, data),
};

// 规则集验证 API
export const ruleSetApi = {
  validate: (type: 'geosite' | 'geoip', name: string) =>
    api.get('/ruleset/validate', { params: { type, name } }),
};

// 设置 API
export const settingsApi = {
  get: () => api.get('/settings'),
  update: (data: any) => api.put('/settings', data),
  getSystemHosts: () => api.get('/system-hosts'),
};

// 配置 API
export const configApi = {
  generate: () => api.post('/config/generate'),
  preview: () => api.get('/config/preview'),
  apply: () => api.post('/config/apply'),
  // 导出 sing-box 配置（返回下载 URL）
  exportUrl: () => '/api/config/export',
};

// 备份恢复 API
export const backupApi = {
  // 导出备份（返回下载 URL）
  exportUrl: () => '/api/backup',
  // 导入备份
  restore: (file: File) => {
    const formData = new FormData();
    formData.append('file', file);
    return api.post('/backup/restore', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
  },
};

// Profile API
export const profileApi = {
  getAll: () => api.get('/profiles'),
  get: (id: string) => api.get(`/profiles/${id}`),
  create: (name: string, description: string) => api.post('/profiles', { name, description }),
  update: (id: string, data: { name: string; description: string }) => api.put(`/profiles/${id}`, data),
  delete: (id: string) => api.delete(`/profiles/${id}`),
  activate: (id: string) => api.post(`/profiles/${id}/activate`),
  snapshot: (id: string) => api.post(`/profiles/${id}/snapshot`),
  exportUrl: (id: string) => `/api/profiles/${id}/export`,
  import: (file: File) => {
    const formData = new FormData();
    formData.append('file', file);
    return api.post('/profiles/import', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
  },
};

// 服务 API
export const serviceApi = {
  status: () => api.get('/service/status'),
  start: () => api.post('/service/start'),
  stop: () => api.post('/service/stop'),
  restart: () => api.post('/service/restart'),
  reload: () => api.post('/service/reload'),
};

// launchd API
export const launchdApi = {
  status: () => api.get('/launchd/status'),
  install: () => api.post('/launchd/install'),
  uninstall: () => api.post('/launchd/uninstall'),
  restart: () => api.post('/launchd/restart'),
};

// 统一守护进程 API（自动判断系统）
export const daemonApi = {
  status: () => api.get('/daemon/status'),
  install: () => api.post('/daemon/install'),
  uninstall: () => api.post('/daemon/uninstall'),
  restart: () => api.post('/daemon/restart'),
};

// 监控 API
export const monitorApi = {
  system: () => api.get('/monitor/system'),
  logs: () => api.get('/monitor/logs'),
  appLogs: (lines: number = 200) => api.get(`/monitor/logs/sbm?lines=${lines}`),
  singboxLogs: (lines: number = 200) => api.get(`/monitor/logs/singbox?lines=${lines}`),
};

// 节点 API
export const nodeApi = {
  getAll: () => api.get('/nodes'),
  getGrouped: () => api.get('/nodes/grouped'),
  getCountries: () => api.get('/nodes/countries'),
  getByCountry: (code: string) => api.get(`/nodes/country/${code}`),
  parse: (url: string) => api.post('/nodes/parse', { url }),
  // 获取所有节点的延迟（从数据库）
  getDelays: () => api.get('/nodes/delays'),
  // 测试单个节点的延迟
  testDelay: (tag: string) => api.post(`/nodes/${encodeURIComponent(tag)}/delay`),
  // 批量刷新所有节点延迟
  refreshAllDelays: () => api.post('/nodes/delays/refresh'),
};

// 手动节点 API
export const manualNodeApi = {
  getAll: () => api.get('/manual-nodes'),
  add: (data: any) => api.post('/manual-nodes', data),
  update: (id: string, data: any) => api.put(`/manual-nodes/${id}`, data),
  delete: (id: string) => api.delete(`/manual-nodes/${id}`),
};

// 入站端口 API
export const inboundPortApi = {
  getAll: () => api.get('/inbound-ports'),
  add: (data: any) => api.post('/inbound-ports', data),
  update: (id: string, data: any) => api.put(`/inbound-ports/${id}`, data),
  delete: (id: string) => api.delete(`/inbound-ports/${id}`),
};

// 代理链路 API
export const proxyChainApi = {
  getAll: () => api.get('/proxy-chains'),
  add: (data: any) => api.post('/proxy-chains', data),
  update: (id: string, data: any) => api.put(`/proxy-chains/${id}`, data),
  delete: (id: string) => api.delete(`/proxy-chains/${id}`),
  // 健康检测
  getAllHealth: () => api.get('/proxy-chains/health'),
  getHealth: (id: string) => api.get(`/proxy-chains/${id}/health`),
  checkHealth: (id: string) => api.post(`/proxy-chains/${id}/health/check`),
  // 速度测试
  checkSpeed: (id: string) => api.post(`/proxy-chains/${id}/speed`),
};

// 内核管理 API
export const kernelApi = {
  getInfo: () => api.get('/kernel/info'),
  getReleases: () => api.get('/kernel/releases'),
  download: (version: string) => api.post('/kernel/download', { version }),
  getProgress: () => api.get('/kernel/progress'),
};

// 标签 API
export const tagApi = {
  // 标签管理
  getTags: () => api.get('/tags'),
  getTag: (id: number) => api.get(`/tags/${id}`),
  createTag: (data: any) => api.post('/tags', data),
  updateTag: (id: number, data: any) => api.put(`/tags/${id}`, data),
  deleteTag: (id: number) => api.delete(`/tags/${id}`),
  getGroups: () => api.get('/tags/groups'),
  // 标签规则
  getRules: () => api.get('/tag-rules'),
  getRule: (id: number) => api.get(`/tag-rules/${id}`),
  createRule: (data: any) => api.post('/tag-rules', data),
  updateRule: (id: number, data: any) => api.put(`/tag-rules/${id}`, data),
  deleteRule: (id: number) => api.delete(`/tag-rules/${id}`),
  // 节点标签
  getNodeTags: (nodeId: number) => api.get(`/nodes/${nodeId}/tags`),
  setNodeTags: (nodeId: number, tagIds: number[]) => api.put(`/nodes/${nodeId}/tags`, { tag_ids: tagIds }),
  getNodesByTag: (tagId: number) => api.get(`/tags/${tagId}/nodes`),
  // 应用规则
  applyRules: (triggerType: string, nodeIds?: number[]) =>
    api.post('/tags/apply-rules', { trigger_type: triggerType, node_ids: nodeIds }),
};

// 测速 API
export const speedtestApi = {
  // 策略管理
  getProfiles: () => api.get('/speedtest/profiles'),
  getProfile: (id: number) => api.get(`/speedtest/profiles/${id}`),
  createProfile: (data: any) => api.post('/speedtest/profiles', data),
  updateProfile: (id: number, data: any) => api.put(`/speedtest/profiles/${id}`, data),
  deleteProfile: (id: number) => api.delete(`/speedtest/profiles/${id}`),
  // 任务执行
  runTest: (profileId?: number, nodeIds?: number[]) =>
    api.post('/speedtest/run', { profile_id: profileId, node_ids: nodeIds }),
  getTasks: (limit?: number) => api.get('/speedtest/tasks', { params: { limit } }),
  getTask: (id: string) => api.get(`/speedtest/tasks/${id}`),
  cancelTask: (id: string) => api.post(`/speedtest/tasks/${id}/cancel`),
  // 历史记录
  getNodeHistory: (nodeId: number, limit?: number) =>
    api.get(`/speedtest/nodes/${nodeId}/history`, { params: { limit } }),
};

export default api;
