import { BrowserRouter, Routes, Route } from 'react-router-dom';
import Layout from './components/Layout';
import Dashboard from './pages/Dashboard';
import Subscriptions from './pages/Subscriptions';
import Rules from './pages/Rules';
import Profiles from './pages/Profiles';
import ProxyChains from './pages/ProxyChains';
import InboundPorts from './pages/InboundPorts';
import Tasks from './pages/Tasks';
import Tags from './pages/Tags';
import Settings from './pages/Settings';
import Logs from './pages/Logs';
import { ToastContainer } from './components/Toast';

function App() {
  return (
    <BrowserRouter>
      <ToastContainer />
      <Layout>
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/subscriptions" element={<Subscriptions />} />
          <Route path="/inbound-ports" element={<InboundPorts />} />
          <Route path="/rules" element={<Rules />} />
          <Route path="/profiles" element={<Profiles />} />
          <Route path="/proxy-chains" element={<ProxyChains />} />
          <Route path="/tasks" element={<Tasks />} />
          <Route path="/tags" element={<Tags />} />
          <Route path="/logs" element={<Logs />} />
          <Route path="/settings" element={<Settings />} />
        </Routes>
      </Layout>
    </BrowserRouter>
  );
}

export default App;
