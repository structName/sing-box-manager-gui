import { useEffect } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { LayoutDashboard, Globe, FileText, Settings, Activity, ScrollText, Layers, Link2, Network } from 'lucide-react';
import { useStore } from '../store';

const menuItems = [
  { path: '/', icon: LayoutDashboard, label: '仪表盘' },
  { path: '/subscriptions', icon: Globe, label: '节点' },
  { path: '/inbound-ports', icon: Network, label: '入站' },
  { path: '/rules', icon: FileText, label: '规则' },
  { path: '/proxy-chains', icon: Link2, label: '链路' },
  { path: '/profiles', icon: Layers, label: '配置方案' },
  { path: '/logs', icon: ScrollText, label: '日志' },
  { path: '/settings', icon: Settings, label: '设置' },
];

interface LayoutProps {
  children: React.ReactNode;
}

export default function Layout({ children }: LayoutProps) {
  const location = useLocation();
  const { settings, fetchSettings, serviceStatus, fetchServiceStatus } = useStore();

  useEffect(() => {
    if (!settings) {
      fetchSettings();
    }
    if (!serviceStatus) {
      fetchServiceStatus();
    }
  }, []);

  const clashApiPort = settings?.clash_api_port || 9091;

  return (
    <div className="flex min-h-screen bg-gray-50 dark:bg-gray-900">
      {/* 侧边栏 */}
      <aside className="w-64 bg-white dark:bg-gray-800 border-r border-gray-200 dark:border-gray-700 fixed h-full overflow-y-auto">
        <div className="p-6">
          <h1 className="text-xl font-bold text-gray-800 dark:text-white flex items-center gap-2">
            <Activity className="w-6 h-6 text-primary" />
            SingBox Manager
          </h1>
        </div>

        <nav className="px-4">
          {menuItems.map((item) => {
            const isActive = location.pathname === item.path;
            const Icon = item.icon;

            return (
              <Link
                key={item.path}
                to={item.path}
                className={`flex items-center gap-3 px-4 py-3 rounded-lg mb-1 transition-colors ${
                  isActive
                    ? 'bg-primary text-white'
                    : 'text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700'
                }`}
              >
                <Icon className="w-5 h-5" />
                <span>{item.label}</span>
              </Link>
            );
          })}
        </nav>

        {/* 底部链接 */}
        <div className="sticky bottom-4 left-4 right-4 mt-auto pt-4">
          <a
            href={`http://127.0.0.1:${clashApiPort}/ui/`}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center justify-center gap-2 px-4 py-2 text-sm text-gray-500 dark:text-gray-400 hover:text-primary transition-colors"
          >
            打开 Zashboard
          </a>
          {serviceStatus?.sbm_version && (
            <p className="text-center text-xs text-gray-400 dark:text-gray-500 mt-2">
              v{serviceStatus.sbm_version}
            </p>
          )}
        </div>
      </aside>

      {/* 主内容区 */}
      <main className="flex-1 p-8 overflow-auto ml-64">
        {children}
      </main>
    </div>
  );
}
