import { create } from 'zustand';
import {
  type ScheduleEntry,
  type SchedulerStatus,
  getSchedulerStatus,
  getSchedulerEntries,
  enableScheduleEntry,
  disableScheduleEntry,
  triggerScheduleEntry,
  pauseScheduler,
  resumeScheduler,
} from '../api/scheduler';

interface SchedulerStore {
  status: SchedulerStatus | null;
  entries: ScheduleEntry[];
  loading: boolean;

  fetchStatus: () => Promise<void>;
  fetchEntries: (type?: string) => Promise<void>;
  enableEntry: (key: string) => Promise<void>;
  disableEntry: (key: string) => Promise<void>;
  triggerEntry: (key: string) => Promise<void>;
  pause: () => Promise<void>;
  resume: () => Promise<void>;
}

export const useSchedulerStore = create<SchedulerStore>((set, get) => ({
  status: null,
  entries: [],
  loading: false,

  fetchStatus: async () => {
    try {
      const status = await getSchedulerStatus();
      set({ status });
    } catch (error) {
      console.error('获取调度器状态失败:', error);
    }
  },

  fetchEntries: async (type?: string) => {
    set({ loading: true });
    try {
      const entries = await getSchedulerEntries(type);
      set({ entries });
    } catch (error) {
      console.error('获取调度条目失败:', error);
    } finally {
      set({ loading: false });
    }
  },

  enableEntry: async (key: string) => {
    await enableScheduleEntry(key);
    await get().fetchEntries();
    await get().fetchStatus();
  },

  disableEntry: async (key: string) => {
    await disableScheduleEntry(key);
    await get().fetchEntries();
    await get().fetchStatus();
  },

  triggerEntry: async (key: string) => {
    await triggerScheduleEntry(key);
  },

  pause: async () => {
    await pauseScheduler();
    await get().fetchStatus();
  },

  resume: async () => {
    await resumeScheduler();
    await get().fetchStatus();
  },
}));
