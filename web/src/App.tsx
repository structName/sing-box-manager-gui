import { useEffect } from 'react';
import { BrowserRouter, Route, Routes } from 'react-router-dom';
import { ProtectedRoute, PublicAuthRoute } from './components/ProtectedRoute';
import Dashboard from './pages/Dashboard';
import Subscriptions from './pages/Subscriptions';
import Profiles from './pages/Profiles';
import ProxyChains from './pages/ProxyChains';
import Deployments from './pages/Deployments';
import InboundPorts from './pages/InboundPorts';
import Tasks from './pages/Tasks';
import Tags from './pages/Tags';
import Settings from './pages/Settings';
import Logs from './pages/Logs';
import { ToastContainer } from './components/Toast';
import Login from './pages/Login';
import Setup from './pages/Setup';
import { registerAuthErrorHandlers, registerConfigSkipHandler } from './api';
import { useAuthStore } from './store/authStore';
import { useStore } from './store';
import { toast } from './components/Toast';

function App() {
  const fetchSession = useAuthStore((state) => state.fetchSession);
  const markUnauthenticated = useAuthStore((state) => state.markUnauthenticated);
  const markBootstrapRequired = useAuthStore((state) => state.markBootstrapRequired);

  useEffect(() => {
    registerAuthErrorHandlers({
      onUnauthorized: markUnauthenticated,
      onBootstrapRequired: markBootstrapRequired,
    });
    registerConfigSkipHandler((payload, options) => {
      useStore.getState().noteConfigSkip(payload);
      if (!options.toast) {
        return;
      }
      const warning = useStore.getState().configSkipWarning;
      if (warning) {
        toast.info(warning, 8000);
      }
    });
    void fetchSession();
  }, [fetchSession, markBootstrapRequired, markUnauthenticated]);

  return (
    <BrowserRouter>
      <ToastContainer />
      <Routes>
        <Route
          path="/login"
          element={(
            <PublicAuthRoute mode="login">
              <Login />
            </PublicAuthRoute>
          )}
        />
        <Route
          path="/setup"
          element={(
            <PublicAuthRoute mode="setup">
              <Setup />
            </PublicAuthRoute>
          )}
        />
        <Route element={<ProtectedRoute />}>
          <Route path="/" element={<Dashboard />} />
          <Route path="/subscriptions" element={<Subscriptions />} />
          <Route path="/inbound-ports" element={<InboundPorts />} />
          <Route path="/profiles" element={<Profiles />} />
          <Route path="/proxy-chains" element={<ProxyChains />} />
          <Route path="/deployments" element={<Deployments />} />
          <Route path="/tasks" element={<Tasks />} />
          <Route path="/tags" element={<Tags />} />
          <Route path="/logs" element={<Logs />} />
          <Route path="/settings" element={<Settings />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}

export default App;
