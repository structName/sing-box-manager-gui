import { Navigate, Outlet, useLocation } from 'react-router-dom';
import Layout from './Layout';
import { useAuthStore } from '../store/authStore';

function FullScreenMessage({ message }: { message: string }) {
  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-50 dark:bg-gray-900">
      <div className="rounded-2xl border border-slate-200/70 bg-white px-6 py-5 text-sm text-slate-600 shadow-sm dark:border-slate-800 dark:bg-slate-900 dark:text-slate-300">
        {message}
      </div>
    </div>
  );
}

export function ProtectedRoute() {
  const location = useLocation();
  const { checking, bootstrapped, authenticated } = useAuthStore();

  if (checking) {
    return <FullScreenMessage message="正在校验登录状态" />;
  }

  if (!bootstrapped) {
    return <Navigate to="/setup" replace state={{ from: location }} />;
  }

  if (!authenticated) {
    return <Navigate to="/login" replace state={{ from: location }} />;
  }

  return (
    <Layout>
      <Outlet />
    </Layout>
  );
}

export function PublicAuthRoute({ mode, children }: { mode: 'login' | 'setup'; children: React.ReactNode }) {
  const { checking, bootstrapped, authenticated } = useAuthStore();

  if (checking) {
    return <FullScreenMessage message="正在校验登录状态" />;
  }

  if (authenticated) {
    return <Navigate to="/" replace />;
  }

  if (!bootstrapped && mode === 'login') {
    return <Navigate to="/setup" replace />;
  }

  if (bootstrapped && mode === 'setup') {
    return <Navigate to="/login" replace />;
  }

  return <>{children}</>;
}
