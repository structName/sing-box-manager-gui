import { useEffect, useState } from 'react';
import { daemonApi } from '../../api';
import { toast } from '../../components/Toast';
import type { DaemonStatus } from './types';

function getApiErrorMessage(error: unknown, fallback: string) {
  const resp = (error as { response?: { data?: { error?: string } } })?.response;
  return resp?.data?.error || fallback;
}

export function useDaemonManager() {
  const [daemonStatus, setDaemonStatus] = useState<DaemonStatus | null>(null);

  const fetchDaemonStatus = async () => {
    try {
      const res = await daemonApi.status();
      setDaemonStatus(res.data.data);
    } catch (error) {
      console.error('获取守护进程状态失败:', error);
    }
  };

  useEffect(() => {
    fetchDaemonStatus();
  }, []);

  const installDaemon = async () => {
    try {
      const res = await daemonApi.install();
      const data = res.data;
      if (data.action === 'exit') {
        toast.success(data.message);
      } else if (data.action === 'manual') {
        toast.info(data.message);
      } else {
        toast.success(data.message || '服务已安装');
      }
      await fetchDaemonStatus();
    } catch (error: unknown) {
      console.error('安装守护进程服务失败:', error);
      toast.error(getApiErrorMessage(error, '安装服务失败'));
    }
  };

  const uninstallDaemon = async () => {
    if (!confirm('确定要卸载后台服务吗？卸载后 sbm 将不再开机自启。')) {
      return;
    }
    try {
      await daemonApi.uninstall();
      toast.success('服务已卸载');
      await fetchDaemonStatus();
    } catch (error: unknown) {
      console.error('卸载守护进程服务失败:', error);
      toast.error(getApiErrorMessage(error, '卸载服务失败'));
    }
  };

  const restartDaemon = async () => {
    try {
      await daemonApi.restart();
      toast.success('服务已重启');
      await fetchDaemonStatus();
    } catch (error: unknown) {
      console.error('重启守护进程服务失败:', error);
      toast.error(getApiErrorMessage(error, '重启服务失败'));
    }
  };

  return { daemonStatus, installDaemon, uninstallDaemon, restartDaemon };
}
