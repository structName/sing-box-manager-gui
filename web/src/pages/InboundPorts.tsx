import { useEffect, useState } from 'react';
import { Card, CardBody, CardHeader, Input, Button, Switch, Chip, Modal, ModalContent, ModalHeader, ModalBody, ModalFooter, Select, SelectItem, Accordion, AccordionItem, useDisclosure } from '@nextui-org/react';
import { Plus, Pencil, Trash2, Network, Download, RefreshCw } from 'lucide-react';
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
  const [portFormData, setPortFormData] = useState({
    name: '',
    type: 'mixed',
    listen: '0.0.0.0',
    port: 2081,
    username: '',
    password: '',
    outbound: 'Proxy',
    enabled: true,
  });

  // 出站选择筛选状态
  const [outboundType, setOutboundType] = useState<string>('');
  const [selectedCountry, setSelectedCountry] = useState<string>('');
  const [selectedSource, setSelectedSource] = useState<string>('');
  const [searchText, setSearchText] = useState<string>('');

  useEffect(() => {
    fetchSettings();
    fetchInboundPorts();
    fetchFilters();
    fetchProxyChains();
    fetchCountryGroups();
    fetchNodes();
    fetchNodeGroups();
  }, []);

  useEffect(() => {
    if (settings) {
      setFormData(settings);
    }
  }, [settings]);

  const fetchInboundPorts = async () => {
    try {
      const res = await inboundPortApi.getAll();
      setInboundPorts(res.data.data || []);
    } catch (error) {
      console.error('获取入站端口失败:', error);
    }
  };

  const fetchFilters = async () => {
    try {
      const res = await filterApi.getAll();
      setFilters(res.data.data || []);
    } catch (error) {
      console.error('获取过滤器列表失败:', error);
    }
  };

  const fetchProxyChains = async () => {
    try {
      const res = await proxyChainApi.getAll();
      setProxyChains(res.data.data || []);
    } catch (error) {
      console.error('获取代理链路失败:', error);
    }
  };

  const fetchCountryGroups = async () => {
    try {
      const res = await nodeApi.getCountries();
      setCountryGroups(res.data.data || []);
    } catch (error) {
      console.error('获取地区分组失败:', error);
    }
  };

  const fetchNodes = async () => {
    try {
      const res = await nodeApi.getAll();
      setNodes(res.data.data || []);
    } catch (error) {
      console.error('获取节点列表失败:', error);
    }
  };

  const fetchNodeGroups = async () => {
    try {
      const res = await nodeApi.getGrouped();
      setNodeGroups(res.data.data || []);
    } catch (error) {
      console.error('获取节点分组失败:', error);
    }
  };

  // 入站端口处理函数
  const handleAddPort = () => {
    setEditingPort(null);
    setPortFormData({
      name: '',
      type: 'mixed',
      listen: '0.0.0.0',
      port: 2081,
      username: '',
      password: '',
      outbound: 'Proxy',
      enabled: true,
    });
    // 重置筛选状态
    setOutboundType('');
    setSelectedCountry('');
    setSelectedSource('');
    setSearchText('');
    onPortModalOpen();
  };

  const handleEditPort = (port: InboundPort) => {
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
    setOutboundType('');
    setSelectedCountry('');
    setSelectedSource('');
    setSearchText('');
    onPortModalOpen();
  };

  const handleDeletePort = async (id: string) => {
    if (!confirm('确定要删除这个入站端口吗？')) return;
    try {
      await inboundPortApi.delete(id);
      toast.success('端口已删除');
      fetchInboundPorts();
    } catch (error: any) {
      toast.error(error.response?.data?.error || '删除失败');
    }
  };

  const handleTogglePort = async (port: InboundPort) => {
    try {
      await inboundPortApi.update(port.id, { ...port, enabled: !port.enabled });
      fetchInboundPorts();
    } catch (error: any) {
      toast.error(error.response?.data?.error || '更新失败');
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

    const data: any = {
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
    } catch (error: any) {
      toast.error(error.response?.data?.error || '操作失败');
    }
  };

  const handleSaveBasicSettings = async () => {
    if (formData) {
      try {
        await updateSettings(formData);
        toast.success('入站配置已保存');
      } catch (error: any) {
        toast.error(error.response?.data?.error || '保存设置失败');
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
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold text-gray-800 dark:text-white">入站管理</h1>
      </div>

      {/* 基础入站配置 */}
      <Card>
        <CardHeader className="flex justify-between items-center">
          <div className="flex items-center">
            <Download className="w-5 h-5 mr-2" />
            <h2 className="text-lg font-semibold">基础配置</h2>
          </div>
          <Button
            color="primary"
            size="sm"
            onPress={handleSaveBasicSettings}
          >
            保存
          </Button>
        </CardHeader>
        <CardBody className="space-y-4">
          <Input
            type="number"
            label="混合代理端口"
            placeholder="2080"
            description="默认的 HTTP/SOCKS5 混合代理端口"
            value={String(formData.mixed_port)}
            onChange={(e) => setFormData({ ...formData, mixed_port: parseInt(e.target.value) || 2080 })}
          />
          <div className="flex items-center justify-between p-3 bg-default-100 rounded-lg">
            <div>
              <p className="font-medium">TUN 模式</p>
              <p className="text-sm text-gray-500">启用 TUN 模式进行透明代理</p>
            </div>
            <Switch
              isSelected={formData.tun_enabled}
              onValueChange={(enabled) => setFormData({ ...formData, tun_enabled: enabled })}
            />
          </div>
        </CardBody>
      </Card>

      {/* 多端口管理 */}
      <Card>
        <CardHeader className="flex justify-between items-center">
          <div className="flex items-center">
            <Network className="w-5 h-5 mr-2" />
            <h2 className="text-lg font-semibold">多端口管理</h2>
          </div>
          <Button
            color="primary"
            size="sm"
            startContent={<Plus className="w-4 h-4" />}
            onPress={handleAddPort}
          >
            添加端口
          </Button>
        </CardHeader>
        <CardBody className="space-y-4">
          <p className="text-sm text-gray-500">
            配置多个入站端口，每个端口可以使用不同的出站线路，支持用户名密码认证。适用于多设备、多场景分流需求。
          </p>

          {inboundPorts.length === 0 ? (
            <div className="text-center py-8 text-gray-500">
              <Network className="w-12 h-12 mx-auto mb-4 opacity-50" />
              <p>暂无自定义入站端口</p>
              <p className="text-sm mt-1">点击"添加端口"创建新的入站端口</p>
            </div>
          ) : (
            <div className="space-y-3">
              {inboundPorts.map((port) => (
                <div
                  key={port.id}
                  className="flex items-center justify-between p-4 bg-default-100 rounded-lg"
                >
                  <div className="flex-1">
                    <div className="flex items-center gap-2">
                      <span className="font-medium">{port.name}</span>
                      <Chip size="sm" variant="flat">{port.type}</Chip>
                      {port.auth && <Chip size="sm" color="warning" variant="flat">需认证</Chip>}
                      {!port.enabled && <Chip size="sm" variant="flat">已禁用</Chip>}
                    </div>
                    <p className="text-sm text-gray-500 mt-1">
                      {port.listen}:{port.port} → {port.outbound}
                    </p>
                  </div>
                  <div className="flex items-center gap-1">
                    <Button isIconOnly size="sm" variant="light" onPress={() => handleEditPort(port)}>
                      <Pencil className="w-4 h-4" />
                    </Button>
                    <Button isIconOnly size="sm" variant="light" color="danger" onPress={() => handleDeletePort(port.id)}>
                      <Trash2 className="w-4 h-4" />
                    </Button>
                    <Switch
                      size="sm"
                      isSelected={port.enabled}
                      onValueChange={() => handleTogglePort(port)}
                    />
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardBody>
      </Card>

      {/* 入站端口编辑弹窗 */}
      <Modal isOpen={isPortModalOpen} onClose={onPortModalClose} size="4xl" scrollBehavior="inside">
        <ModalContent>
          <ModalHeader>{editingPort ? '编辑入站端口' : '添加入站端口'}</ModalHeader>
          <ModalBody className="gap-4">
            <Input
              label="端口名称"
              placeholder="例如：家人专用、办公室"
              value={portFormData.name}
              onChange={(e) => setPortFormData({ ...portFormData, name: e.target.value })}
            />
            <div className="grid grid-cols-3 gap-4">
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
              />
            </div>

            {/* 出站线路选择 - 分栏布局 */}
            <Card className="mt-2">
              <CardHeader className="flex justify-between items-center pb-2">
                <h3 className="font-medium">出站线路</h3>
                <div className="flex items-center gap-2">
                  <span className="text-sm text-gray-500">当前选择：</span>
                  <Chip color="primary" variant="flat">
                    {getOutboundDisplayName(portFormData.outbound)}
                  </Chip>
                </div>
              </CardHeader>
              <CardBody className="pt-0">
                {/* 筛选器 */}
                <div className="flex gap-2 mb-3">
                  <Select
                    placeholder="类型"
                    size="sm"
                    selectedKeys={outboundType ? [outboundType] : []}
                    onSelectionChange={(keys) => {
                      const selected = Array.from(keys)[0] as string;
                      setOutboundType(selected || '');
                    }}
                    className="w-28"
                  >
                    <SelectItem key="basic">基础出站</SelectItem>
                    <SelectItem key="country">地区节点</SelectItem>
                    <SelectItem key="filter">过滤器</SelectItem>
                    <SelectItem key="chain">链路</SelectItem>
                    <SelectItem key="node">单独节点</SelectItem>
                  </Select>
                  {(outboundType === 'node' || outboundType === '') && (
                    <>
                      <Select
                        placeholder="来源"
                        size="sm"
                        selectedKeys={selectedSource ? [selectedSource] : []}
                        onSelectionChange={(keys) => {
                          const selected = Array.from(keys)[0] as string;
                          setSelectedSource(selected || '');
                        }}
                        className="w-28"
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
                        }}
                        className="w-28"
                      >
                        {availableCountries.map((opt) => (
                          <SelectItem key={opt.code} textValue={opt.name}>
                            {opt.emoji} {opt.name}
                          </SelectItem>
                        ))}
                      </Select>
                    </>
                  )}
                  <Input
                    placeholder="搜索..."
                    size="sm"
                    value={searchText}
                    onChange={(e) => setSearchText(e.target.value)}
                    className="flex-1"
                  />
                  <Button
                    size="sm"
                    variant="light"
                    startContent={<RefreshCw className="w-3 h-3" />}
                    onPress={() => {
                      fetchNodes();
                      fetchNodeGroups();
                      fetchCountryGroups();
                      fetchFilters();
                      fetchProxyChains();
                    }}
                  >
                    刷新
                  </Button>
                </div>

                {/* 出站选项列表 */}
                <div className="max-h-72 overflow-y-auto">
                  {/* 基础出站 */}
                  {(outboundType === '' || outboundType === 'basic') && (
                    <div className="mb-3">
                      <div className="flex items-center gap-2 mb-2">
                        <Chip size="sm" color="primary" variant="flat">基础出站</Chip>
                        <span className="text-xs text-gray-500">3 个选项</span>
                      </div>
                      <div className="space-y-1">
                        {[
                          { key: 'Proxy', label: 'Proxy（主代理）' },
                          { key: 'DIRECT', label: 'DIRECT（直连）' },
                          { key: 'Auto', label: 'Auto（自动选择）' },
                        ].filter(item => !searchText || item.label.toLowerCase().includes(searchText.toLowerCase()))
                          .map((item) => (
                          <div
                            key={item.key}
                            className={`flex items-center justify-between p-2 hover:bg-default-100 rounded-lg cursor-pointer ${portFormData.outbound === item.key ? 'bg-primary-100' : ''}`}
                            onClick={() => selectOutbound(item.key)}
                          >
                            <span className="text-sm">{item.label}</span>
                            {portFormData.outbound === item.key && (
                              <Chip size="sm" color="primary">已选择</Chip>
                            )}
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* 地区节点 */}
                  {(outboundType === '' || outboundType === 'country') && countryGroups.length > 0 && (
                    <div className="mb-3">
                      <div className="flex items-center gap-2 mb-2">
                        <Chip size="sm" color="secondary" variant="flat">地区节点</Chip>
                        <span className="text-xs text-gray-500">{countryGroups.length} 个地区</span>
                      </div>
                      <div className="space-y-1">
                        {countryGroups
                          .filter(country => !searchText ||
                            country.name.toLowerCase().includes(searchText.toLowerCase()) ||
                            country.code.toLowerCase().includes(searchText.toLowerCase())
                          )
                          .map((country) => (
                          <div
                            key={country.code}
                            className={`flex items-center justify-between p-2 hover:bg-default-100 rounded-lg cursor-pointer ${portFormData.outbound === country.code ? 'bg-primary-100' : ''}`}
                            onClick={() => selectOutbound(country.code)}
                          >
                            <div className="flex items-center gap-2">
                              <span>{country.emoji}</span>
                              <span className="text-sm">{country.name}</span>
                              <span className="text-xs text-gray-500">({country.node_count} 节点)</span>
                            </div>
                            {portFormData.outbound === country.code && (
                              <Chip size="sm" color="primary">已选择</Chip>
                            )}
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* 过滤器 */}
                  {(outboundType === '' || outboundType === 'filter') && enabledFilters.length > 0 && (
                    <div className="mb-3">
                      <div className="flex items-center gap-2 mb-2">
                        <Chip size="sm" color="warning" variant="flat">过滤器</Chip>
                        <span className="text-xs text-gray-500">{enabledFilters.length} 个</span>
                      </div>
                      <div className="space-y-1">
                        {enabledFilters
                          .filter(filter => !searchText || filter.name.toLowerCase().includes(searchText.toLowerCase()))
                          .map((filter) => (
                          <div
                            key={filter.name}
                            className={`flex items-center justify-between p-2 hover:bg-default-100 rounded-lg cursor-pointer ${portFormData.outbound === filter.name ? 'bg-primary-100' : ''}`}
                            onClick={() => selectOutbound(filter.name)}
                          >
                            <span className="text-sm">{filter.name}</span>
                            {portFormData.outbound === filter.name && (
                              <Chip size="sm" color="primary">已选择</Chip>
                            )}
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* 代理链路 */}
                  {(outboundType === '' || outboundType === 'chain') && enabledChains.length > 0 && (
                    <div className="mb-3">
                      <div className="flex items-center gap-2 mb-2">
                        <Chip size="sm" color="success" variant="flat">代理链路</Chip>
                        <span className="text-xs text-gray-500">{enabledChains.length} 个</span>
                      </div>
                      <div className="space-y-1">
                        {enabledChains
                          .filter(chain => !searchText || chain.name.toLowerCase().includes(searchText.toLowerCase()))
                          .map((chain) => (
                          <div
                            key={chain.name}
                            className={`flex items-center justify-between p-2 hover:bg-default-100 rounded-lg cursor-pointer ${portFormData.outbound === chain.name ? 'bg-primary-100' : ''}`}
                            onClick={() => selectOutbound(chain.name)}
                          >
                            <span className="text-sm">{chain.name}</span>
                            {portFormData.outbound === chain.name && (
                              <Chip size="sm" color="primary">已选择</Chip>
                            )}
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* 单独节点 - 按来源分组 */}
                  {(outboundType === '' || outboundType === 'node') && filteredNodeGroups.length > 0 && (
                    <Accordion
                      selectionMode="multiple"
                      defaultExpandedKeys={filteredNodeGroups.map(g => `node-${g.source}`)}
                      className="p-0"
                    >
                      {filteredNodeGroups.map((group) => (
                        <AccordionItem
                          key={`node-${group.source}`}
                          title={
                            <div className="flex items-center gap-2">
                              <Chip
                                size="sm"
                                color={group.source === 'manual' ? 'primary' : 'default'}
                                variant="flat"
                              >
                                {group.source_name}
                              </Chip>
                              <span className="text-xs text-gray-500">{group.nodes.length} 个节点</span>
                            </div>
                          }
                          classNames={{ content: "p-0" }}
                        >
                          <div className="space-y-1">
                            {group.nodes.slice(0, 50).map((node) => (
                              <div
                                key={node.tag}
                                className={`flex items-center justify-between p-2 hover:bg-default-100 rounded-lg cursor-pointer ${portFormData.outbound === node.tag ? 'bg-primary-100' : ''}`}
                                onClick={() => selectOutbound(node.tag)}
                              >
                                <div className="flex items-center gap-2 flex-1 min-w-0">
                                  {node.country_emoji && <span>{node.country_emoji}</span>}
                                  <span className="text-sm truncate">{node.tag}</span>
                                </div>
                                <div className="flex items-center gap-2">
                                  <Chip size="sm" variant="flat">{node.type}</Chip>
                                  {portFormData.outbound === node.tag && (
                                    <Chip size="sm" color="primary">已选择</Chip>
                                  )}
                                </div>
                              </div>
                            ))}
                            {group.nodes.length > 50 && (
                              <p className="text-xs text-gray-500 text-center py-2">
                                还有 {group.nodes.length - 50} 个节点，请使用搜索
                              </p>
                            )}
                          </div>
                        </AccordionItem>
                      ))}
                    </Accordion>
                  )}
                </div>
              </CardBody>
            </Card>

            <div className="mt-2 pt-4 border-t border-divider">
              <p className="text-sm font-medium mb-3">用户认证（可选）</p>
              <div className="grid grid-cols-2 gap-4">
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

            <div className="flex items-center justify-between">
              <span>启用</span>
              <Switch
                isSelected={portFormData.enabled}
                onValueChange={(enabled) => setPortFormData({ ...portFormData, enabled })}
              />
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
