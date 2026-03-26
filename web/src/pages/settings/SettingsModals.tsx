import { Button, Input, Modal, ModalBody, ModalContent, ModalFooter, ModalHeader, Progress, Select, SelectItem, Switch, Textarea } from '@nextui-org/react';
import type { DownloadProgress, GithubRelease, HostFormState, KernelInfo } from './types';

interface KernelDownloadModalProps {
  isOpen: boolean;
  isDownloading: boolean;
  releases: GithubRelease[];
  selectedVersion: string;
  kernelInfo: KernelInfo | null;
  downloadProgress: DownloadProgress | null;
  onClose: () => void;
  onSelectionChange: (version: string) => void;
  onStartDownload: () => void;
}

interface HostEditorModalProps {
  isOpen: boolean;
  editing: boolean;
  hostFormData: HostFormState;
  ipsText: string;
  onClose: () => void;
  onChange: (data: HostFormState) => void;
  onIpsChange: (value: string) => void;
  onSubmit: () => void;
}

export function KernelDownloadModal({
  isOpen,
  isDownloading,
  releases,
  selectedVersion,
  kernelInfo,
  downloadProgress,
  onClose,
  onSelectionChange,
  onStartDownload,
}: KernelDownloadModalProps) {
  return (
    <Modal isOpen={isOpen} onClose={onClose}>
      <ModalContent>
        <ModalHeader>下载 sing-box 内核</ModalHeader>
        <ModalBody className="gap-4">
          <Select
            label="选择版本"
            placeholder="选择要下载的版本"
            selectedKeys={selectedVersion ? [selectedVersion] : []}
            isDisabled={isDownloading}
            onSelectionChange={(keys) => {
              const selected = Array.from(keys)[0] as string;
              onSelectionChange(selected || '');
            }}
          >
            {releases.map((release) => (
              <SelectItem key={release.tag_name} textValue={release.tag_name}>
                {release.tag_name} {release.name && `- ${release.name}`}
              </SelectItem>
            ))}
          </Select>

          {kernelInfo && (
            <p className="text-sm text-slate-500 dark:text-slate-400">
              将下载适用于 {kernelInfo.os}/{kernelInfo.arch} 的版本
            </p>
          )}

          {downloadProgress && (
            <div className="space-y-2">
              <Progress
                value={downloadProgress.progress}
                color={downloadProgress.status === 'error' ? 'danger' : downloadProgress.status === 'completed' ? 'success' : 'primary'}
                showValueLabel
              />
              <p className={`text-sm ${downloadProgress.status === 'error' ? 'text-danger' : downloadProgress.status === 'completed' ? 'text-success' : 'text-slate-500 dark:text-slate-400'}`}>
                {downloadProgress.message}
              </p>
            </div>
          )}
        </ModalBody>
        <ModalFooter>
          <Button variant="flat" onPress={onClose} isDisabled={isDownloading}>
            取消
          </Button>
          <Button color="primary" onPress={onStartDownload} isLoading={isDownloading} isDisabled={!selectedVersion || isDownloading}>
            开始下载
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
}

export function HostEditorModal({
  isOpen,
  editing,
  hostFormData,
  ipsText,
  onClose,
  onChange,
  onIpsChange,
  onSubmit,
}: HostEditorModalProps) {
  return (
    <Modal isOpen={isOpen} onClose={onClose}>
      <ModalContent>
        <ModalHeader>{editing ? '编辑 Host' : '添加 Host'}</ModalHeader>
        <ModalBody className="gap-4">
          <Input
            label="域名"
            placeholder="例如：example.com"
            value={hostFormData.domain}
            onChange={(event) => onChange({ ...hostFormData, domain: event.target.value })}
          />
          <Textarea
            label="IP 地址"
            placeholder={'每行一个 IP 地址\n例如：\n192.168.1.1\n192.168.1.2'}
            value={ipsText}
            minRows={3}
            onChange={(event) => onIpsChange(event.target.value)}
          />
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium text-slate-700 dark:text-slate-200">启用</span>
            <Switch isSelected={hostFormData.enabled} onValueChange={(enabled) => onChange({ ...hostFormData, enabled })} />
          </div>
        </ModalBody>
        <ModalFooter>
          <Button variant="flat" onPress={onClose}>取消</Button>
          <Button color="primary" onPress={onSubmit} isDisabled={!hostFormData.domain || !ipsText.trim()}>
            {editing ? '保存' : '添加'}
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
}
