import { useEffect, useState, useCallback } from 'react';
import {
  Card,
  CardBody,
  CardHeader,
  Button,
  Input,
  Modal,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  useDisclosure,
  Chip,
  Select,
  SelectItem,
  Switch,
  Spinner,
  Table,
  TableHeader,
  TableColumn,
  TableBody,
  TableRow,
  TableCell,
  Tabs,
  Tab,
  Tooltip,
  Accordion,
  AccordionItem,
} from '@nextui-org/react';
import {
  Tag as TagIcon,
  Plus,
  Pencil,
  Trash2,
  Play,
  Filter,
} from 'lucide-react';
import { tagApi } from '../api';
import { toast } from '../components/Toast';

// 类型定义
interface Tag {
  ID: number;
  name: string;
  color: string;
  description: string;
  tag_group: string;
  priority: number;
  created_at: string;
}

interface TagCondition {
  field: string;
  operator: string;
  value: any;
}

interface TagConditions {
  logic: string;
  conditions: TagCondition[];
}

interface TagRule {
  ID: number;
  name: string;
  description: string;
  tag_id: number;
  tag?: Tag;
  conditions: TagConditions;
  trigger_type: string;
  priority: number;
  enabled: boolean;
  created_at: string;
}

// 颜色选项
const colorOptions = [
  { value: 'default', label: '默认', bg: 'bg-gray-500' },
  { value: 'primary', label: '蓝色', bg: 'bg-blue-500' },
  { value: 'secondary', label: '紫色', bg: 'bg-purple-500' },
  { value: 'success', label: '绿色', bg: 'bg-green-500' },
  { value: 'warning', label: '黄色', bg: 'bg-yellow-500' },
  { value: 'danger', label: '红色', bg: 'bg-red-500' },
];

// 条件字段选项（参考 sublinkPro 扩展）
const conditionFieldOptions = [
  // 测速相关
  { value: 'delay', label: '延迟 (ms)', type: 'number' },
  { value: 'speed', label: '速度 (MB/s)', type: 'number' },
  { value: 'delay_status', label: '延迟状态', type: 'status' },
  { value: 'speed_status', label: '速度状态', type: 'status' },
  // 地理信息
  { value: 'country', label: '国家代码', type: 'string' },
  { value: 'landing_ip', label: '落地 IP', type: 'string' },
  // 节点属性
  { value: 'name', label: '节点名称', type: 'string' },
  { value: 'type', label: '协议类型', type: 'protocol' },
  { value: 'server', label: '服务器地址', type: 'string' },
  { value: 'server_port', label: '端口', type: 'number' },
  { value: 'source', label: '来源', type: 'string' },
  { value: 'source_name', label: '来源名称', type: 'string' },
];

// 状态选项
const statusOptions = [
  { value: 'untested', label: '未测试' },
  { value: 'success', label: '成功' },
  { value: 'timeout', label: '超时' },
  { value: 'error', label: '失败' },
];

// 协议类型选项
const protocolOptions = [
  { value: 'ss', label: 'Shadowsocks' },
  { value: 'vmess', label: 'VMess' },
  { value: 'vless', label: 'VLESS' },
  { value: 'trojan', label: 'Trojan' },
  { value: 'hysteria2', label: 'Hysteria2' },
  { value: 'tuic', label: 'TUIC' },
];

// 常用国家代码选项
const countryOptions = [
  { value: 'HK', label: '🇭🇰 香港' },
  { value: 'TW', label: '🇹🇼 台湾' },
  { value: 'JP', label: '🇯🇵 日本' },
  { value: 'SG', label: '🇸🇬 新加坡' },
  { value: 'US', label: '🇺🇸 美国' },
  { value: 'KR', label: '🇰🇷 韩国' },
  { value: 'DE', label: '🇩🇪 德国' },
  { value: 'GB', label: '🇬🇧 英国' },
  { value: 'FR', label: '🇫🇷 法国' },
  { value: 'AU', label: '🇦🇺 澳大利亚' },
];

// 操作符选项
const operatorOptions = [
  { value: 'eq', label: '等于' },
  { value: 'ne', label: '不等于' },
  { value: 'gt', label: '大于' },
  { value: 'lt', label: '小于' },
  { value: 'gte', label: '大于等于' },
  { value: 'lte', label: '小于等于' },
  { value: 'contains', label: '包含' },
  { value: 'not_contains', label: '不包含' },
  { value: 'regex', label: '正则匹配' },
  { value: 'in', label: '在列表中' },
  { value: 'not_in', label: '不在列表中' },
];

// 触发类型选项
const triggerTypeOptions = [
  { value: 'manual', label: '手动触发' },
  { value: 'speed_test', label: '测速完成后' },
  { value: 'subscription_update', label: '订阅更新后' },
];

// 默认表单
const defaultTagForm = {
  name: '',
  color: 'default',
  description: '',
  tag_group: '',
  priority: 0,
};

const defaultCondition: TagCondition = {
  field: 'delay',
  operator: 'lt',
  value: '',
};

const defaultRuleForm = {
  name: '',
  description: '',
  tag_id: 0,
  conditions: {
    logic: 'AND',
    conditions: [{ ...defaultCondition }],
  } as TagConditions,
  trigger_type: 'manual',
  priority: 0,
  enabled: true,
};

export default function Tags() {
  // 状态
  const [tags, setTags] = useState<Tag[]>([]);
  const [rules, setRules] = useState<TagRule[]>([]);
  const [groups, setGroups] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState('tags');

  // 标签弹窗
  const { isOpen: isTagOpen, onOpen: onTagOpen, onClose: onTagClose } = useDisclosure();
  const [editingTag, setEditingTag] = useState<Tag | null>(null);
  const [tagForm, setTagForm] = useState(defaultTagForm);
  const [isSubmitting, setIsSubmitting] = useState(false);

  // 规则弹窗
  const { isOpen: isRuleOpen, onOpen: onRuleOpen, onClose: onRuleClose } = useDisclosure();
  const [editingRule, setEditingRule] = useState<TagRule | null>(null);
  const [ruleForm, setRuleForm] = useState(defaultRuleForm);

  // 加载数据
  const loadTags = useCallback(async () => {
    try {
      const response = await tagApi.getTags();
      setTags(response.data.data || []);
    } catch (error: any) {
      toast.error(error.response?.data?.error || '加载标签失败');
    }
  }, []);

  const loadRules = useCallback(async () => {
    try {
      const response = await tagApi.getRules();
      setRules(response.data.data || []);
    } catch (error: any) {
      toast.error(error.response?.data?.error || '加载规则失败');
    }
  }, []);

  const loadGroups = useCallback(async () => {
    try {
      const response = await tagApi.getGroups();
      setGroups(response.data.data || []);
    } catch (error: any) {
      console.error('加载标签组失败:', error);
    }
  }, []);

  const loadAll = useCallback(async () => {
    setLoading(true);
    await Promise.all([loadTags(), loadRules(), loadGroups()]);
    setLoading(false);
  }, [loadTags, loadRules, loadGroups]);

  useEffect(() => {
    loadAll();
  }, [loadAll]);

  // 标签操作
  const handleCreateTag = () => {
    setEditingTag(null);
    setTagForm(defaultTagForm);
    onTagOpen();
  };

  const handleEditTag = (tag: Tag) => {
    setEditingTag(tag);
    setTagForm({
      name: tag.name,
      color: tag.color || 'default',
      description: tag.description || '',
      tag_group: tag.tag_group || '',
      priority: tag.priority || 0,
    });
    onTagOpen();
  };

  const handleSaveTag = async () => {
    if (!tagForm.name.trim()) {
      toast.error('请输入标签名称');
      return;
    }

    setIsSubmitting(true);
    try {
      if (editingTag) {
        await tagApi.updateTag(editingTag.ID, tagForm);
        toast.success('更新成功');
      } else {
        await tagApi.createTag(tagForm);
        toast.success('创建成功');
      }
      onTagClose();
      loadTags();
      loadGroups();
    } catch (error: any) {
      toast.error(error.response?.data?.error || '保存失败');
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleDeleteTag = async (tag: Tag) => {
    if (!confirm(`确定要删除标签 "${tag.name}" 吗？`)) return;

    try {
      await tagApi.deleteTag(tag.ID);
      toast.success('删除成功');
      loadTags();
    } catch (error: any) {
      toast.error(error.response?.data?.error || '删除失败');
    }
  };

  // 规则操作
  const handleCreateRule = () => {
    setEditingRule(null);
    setRuleForm(defaultRuleForm);
    onRuleOpen();
  };

  const handleEditRule = (rule: TagRule) => {
    setEditingRule(rule);
    setRuleForm({
      name: rule.name,
      description: rule.description || '',
      tag_id: rule.tag_id,
      conditions: rule.conditions || { logic: 'AND', conditions: [{ ...defaultCondition }] },
      trigger_type: rule.trigger_type || 'manual',
      priority: rule.priority || 0,
      enabled: rule.enabled,
    });
    onRuleOpen();
  };

  const handleSaveRule = async () => {
    if (!ruleForm.name.trim()) {
      toast.error('请输入规则名称');
      return;
    }
    if (!ruleForm.tag_id) {
      toast.error('请选择目标标签');
      return;
    }

    setIsSubmitting(true);
    try {
      if (editingRule) {
        await tagApi.updateRule(editingRule.ID, ruleForm);
        toast.success('更新成功');
      } else {
        await tagApi.createRule(ruleForm);
        toast.success('创建成功');
      }
      onRuleClose();
      loadRules();
    } catch (error: any) {
      toast.error(error.response?.data?.error || '保存失败');
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleDeleteRule = async (rule: TagRule) => {
    if (!confirm(`确定要删除规则 "${rule.name}" 吗？`)) return;

    try {
      await tagApi.deleteRule(rule.ID);
      toast.success('删除成功');
      loadRules();
    } catch (error: any) {
      toast.error(error.response?.data?.error || '删除失败');
    }
  };

  // 应用规则
  const handleApplyRules = async (triggerType: string) => {
    try {
      const response = await tagApi.applyRules(triggerType);
      const result = response.data.data;
      toast.success(`应用完成：处理 ${result.processed_nodes} 个节点`);
      loadTags();
    } catch (error: any) {
      toast.error(error.response?.data?.error || '应用规则失败');
    }
  };

  // 条件管理
  const addCondition = () => {
    setRuleForm({
      ...ruleForm,
      conditions: {
        ...ruleForm.conditions,
        conditions: [...ruleForm.conditions.conditions, { ...defaultCondition }],
      },
    });
  };

  const updateCondition = (index: number, field: keyof TagCondition, value: any) => {
    const newConditions = [...ruleForm.conditions.conditions];
    newConditions[index] = { ...newConditions[index], [field]: value };
    setRuleForm({
      ...ruleForm,
      conditions: { ...ruleForm.conditions, conditions: newConditions },
    });
  };

  const removeCondition = (index: number) => {
    const newConditions = ruleForm.conditions.conditions.filter((_, i) => i !== index);
    setRuleForm({
      ...ruleForm,
      conditions: { ...ruleForm.conditions, conditions: newConditions },
    });
  };

  // 获取标签颜色
  const getTagColor = (color: string): 'default' | 'primary' | 'secondary' | 'success' | 'warning' | 'danger' => {
    const validColors = ['default', 'primary', 'secondary', 'success', 'warning', 'danger'];
    return validColors.includes(color) ? color as any : 'default';
  };

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-gray-800 dark:text-white">标签管理</h1>
          <p className="text-sm text-gray-500 mt-1">
            创建标签和自动打标规则，智能分类节点
          </p>
        </div>
        <div className="flex gap-2">
          <Button
            color="success"
            variant="flat"
            startContent={<Play className="w-4 h-4" />}
            onPress={() => handleApplyRules('manual')}
          >
            应用规则
          </Button>
          <Button
            color="primary"
            startContent={<Plus className="w-4 h-4" />}
            onPress={activeTab === 'tags' ? handleCreateTag : handleCreateRule}
          >
            {activeTab === 'tags' ? '新建标签' : '新建规则'}
          </Button>
        </div>
      </div>

      <Tabs
        selectedKey={activeTab}
        onSelectionChange={(key) => setActiveTab(key as string)}
        aria-label="标签管理"
      >
        {/* 标签列表 Tab */}
        <Tab key="tags" title="标签列表">
          {loading ? (
            <div className="flex justify-center py-12">
              <Spinner size="lg" />
            </div>
          ) : tags.length === 0 ? (
            <Card className="mt-4">
              <CardBody className="py-12 text-center">
                <TagIcon className="w-12 h-12 mx-auto text-gray-300 mb-4" />
                <p className="text-gray-500">暂无标签</p>
                <Button
                  color="primary"
                  variant="flat"
                  className="mt-4"
                  onPress={handleCreateTag}
                >
                  创建第一个标签
                </Button>
              </CardBody>
            </Card>
          ) : (
            <div className="mt-4">
              {/* 按组显示标签 */}
              {groups.length > 0 ? (
                <Accordion variant="bordered" selectionMode="multiple" defaultExpandedKeys={groups}>
                  {groups.map((group) => {
                    const groupTags = tags.filter((t) => t.tag_group === group);
                    if (groupTags.length === 0) return null;
                    return (
                      <AccordionItem
                        key={group}
                        aria-label={group}
                        title={
                          <div className="flex items-center gap-2">
                            <span>{group || '未分组'}</span>
                            <Chip size="sm" variant="flat">{groupTags.length}</Chip>
                            <Chip size="sm" variant="flat" color="warning">互斥</Chip>
                          </div>
                        }
                      >
                        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-3 pb-2">
                          {groupTags.map((tag) => (
                            <TagCard
                              key={tag.ID}
                              tag={tag}
                              onEdit={() => handleEditTag(tag)}
                              onDelete={() => handleDeleteTag(tag)}
                              getTagColor={getTagColor}
                            />
                          ))}
                        </div>
                      </AccordionItem>
                    );
                  })}
                </Accordion>
              ) : null}

              {/* 未分组标签 */}
              {(() => {
                const ungroupedTags = tags.filter((t) => !t.tag_group);
                if (ungroupedTags.length === 0) return null;
                return (
                  <div className="mt-4">
                    <h3 className="text-sm font-medium text-gray-500 mb-3">独立标签</h3>
                    <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-3">
                      {ungroupedTags.map((tag) => (
                        <TagCard
                          key={tag.ID}
                          tag={tag}
                          onEdit={() => handleEditTag(tag)}
                          onDelete={() => handleDeleteTag(tag)}
                          getTagColor={getTagColor}
                        />
                      ))}
                    </div>
                  </div>
                );
              })()}
            </div>
          )}
        </Tab>

        {/* 规则列表 Tab */}
        <Tab key="rules" title="自动规则">
          {loading ? (
            <div className="flex justify-center py-12">
              <Spinner size="lg" />
            </div>
          ) : rules.length === 0 ? (
            <Card className="mt-4">
              <CardBody className="py-12 text-center">
                <Filter className="w-12 h-12 mx-auto text-gray-300 mb-4" />
                <p className="text-gray-500">暂无自动打标规则</p>
                <p className="text-xs text-gray-400 mt-2">
                  创建规则后，系统会根据条件自动为节点打标签
                </p>
                <Button
                  color="primary"
                  variant="flat"
                  className="mt-4"
                  onPress={handleCreateRule}
                >
                  创建第一个规则
                </Button>
              </CardBody>
            </Card>
          ) : (
            <Table aria-label="规则列表" className="mt-4">
              <TableHeader>
                <TableColumn>规则名称</TableColumn>
                <TableColumn>目标标签</TableColumn>
                <TableColumn>触发条件</TableColumn>
                <TableColumn>状态</TableColumn>
                <TableColumn>操作</TableColumn>
              </TableHeader>
              <TableBody>
                {rules.map((rule) => (
                  <TableRow key={rule.ID}>
                    <TableCell>
                      <div>
                        <p className="font-medium">{rule.name}</p>
                        {rule.description && (
                          <p className="text-xs text-gray-400">{rule.description}</p>
                        )}
                      </div>
                    </TableCell>
                    <TableCell>
                      {rule.tag ? (
                        <Chip size="sm" color={getTagColor(rule.tag.color)} variant="flat">
                          {rule.tag.name}
                        </Chip>
                      ) : (
                        <span className="text-gray-400">-</span>
                      )}
                    </TableCell>
                    <TableCell>
                      <Chip size="sm" variant="flat">
                        {triggerTypeOptions.find((t) => t.value === rule.trigger_type)?.label || rule.trigger_type}
                      </Chip>
                    </TableCell>
                    <TableCell>
                      <Chip
                        size="sm"
                        color={rule.enabled ? 'success' : 'default'}
                        variant="flat"
                      >
                        {rule.enabled ? '启用' : '禁用'}
                      </Chip>
                    </TableCell>
                    <TableCell>
                      <div className="flex gap-1">
                        <Tooltip content="编辑">
                          <Button
                            isIconOnly
                            size="sm"
                            variant="light"
                            onPress={() => handleEditRule(rule)}
                          >
                            <Pencil className="w-4 h-4" />
                          </Button>
                        </Tooltip>
                        <Tooltip content="删除">
                          <Button
                            isIconOnly
                            size="sm"
                            color="danger"
                            variant="light"
                            onPress={() => handleDeleteRule(rule)}
                          >
                            <Trash2 className="w-4 h-4" />
                          </Button>
                        </Tooltip>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </Tab>
      </Tabs>

      {/* 标签编辑弹窗 */}
      <Modal isOpen={isTagOpen} onClose={onTagClose} size="lg">
        <ModalContent>
          <ModalHeader>{editingTag ? '编辑标签' : '新建标签'}</ModalHeader>
          <ModalBody>
            <div className="space-y-4">
              <Input
                label="标签名称"
                placeholder="例如：高速、低延迟、香港优选"
                value={tagForm.name}
                onChange={(e) => setTagForm({ ...tagForm, name: e.target.value })}
                isRequired
              />

              <Select
                label="标签颜色"
                selectedKeys={[tagForm.color]}
                onChange={(e) => setTagForm({ ...tagForm, color: e.target.value })}
              >
                {colorOptions.map((opt) => (
                  <SelectItem key={opt.value} value={opt.value}>
                    <div className="flex items-center gap-2">
                      <span className={`w-3 h-3 rounded-full ${opt.bg}`} />
                      {opt.label}
                    </div>
                  </SelectItem>
                ))}
              </Select>

              <Input
                label="标签组（互斥组）"
                placeholder="同组标签互斥，如：速度等级、地区等级"
                value={tagForm.tag_group}
                onChange={(e) => setTagForm({ ...tagForm, tag_group: e.target.value })}
                description="同一组内的标签互斥，节点只能有一个"
              />

              <Input
                label="描述"
                placeholder="标签用途说明"
                value={tagForm.description}
                onChange={(e) => setTagForm({ ...tagForm, description: e.target.value })}
              />

              <Input
                type="number"
                label="优先级"
                value={String(tagForm.priority)}
                onChange={(e) => setTagForm({ ...tagForm, priority: parseInt(e.target.value) || 0 })}
                description="数值越大优先级越高"
              />
            </div>
          </ModalBody>
          <ModalFooter>
            <Button variant="flat" onPress={onTagClose}>
              取消
            </Button>
            <Button
              color="primary"
              onPress={handleSaveTag}
              isLoading={isSubmitting}
              isDisabled={!tagForm.name.trim()}
            >
              {editingTag ? '保存' : '创建'}
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      {/* 规则编辑弹窗 */}
      <Modal isOpen={isRuleOpen} onClose={onRuleClose} size="2xl" scrollBehavior="inside">
        <ModalContent>
          <ModalHeader>{editingRule ? '编辑规则' : '新建规则'}</ModalHeader>
          <ModalBody>
            <div className="space-y-4">
              <Input
                label="规则名称"
                placeholder="例如：低延迟节点打标"
                value={ruleForm.name}
                onChange={(e) => setRuleForm({ ...ruleForm, name: e.target.value })}
                isRequired
              />

              <Select
                label="目标标签"
                placeholder="选择要打的标签"
                selectedKeys={ruleForm.tag_id ? [String(ruleForm.tag_id)] : []}
                onChange={(e) => setRuleForm({ ...ruleForm, tag_id: parseInt(e.target.value) || 0 })}
                isRequired
              >
                {tags.map((tag) => (
                  <SelectItem key={String(tag.ID)} value={String(tag.ID)}>
                    {tag.name}
                  </SelectItem>
                ))}
              </Select>

              <Select
                label="触发时机"
                selectedKeys={[ruleForm.trigger_type]}
                onChange={(e) => setRuleForm({ ...ruleForm, trigger_type: e.target.value })}
              >
                {triggerTypeOptions.map((opt) => (
                  <SelectItem key={opt.value} value={opt.value}>
                    {opt.label}
                  </SelectItem>
                ))}
              </Select>

              {/* 条件配置 */}
              <Card>
                <CardHeader className="flex justify-between items-center py-2">
                  <div className="flex items-center gap-2">
                    <span className="font-medium">匹配条件</span>
                    <Select
                      size="sm"
                      className="w-24"
                      selectedKeys={[ruleForm.conditions.logic]}
                      onChange={(e) =>
                        setRuleForm({
                          ...ruleForm,
                          conditions: { ...ruleForm.conditions, logic: e.target.value },
                        })
                      }
                    >
                      <SelectItem key="AND" value="AND">全部满足</SelectItem>
                      <SelectItem key="OR" value="OR">任一满足</SelectItem>
                    </Select>
                  </div>
                  <Button size="sm" variant="flat" onPress={addCondition}>
                    <Plus className="w-4 h-4" />
                    添加条件
                  </Button>
                </CardHeader>
                <CardBody className="space-y-3">
                  {ruleForm.conditions.conditions.map((cond, index) => {
                    // 获取当前字段的类型
                    const fieldConfig = conditionFieldOptions.find(f => f.value === cond.field);
                    const fieldType = fieldConfig?.type || 'string';

                    // 根据字段类型渲染不同的值输入组件
                    const renderValueInput = () => {
                      if (fieldType === 'status') {
                        return (
                          <Select
                            size="sm"
                            label="值"
                            className="flex-1"
                            selectedKeys={cond.value ? [String(cond.value)] : []}
                            onChange={(e) => updateCondition(index, 'value', e.target.value)}
                          >
                            {statusOptions.map((opt) => (
                              <SelectItem key={opt.value} value={opt.value}>
                                {opt.label}
                              </SelectItem>
                            ))}
                          </Select>
                        );
                      }

                      if (fieldType === 'protocol') {
                        return (
                          <Select
                            size="sm"
                            label="值"
                            className="flex-1"
                            selectedKeys={cond.value ? [String(cond.value)] : []}
                            onChange={(e) => updateCondition(index, 'value', e.target.value)}
                          >
                            {protocolOptions.map((opt) => (
                              <SelectItem key={opt.value} value={opt.value}>
                                {opt.label}
                              </SelectItem>
                            ))}
                          </Select>
                        );
                      }

                      if (cond.field === 'country') {
                        return (
                          <Select
                            size="sm"
                            label="值"
                            className="flex-1"
                            selectedKeys={cond.value ? [String(cond.value)] : []}
                            onChange={(e) => updateCondition(index, 'value', e.target.value)}
                          >
                            {countryOptions.map((opt) => (
                              <SelectItem key={opt.value} value={opt.value}>
                                {opt.label}
                              </SelectItem>
                            ))}
                          </Select>
                        );
                      }

                      return (
                        <Input
                          size="sm"
                          label="值"
                          className="flex-1"
                          type={fieldType === 'number' ? 'number' : 'text'}
                          value={String(cond.value)}
                          onChange={(e) => updateCondition(index, 'value', fieldType === 'number' ? parseFloat(e.target.value) || 0 : e.target.value)}
                          placeholder={fieldType === 'number' ? '输入数值' : '输入文本'}
                        />
                      );
                    };

                    return (
                      <div key={index} className="flex gap-2 items-end">
                        <Select
                          size="sm"
                          label="字段"
                          className="flex-1"
                          selectedKeys={[cond.field]}
                          onChange={(e) => updateCondition(index, 'field', e.target.value)}
                        >
                          {conditionFieldOptions.map((opt) => (
                            <SelectItem key={opt.value} value={opt.value}>
                              {opt.label}
                            </SelectItem>
                          ))}
                        </Select>
                        <Select
                          size="sm"
                          label="操作"
                          className="flex-1"
                          selectedKeys={[cond.operator]}
                          onChange={(e) => updateCondition(index, 'operator', e.target.value)}
                        >
                          {operatorOptions.map((opt) => (
                            <SelectItem key={opt.value} value={opt.value}>
                              {opt.label}
                            </SelectItem>
                          ))}
                        </Select>
                        {renderValueInput()}
                        <Button
                          isIconOnly
                          size="sm"
                          color="danger"
                          variant="light"
                          onPress={() => removeCondition(index)}
                          isDisabled={ruleForm.conditions.conditions.length <= 1}
                        >
                          <Trash2 className="w-4 h-4" />
                        </Button>
                      </div>
                    );
                  })}
                </CardBody>
              </Card>

              <Input
                label="描述"
                placeholder="规则说明"
                value={ruleForm.description}
                onChange={(e) => setRuleForm({ ...ruleForm, description: e.target.value })}
              />

              <div className="flex items-center justify-between">
                <span>启用规则</span>
                <Switch
                  isSelected={ruleForm.enabled}
                  onValueChange={(checked) => setRuleForm({ ...ruleForm, enabled: checked })}
                />
              </div>
            </div>
          </ModalBody>
          <ModalFooter>
            <Button variant="flat" onPress={onRuleClose}>
              取消
            </Button>
            <Button
              color="primary"
              onPress={handleSaveRule}
              isLoading={isSubmitting}
              isDisabled={!ruleForm.name.trim() || !ruleForm.tag_id}
            >
              {editingRule ? '保存' : '创建'}
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </div>
  );
}

// 标签卡片组件
interface TagCardProps {
  tag: Tag;
  onEdit: () => void;
  onDelete: () => void;
  getTagColor: (color: string) => 'default' | 'primary' | 'secondary' | 'success' | 'warning' | 'danger';
}

function TagCard({ tag, onEdit, onDelete, getTagColor }: TagCardProps) {
  return (
    <Card className="hover:shadow-md transition-shadow">
      <CardBody className="flex flex-row items-center justify-between py-3">
        <div className="flex items-center gap-2">
          <Chip size="sm" color={getTagColor(tag.color)} variant="flat">
            {tag.name}
          </Chip>
          {tag.description && (
            <Tooltip content={tag.description}>
              <span className="text-xs text-gray-400 truncate max-w-[100px]">
                {tag.description}
              </span>
            </Tooltip>
          )}
        </div>
        <div className="flex gap-1">
          <Button isIconOnly size="sm" variant="light" onPress={onEdit}>
            <Pencil className="w-3 h-3" />
          </Button>
          <Button isIconOnly size="sm" color="danger" variant="light" onPress={onDelete}>
            <Trash2 className="w-3 h-3" />
          </Button>
        </div>
      </CardBody>
    </Card>
  );
}
