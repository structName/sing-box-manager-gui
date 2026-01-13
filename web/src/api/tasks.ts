import api from './index';

export interface Task {
  id: string;
  type: string;
  name: string;
  status: 'pending' | 'running' | 'completed' | 'cancelled' | 'error';
  trigger: string;
  progress: number;
  total: number;
  current_item?: string;
  message?: string;
  result?: Record<string, unknown>;
  started_at?: string;
  completed_at?: string;
  created_at: string;
}

export interface TaskStats {
  total: number;
  running: number;
  pending: number;
  completed: number;
  failed: number;
}

// 获取任务列表
export const getTasks = async (params?: {
  limit?: number;
  offset?: number;
  type?: string;
  status?: string;
}): Promise<Task[]> => {
  const response = await api.get('/tasks', { params });
  return response.data.data;
};

// 获取单个任务
export const getTask = async (id: string): Promise<Task> => {
  const response = await api.get(`/tasks/${id}`);
  return response.data.data;
};

// 取消任务
export const cancelTask = async (id: string): Promise<void> => {
  await api.post(`/tasks/${id}/cancel`);
};

// 获取运行中的任务
export const getRunningTasks = async (): Promise<Task[]> => {
  const response = await api.get('/tasks/running');
  return response.data.data;
};

// 获取任务统计
export const getTaskStats = async (): Promise<TaskStats> => {
  const response = await api.get('/tasks/stats');
  return response.data.data;
};

// 清理历史任务
export const cleanupTasks = async (days?: number): Promise<void> => {
  await api.delete('/tasks/history', { params: { days } });
};

// SSE 任务事件流
export const subscribeTaskEvents = (
  onTask: (task: Task) => void,
  onError?: (error: Event) => void
): (() => void) => {
  const baseURL = api.defaults.baseURL || '';
  const eventSource = new EventSource(`${baseURL}/events/stream`);

  eventSource.onmessage = (event) => {
    try {
      const task = JSON.parse(event.data) as Task;
      onTask(task);
    } catch (e) {
      console.error('解析任务事件失败:', e);
    }
  };

  eventSource.onerror = (error) => {
    console.error('SSE 连接错误:', error);
    onError?.(error);
  };

  return () => eventSource.close();
};
