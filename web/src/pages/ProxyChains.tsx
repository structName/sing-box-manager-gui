import { useEffect, useState } from 'react';
import { Card, CardBody, CardHeader, Button, Chip, Modal, ModalContent, ModalHeader, ModalBody, ModalFooter, Input, Textarea, useDisclosure, Switch, Select, SelectItem, Accordion, AccordionItem } from '@nextui-org/react';
import { Plus, Link2, Trash2, Pencil, ArrowRight, ChevronUp, ChevronDown, Activity, RefreshCw } from 'lucide-react';
import { proxyChainApi, nodeApi } from '../api';
import { toast } from '../components/Toast';

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

// ChainNode 类型
interface ChainNode {
  original_tag: string;
  copy_tag: string;
  source: string;
}

// ProxyChain 类型
interface ProxyChain {
  id: string;
  name: string;
  description: string;
  nodes: string[];
  chain_nodes?: ChainNode[];
  enabled: boolean;
}

// Node 类型
interface Node {
  tag: string;
  type: string;
  server: string;
  country?: string;
  country_emoji?: string;
  source?: string;
  source_name?: string;
}

// NodeGroup 类型
interface NodeGroup {
  source: string;
  source_name: string;
  nodes: Node[];
}

// ChainHealthStatus 类型
interface ChainHealthStatus {
  chain_id: string;
  last_check: string;
  status: 'healthy' | 'degraded' | 'unhealthy';
  latency: number;
  node_statuses: {
    tag: string;
    status: string;
    latency: number;
    error?: string;
  }[];
}

export default function ProxyChains() {
  const [chains, setChains] = useState<ProxyChain[]>([]);
  const [nodes, setNodes] = useState<Node[]>([]);
  const [nodeGroups, setNodeGroups] = useState<NodeGroup[]>([]);
  const [healthStatuses, setHealthStatuses] = useState<Record<string, ChainHealthStatus>>({});
  const [loading, setLoading] = useState(true);
  const [testingChain, setTestingChain] = useState<string | null>(null);

  // 创建/编辑 Modal
  const { isOpen, onOpen, onClose } = useDisclosure();
  const [editingChain, setEditingChain] = useState<ProxyChain | null>(null);
  const [formData, setFormData] = useState({
    name: '',
    description: '',
    nodes: [] as string[],
    enabled: true,
  });
  const [submitting, setSubmitting] = useState(false);

  // 节点选择状态
  const [searchText, setSearchText] = useState('');
  const [selectedCountry, setSelectedCountry] = useState('');
  const [selectedSource, setSelectedSource] = useState('');

  useEffect(() => {
    fetchChains();
    fetchNodes();
    fetchNodeGroups();
    fetchAllHealth();
  }, []);

  const fetchChains = async () => {
    try {
      const res = await proxyChainApi.getAll();
      setChains(res.data.data || []);
    } catch (error: any) {
      toast.error('获取链路列表失败');
    } finally {
      setLoading(false);
    }
  };

  const fetchNodes = async () => {
    try {
      const res = await nodeApi.getAll();
      setNodes(res.data.data || []);
    } catch (error: any) {
      console.error('获取节点列表失败:', error);
    }
  };

  const fetchNodeGroups = async () => {
    try {
      const res = await nodeApi.getGrouped();
      setNodeGroups(res.data.data || []);
    } catch (error: any) {
      console.error('获取节点分组失败:', error);
    }
  };

  const fetchAllHealth = async () => {
    try {
      const res = await proxyChainApi.getAllHealth();
      setHealthStatuses(res.data.data || {});
    } catch (error: any) {
      console.error('获取健康状态失败:', error);
    }
  };

  const checkChainHealth = async (chainId: string) => {
    setTestingChain(chainId);
    try {
      const res = await proxyChainApi.checkHealth(chainId);
      setHealthStatuses(prev => ({
        ...prev,
        [chainId]: res.data.data
      }));
      toast.success('健康检测完成');
    } catch (error: any) {
      toast.error('健康检测失败');
    } finally {
      setTestingChain(null);
    }
  };

  const handleCreate = () => {
    setEditingChain(null);
    setFormData({ name: '', description: '', nodes: [], enabled: true });
    setSearchText('');
    setSelectedCountry('');
    setSelectedSource('');
    onOpen();
  };

  const handleEdit = (chain: ProxyChain) => {
    setEditingChain(chain);
    setFormData({
      name: chain.name,
      description: chain.description,
      nodes: [...chain.nodes],
      enabled: chain.enabled,
    });
    setSearchText('');
    setSelectedCountry('');
    setSelectedSource('');
    onOpen();
  };

  const handleSubmit = async () => {
    if (!formData.name.trim()) {
      toast.error('请输入链路名称');
      return;
    }

    if (formData.nodes.length < 2) {
      toast.error('链路至少需要2个节点');
      return;
    }

    setSubmitting(true);
    try {
      if (editingChain) {
        await proxyChainApi.update(editingChain.id, formData);
        toast.success('链路已更新');
      } else {
        await proxyChainApi.add(formData);
        toast.success('链路已创建');
      }
      onClose();
      fetchChains();
    } catch (error: any) {
      toast.error(error.response?.data?.error || '操作失败');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async (chain: ProxyChain) => {
    if (!confirm(`确定要删除链路 "${chain.name}" 吗？`)) {
      return;
    }

    try {
      await proxyChainApi.delete(chain.id);
      toast.success('链路已删除');
      fetchChains();
    } catch (error: any) {
      toast.error(error.response?.data?.error || '删除失败');
    }
  };

  const handleToggle = async (chain: ProxyChain) => {
    try {
      await proxyChainApi.update(chain.id, { ...chain, enabled: !chain.enabled });
      fetchChains();
    } catch (error: any) {
      toast.error(error.response?.data?.error || '更新失败');
    }
  };

  // 添加节点到链路
  const addNodeToChain = (nodeTag: string) => {
    if (!formData.nodes.includes(nodeTag)) {
      setFormData({ ...formData, nodes: [...formData.nodes, nodeTag] });
    }
  };

  // 从链路移除节点
  const removeNodeFromChain = (index: number) => {
    const newNodes = [...formData.nodes];
    newNodes.splice(index, 1);
    setFormData({ ...formData, nodes: newNodes });
  };

  // 移动节点位置
  const moveNode = (index: number, direction: 'up' | 'down') => {
    const newNodes = [...formData.nodes];
    const newIndex = direction === 'up' ? index - 1 : index + 1;
    if (newIndex >= 0 && newIndex < newNodes.length) {
      [newNodes[index], newNodes[newIndex]] = [newNodes[newIndex], newNodes[index]];
      setFormData({ ...formData, nodes: newNodes });
    }
  };

  // 过滤可用节点（从分组数据中过滤）
  const getFilteredNodesByGroup = () => {
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
        const notInChain = !formData.nodes.includes(node.tag);
        return matchSearch && matchCountry && notInChain;
      })
    })).filter(group => group.nodes.length > 0);
  };

  // 获取当前节点中存在的国家列表
  const availableCountries = countryOptions.filter(
    opt => nodes.some(node => node.country === opt.code && !formData.nodes.includes(node.tag))
  );

  // 获取节点信息
  const getNodeInfo = (tag: string): Node | undefined => {
    return nodes.find(n => n.tag === tag);
  };

  // 获取健康状态颜色
  const getHealthColor = (status?: string) => {
    switch (status) {
      case 'healthy': return 'success';
      case 'degraded': return 'warning';
      case 'unhealthy': return 'danger';
      default: return 'default';
    }
  };

  // 获取健康状态文本
  const getHealthText = (status?: string) => {
    switch (status) {
      case 'healthy': return '正常';
      case 'degraded': return '部分可用';
      case 'unhealthy': return '不可用';
      default: return '未检测';
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <p className="text-gray-500">加载中...</p>
      </div>
    );
  }

  const filteredGroups = getFilteredNodesByGroup();

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-gray-800 dark:text-white">代理链路</h1>
          <p className="text-sm text-gray-500 mt-1">
            配置多级中转链路，实现 机场节点 → 自建中转 → 最终出口 的级联代理
          </p>
        </div>
        <Button
          color="primary"
          startContent={<Plus className="w-4 h-4" />}
          onPress={handleCreate}
        >
          新建链路
        </Button>
      </div>

      {/* 链路列表 */}
      {chains.length === 0 ? (
        <Card>
          <CardBody className="text-center py-12">
            <Link2 className="w-12 h-12 mx-auto text-gray-400 mb-4" />
            <p className="text-gray-500 mb-4">暂无代理链路</p>
            <p className="text-sm text-gray-400 mb-4">
              创建链路可以让流量依次通过多个节点转发，例如：<br />
              机场节点 → 自建 VPS → 最终出口
            </p>
            <Button color="primary" onPress={handleCreate}>
              创建第一个链路
            </Button>
          </CardBody>
        </Card>
      ) : (
        <div className="space-y-4">
          {chains.map((chain) => {
            const health = healthStatuses[chain.id];
            return (
              <Card key={chain.id}>
                <CardBody className="p-4">
                  <div className="flex justify-between items-start">
                    <div className="flex-1">
                      <div className="flex items-center gap-2 mb-2">
                        <Link2 className="w-5 h-5 text-primary" />
                        <h3 className="font-semibold text-lg">{chain.name}</h3>
                        {!chain.enabled && (
                          <Chip size="sm" variant="flat">已禁用</Chip>
                        )}
                        {/* 健康状态指示器 */}
                        {health && (
                          <Chip size="sm" color={getHealthColor(health.status)} variant="flat">
                            {getHealthText(health.status)}
                            {health.latency > 0 && ` ${health.latency}ms`}
                          </Chip>
                        )}
                      </div>
                      {chain.description && (
                        <p className="text-sm text-gray-500 mb-3">{chain.description}</p>
                      )}

                      {/* 链路可视化 */}
                      <div className="flex items-center gap-2 flex-wrap">
                        {chain.nodes.map((nodeTag, index) => {
                          const node = getNodeInfo(nodeTag);
                          const nodeHealth = health?.node_statuses?.find(ns => ns.tag === nodeTag);
                          return (
                            <div key={index} className="flex items-center gap-2">
                              <Chip
                                variant="bordered"
                                startContent={node?.country_emoji}
                                color={nodeHealth ? getHealthColor(nodeHealth.status) : 'default'}
                              >
                                {nodeTag}
                                {nodeHealth && nodeHealth.latency && nodeHealth.latency > 0 && (
                                  <span className="text-xs ml-1 opacity-70">{nodeHealth.latency}ms</span>
                                )}
                              </Chip>
                              {index < chain.nodes.length - 1 && (
                                <ArrowRight className="w-4 h-4 text-gray-400" />
                              )}
                            </div>
                          );
                        })}
                      </div>
                    </div>

                    <div className="flex items-center gap-1">
                      <Button
                        isIconOnly
                        size="sm"
                        variant="light"
                        onPress={() => checkChainHealth(chain.id)}
                        isLoading={testingChain === chain.id}
                        title="测速"
                      >
                        <Activity className="w-4 h-4" />
                      </Button>
                      <Button isIconOnly size="sm" variant="light" onPress={() => handleEdit(chain)}>
                        <Pencil className="w-4 h-4" />
                      </Button>
                      <Button isIconOnly size="sm" variant="light" color="danger" onPress={() => handleDelete(chain)}>
                        <Trash2 className="w-4 h-4" />
                      </Button>
                      <Switch
                        size="sm"
                        isSelected={chain.enabled}
                        onValueChange={() => handleToggle(chain)}
                      />
                    </div>
                  </div>
                </CardBody>
              </Card>
            );
          })}
        </div>
      )}

      {/* 创建/编辑 Modal */}
      <Modal isOpen={isOpen} onClose={onClose} size="4xl">
        <ModalContent>
          <ModalHeader>
            {editingChain ? '编辑链路' : '新建链路'}
          </ModalHeader>
          <ModalBody className="gap-4">
            <Input
              label="链路名称"
              placeholder="例如：香港中转链"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
            />
            <Textarea
              label="描述（可选）"
              placeholder="链路的用途说明"
              value={formData.description}
              onChange={(e) => setFormData({ ...formData, description: e.target.value })}
              minRows={2}
            />

            <div className="grid grid-cols-2 gap-4 mt-2">
              {/* 已选节点 */}
              <Card>
                <CardHeader>
                  <h3 className="font-medium">链路节点（{formData.nodes.length}）</h3>
                </CardHeader>
                <CardBody className="pt-0">
                  <p className="text-xs text-gray-500 mb-3">
                    从上到下依次为：入口节点 → 中转节点 → ... → 出口节点
                  </p>
                  {formData.nodes.length === 0 ? (
                    <p className="text-gray-500 text-center py-4">
                      从右侧添加节点到链路
                    </p>
                  ) : (
                    <div className="space-y-2 max-h-64 overflow-y-auto">
                      {formData.nodes.map((nodeTag, index) => {
                        const node = getNodeInfo(nodeTag);
                        return (
                          <div
                            key={index}
                            className="flex items-center justify-between p-2 bg-default-100 rounded-lg"
                          >
                            <div className="flex items-center gap-2 flex-1 min-w-0">
                              <span className="text-xs text-gray-500 w-6">{index + 1}.</span>
                              {node?.country_emoji && <span>{node.country_emoji}</span>}
                              <div className="flex flex-col min-w-0">
                                <span className="text-sm truncate">{nodeTag}</span>
                                {node?.source_name && (
                                  <span className="text-xs text-gray-400 truncate">{node.source_name}</span>
                                )}
                              </div>
                            </div>
                            <div className="flex items-center gap-1">
                              <Button
                                isIconOnly
                                size="sm"
                                variant="light"
                                isDisabled={index === 0}
                                onPress={() => moveNode(index, 'up')}
                              >
                                <ChevronUp className="w-4 h-4" />
                              </Button>
                              <Button
                                isIconOnly
                                size="sm"
                                variant="light"
                                isDisabled={index === formData.nodes.length - 1}
                                onPress={() => moveNode(index, 'down')}
                              >
                                <ChevronDown className="w-4 h-4" />
                              </Button>
                              <Button
                                isIconOnly
                                size="sm"
                                variant="light"
                                color="danger"
                                onPress={() => removeNodeFromChain(index)}
                              >
                                <Trash2 className="w-4 h-4" />
                              </Button>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  )}

                  {/* 链路预览 - 显示副本 Tag */}
                  {formData.nodes.length >= 2 && formData.name && (
                    <div className="mt-4 pt-3 border-t border-divider">
                      <p className="text-xs text-gray-500 mb-2">生成的副本节点：</p>
                      <div className="space-y-1">
                        {formData.nodes.map((tag, index) => (
                          <div key={index} className="flex items-center gap-2 text-xs">
                            <span className="text-gray-400">{index + 1}.</span>
                            <span className="font-mono text-primary">{formData.name}-{tag}</span>
                            {index < formData.nodes.length - 1 && (
                              <span className="text-gray-400">→ detour: {formData.name}-{formData.nodes[index + 1]}</span>
                            )}
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                </CardBody>
              </Card>

              {/* 可用节点 - 按来源分组 */}
              <Card>
                <CardHeader className="flex justify-between items-center">
                  <h3 className="font-medium">可用节点</h3>
                  <Button
                    size="sm"
                    variant="light"
                    startContent={<RefreshCw className="w-3 h-3" />}
                    onPress={fetchNodeGroups}
                  >
                    刷新
                  </Button>
                </CardHeader>
                <CardBody className="pt-0">
                  <div className="flex gap-2 mb-3">
                    <Select
                      placeholder="筛选来源"
                      size="sm"
                      selectedKeys={selectedSource ? [selectedSource] : []}
                      onSelectionChange={(keys) => {
                        const selected = Array.from(keys)[0] as string;
                        setSelectedSource(selected || '');
                      }}
                      className="w-32"
                    >
                      {nodeGroups.map((group) => (
                        <SelectItem key={group.source} textValue={group.source_name}>
                          {group.source_name} ({group.nodes.length})
                        </SelectItem>
                      ))}
                    </Select>
                    <Select
                      placeholder="筛选国家"
                      size="sm"
                      selectedKeys={selectedCountry ? [selectedCountry] : []}
                      onSelectionChange={(keys) => {
                        const selected = Array.from(keys)[0] as string;
                        setSelectedCountry(selected || '');
                      }}
                      className="w-32"
                    >
                      {availableCountries.map((opt) => (
                        <SelectItem key={opt.code} textValue={opt.name}>
                          {opt.emoji} {opt.name}
                        </SelectItem>
                      ))}
                    </Select>
                    <Input
                      placeholder="搜索节点..."
                      size="sm"
                      value={searchText}
                      onChange={(e) => setSearchText(e.target.value)}
                      className="flex-1"
                    />
                  </div>

                  {/* 按来源分组显示节点 */}
                  <div className="max-h-72 overflow-y-auto">
                    {filteredGroups.length === 0 ? (
                      <p className="text-gray-500 text-center py-4">
                        没有可用节点
                      </p>
                    ) : (
                      <Accordion
                        selectionMode="multiple"
                        defaultExpandedKeys={filteredGroups.map(g => g.source)}
                        className="p-0"
                      >
                        {filteredGroups.map((group) => (
                          <AccordionItem
                            key={group.source}
                            title={
                              <div className="flex items-center gap-2">
                                <Chip
                                  size="sm"
                                  color={group.source === 'manual' ? 'primary' : 'secondary'}
                                  variant="flat"
                                >
                                  {group.source_name}
                                </Chip>
                                <span className="text-xs text-gray-500">
                                  {group.nodes.length} 个节点
                                </span>
                              </div>
                            }
                            classNames={{
                              content: "p-0",
                            }}
                          >
                            <div className="space-y-1">
                              {group.nodes.slice(0, 30).map((node) => (
                                <div
                                  key={node.tag}
                                  className="flex items-center justify-between p-2 hover:bg-default-100 rounded-lg cursor-pointer"
                                  onClick={() => addNodeToChain(node.tag)}
                                >
                                  <div className="flex items-center gap-2 flex-1 min-w-0">
                                    {node.country_emoji && <span>{node.country_emoji}</span>}
                                    <span className="text-sm truncate">{node.tag}</span>
                                  </div>
                                  <Chip size="sm" variant="flat">{node.type}</Chip>
                                </div>
                              ))}
                              {group.nodes.length > 30 && (
                                <p className="text-xs text-gray-500 text-center py-2">
                                  还有 {group.nodes.length - 30} 个节点，请使用搜索
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
            </div>

            <div className="flex items-center justify-between">
              <span>启用链路</span>
              <Switch
                isSelected={formData.enabled}
                onValueChange={(enabled) => setFormData({ ...formData, enabled })}
              />
            </div>
          </ModalBody>
          <ModalFooter>
            <Button variant="flat" onPress={onClose}>
              取消
            </Button>
            <Button
              color="primary"
              onPress={handleSubmit}
              isLoading={submitting}
              isDisabled={!formData.name.trim() || formData.nodes.length < 2}
            >
              {editingChain ? '保存' : '创建'}
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </div>
  );
}
