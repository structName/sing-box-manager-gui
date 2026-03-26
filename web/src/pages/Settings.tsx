import { Spinner } from '@nextui-org/react';
import type { Settings as SettingsType } from '../store';
import { buildZashboardSetupPreviewUrl } from '../utils/zashboard';
import {
  AutomationCard,
  ControlPanelCard,
  CoreSettingsCard,
} from './settings/GeneralSettingsCards';
import {
  BackupCard,
  DaemonCard,
  DnsSettingsCard,
  HostsCard,
} from './settings/HostsAndOpsCards';
import { SettingsHero } from './settings/SettingsHero';
import { HostEditorModal, KernelDownloadModal } from './settings/SettingsModals';
import { useBackupManager } from './settings/useBackupManager';
import { useDaemonManager } from './settings/useDaemonManager';
import { useHostsManager } from './settings/useHostsManager';
import { useKernelManager } from './settings/useKernelManager';
import { useSettingsForm } from './settings/useSettingsForm';

const DEFAULT_CLASH_API_PORT = 9091;

function LoadingState() {
  return (
    <div className="flex min-h-[40vh] items-center justify-center">
      <div className="flex items-center gap-3 rounded-xl border border-slate-200/60 bg-white px-5 py-4 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <Spinner size="sm" />
        <span className="text-sm text-slate-600 dark:text-slate-300">正在加载设置工作台</span>
      </div>
    </div>
  );
}

export default function Settings() {
  const { formData, setFormData, handleSave } = useSettingsForm();
  const { daemonStatus, installDaemon, restartDaemon, uninstallDaemon } = useDaemonManager();
  const {
    kernelInfo,
    releases,
    selectedVersion,
    showDownloadModal,
    downloading,
    downloadProgress,
    setSelectedVersion,
    openDownloadModal,
    closeDownloadModal,
    startDownload,
  } = useKernelManager();
  const {
    systemHosts,
    isHostModalOpen,
    editingHost,
    hostFormData,
    ipsText,
    setHostFormData,
    setIpsText,
    openAddHostModal,
    openEditHostModal,
    closeHostModal,
    handleDeleteHost,
    handleToggleHost,
    handleSubmitHost,
  } = useHostsManager(formData, setFormData);
  const {
    backupInputRef,
    isRestoring,
    handleExportBackup,
    handleImportBackup,
    handleBackupFileChange,
  } = useBackupManager();

  if (!formData) {
    return <LoadingState />;
  }

  const handleValueChange = <K extends keyof SettingsType>(field: K, value: SettingsType[K]) => {
    setFormData((current) => (current ? { ...current, [field]: value } : current));
  };

  const handleNumberChange = (field: 'clash_api_port', value: string) => {
    const nextValue = Number.parseInt(value, 10) || DEFAULT_CLASH_API_PORT;
    handleValueChange(field, nextValue as SettingsType[typeof field]);
  };

  const customHosts = formData.hosts || [];
  const previewSetupUrl = buildZashboardSetupPreviewUrl(formData.clash_api_port, formData.clash_api_secret);

  return (
    <div className="space-y-6">
      <SettingsHero
        hostCount={customHosts.length + systemHosts.length}
        zashboardEnabled={formData.clash_ui_enabled}
        hasSecret={Boolean(formData.clash_api_secret)}
        kernelInstalled={Boolean(kernelInfo?.installed)}
        onSave={handleSave}
      />

      <div className="grid gap-6 2xl:grid-cols-[minmax(0,1.1fr)_minmax(360px,0.9fr)]">
        <div className="space-y-6">
          <CoreSettingsCard
            formData={formData}
            kernelInfo={kernelInfo}
            onValueChange={handleValueChange}
            onDownloadKernel={openDownloadModal}
          />
          <DnsSettingsCard formData={formData} onValueChange={handleValueChange} />
          <HostsCard
            customHosts={customHosts}
            systemHosts={systemHosts}
            onAddHost={openAddHostModal}
            onEditHost={openEditHostModal}
            onDeleteHost={handleDeleteHost}
            onToggleHost={handleToggleHost}
          />
        </div>

        <div className="space-y-6">
          <ControlPanelCard
            formData={formData}
            onValueChange={handleValueChange}
            onNumberChange={handleNumberChange}
            setupUrl={previewSetupUrl}
          />
          <AutomationCard
            autoApply={formData.auto_apply}
            onToggle={(enabled) => handleValueChange('auto_apply', enabled)}
          />
          <BackupCard
            isRestoring={isRestoring}
            backupInputRef={backupInputRef}
            onExport={handleExportBackup}
            onImport={handleImportBackup}
            onFileChange={handleBackupFileChange}
          />
          <DaemonCard
            daemonStatus={daemonStatus}
            onInstall={installDaemon}
            onRestart={restartDaemon}
            onUninstall={uninstallDaemon}
          />
        </div>
      </div>

      <KernelDownloadModal
        isOpen={showDownloadModal}
        isDownloading={downloading}
        releases={releases}
        selectedVersion={selectedVersion}
        kernelInfo={kernelInfo}
        downloadProgress={downloadProgress}
        onClose={closeDownloadModal}
        onSelectionChange={setSelectedVersion}
        onStartDownload={startDownload}
      />
      <HostEditorModal
        isOpen={isHostModalOpen}
        editing={Boolean(editingHost)}
        hostFormData={hostFormData}
        ipsText={ipsText}
        onClose={closeHostModal}
        onChange={setHostFormData}
        onIpsChange={setIpsText}
        onSubmit={handleSubmitHost}
      />
    </div>
  );
}
