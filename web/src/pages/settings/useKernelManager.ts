import { useEffect, useRef, useState, type RefObject } from 'react';
import { kernelApi } from '../../api';
import type { DownloadProgress, GithubRelease, KernelInfo } from './types';

const DOWNLOAD_POLL_INTERVAL_MS = 500;
const MODAL_CLOSE_DELAY_MS = 1500;

function stopPolling(timerRef: RefObject<ReturnType<typeof setInterval> | null>) {
  if (!timerRef.current) {
    return;
  }
  clearInterval(timerRef.current);
  timerRef.current = null;
}

function getApiErrorMessage(error: unknown, fallback: string) {
  const resp = (error as { response?: { data?: { error?: string } } })?.response;
  return resp?.data?.error || fallback;
}

export function useKernelManager() {
  const [kernelInfo, setKernelInfo] = useState<KernelInfo | null>(null);
  const [releases, setReleases] = useState<GithubRelease[]>([]);
  const [selectedVersion, setSelectedVersion] = useState('');
  const [showDownloadModal, setShowDownloadModal] = useState(false);
  const [downloading, setDownloading] = useState(false);
  const [downloadProgress, setDownloadProgress] = useState<DownloadProgress | null>(null);
  const pollIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchKernelInfo = async () => {
    try {
      const res = await kernelApi.getInfo();
      setKernelInfo(res.data.data);
    } catch (error) {
      console.error('获取内核信息失败:', error);
    }
  };

  const fetchReleases = async () => {
    try {
      const res = await kernelApi.getReleases();
      const nextReleases = res.data.data || [];
      setReleases(nextReleases);
      setSelectedVersion(nextReleases[0]?.tag_name || '');
    } catch (error) {
      console.error('获取版本列表失败:', error);
    }
  };

  useEffect(() => {
    fetchKernelInfo();
    return () => stopPolling(pollIntervalRef);
  }, []);

  const openDownloadModal = async () => {
    await fetchReleases();
    setDownloadProgress(null);
    setShowDownloadModal(true);
  };

  const closeDownloadModal = () => {
    if (!downloading) {
      setShowDownloadModal(false);
    }
  };

  const startDownload = async () => {
    if (!selectedVersion) {
      return;
    }

    setDownloading(true);
    setDownloadProgress({ status: 'preparing', progress: 0, message: '正在准备下载...' });

    try {
      await kernelApi.download(selectedVersion);
      pollIntervalRef.current = setInterval(async () => {
        try {
          const res = await kernelApi.getProgress();
          const progress = res.data.data as DownloadProgress;
          setDownloadProgress(progress);

          if (progress.status === 'completed' || progress.status === 'error') {
            stopPolling(pollIntervalRef);
            setDownloading(false);
            if (progress.status === 'completed') {
              await fetchKernelInfo();
              window.setTimeout(() => setShowDownloadModal(false), MODAL_CLOSE_DELAY_MS);
            }
          }
        } catch (error) {
          console.error('获取进度失败:', error);
        }
      }, DOWNLOAD_POLL_INTERVAL_MS);
    } catch (error: unknown) {
      setDownloading(false);
      setDownloadProgress({
        status: 'error',
        progress: 0,
        message: getApiErrorMessage(error, '下载失败'),
      });
    }
  };

  return {
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
  };
}
