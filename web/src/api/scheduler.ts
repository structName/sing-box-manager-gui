import axios from './index';

// 调度条目类型
export interface ScheduleEntry {
  key: string;
  type: 'speed_test' | 'sub_update' | 'chain_check' | 'tag_rule' | 'config_watch';
  name: string;
  cron_expr: string;
  enabled: boolean;
  next_run: string | null;
  last_run: string | null;
}

// 调度器状态
export interface SchedulerStatus {
  running: boolean;
  entry_count: number;
  enabled: number;
  disabled: number;
}

// 获取调度器状态
export const getSchedulerStatus = async (): Promise<SchedulerStatus> => {
  const res = await axios.get('/scheduler/status');
  return res.data.data;
};

// 获取所有调度条目
export const getSchedulerEntries = async (type?: string): Promise<ScheduleEntry[]> => {
  const params = type ? { type } : {};
  const res = await axios.get('/scheduler/entries', { params });
  return res.data.data || [];
};

// 启用调度条目
export const enableScheduleEntry = async (key: string): Promise<void> => {
  await axios.post(`/scheduler/entries/${encodeURIComponent(key)}/enable`);
};

// 禁用调度条目
export const disableScheduleEntry = async (key: string): Promise<void> => {
  await axios.post(`/scheduler/entries/${encodeURIComponent(key)}/disable`);
};

// 立即触发调度
export const triggerScheduleEntry = async (key: string): Promise<void> => {
  await axios.post(`/scheduler/entries/${encodeURIComponent(key)}/trigger`);
};

// 暂停调度器
export const pauseScheduler = async (): Promise<void> => {
  await axios.post('/scheduler/pause');
};

// 恢复调度器
export const resumeScheduler = async (): Promise<void> => {
  await axios.post('/scheduler/resume');
};
