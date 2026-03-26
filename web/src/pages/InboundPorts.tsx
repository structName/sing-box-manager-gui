import { useEffect, useState } from 'react';
import { Card, CardBody, Input, Button, Switch, Chip, Modal, ModalContent, ModalHeader, ModalBody, ModalFooter, Select, SelectItem, Pagination, useDisclosure } from '@nextui-org/react';
import { Plus, Pencil, Trash2, Network, RefreshCw } from 'lucide-react';
import { useStore } from '../store';
import type { Settings as SettingsType } from '../store';
import { inboundPortApi, filterApi, proxyChainApi, nodeApi } from '../api';
import { toast } from '../components/Toast';

// 入站端口类型
interface InboundPort {
  id: string;
  name: string;
  type: string;
  listen: string;
  port: number;
  auth?: {
    username: string;
    password: string;
  };
  outbound: string;
  enabled: boolean;
}

type InboundPortPayload = Omit<InboundPort, 'id'>;

// 过滤器类型
interface Filter {
  id: string;
  name: string;
  enabled: boolean;
}

// 代理链路类型
interface ProxyChain {
  id: string;
  name: string;
  enabled: boolean;
}

// 地区分组类型
interface CountryGroup {
  code: string;
  name: string;
  emoji: string;
  node_count: number;
}

// 节点类型
interface Node {
  tag: string;
  type: string;
  server: string;
  country?: string;
  country_emoji?: string;
  source?: string;
  source_name?: string;
}

// 节点分组类型
interface NodeGroup {
  source: string;
  source_name: string;
  nodes: Node[];
}

type OutboundCategory = 'basic' | 'country' | 'filter' | 'chain' | 'node';

function getClientAddressHint(listen: string): string {
  if (listen === '0.0.0.0' || listen === '::' || listen === '') {
    const currentHost = window.location.hostname;
    if (currentHost && currentHost !== '0.0.0.0' && currentHost !== '::') {
      return `${currentHost}（当前访问地址）或本机局域网 IP`;
    }
    return '127.0.0.1 或本机局域网 IP';
  }

  if (listen === 'localhost') {
    return '127.0.0.1';
  }

  return listen;
}

// 国家选项
const countryOptions = [
  { code: 'HK', name: '香港', emoji: '🇭🇰' },
  { code: 'TW', name: '台湾', emoji: '🇹🇼' },
  { code: 'JP', name: '日本', emoji: '🇯🇵' },
  { code: 'KR', name: '韩国', emoji: '🇰🇷' },
  { code: 'SG', name: '新加坡', emoji: '🇸🇬' },
  { code: 'US', name: '美国', emoji: '🇺🇸' },
  { code: 'GB', name: '英国', emoji: '🇬🇧' },
  { code: 'DE', name: '德国', emoji: '🇩🇪' },
  { code: 'FR', name: '法国', emoji: '🇫🇷' },
  { code: 'NL', name: '荷兰', emoji: '🇳🇱' },
  { code: 'AU', name: '澳大利亚', emoji: '🇦🇺' },
  { code: 'CA', name: '加拿大', emoji: '🇨🇦' },
  { code: 'RU', name: '俄罗斯', emoji: '🇷🇺' },
  { code: 'IN', name: '印度', emoji: '🇮🇳' },
  { code: 'TR', name: '土耳其', emoji: '🇹🇷' },
];

const NODES_PER_PAGE = 20;

function createDefaultPortFormData() {
  return {
    name: '',
    type: 'mixed',
    listen: '0.0.0.0',
    port: 2081,
    username: '',
    password: '',
    outbound: 'Proxy',
    enabled: true,
  };
}

function getApiErrorMessage(error: unknown, fallback: string): string {
  const responseError = (error as { response?: { data?: { error?: string } } })?.response?.data?.error;
  return typeof responseError === 'string' && responseError ? responseError : fallback;
}

function getSelectableItemClasses(isSelected: boolean): string {
  return [
    'cursor-pointer rounded-xl border p-3 transition-colors',
    isSelected
      ? 'border-primary bg-primary-50 shadow-sm dark:border-primary-400 dark:bg-primary-500/10'
      : 'border-default-200 bg-white/80 hover:border-primary-200 hover:bg-default-50 dark:bg-default-100/70'
  ].join(' ');
}

export default function InboundPorts() {
  const { settings, fetchSettings, updateSettings } = useStore();
  const [formData, setFormData] = useState<SettingsType | null>(null);

  // 入站端口相关状态
  const [inboundPorts, setInboundPorts] = useState<InboundPort[]>([]);
  const [filters, setFilters] = useState<Filter[]>([]);
  const [proxyChains, setProxyChains] = useState<ProxyChain[]>([]);
  const [countryGroups, setCountryGroups] = useState<CountryGroup[]>([]);
  const [nodes, setNodes] = useState<Node[]>([]);
  const [nodeGroups, setNodeGroups] = useState<NodeGroup[]>([]);
  const { isOpen: isPortModalOpen, onOpen: onPortModalOpen, onClose: onPortModalClose } = useDisclosure();
  const [editingPort, setEditingPort] = useState<InboundPort | null>(null);
  const [portFormData, setPortFormData] = useState(createDefaultPortFormData);

  // 出站选择筛选状态
  const [outboundType, setOutboundType] = useState<OutboundCategory>('basic');
  const [selectedCountry, setSelectedCountry] = useState<string>('');
  const [selectedSource, setSelectedSource] = useState<string>('');
  const [searchText, setSearchText] = useState<string>('');
  const [nodePage, setNodePage] = useState(1);

  async function fetchInboundPorts() {
    try {
      const res = await inboundPortApi.getAll();
      setInboundPorts(res.data.data || []);
    } catch (error) {
      console.error('获取入站端口失败:', error);
    }
  }

  async function fetchFilters() {
    try {
      const res = await filterApi.getAll();
      setFilters(res.data.data || []);
    } catch (error) {
      console.error('获取过滤器列表失败:', error);
    }
  }

  async function fetchProxyChains() {
    try {
      const res = await proxyChainApi.getAll();
      setProxyChains(res.data.data || []);
    } catch (error) {
      console.error('获取代理链路失败:', error);
    }
  }

  async function fetchCountryGroups() {
    try {
      const res = await nodeApi.getCountries();
      setCountryGroups(res.data.data || []);
    } catch (error) {
      console.error('获取地区分组失败:', error);
    }
  }

  async function fetchNodes() {
    try {
      const res = await nodeApi.getAll();
      setNodes(res.data.data || []);
    } catch (error) {
      console.error('获取节点列表失败:', error);
    }
  }

  async function fetchNodeGroups() {
    try {
      const res = await nodeApi.getGrouped();
      setNodeGroups(res.data.data || []);
    } catch (error) {
      console.error('获取节点分组失败:', error);
    }
  }

  async function refreshOutboundResources() {
    await Promise.all([
      fetchNodes(),
      fetchNodeGroups(),
      fetchCountryGroups(),
      fetchFilters(),
      fetchProxyChains(),
    ]);
  }

  useEffect(() => {
    fetchSettings();
    fetchInboundPorts();
    fetchFilters();
    fetchProxyChains();
    fetchCountryGroups();
    fetchNodes();
    fetchNodeGroups();
  }, [fetchSettings]);

  useEffect(() => {
    if (settings) {
      setFormData(settings);
    }
  }, [settings]);

  // 入站端口处理函数
  const handleAddPort = () => {
    setEditingPort(null);
    setPortFormData(createDefaultPortFormData());
    // 重置筛选状态
    setOutboundType('basic');
    setSelectedCountry('');
    setSelectedSource('');
    setSearchText('');
    setNodePage(1);
    onPortModalOpen();
  };

  const handleEditPort = (port: InboundPort) => {
    const matchedNode = nodes.find((node) => node.tag === port.outbound);
    const matchedCategory: OutboundCategory = ['Proxy', 'DIRECT', 'Auto'].includes(port.outbound)
      ? 'basic'
      : countryGroups.some((country) => country.code === port.outbound)
        ? 'country'
        : filters.some((filter) => filter.name === port.outbound)
          ? 'filter'
          : proxyChains.some((chain) => chain.name === port.outbound)
            ? 'chain'
            : 'node';

    setEditingPort(port);
    setPortFormData({
      name: port.name,
      type: port.type,
      listen: port.listen,
      port: port.port,
      username: port.auth?.username || '',
      password: port.auth?.password || '',
      outbound: port.outbound,
      enabled: port.enabled,
    });
    // 重置筛选状态
    setOutboundType(matchedCategory);
    setSelectedCountry(matchedCategory === 'node' ? matchedNode?.country || '' : '');
    setSelectedSource(matchedCategory === 'node' ? matchedNode?.source || '' : '');
    setSearchText('');
    setNodePage(1);
    onPortModalOpen();
  };

  const handleDeletePort = async (id: string) => {
    if (!confirm('确定要删除这个入站端口吗？')) return;
    try {
      await inboundPortApi.delete(id);
      toast.success('端口已删除');
      fetchInboundPorts();
    } catch (error: unknown) {
      toast.error(getApiErrorMessage(error, '删除失败'));
    }
  };

  const handleTogglePort = async (port: InboundPort) => {
    try {
      await inboundPortApi.update(port.id, { ...port, enabled: !port.enabled });
      fetchInboundPorts();
    } catch (error: unknown) {
      toast.error(getApiErrorMessage(error, '更新失败'));
    }
  };

  const handleSubmitPort = async () => {
    if (!portFormData.name.trim()) {
      toast.error('请输入端口名称');
      return;
    }
    if (portFormData.port < 1 || portFormData.port > 65535) {
      toast.error('端口号必须在 1-65535 之间');
      return;
    }

    const data: InboundPortPayload = {
      name: portFormData.name,
      type: portFormData.type,
      listen: portFormData.listen,
      port: portFormData.port,
      outbound: portFormData.outbound,
      enabled: portFormData.enabled,
    };

    // 如果有用户名和密码，添加认证
    if (portFormData.username && portFormData.password) {
      data.auth = {
        username: portFormData.username,
        password: portFormData.password,
      };
    }

    try {
      if (editingPort) {
        await inboundPortApi.update(editingPort.id, data);
        toast.success('端口已更新');
      } else {
        await inboundPortApi.add(data);
        toast.success('端口已添加');
      }
      onPortModalClose();
      fetchInboundPorts();
    } catch (error: unknown) {
      toast.error(getApiErrorMessage(error, '操作失败'));
    }
  };

  const handleSaveBasicSettings = async () => {
    if (formData) {
      try {
        await updateSettings(formData);
        toast.success('入站配置已保存');
      } catch (error: unknown) {
        toast.error(getApiErrorMessage(error, '保存设置失败'));
      }
    }
  };

  // 选择出站线路
  const selectOutbound = (outbound: string) => {
    setPortFormData({ ...portFormData, outbound });
  };

  if (!formData) {
    return <div>加载中...</div>;
  }

  // 构建出站选项
  const enabledFilters = filters.filter(f => f.enabled);
  const enabledChains = proxyChains.filter(c => c.enabled);

  // 获取当前节点中存在的国家列表
  const availableCountries = countryOptions.filter(
    opt => nodes.some(node => node.country === opt.code)
  );

  const enabledInboundCount = inboundPorts.filter(port => port.enabled).length;
  const authProtectedInboundCount = inboundPorts.filter(port => Boolean(port.auth)).length;
  const customInboundCount = inboundPorts.length;

  // 过滤节点分组（用于单独节点选择）
  const getFilteredNodeGroups = () => {
    let groups = nodeGroups;

    // 按来源筛选
    if (selectedSource) {
      groups = groups.filter(g => g.source === selectedSource);
    }

    // 对每个分组内的节点进行筛选
    return groups.map(group => ({
      ...group,
      nodes: group.nodes.filter(node => {
        const matchSearch = !searchText ||
          node.tag.toLowerCase().includes(searchText.toLowerCase()) ||
          node.server.toLowerCase().includes(searchText.toLowerCase());
        const matchCountry = !selectedCountry || node.country === selectedCountry;
        return matchSearch && matchCountry;
      })
    })).filter(group => group.nodes.length > 0);
  };

  const filteredNodeGroups = getFilteredNodeGroups();
  const flatFilteredNodes = filteredNodeGroups.flatMap(g =>
    g.nodes.map(n => ({ ...n, source_name: g.source_name }))
  );
  const nodeTotalPages = Math.max(1, Math.ceil(flatFilteredNodes.length / NODES_PER_PAGE));
  const safeNodePage = Math.min(nodePage, nodeTotalPages);
  const pagedNodes = flatFilteredNodes.slice(
    (safeNodePage - 1) * NODES_PER_PAGE,
    safeNodePage * NODES_PER_PAGE
  );
  const basicOutboundOptions = [
    { key: 'Proxy', label: 'Proxy（主代理）', description: '沿用主代理策略，适合作为常用默认出口。' },
    { key: 'DIRECT', label: 'DIRECT（直连）', description: '流量直接连接目标地址，不经过代理节点。' },
    { key: 'Auto', label: 'Auto（自动选择）', description: '按当前测速或策略结果自动选择更优线路。' },
  ].filter(item => !searchText || item.label.toLowerCase().includes(searchText.toLowerCase()));

  const filteredCountries = countryGroups.filter(
    country => !searchText
      || country.name.toLowerCase().includes(searchText.toLowerCase())
      || country.code.toLowerCase().includes(searchText.toLowerCase())
  );

  const filteredFilters = enabledFilters.filter(
    filter => !searchText || filter.name.toLowerCase().includes(searchText.toLowerCase())
  );

  const filteredChains = enabledChains.filter(
    chain => !searchText || chain.name.toLowerCase().includes(searchText.toLowerCase())
  );

  const outboundNavItems = [
    {
      key: 'basic' as const,
      title: '基础出站',
      description: 'Proxy、DIRECT 与 Auto',
      count: 3,
      visible: true,
    },
    {
      key: 'country' as const,
      title: '地区节点',
      description: '按国家或地区聚合选择',
      count: countryGroups.length,
      visible: countryGroups.length > 0,
    },
    {
      key: 'filter' as const,
      title: '过滤器',
      description: '复用已启用过滤规则',
      count: enabledFilters.length,
      visible: enabledFilters.length > 0,
    },
    {
      key: 'chain' as const,
      title: '代理链路',
      description: '选择已启用链路编排',
      count: enabledChains.length,
      visible: enabledChains.length > 0,
    },
    {
      key: 'node' as const,
      title: '单独节点',
      description: '从来源分组中直选节点',
      count: nodes.length,
      visible: nodes.length > 0,
    },
  ].filter(item => item.visible);
  const activeOutboundMeta = outboundNavItems.find(item => item.key === outboundType);
  const hasOutboundResults = outboundType === 'basic'
    ? basicOutboundOptions.length > 0
    : outboundType === 'country'
      ? filteredCountries.length > 0
      : outboundType === 'filter'
        ? filteredFilters.length > 0
        : outboundType === 'chain'
          ? filteredChains.length > 0
          : filteredNodeGroups.length > 0;

  // 获取出站类型对应的显示名称
  const getOutboundDisplayName = (outbound: string) => {
    // 基础出站
    if (outbound === 'Proxy') return 'Proxy（主代理）';
    if (outbound === 'DIRECT') return 'DIRECT（直连）';
    if (outbound === 'Auto') return 'Auto（自动选择）';

    // 地区节点
    const country = countryGroups.find(c => c.code === outbound);
    if (country) return `${country.emoji} ${country.name}`;

    // 过滤器
    const filter = filters.find(f => f.name === outbound);
    if (filter) return `${outbound}（过滤器）`;

    // 链路
    const chain = proxyChains.find(c => c.name === outbound);
    if (chain) return `${outbound}（链路）`;

    // 单独节点
    const node = nodes.find(n => n.tag === outbound);
    if (node) return `${node.country_emoji || ''} ${outbound}`;

    return outbound;
  };

  return (
    <div className="space-y-6">
      {/* Page header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-800 dark:text-white">入站管理</h1>
          <p className="text-sm text-default-500">
            为不同设备、成员或使用场景分配独立入站端口与出站线路。
          </p>
        </div>
        <Button
          color="primary"
          startContent={<Plus className="w-4 h-4" />}
          onPress={handleAddPort}
        >
          添加端口
        </Button>
      </div>

      {/* System-level settings bar */}
      <Card>
        <CardBody className="p-4">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
            <div className="flex items-center gap-3">
              <span className="text-sm text-default-500">TUN 模式</span>
              <Switch
                size="sm"
                isSelected={formData.tun_enabled}
                onValueChange={(enabled) => setFormData({ ...formData, tun_enabled: enabled })}
              />
            </div>
            <div className="flex items-center gap-3">
              <div className="flex flex-wrap gap-2">
                <Chip size="sm" variant="flat" color="primary">{customInboundCount} 端口</Chip>
                <Chip size="sm" variant="flat" color="success">{enabledInboundCount} 启用</Chip>
                {authProtectedInboundCount > 0 && (
                  <Chip size="sm" variant="flat" color="warning">{authProtectedInboundCount} 认证</Chip>
                )}
              </div>
              <Button size="sm" color="primary" variant="flat" onPress={handleSaveBasicSettings}>
                保存设置
              </Button>
            </div>
          </div>
        </CardBody>
      </Card>

      {/* Port list */}
      {inboundPorts.length === 0 ? (
        <div className="rounded-2xl border border-dashed border-default-300 py-16 text-center text-default-500">
          <Network className="w-12 h-12 mx-auto mb-4 opacity-40" />
          <p className="font-medium">暂无入站端口</p>
          <p className="mt-1 text-sm">点击右上角「添加端口」创建第一个入站端口。</p>
        </div>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {inboundPorts.map((port) => (
            <Card key={port.id} className={!port.enabled ? 'opacity-60' : ''}>
              <CardBody className="p-4">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0 space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="truncate text-base font-semibold">{port.name}</span>
                      <Chip size="sm" variant="flat">{port.type}</Chip>
                      {port.auth && <Chip size="sm" color="warning" variant="flat">认证</Chip>}
                    </div>
                    <p className="text-sm text-default-500 tabular-nums">
                      {port.listen}:{port.port}
                    </p>
                  </div>
                  <div className="flex shrink-0 items-center gap-1">
                    <Button isIconOnly size="sm" variant="light" onPress={() => handleEditPort(port)}>
                      <Pencil className="w-4 h-4" />
                    </Button>
                    <Button isIconOnly size="sm" variant="light" color="danger" onPress={() => handleDeletePort(port.id)}>
                      <Trash2 className="w-4 h-4" />
                    </Button>
                    <Switch size="sm" isSelected={port.enabled} onValueChange={() => handleTogglePort(port)} />
                  </div>
                </div>

                <div className="mt-3 rounded-lg bg-default-50 p-2.5 dark:bg-default-100/70">
                  <p className="text-xs text-default-500">出口线路</p>
                  <p className="mt-0.5 text-sm font-medium">{getOutboundDisplayName(port.outbound)}</p>
                </div>

                {(port.listen === '0.0.0.0' || port.listen === '::' || port.listen === '') && (
                  <p className="mt-2 text-xs text-warning-600">
                    客户端请使用 {getClientAddressHint(port.listen)}:{port.port}
                  </p>
                )}
              </CardBody>
            </Card>
          ))}
        </div>
      )}

      {/* 入站端口编辑弹窗 */}
      <Modal isOpen={isPortModalOpen} onClose={onPortModalClose} size="5xl" scrollBehavior="inside">
        <ModalContent className="max-h-[90vh]">
          <ModalHeader>{editingPort ? '编辑入站端口' : '添加入站端口'}</ModalHeader>
          <ModalBody className="gap-0 overflow-hidden p-0">
            <div className="grid min-h-0 gap-0 xl:grid-cols-[360px_minmax(0,1fr)]">
              <div className="space-y-5 border-b border-divider p-6 xl:max-h-[calc(90vh-140px)] xl:overflow-y-auto xl:border-b-0 xl:border-r">
                <div className="space-y-4">
                  <div>
                    <p className="text-sm font-medium text-default-700">基础信息</p>
                    <p className="mt-1 text-sm text-default-500">先定义端口身份，再选择对应出口线路。</p>
                  </div>
                  <Input
                    label="端口名称"
                    placeholder="例如：家人专用、办公室"
                    value={portFormData.name}
                    onChange={(e) => setPortFormData({ ...portFormData, name: e.target.value })}
                  />
                  <div className="grid gap-4">
                    <Select
                      label="协议类型"
                      selectedKeys={[portFormData.type]}
                      onSelectionChange={(keys) => {
                        const selected = Array.from(keys)[0] as string;
                        if (selected) setPortFormData({ ...portFormData, type: selected });
                      }}
                    >
                      <SelectItem key="mixed">Mixed (HTTP + SOCKS5)</SelectItem>
                      <SelectItem key="http">HTTP</SelectItem>
                      <SelectItem key="socks">SOCKS5</SelectItem>
                    </Select>
                    <Input
                      type="number"
                      label="端口号"
                      placeholder="2081"
                      value={String(portFormData.port)}
                      onChange={(e) => setPortFormData({ ...portFormData, port: parseInt(e.target.value) || 2081 })}
                    />
                    <Input
                      label="监听地址"
                      placeholder="0.0.0.0"
                      value={portFormData.listen}
                      onChange={(e) => setPortFormData({ ...portFormData, listen: e.target.value })}
                      description={
                        portFormData.listen === '0.0.0.0' || portFormData.listen === '::' || portFormData.listen === ''
                          ? `客户端请使用 ${getClientAddressHint(portFormData.listen)}:${portFormData.port}，不要直接使用 0.0.0.0`
                          : `客户端连接地址：${getClientAddressHint(portFormData.listen)}:${portFormData.port}`
                      }
                    />
                  </div>
                </div>

                <div className="rounded-2xl border border-default-200 bg-default-50/60 p-4">
                  <p className="text-xs font-medium uppercase tracking-wide text-default-500">当前出口</p>
                  <p className="mt-2 text-base font-semibold text-default-900">
                    {getOutboundDisplayName(portFormData.outbound)}
                  </p>
                  <p className="mt-1 text-sm text-default-500">
                    右侧选择后会立即更新这里的结果。
                  </p>
                </div>

                <div className="border-t border-divider pt-5">
                  <p className="mb-3 text-sm font-medium">用户认证（可选）</p>
                  <div className="grid gap-4">
                    <Input
                      label="用户名"
                      placeholder="留空表示无需认证"
                      value={portFormData.username}
                      onChange={(e) => setPortFormData({ ...portFormData, username: e.target.value })}
                    />
                    <Input
                      label="密码"
                      type="password"
                      placeholder="留空表示无需认证"
                      value={portFormData.password}
                      onChange={(e) => setPortFormData({ ...portFormData, password: e.target.value })}
                    />
                  </div>
                </div>

                <div className="flex items-center justify-between rounded-2xl border border-default-200 bg-default-50/60 p-4">
                  <div>
                    <p className="font-medium text-default-900">启用端口</p>
                    <p className="text-sm text-default-500">保存后立即参与入站配置。</p>
                  </div>
                  <Switch
                    isSelected={portFormData.enabled}
                    onValueChange={(enabled) => setPortFormData({ ...portFormData, enabled })}
                  />
                </div>
              </div>

              <div className="min-h-0 space-y-4 p-6 xl:max-h-[calc(90vh-140px)] xl:overflow-y-auto">
                {/* Category tabs */}
                <div className="flex flex-wrap gap-2">
                  {outboundNavItems.map((item) => (
                    <button
                      key={item.key}
                      type="button"
                      className={[
                        'inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-sm transition-colors',
                        outboundType === item.key
                          ? 'border-primary bg-primary-50 font-medium text-primary dark:border-primary-400 dark:bg-primary-500/10'
                          : 'border-default-200 bg-white/80 text-default-700 hover:border-primary-200 hover:bg-default-50 dark:bg-default-100/70',
                      ].join(' ')}
                      onClick={() => {
                        setOutboundType(item.key);
                        setSearchText('');
                        setNodePage(1);
                        if (item.key !== 'node') {
                          setSelectedCountry('');
                          setSelectedSource('');
                        }
                      }}
                    >
                      {item.title}
                      <span className={[
                        'inline-flex h-5 min-w-5 items-center justify-center rounded-full px-1 text-xs',
                        outboundType === item.key
                          ? 'bg-primary/10 text-primary'
                          : 'bg-default-100 text-default-500',
                      ].join(' ')}>
                        {item.count}
                      </span>
                    </button>
                  ))}
                </div>

                {/* Search & filter bar */}
                <div className="flex flex-col gap-3 lg:flex-row lg:items-center">
                  <Input
                    placeholder={`搜索${activeOutboundMeta?.title || '出站线路'}...`}
                    size="sm"
                    value={searchText}
                    onChange={(e) => { setSearchText(e.target.value); setNodePage(1); }}
                    className="flex-1"
                  />
                  {outboundType === 'node' && (
                    <>
                      <Select
                        placeholder="来源"
                        size="sm"
                        selectedKeys={selectedSource ? [selectedSource] : []}
                        onSelectionChange={(keys) => {
                          const selected = Array.from(keys)[0] as string;
                          setSelectedSource(selected || '');
                          setNodePage(1);
                        }}
                        className="lg:w-40"
                      >
                        {nodeGroups.map((group) => (
                          <SelectItem key={group.source} textValue={group.source_name}>
                            {group.source_name}
                          </SelectItem>
                        ))}
                      </Select>
                      <Select
                        placeholder="国家"
                        size="sm"
                        selectedKeys={selectedCountry ? [selectedCountry] : []}
                        onSelectionChange={(keys) => {
                          const selected = Array.from(keys)[0] as string;
                          setSelectedCountry(selected || '');
                          setNodePage(1);
                        }}
                        className="lg:w-36"
                      >
                        {availableCountries.map((opt) => (
                          <SelectItem key={opt.code} textValue={opt.name}>
                            {opt.emoji} {opt.name}
                          </SelectItem>
                        ))}
                      </Select>
                    </>
                  )}
                  <Button
                    size="sm"
                    variant="light"
                    isIconOnly
                    onPress={refreshOutboundResources}
                  >
                    <RefreshCw className="w-3.5 h-3.5" />
                  </Button>
                </div>

                {/* Results */}
                {!hasOutboundResults ? (
                  <div className="rounded-xl border border-dashed border-default-300 bg-default-50/60 px-4 py-10 text-center text-sm text-default-500">
                    当前筛选条件下没有可选线路。
                  </div>
                ) : (
                  <div className="max-h-[420px] space-y-3 overflow-y-auto pr-1">
                    {outboundType === 'basic' && (
                      <div className="grid gap-2.5 sm:grid-cols-3">
                        {basicOutboundOptions.map((item) => (
                          <div
                            key={item.key}
                            className={getSelectableItemClasses(portFormData.outbound === item.key)}
                            onClick={() => selectOutbound(item.key)}
                          >
                            <p className="font-medium text-default-900">{item.label}</p>
                            <p className="mt-1 text-xs text-default-500">{item.description}</p>
                          </div>
                        ))}
                      </div>
                    )}

                    {outboundType === 'country' && (
                      <div className="grid gap-2.5 sm:grid-cols-2 lg:grid-cols-3">
                        {filteredCountries.map((country) => (
                          <div
                            key={country.code}
                            className={getSelectableItemClasses(portFormData.outbound === country.code)}
                            onClick={() => selectOutbound(country.code)}
                          >
                            <div className="flex items-center gap-2.5">
                              <span className="text-lg">{country.emoji}</span>
                              <div>
                                <p className="font-medium text-default-900">{country.name}</p>
                                <p className="text-xs text-default-500">{country.node_count} 个节点</p>
                              </div>
                            </div>
                          </div>
                        ))}
                      </div>
                    )}

                    {outboundType === 'filter' && (
                      <div className="grid gap-2.5 sm:grid-cols-2">
                        {filteredFilters.map((filter) => (
                          <div
                            key={filter.name}
                            className={getSelectableItemClasses(portFormData.outbound === filter.name)}
                            onClick={() => selectOutbound(filter.name)}
                          >
                            <p className="font-medium text-default-900">{filter.name}</p>
                            <p className="mt-1 text-xs text-default-500">按既有过滤规则组织出口策略。</p>
                          </div>
                        ))}
                      </div>
                    )}

                    {outboundType === 'chain' && (
                      <div className="grid gap-2.5 sm:grid-cols-2">
                        {filteredChains.map((chain) => (
                          <div
                            key={chain.name}
                            className={getSelectableItemClasses(portFormData.outbound === chain.name)}
                            onClick={() => selectOutbound(chain.name)}
                          >
                            <p className="font-medium text-default-900">{chain.name}</p>
                            <p className="mt-1 text-xs text-default-500">复用预先编排好的代理链路。</p>
                          </div>
                        ))}
                      </div>
                    )}

                    {outboundType === 'node' && (
                      <div className="space-y-2">
                        {pagedNodes.map((node) => (
                          <div
                            key={node.tag}
                            className={getSelectableItemClasses(portFormData.outbound === node.tag)}
                            onClick={() => selectOutbound(node.tag)}
                          >
                            <div className="flex items-center justify-between gap-3">
                              <div className="flex min-w-0 items-center gap-2.5">
                                {node.country_emoji && <span className="text-base">{node.country_emoji}</span>}
                                <div className="min-w-0">
                                  <p className="truncate font-medium text-default-900">{node.tag}</p>
                                  <p className="text-xs text-default-500">
                                    {node.server} · {node.type}
                                    <span className="ml-2 text-default-400">{node.source_name}</span>
                                  </p>
                                </div>
                              </div>
                              {portFormData.outbound === node.tag && (
                                <Chip size="sm" color="primary" variant="flat">已选</Chip>
                              )}
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                )}

                {/* Pagination for nodes */}
                {outboundType === 'node' && flatFilteredNodes.length > NODES_PER_PAGE && (
                  <div className="flex items-center justify-between border-t border-divider pt-3">
                    <p className="text-xs text-default-500">
                      共 {flatFilteredNodes.length} 个节点
                    </p>
                    <Pagination
                      size="sm"
                      total={nodeTotalPages}
                      page={safeNodePage}
                      onChange={setNodePage}
                      showControls
                    />
                  </div>
                )}
              </div>
            </div>
          </ModalBody>
          <ModalFooter>
            <Button variant="flat" onPress={onPortModalClose}>取消</Button>
            <Button
              color="primary"
              onPress={handleSubmitPort}
              isDisabled={!portFormData.name.trim()}
            >
              {editingPort ? '保存' : '添加'}
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </div>
  );
}
