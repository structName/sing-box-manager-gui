import { Button, Card, CardBody, CardHeader, Chip, Input, Switch } from '@nextui-org/react';
import { FileDown, FileUp, Pencil, Plus, Server, Trash2 } from 'lucide-react';
import type { ChangeEvent, RefObject } from 'react';
import type { HostEntry, Settings as SettingsType } from '../../store';
import type { DaemonStatus } from './types';

interface DnsSettingsCardProps {
  formData: SettingsType;
  onValueChange: <K extends keyof SettingsType>(field: K, value: SettingsType[K]) => void;
}

interface HostsCardProps {
  customHosts: HostEntry[];
  systemHosts: HostEntry[];
  onAddHost: () => void;
  onEditHost: (host: HostEntry) => void;
  onDeleteHost: (id: string) => void;
  onToggleHost: (id: string, enabled: boolean) => void;
}

interface BackupCardProps {
  isRestoring: boolean;
  backupInputRef: RefObject<HTMLInputElement | null>;
  onExport: () => void;
  onImport: () => void;
  onFileChange: (event: ChangeEvent<HTMLInputElement>) => void;
}

interface DaemonCardProps {
  daemonStatus: DaemonStatus | null;
  onInstall: () => void;
  onRestart: () => void;
  onUninstall: () => void;
}

function HostRow({
  host,
  readonly,
  onEdit,
  onDelete,
  onToggle,
}: {
  host: HostEntry;
  readonly?: boolean;
  onEdit?: (host: HostEntry) => void;
  onDelete?: (id: string) => void;
  onToggle?: (id: string, enabled: boolean) => void;
}) {
  return (
    <div className="rounded-xl border border-slate-200/60 bg-white p-4 dark:border-slate-800 dark:bg-slate-800/40">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div className="space-y-2.5">
          <div className="flex flex-wrap items-center gap-2">
            <Server className="h-4 w-4 text-slate-400" />
            <span className="font-medium text-slate-900 dark:text-white">{host.domain}</span>
            <Chip size="sm" variant="flat" color={readonly ? 'secondary' : host.enabled ? 'success' : 'default'}>
              {readonly ? '系统' : host.enabled ? '启用' : '停用'}
            </Chip>
          </div>
          <div className="flex flex-wrap gap-1.5">
            {host.ips.map((ip) => (
              <Chip key={`${host.id}-${ip}`} size="sm" variant="bordered">
                {ip}
              </Chip>
            ))}
          </div>
        </div>
        {!readonly && onToggle && onDelete && onEdit && (
          <div className="flex flex-wrap items-center gap-2">
            <Switch isSelected={host.enabled} onValueChange={(enabled) => onToggle(host.id, enabled)} />
            <Button isIconOnly variant="flat" onPress={() => onEdit(host)}>
              <Pencil className="h-4 w-4" />
            </Button>
            <Button isIconOnly color="danger" variant="flat" onPress={() => onDelete(host.id)}>
              <Trash2 className="h-4 w-4" />
            </Button>
          </div>
        )}
      </div>
    </div>
  );
}

export function DnsSettingsCard({ formData, onValueChange }: DnsSettingsCardProps) {
  return (
    <Card className="border border-slate-200/60 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900">
      <CardHeader>
        <div>
          <h2 className="text-lg font-semibold text-slate-900 dark:text-white">DNS 与解析</h2>
          <p className="text-sm text-slate-500 dark:text-slate-400">控制代理 DNS、直连 DNS 与 FakeIP 行为。</p>
        </div>
      </CardHeader>
      <CardBody className="space-y-4">
        <div className="grid gap-4 md:grid-cols-2">
          <Input label="代理 DNS" value={formData.proxy_dns} onChange={(event) => onValueChange('proxy_dns', event.target.value)} />
          <Input label="直连 DNS" value={formData.direct_dns} onChange={(event) => onValueChange('direct_dns', event.target.value)} />
        </div>
        <div className="flex items-center justify-between rounded-xl bg-slate-50 p-4 dark:bg-slate-800/50">
          <div>
            <p className="font-medium text-slate-900 dark:text-white">启用 FakeIP</p>
            <p className="text-sm text-slate-500 dark:text-slate-400">在代理场景中减少真实 DNS 暴露，并持久化映射缓存。</p>
          </div>
          <Switch isSelected={Boolean(formData.fakeip_enabled)} onValueChange={(enabled) => onValueChange('fakeip_enabled', enabled)} />
        </div>
      </CardBody>
    </Card>
  );
}

export function HostsCard({
  customHosts,
  systemHosts,
  onAddHost,
  onEditHost,
  onDeleteHost,
  onToggleHost,
}: HostsCardProps) {
  return (
    <Card className="border border-slate-200/60 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900">
      <CardHeader className="flex items-start justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold text-slate-900 dark:text-white">Hosts 映射</h2>
          <p className="text-sm text-slate-500 dark:text-slate-400">将本地覆盖与系统 hosts 并列展示，便于核对解析来源。</p>
        </div>
        <Button color="primary" variant="flat" startContent={<Plus className="h-4 w-4" />} onPress={onAddHost}>
          添加映射
        </Button>
      </CardHeader>
      <CardBody className="space-y-4">
        {customHosts.length > 0 && (
          <div className="space-y-3">
            <div className="flex items-center gap-2">
              <p className="text-sm font-medium text-slate-700 dark:text-slate-200">自定义 hosts</p>
              <Chip size="sm" variant="flat">{customHosts.length}</Chip>
            </div>
            {customHosts.map((host) => (
              <HostRow key={host.id} host={host} onEdit={onEditHost} onDelete={onDeleteHost} onToggle={onToggleHost} />
            ))}
          </div>
        )}

        {systemHosts.length > 0 && (
          <div className="space-y-3">
            <div className="flex items-center gap-2">
              <p className="text-sm font-medium text-slate-700 dark:text-slate-200">系统 hosts</p>
              <Chip size="sm" color="secondary" variant="flat">只读</Chip>
            </div>
            {systemHosts.map((host) => (
              <HostRow key={host.id} host={host} readonly />
            ))}
          </div>
        )}

        {customHosts.length === 0 && systemHosts.length === 0 && (
          <div className="rounded-xl border border-dashed border-slate-300 p-8 text-center text-sm text-slate-500 dark:border-slate-700 dark:text-slate-400">
            当前没有任何 hosts 映射。
          </div>
        )}
      </CardBody>
    </Card>
  );
}

export function BackupCard({
  isRestoring,
  backupInputRef,
  onExport,
  onImport,
  onFileChange,
}: BackupCardProps) {
  return (
    <Card className="border border-slate-200/60 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900">
      <CardHeader>
        <div>
          <h2 className="text-lg font-semibold text-slate-900 dark:text-white">备份与恢复</h2>
          <p className="text-sm text-slate-500 dark:text-slate-400">完整导出订阅、规则、配置方案和当前全局设置。</p>
        </div>
      </CardHeader>
      <CardBody className="space-y-4">
        <div className="rounded-xl bg-slate-50 p-4 dark:bg-slate-800/50">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <p className="font-medium text-slate-900 dark:text-white">应用数据备份</p>
              <p className="text-sm text-slate-500 dark:text-slate-400">导入会覆盖当前工作集，适合迁移或回滚。</p>
            </div>
            <div className="flex gap-2">
              <Button variant="flat" startContent={<FileUp className="h-4 w-4" />} onPress={onImport} isLoading={isRestoring}>
                恢复
              </Button>
              <Button color="primary" startContent={<FileDown className="h-4 w-4" />} onPress={onExport}>
                备份
              </Button>
            </div>
          </div>
        </div>
        <input ref={backupInputRef} type="file" accept=".json" className="hidden" onChange={onFileChange} />
      </CardBody>
    </Card>
  );
}

export function DaemonCard({ daemonStatus, onInstall, onRestart, onUninstall }: DaemonCardProps) {
  if (!daemonStatus?.supported) {
    return null;
  }

  return (
    <Card className="border border-slate-200/60 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900">
      <CardHeader className="flex items-start justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold text-slate-900 dark:text-white">后台服务</h2>
          <p className="text-sm text-slate-500 dark:text-slate-400">保持 sbm 常驻运行，并在系统启动时自动恢复。</p>
        </div>
        <Chip size="sm" variant="flat" color={daemonStatus.installed ? 'success' : 'default'}>
          {daemonStatus.installed ? '已安装' : '未安装'}
        </Chip>
      </CardHeader>
      <CardBody className="space-y-4">
        <div className="rounded-xl bg-slate-50 p-4 dark:bg-slate-800/50">
          <p className="text-sm leading-6 text-slate-600 dark:text-slate-300">
            服务模式会在关闭终端后继续保留 Web 管理界面，并在崩溃后自动拉起。
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          {daemonStatus.installed ? (
            <>
              <Button color="primary" variant="flat" onPress={onRestart}>
                重启服务
              </Button>
              <Button color="danger" variant="flat" onPress={onUninstall}>
                卸载服务
              </Button>
            </>
          ) : (
            <Button color="primary" onPress={onInstall}>
              安装后台服务
            </Button>
          )}
        </div>
      </CardBody>
    </Card>
  );
}
