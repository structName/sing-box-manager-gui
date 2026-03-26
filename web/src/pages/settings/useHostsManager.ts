import { useEffect, useState, type Dispatch, type SetStateAction } from 'react';
import { settingsApi } from '../../api';
import { toast } from '../../components/Toast';
import type { HostEntry, Settings as SettingsType } from '../../store';
import type { HostFormState } from './types';

const IPV4_REGEX = /^(\d{1,3}\.){3}\d{1,3}$/;
const IPV6_REGEX = /^([a-fA-F0-9:]+)$/;
const DOMAIN_REGEX = /^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)*$/;

const EMPTY_HOST_FORM: HostFormState = { domain: '', enabled: true };

function validateHostForm(domain: string, ips: string[]) {
  const invalidIps = ips.filter((ip) => !IPV4_REGEX.test(ip) && !IPV6_REGEX.test(ip));
  if (invalidIps.length > 0) {
    toast.error(`无效的 IP 地址: ${invalidIps.join(', ')}`);
    return false;
  }
  if (!DOMAIN_REGEX.test(domain)) {
    toast.error('无效的域名格式');
    return false;
  }
  if (ips.length === 0) {
    toast.error('请输入至少一个 IP 地址');
    return false;
  }
  return true;
}

function parseIpLines(text: string) {
  return text.split('\n').map((line) => line.trim()).filter(Boolean);
}

export function useHostsManager(
  formData: SettingsType | null,
  setFormData: Dispatch<SetStateAction<SettingsType | null>>,
) {
  const [systemHosts, setSystemHosts] = useState<HostEntry[]>([]);
  const [isHostModalOpen, setIsHostModalOpen] = useState(false);
  const [editingHost, setEditingHost] = useState<HostEntry | null>(null);
  const [hostFormData, setHostFormData] = useState<HostFormState>(EMPTY_HOST_FORM);
  const [ipsText, setIpsText] = useState('');

  useEffect(() => {
    async function fetchSystemHosts() {
      try {
        const res = await settingsApi.getSystemHosts();
        setSystemHosts(res.data.data || []);
      } catch (error) {
        console.error('获取系统 hosts 失败:', error);
      }
    }
    fetchSystemHosts();
  }, []);

  const openAddHostModal = () => {
    setEditingHost(null);
    setHostFormData(EMPTY_HOST_FORM);
    setIpsText('');
    setIsHostModalOpen(true);
  };

  const openEditHostModal = (host: HostEntry) => {
    setEditingHost(host);
    setHostFormData({ domain: host.domain, enabled: host.enabled });
    setIpsText(host.ips.join('\n'));
    setIsHostModalOpen(true);
  };

  const closeHostModal = () => setIsHostModalOpen(false);

  const handleDeleteHost = (id: string) => {
    if (!formData?.hosts) {
      return;
    }
    setFormData((current) =>
      current ? { ...current, hosts: current.hosts?.filter((h) => h.id !== id) || [] } : current,
    );
  };

  const handleToggleHost = (id: string, enabled: boolean) => {
    if (!formData?.hosts) {
      return;
    }
    setFormData((current) =>
      current
        ? { ...current, hosts: current.hosts?.map((h) => (h.id === id ? { ...h, enabled } : h)) || [] }
        : current,
    );
  };

  const handleSubmitHost = () => {
    const ips = parseIpLines(ipsText);
    if (!validateHostForm(hostFormData.domain, ips)) {
      return;
    }

    setFormData((current) => {
      if (!current) {
        return current;
      }
      const hosts = current.hosts || [];
      const nextHosts = editingHost
        ? hosts.map((h) =>
            h.id === editingHost.id
              ? { ...h, domain: hostFormData.domain, ips, enabled: hostFormData.enabled }
              : h,
          )
        : [...hosts, { id: crypto.randomUUID(), domain: hostFormData.domain, ips, enabled: hostFormData.enabled }];
      return { ...current, hosts: nextHosts };
    });

    setIsHostModalOpen(false);
  };

  return {
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
  };
}
