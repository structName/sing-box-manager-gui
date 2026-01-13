import { create } from 'zustand';
import type { Task, TaskStats } from '../api/tasks';
import { getTasks, getTask, cancelTask, getRunningTasks, getTaskStats, cleanupTasks, subscribeTaskEvents } from '../api/tasks';

interface TaskState {
  tasks: Task[];
  runningTasks: Task[];
  stats: TaskStats | null;
  loading: boolean;
  error: string | null;
  sseConnected: boolean;
  unsubscribe: (() => void) | null;

  fetchTasks: (params?: { limit?: number; offset?: number; type?: string; status?: string }) => Promise<void>;
  fetchTask: (id: string) => Promise<Task | null>;
  fetchRunningTasks: () => Promise<void>;
  fetchStats: () => Promise<void>;
  cancelTask: (id: string) => Promise<void>;
  cleanupTasks: (days?: number) => Promise<void>;
  subscribeSSE: () => void;
  unsubscribeSSE: () => void;
  updateTaskFromSSE: (task: Task) => void;
}

export const useTaskStore = create<TaskState>((set, get) => ({
  tasks: [],
  runningTasks: [],
  stats: null,
  loading: false,
  error: null,
  sseConnected: false,
  unsubscribe: null,

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

  subscribeSSE: () => {
    const { unsubscribe, sseConnected } = get();
    if (sseConnected || unsubscribe) return;

    const unsub = subscribeTaskEvents(
      (task) => get().updateTaskFromSSE(task),
      () => set({ sseConnected: false })
    );
    set({ unsubscribe: unsub, sseConnected: true });
  },

  unsubscribeSSE: () => {
    const { unsubscribe } = get();
    if (unsubscribe) {
      unsubscribe();
      set({ unsubscribe: null, sseConnected: false });
    }
  },

  updateTaskFromSSE: (task) => {
    set((state) => {
      // 更新 tasks 列表
      const taskIndex = state.tasks.findIndex((t) => t.id === task.id);
      let newTasks: Task[];
      if (taskIndex >= 0) {
        newTasks = [...state.tasks];
        newTasks[taskIndex] = task;
      } else {
        newTasks = [task, ...state.tasks];
      }

      // 更新 runningTasks
      const isRunning = task.status === 'running' || task.status === 'pending';
      let newRunningTasks: Task[];
      const runningIndex = state.runningTasks.findIndex((t) => t.id === task.id);
      if (isRunning) {
        if (runningIndex >= 0) {
          newRunningTasks = [...state.runningTasks];
          newRunningTasks[runningIndex] = task;
        } else {
          newRunningTasks = [task, ...state.runningTasks];
        }
      } else {
        newRunningTasks = state.runningTasks.filter((t) => t.id !== task.id);
      }

      // 更新统计
      const newStats = state.stats ? { ...state.stats } : { total: 0, running: 0, pending: 0, completed: 0, failed: 0 };
      if (taskIndex < 0) newStats.total++;
      // 简单更新 running 计数
      newStats.running = newRunningTasks.filter((t) => t.status === 'running').length;
      newStats.pending = newRunningTasks.filter((t) => t.status === 'pending').length;

      return { tasks: newTasks, runningTasks: newRunningTasks, stats: newStats };
    });
  },
}));
