import { create } from 'zustand';
import type { Task, TaskStats } from '../api/tasks';
import { getTasks, getTask, cancelTask, getRunningTasks, getTaskStats, cleanupTasks } from '../api/tasks';

interface TaskState {
  tasks: Task[];
  runningTasks: Task[];
  stats: TaskStats | null;
  loading: boolean;
  error: string | null;

  fetchTasks: (params?: { limit?: number; offset?: number; type?: string; status?: string }) => Promise<void>;
  fetchTask: (id: string) => Promise<Task | null>;
  fetchRunningTasks: () => Promise<void>;
  fetchStats: () => Promise<void>;
  cancelTask: (id: string) => Promise<void>;
  cleanupTasks: (days?: number) => Promise<void>;
}

export const useTaskStore = create<TaskState>((set, get) => ({
  tasks: [],
  runningTasks: [],
  stats: null,
  loading: false,
  error: null,

  fetchTasks: async (params) => {
    set({ loading: true, error: null });
    try {
      const tasks = await getTasks(params);
      set({ tasks, loading: false });
    } catch (err) {
      set({ error: (err as Error).message, loading: false });
    }
  },

  fetchTask: async (id) => {
    try {
      return await getTask(id);
    } catch {
      return null;
    }
  },

  fetchRunningTasks: async () => {
    try {
      const runningTasks = await getRunningTasks();
      set({ runningTasks });
    } catch (err) {
      console.error('获取运行中任务失败:', err);
    }
  },

  fetchStats: async () => {
    try {
      const stats = await getTaskStats();
      set({ stats });
    } catch (err) {
      console.error('获取任务统计失败:', err);
    }
  },

  cancelTask: async (id) => {
    try {
      await cancelTask(id);
      // 刷新任务列表
      await get().fetchTasks();
      await get().fetchRunningTasks();
      await get().fetchStats();
    } catch (err) {
      set({ error: (err as Error).message });
    }
  },

  cleanupTasks: async (days) => {
    try {
      await cleanupTasks(days);
      await get().fetchTasks();
      await get().fetchStats();
    } catch (err) {
      set({ error: (err as Error).message });
    }
  },
}));
