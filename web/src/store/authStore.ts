import { create } from 'zustand';
import { authApi, type AuthStatus } from '../api/auth';

interface AuthState extends AuthStatus {
  checking: boolean;
  fetchSession: () => Promise<void>;
  bootstrap: (password: string, confirmPassword: string) => Promise<void>;
  login: (password: string) => Promise<void>;
  logout: () => Promise<void>;
  markUnauthenticated: () => void;
  markBootstrapRequired: () => void;
}

const unauthenticatedState: AuthStatus = {
  bootstrapped: true,
  authenticated: false,
};

export const useAuthStore = create<AuthState>((set) => ({
  ...unauthenticatedState,
  checking: true,

  fetchSession: async () => {
    set({ checking: true });
    try {
      const status = await authApi.me();
      set({ ...status, checking: false });
    } catch {
      set({ ...unauthenticatedState, checking: false });
    }
  },

  bootstrap: async (password: string, confirmPassword: string) => {
    const status = await authApi.bootstrap(password, confirmPassword);
    set({ ...status, checking: false });
  },

  login: async (password: string) => {
    const status = await authApi.login(password);
    set({ ...status, checking: false });
  },

  logout: async () => {
    await authApi.logout();
    set({ ...unauthenticatedState, checking: false });
  },

  markUnauthenticated: () => {
    set({ ...unauthenticatedState, checking: false });
  },

  markBootstrapRequired: () => {
    set({ bootstrapped: false, authenticated: false, checking: false });
  },
}));
