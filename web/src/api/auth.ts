import api from './index';

export interface AuthStatus {
  bootstrapped: boolean;
  authenticated: boolean;
}

export const authApi = {
  me: async () => {
    const response = await api.get<{ data: AuthStatus }>('/auth/me');
    return response.data.data;
  },
  bootstrap: async (password: string, confirmPassword: string) => {
    const response = await api.post<{ data: AuthStatus }>('/auth/bootstrap', {
      password,
      confirm_password: confirmPassword,
    });
    return response.data.data;
  },
  login: async (password: string) => {
    const response = await api.post<{ data: AuthStatus }>('/auth/login', { password });
    return response.data.data;
  },
  logout: () => api.post('/auth/logout'),
};
