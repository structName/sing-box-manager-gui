import { useEffect, useState } from 'react';
import { Card, CardBody, CardHeader, Button, Chip, Modal, ModalContent, ModalHeader, ModalBody, ModalFooter, Input, Textarea, useDisclosure, Switch, Select, SelectItem } from '@nextui-org/react';
import { Plus, Link2, Trash2, Pencil, ArrowRight, ChevronUp, ChevronDown } from 'lucide-react';
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

// ProxyChain 类型
interface ProxyChain {
  id: string;
  name: string;
  description: string;
  nodes: string[];
  enabled: boolean;
}

// Node 类型
interface Node {
  tag: string;
  type: string;
  server: string;
  country?: string;
  country_emoji?: string;
}

export default function ProxyChains() {
  const [chains, setChains] = useState<ProxyChain[]>([]);
  const [nodes, setNodes] = useState<Node[]>([]);
  const [loading, setLoading] = useState(true);

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

  useEffect(() => {
    fetchChains();
    fetchNodes();
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

  const handleCreate = () => {
    setEditingChain(null);
    setFormData({ name: '', description: '', nodes: [], enabled: true });
    setSearchText('');
    setSelectedCountry('');
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

  // 过滤可用节点
  const filteredNodes = nodes.filter(node => {
    const matchSearch = !searchText ||
      node.tag.toLowerCase().includes(searchText.toLowerCase()) ||
      node.server.toLowerCase().includes(searchText.toLowerCase());
    const matchCountry = !selectedCountry || node.country === selectedCountry;
    const notInChain = !formData.nodes.includes(node.tag);
    return matchSearch && matchCountry && notInChain;
  });

  // 获取当前节点中存在的国家列表
  const availableCountries = countryOptions.filter(
    opt => nodes.some(node => node.country === opt.code && !formData.nodes.includes(node.tag))
  );

  // 获取节点信息
  const getNodeInfo = (tag: string): Node | undefined => {
    return nodes.find(n => n.tag === tag);
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <p className="text-gray-500">加载中...</p>
      </div>
    );
  }

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
          {chains.map((chain) => (
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
                    </div>
                    {chain.description && (
                      <p className="text-sm text-gray-500 mb-3">{chain.description}</p>
                    )}

                    {/* 链路可视化 */}
                    <div className="flex items-center gap-2 flex-wrap">
                      {chain.nodes.map((nodeTag, index) => {
                        const node = getNodeInfo(nodeTag);
                        return (
                          <div key={index} className="flex items-center gap-2">
                            <Chip
                              variant="bordered"
                              startContent={node?.country_emoji}
                            >
                              {nodeTag}
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
          ))}
        </div>
      )}

      {/* 创建/编辑 Modal */}
      <Modal isOpen={isOpen} onClose={onClose} size="3xl">
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
                            <div className="flex items-center gap-2">
                              <span className="text-xs text-gray-500 w-6">{index + 1}.</span>
                              {node?.country_emoji && <span>{node.country_emoji}</span>}
                              <span className="text-sm">{nodeTag}</span>
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

                  {/* 链路预览 */}
                  {formData.nodes.length >= 2 && (
                    <div className="mt-4 pt-3 border-t border-divider">
                      <p className="text-xs text-gray-500 mb-2">链路预览：</p>
                      <div className="flex items-center gap-1 flex-wrap">
                        {formData.nodes.map((tag, index) => (
                          <div key={index} className="flex items-center gap-1">
                            <Chip size="sm" variant="flat">
                              {getNodeInfo(tag)?.country_emoji || ''} {tag}
                            </Chip>
                            {index < formData.nodes.length - 1 && (
                              <ArrowRight className="w-3 h-3 text-gray-400" />
                            )}
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                </CardBody>
              </Card>

              {/* 可用节点 */}
              <Card>
                <CardHeader>
                  <h3 className="font-medium">可用节点（{filteredNodes.length}）</h3>
                </CardHeader>
                <CardBody className="pt-0">
                  <div className="flex gap-2 mb-3">
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
                  <div className="space-y-1 max-h-64 overflow-y-auto">
                    {filteredNodes.length === 0 ? (
                      <p className="text-gray-500 text-center py-4">
                        没有可用节点
                      </p>
                    ) : (
                      filteredNodes.slice(0, 50).map((node) => (
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
                      ))
                    )}
                    {filteredNodes.length > 50 && (
                      <p className="text-xs text-gray-500 text-center py-2">
                        还有 {filteredNodes.length - 50} 个节点，请使用搜索
                      </p>
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
