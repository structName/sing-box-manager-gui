import { useRef, useState, type ChangeEvent } from 'react';
import { backupApi } from '../../api';
import { toast } from '../../components/Toast';

export function useBackupManager() {
  const backupInputRef = useRef<HTMLInputElement>(null);
  const [isRestoring, setIsRestoring] = useState(false);

  const handleExportBackup = () => {
    window.open(backupApi.exportUrl(), '_blank');
    toast.success('备份文件已开始下载');
  };

  const handleImportBackup = () => {
    backupInputRef.current?.click();
  };

  const handleBackupFileChange = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) {
      return;
    }
    if (!file.name.endsWith('.json')) {
      toast.error('请选择 JSON 格式的备份文件');
      return;
    }
    if (!confirm('导入备份将覆盖当前所有配置，确定继续吗？')) {
      event.target.value = '';
      return;
    }

    setIsRestoring(true);
    try {
      const res = await backupApi.restore(file);
      toast.success(res.data.message || '数据已恢复');
      window.location.reload();
    } catch (error: any) {
      toast.error(error.response?.data?.error || '恢复失败');
    } finally {
      setIsRestoring(false);
      event.target.value = '';
    }
  };

  return {
    backupInputRef,
    isRestoring,
    handleExportBackup,
    handleImportBackup,
    handleBackupFileChange,
  };
}
