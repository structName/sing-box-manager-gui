import { Button, Chip } from '@nextui-org/react';
import { LockKeyhole, Save, Server, ShieldCheck } from 'lucide-react';

interface SettingsHeroProps {
  hostCount: number;
  zashboardEnabled: boolean;
  hasSecret: boolean;
  kernelInstalled: boolean;
  onSave: () => void;
}

function StatCard({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <div className="rounded-xl border border-slate-200/60 bg-white px-4 py-3 dark:border-slate-700 dark:bg-slate-800">
      <div className="mb-1.5 flex items-center gap-2 text-xs font-medium uppercase tracking-widest text-slate-400 dark:text-slate-500">
        {icon}
        {label}
      </div>
      <p className="text-sm font-semibold text-slate-900 dark:text-white">{value}</p>
    </div>
  );
}

export function SettingsHero({ hostCount, zashboardEnabled, hasSecret, kernelInstalled, onSave }: SettingsHeroProps) {
  return (
    <section className="rounded-2xl border border-slate-200/60 bg-gradient-to-br from-white to-slate-50 p-6 shadow-sm dark:border-slate-800 dark:from-slate-900 dark:to-slate-900/80">
      <div className="flex flex-col gap-6 xl:flex-row xl:items-end xl:justify-between">
        <div className="space-y-3">
          <div className="flex flex-wrap items-center gap-2">
            <Chip size="sm" variant="flat" color="primary">Settings</Chip>
            <Chip size="sm" variant="flat" color={zashboardEnabled ? 'success' : 'default'}>
              {zashboardEnabled ? 'Zashboard 已启用' : 'Zashboard 已关闭'}
            </Chip>
          </div>
          <div className="space-y-1.5">
            <h1 className="text-2xl font-semibold tracking-tight text-slate-900 dark:text-white">
              设置工作台
            </h1>
            <p className="max-w-2xl text-sm leading-relaxed text-slate-500 dark:text-slate-400">
              核心路径、控制面板、DNS 解析、Hosts 覆盖和服务运维，按运行链路统一管理。
            </p>
          </div>
        </div>

        <div className="flex flex-col gap-4 xl:items-end">
          <div className="grid gap-3 sm:grid-cols-3">
            <StatCard icon={<ShieldCheck className="h-3.5 w-3.5" />} label="核心" value={kernelInstalled ? '已就绪' : '待安装'} />
            <StatCard icon={<LockKeyhole className="h-3.5 w-3.5" />} label="鉴权" value={hasSecret ? 'Bearer Secret' : '待初始化'} />
            <StatCard icon={<Server className="h-3.5 w-3.5" />} label="Hosts" value={`${hostCount} 条映射`} />
          </div>
          <Button color="primary" size="lg" startContent={<Save className="h-4 w-4" />} onPress={onSave}>
            保存设置
          </Button>
        </div>
      </div>
    </section>
  );
}
