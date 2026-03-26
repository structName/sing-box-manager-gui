import { Button, Card, CardBody, CardHeader, Chip, Input, Switch } from '@nextui-org/react';
import { AlertCircle, CheckCircle, Download, LockKeyhole, RadioTower, Terminal } from 'lucide-react';
import type { Settings as SettingsType } from '../../store';
import type { KernelInfo } from './types';

interface FieldHandlers {
  onValueChange: <K extends keyof SettingsType>(field: K, value: SettingsType[K]) => void;
  onNumberChange: (field: 'clash_api_port', value: string) => void;
}

interface CoreSettingsCardProps {
  formData: SettingsType;
  kernelInfo: KernelInfo | null;
  onValueChange: <K extends keyof SettingsType>(field: K, value: SettingsType[K]) => void;
  onDownloadKernel: () => void;
}

interface ControlPanelCardProps extends FieldHandlers {
  formData: SettingsType;
  setupUrl: string;
}

interface AutomationCardProps {
  autoApply: boolean;
  onToggle: (enabled: boolean) => void;
}

function KernelStatusBanner({ kernelInfo, onDownloadKernel }: Pick<CoreSettingsCardProps, 'kernelInfo' | 'onDownloadKernel'>) {
  const installed = Boolean(kernelInfo?.installed);

  return (
    <div className={`rounded-xl border p-4 ${
      installed
        ? 'border-emerald-200 bg-emerald-50/80 dark:border-emerald-900/40 dark:bg-emerald-950/20'
        : 'border-amber-200 bg-amber-50/80 dark:border-amber-900/40 dark:bg-amber-950/20'
    }`}>
      <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div className="flex items-center gap-3">
          {installed ? (
            <div className="rounded-lg bg-emerald-100 p-2 dark:bg-emerald-900/30">
              <CheckCircle className="h-5 w-5 text-emerald-600 dark:text-emerald-400" />
            </div>
          ) : (
            <div className="rounded-lg bg-amber-100 p-2 dark:bg-amber-900/30">
              <AlertCircle className="h-5 w-5 text-amber-600 dark:text-amber-400" />
            </div>
          )}
          <div>
            <p className="text-xs font-medium uppercase tracking-widest text-slate-500 dark:text-slate-400">Kernel</p>
            <p className="text-sm font-semibold text-slate-900 dark:text-white">
              {installed ? `sing-box ${kernelInfo?.version || '已安装'}` : '尚未安装 sing-box 内核'}
            </p>
            <p className="text-xs text-slate-500 dark:text-slate-400">
              {installed
                ? `平台 ${kernelInfo?.os}/${kernelInfo?.arch} · 路径 ${kernelInfo?.path || '未记录'}`
                : '下载适配当前系统的版本后，即可由图形界面生成并应用配置。'}
            </p>
          </div>
        </div>
        <Button color="primary" variant={installed ? 'flat' : 'solid'} size="sm" startContent={<Download className="h-4 w-4" />} onPress={onDownloadKernel}>
          {installed ? '升级内核' : '下载内核'}
        </Button>
      </div>
    </div>
  );
}

export function CoreSettingsCard({ formData, kernelInfo, onValueChange, onDownloadKernel }: CoreSettingsCardProps) {
  return (
    <Card className="border border-slate-200/60 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900">
      <CardHeader className="flex items-start gap-3">
        <div className="rounded-xl bg-slate-100 p-2.5 text-slate-600 dark:bg-slate-800 dark:text-slate-300">
          <Terminal className="h-5 w-5" />
        </div>
        <div>
          <h2 className="text-lg font-semibold text-slate-900 dark:text-white">核心路径</h2>
          <p className="text-sm text-slate-500 dark:text-slate-400">统一管理内核位置、生成文件与远程规则源。</p>
        </div>
      </CardHeader>
      <CardBody className="space-y-4">
        <KernelStatusBanner kernelInfo={kernelInfo} onDownloadKernel={onDownloadKernel} />
        <div className="grid gap-4 md:grid-cols-2">
          <Input label="sing-box 可执行文件" value={formData.singbox_path} onChange={(event) => onValueChange('singbox_path', event.target.value)} />
          <Input label="生成配置路径" value={formData.config_path} onChange={(event) => onValueChange('config_path', event.target.value)} />
          <Input label="GitHub 代理" placeholder="https://ghproxy.com/" value={formData.github_proxy} onChange={(event) => onValueChange('github_proxy', event.target.value)} />
          <Input label="规则集基础地址" value={formData.ruleset_base_url} onChange={(event) => onValueChange('ruleset_base_url', event.target.value)} />
        </div>
      </CardBody>
    </Card>
  );
}

export function ControlPanelCard({ formData, onValueChange, onNumberChange, setupUrl }: ControlPanelCardProps) {
  const zashboardEnabled = formData.clash_ui_enabled;
  const hasSecret = Boolean(formData.clash_api_secret);

  return (
    <Card className="border border-slate-200/60 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900">
      <CardHeader className="flex items-start gap-3">
        <div className="rounded-xl bg-cyan-50 p-2.5 text-cyan-600 dark:bg-cyan-950/30 dark:text-cyan-400">
          <RadioTower className="h-5 w-5" />
        </div>
        <div className="space-y-1">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-lg font-semibold text-slate-900 dark:text-white">控制面板</h2>
            <Chip size="sm" variant="flat" color={zashboardEnabled ? 'success' : 'default'}>
              {zashboardEnabled ? 'Zashboard 已启用' : 'Zashboard 已关闭'}
            </Chip>
            <Chip size="sm" variant="flat" color={hasSecret ? 'primary' : 'warning'}>
              {hasSecret ? 'Secret 已就绪' : '等待生成密钥'}
            </Chip>
          </div>
          <p className="text-sm text-slate-500 dark:text-slate-400">为 zashboard 与 Clash API 定义端口、鉴权与面板入口。</p>
        </div>
      </CardHeader>
      <CardBody className="space-y-4">
        <div className="flex items-center justify-between rounded-xl bg-slate-50 p-4 dark:bg-slate-800/50">
          <div>
            <p className="font-medium text-slate-900 dark:text-white">启用 Zashboard</p>
            <p className="text-sm text-slate-500 dark:text-slate-400">关闭后保留 Clash API，但不再挂载外部面板资源。</p>
          </div>
          <Switch isSelected={zashboardEnabled} onValueChange={(enabled) => onValueChange('clash_ui_enabled', enabled)} />
        </div>
        <div className="grid gap-4">
          <Input isDisabled label="Web 管理端口" value={String(formData.web_port)} />
          <Input
            type="number"
            label="Clash API 端口"
            value={String(formData.clash_api_port)}
            onChange={(event) => onNumberChange('clash_api_port', event.target.value)}
          />
          <Input
            label="面板资源路径"
            value={formData.clash_ui_path}
            onChange={(event) => onValueChange('clash_ui_path', event.target.value)}
            isDisabled={!zashboardEnabled}
          />
          <Input
            label="漏网规则出站"
            value={formData.final_outbound}
            onChange={(event) => onValueChange('final_outbound', event.target.value)}
          />
        </div>
        <div className="rounded-xl border border-cyan-100 bg-cyan-50/50 p-4 dark:border-cyan-900/30 dark:bg-cyan-950/15">
          <div className="mb-2 flex items-center gap-2 text-sm font-medium text-cyan-700 dark:text-cyan-300">
            <LockKeyhole className="h-4 w-4" />
            连接预览
          </div>
          <p className="break-all font-mono text-xs leading-6 text-slate-600 dark:text-slate-300">
            {zashboardEnabled ? setupUrl : 'Zashboard 已关闭，保存并应用后不会再提供 /ui/ 面板入口。'}
          </p>
          <p className="mt-2 text-xs text-slate-500 dark:text-slate-400">
            Secret 在首次初始化时自动生成并持久化保存，设置页不再手工编辑。
          </p>
        </div>
      </CardBody>
    </Card>
  );
}

export function AutomationCard({ autoApply, onToggle }: AutomationCardProps) {
  return (
    <Card className="border border-slate-200/60 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900">
      <CardHeader>
        <div>
          <h2 className="text-lg font-semibold text-slate-900 dark:text-white">自动化</h2>
          <p className="text-sm text-slate-500 dark:text-slate-400">在配置变更后立即同步到 sing-box 运行实例。</p>
        </div>
      </CardHeader>
      <CardBody>
        <div className="flex items-center justify-between rounded-xl bg-slate-50 p-4 dark:bg-slate-800/50">
          <div>
            <p className="font-medium text-slate-900 dark:text-white">自动应用配置</p>
            <p className="text-sm text-slate-500 dark:text-slate-400">订阅刷新、规则变更和设置保存后自动重载。</p>
          </div>
          <Switch isSelected={autoApply} onValueChange={onToggle} />
        </div>
      </CardBody>
    </Card>
  );
}
