import { useEffect, useState, useCallback, useMemo } from 'react';
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
  Accordion,
  AccordionItem,
  Spinner,
  Tabs,
  Tab,
  Select,
  SelectItem,
  Switch,
  Divider,
  Dropdown,
  DropdownTrigger,
  DropdownMenu,
  DropdownItem,
  DropdownSection,
} from '@nextui-org/react';
import { Plus, RefreshCw, Trash2, Globe, Server, Pencil, Link, Filter as FilterIcon, ChevronDown, ChevronUp, Zap, Settings, Timer, Search } from 'lucide-react';
import { useStore } from '../store';
import { nodeApi, speedtestApi } from '../api';
import { toast } from '../components/Toast';
import type { Subscription, ManualNode, Node, Filter } from '../store';

// 测速设置类型
interface SpeedTestProfile {
  ID?: number;
  id?: number;
  name: string;
  enabled: boolean;
  is_default?: boolean;
  cron_expression?: string;
  schedule_cron?: string;
  mode: string;  // 'delay' 仅延迟, 'speed' 延迟+速度
  latency_url: string;
  speed_url: string;
  timeout: number;
  latency_concurrency: number;
  speed_concurrency: number;
  include_handshake: boolean;
  detect_country: boolean;
}

// 默认测速设置
const defaultSpeedTestProfile: SpeedTestProfile = {
  name: '默认策略',
  enabled: false,
  cron_expression: '0 */6 * * *', // 每6小时
  mode: 'speed',  // 默认延迟+速度
  latency_url: 'https://cp.cloudflare.com/generate_204',
  speed_url: 'https://speed.cloudflare.com/__down?bytes=5000000',
  timeout: 7,
  latency_concurrency: 50,
  speed_concurrency: 5,
  include_handshake: false,
  detect_country: false,
};

// Cron 预设选项
const cronPresets = [
  { label: '每30分钟', value: '*/30 * * * *' },
  { label: '每1小时', value: '0 * * * *' },
  { label: '每6小时', value: '0 */6 * * *' },
  { label: '每12小时', value: '0 */12 * * *' },
  { label: '每天0点', value: '0 0 * * *' },
  { label: '每周一', value: '0 0 * * 1' },
];

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

const normalizeSpeedTestProfiles = (response: any): any[] => {
  const data = response?.data;
  if (Array.isArray(data)) return data;
  if (Array.isArray(data?.data)) return data.data;
  return [];
};

const getProfileId = (profile: any): number | undefined => profile?.ID ?? profile?.id;

const pickDefaultProfile = (profiles: any[]) =>
  profiles.find((profile) => profile?.is_default || profile?.isDefault) ?? profiles[0];

const nodeTypeOptions = [
  { value: 'shadowsocks', label: 'Shadowsocks' },
  { value: 'vmess', label: 'VMess' },
  { value: 'vless', label: 'VLESS' },
  { value: 'trojan', label: 'Trojan' },
  { value: 'hysteria2', label: 'Hysteria2' },
  { value: 'tuic', label: 'TUIC' },
  { value: 'socks', label: 'SOCKS' },
];

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
];

const defaultNode: Node = {
  tag: '',
  type: 'shadowsocks',
  server: '',
  server_port: 443,
  country: 'HK',
  country_emoji: '🇭🇰',
};

export default function Subscriptions() {
  const {
    subscriptions,
    manualNodes,
    countryGroups,
    filters,
    loading,
    fetchSubscriptions,
    fetchManualNodes,
    fetchCountryGroups,
    fetchFilters,
    addSubscription,
    updateSubscription,
    deleteSubscription,
    refreshSubscription,
    toggleSubscription,
    addManualNode,
    updateManualNode,
    deleteManualNode,
    addFilter,
    updateFilter,
    deleteFilter,
    toggleFilter,
  } = useStore();

  const { isOpen: isSubOpen, onOpen: onSubOpen, onClose: onSubClose } = useDisclosure();
  const { isOpen: isNodeOpen, onOpen: onNodeOpen, onClose: onNodeClose } = useDisclosure();
  const { isOpen: isFilterOpen, onOpen: onFilterOpen, onClose: onFilterClose } = useDisclosure();
  const { isOpen: isSpeedTestOpen, onOpen: onSpeedTestOpen, onClose: onSpeedTestClose } = useDisclosure();
  const [name, setName] = useState('');
  const [url, setUrl] = useState('');
  const [autoUpdate, setAutoUpdate] = useState(true);
  const [updateInterval, setUpdateInterval] = useState(60);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [editingSubscription, setEditingSubscription] = useState<Subscription | null>(null);

  // 测速设置表单
  const [speedTestProfile, setSpeedTestProfile] = useState<SpeedTestProfile>(defaultSpeedTestProfile);
  const [selectedCronPreset, setSelectedCronPreset] = useState<string>('0 */6 * * *');
  const [isLoadingProfiles, setIsLoadingProfiles] = useState(false);

  // 手动节点表单
  const [editingNode, setEditingNode] = useState<ManualNode | null>(null);
  const [nodeForm, setNodeForm] = useState<Node>(defaultNode);
  const [nodeEnabled, setNodeEnabled] = useState(true);
  const [nodeUrl, setNodeUrl] = useState('');
  const [isParsing, setIsParsing] = useState(false);
  const [parseError, setParseError] = useState('');

  // 过滤器表单
  const [editingFilter, setEditingFilter] = useState<Filter | null>(null);
  const defaultFilterForm: Omit<Filter, 'id'> = {
    name: '',
    include: [],
    exclude: [],
    include_countries: [],
    exclude_countries: [],
    mode: 'urltest',
    urltest_config: {
      url: 'https://www.gstatic.com/generate_204',
      interval: '5m',
      tolerance: 50,
    },
    subscriptions: [],
    all_nodes: true,
    enabled: true,
  };
  const [filterForm, setFilterForm] = useState<Omit<Filter, 'id'>>(defaultFilterForm);

  // 全局测速状态
  const [isRunningFullTest, setIsRunningFullTest] = useState(false);
  const [isRefreshingDelays, setIsRefreshingDelays] = useState(false);

  useEffect(() => {
    fetchSubscriptions();
    fetchManualNodes();
    fetchCountryGroups();
    fetchFilters();
  }, []);

  const handleOpenAddSubscription = () => {
    setEditingSubscription(null);
    setName('');
    setUrl('');
    setAutoUpdate(true);
    setUpdateInterval(60);
    onSubOpen();
  };

  const handleOpenEditSubscription = (sub: Subscription) => {
    setEditingSubscription(sub);
    setName(sub.name);
    setUrl(sub.url);
    setAutoUpdate(sub.auto_update ?? true);
    setUpdateInterval(sub.update_interval ?? 60);
    onSubOpen();
  };

  const handleSaveSubscription = async () => {
    if (!name || !url) return;

    setIsSubmitting(true);
    try {
      if (editingSubscription) {
        await updateSubscription(editingSubscription.id, name, url, autoUpdate, updateInterval);
      } else {
        await addSubscription(name, url, autoUpdate, updateInterval);
      }
      setName('');
      setUrl('');
      setEditingSubscription(null);
      onSubClose();
    } catch (error) {
      console.error(editingSubscription ? '更新订阅失败:' : '添加订阅失败:', error);
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleRefresh = async (id: string) => {
    await refreshSubscription(id);
  };

  const handleDeleteSubscription = async (id: string) => {
    if (confirm('确定要删除这个订阅吗？')) {
      await deleteSubscription(id);
    }
  };

  const handleToggleSubscription = async (sub: Subscription) => {
    await toggleSubscription(sub.id, !sub.enabled);
  };

  // 手动节点操作
  const handleOpenAddNode = () => {
    setEditingNode(null);
    setNodeForm(defaultNode);
    setNodeEnabled(true);
    setNodeUrl('');
    setParseError('');
    onNodeOpen();
  };

  const handleOpenEditNode = (mn: ManualNode) => {
    setEditingNode(mn);
    setNodeForm(mn.node);
    setNodeEnabled(mn.enabled);
    setNodeUrl('');
    setParseError('');
    onNodeOpen();
  };

  // 解析节点链接
  const handleParseUrl = async () => {
    if (!nodeUrl.trim()) return;

    setIsParsing(true);
    setParseError('');

    try {
      const response = await nodeApi.parse(nodeUrl.trim());
      const parsedNode = response.data.data as Node;
      setNodeForm(parsedNode);
    } catch (error: any) {
      const message = error.response?.data?.error || '解析失败，请检查链接格式';
      setParseError(message);
    } finally {
      setIsParsing(false);
    }
  };

  const handleSaveNode = async () => {
    if (!nodeForm.tag || !nodeForm.server) return;

    setIsSubmitting(true);
    try {
      const country = countryOptions.find(c => c.code === nodeForm.country);
      const nodeData = {
        ...nodeForm,
        country_emoji: country?.emoji || '🌐',
      };

      if (editingNode) {
        await updateManualNode(editingNode.id, { node: nodeData, enabled: nodeEnabled });
      } else {
        await addManualNode({ node: nodeData, enabled: nodeEnabled });
      }
      onNodeClose();
    } catch (error) {
      console.error('保存节点失败:', error);
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleDeleteNode = async (id: string) => {
    if (confirm('确定要删除这个节点吗？')) {
      await deleteManualNode(id);
    }
  };

  const handleToggleNode = async (mn: ManualNode) => {
    await updateManualNode(mn.id, { ...mn, enabled: !mn.enabled });
  };

  // 过滤器操作
  const handleOpenAddFilter = () => {
    setEditingFilter(null);
    setFilterForm(defaultFilterForm);
    onFilterOpen();
  };

  const handleOpenEditFilter = (filter: Filter) => {
    setEditingFilter(filter);
    setFilterForm({
      name: filter.name,
      include: filter.include || [],
      exclude: filter.exclude || [],
      include_countries: filter.include_countries || [],
      exclude_countries: filter.exclude_countries || [],
      mode: filter.mode || 'urltest',
      urltest_config: filter.urltest_config || {
        url: 'https://www.gstatic.com/generate_204',
        interval: '5m',
        tolerance: 50,
      },
      subscriptions: filter.subscriptions || [],
      all_nodes: filter.all_nodes ?? true,
      enabled: filter.enabled,
    });
    onFilterOpen();
  };

  const handleSaveFilter = async () => {
    if (!filterForm.name) return;

    setIsSubmitting(true);
    try {
      if (editingFilter) {
        await updateFilter(editingFilter.id, filterForm);
      } else {
        await addFilter(filterForm);
      }
      onFilterClose();
    } catch (error) {
      console.error('保存过滤器失败:', error);
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleDeleteFilter = async (id: string) => {
    if (confirm('确定要删除这个过滤器吗？')) {
      await deleteFilter(id);
    }
  };

  const handleToggleFilter = async (filter: Filter) => {
    await toggleFilter(filter.id, !filter.enabled);
  };

  // 测速设置相关函数
  const resolveDefaultProfile = async () => {
    const response = await speedtestApi.getProfiles();
    const profileList = normalizeSpeedTestProfiles(response);
    return pickDefaultProfile(profileList) || null;
  };

  const loadSpeedTestProfiles = useCallback(async () => {
    setIsLoadingProfiles(true);
    try {
      const response = await speedtestApi.getProfiles();
      const profileList = normalizeSpeedTestProfiles(response);
      // 如果有策略，使用第一个策略填充表单
      if (profileList.length > 0) {
        const profile = pickDefaultProfile(profileList);
        setSpeedTestProfile({
          ID: getProfileId(profile),
          name: profile.name || defaultSpeedTestProfile.name,
          enabled: profile.enabled ?? defaultSpeedTestProfile.enabled,
          cron_expression: profile.cron_expression || profile.schedule_cron || defaultSpeedTestProfile.cron_expression,
          mode: profile.mode || defaultSpeedTestProfile.mode,
          latency_url: profile.latency_url || defaultSpeedTestProfile.latency_url,
          speed_url: profile.speed_url || defaultSpeedTestProfile.speed_url,
          timeout: profile.timeout || defaultSpeedTestProfile.timeout,
          latency_concurrency: profile.latency_concurrency || defaultSpeedTestProfile.latency_concurrency,
          speed_concurrency: profile.speed_concurrency || defaultSpeedTestProfile.speed_concurrency,
          include_handshake: typeof profile.include_handshake === 'boolean'
            ? profile.include_handshake
            : defaultSpeedTestProfile.include_handshake,
          detect_country: typeof profile.detect_country === 'boolean'
            ? profile.detect_country
            : defaultSpeedTestProfile.detect_country,
        });
        if (profile.cron_expression || profile.schedule_cron) {
          setSelectedCronPreset(profile.cron_expression || profile.schedule_cron);
        }
      }
    } catch (error: any) {
      console.error('加载测速策略失败:', error);
    } finally {
      setIsLoadingProfiles(false);
    }
  }, []);

  const handleOpenSpeedTestSettings = () => {
    loadSpeedTestProfiles();
    onSpeedTestOpen();
  };

  const handleSaveSpeedTestProfile = async () => {
    setIsSubmitting(true);
    try {
      const profileData = {
        ...speedTestProfile,
        cron_expression: selectedCronPreset,
      };

      if (speedTestProfile.ID) {
        await speedtestApi.updateProfile(speedTestProfile.ID, profileData);
        toast.success('测速设置已保存');
      } else {
        await speedtestApi.createProfile(profileData);
        toast.success('测速设置已创建');
      }
      onSpeedTestClose();
    } catch (error: any) {
      toast.error(error.response?.data?.error || '保存测速设置失败');
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleRunSpeedTest = async () => {
    try {
      let profileId = speedTestProfile.ID;
      if (!profileId) {
        const profile = await resolveDefaultProfile();
        profileId = getProfileId(profile);
      }
      await speedtestApi.runTest(profileId);
      toast.success('测速任务已启动，请到任务管理查看进度');
      onSpeedTestClose();
    } catch (error: any) {
      toast.error(error.response?.data?.error || '启动测速失败');
    }
  };

  // 全量测速（延迟+速度）
  const handleGlobalFullTest = async () => {
    setIsRunningFullTest(true);
    try {
      let profileId = speedTestProfile.ID;
      let profileMode = speedTestProfile.mode;
      if (!profileId) {
        const profile = await resolveDefaultProfile();
        profileId = getProfileId(profile);
        if (profile) {
          profileMode = profile.mode || profileMode;
          setSpeedTestProfile((prev) => ({
            ...prev,
            ID: profileId,
            name: profile.name || prev.name,
            enabled: profile.enabled ?? prev.enabled,
            cron_expression: profile.cron_expression || profile.schedule_cron || prev.cron_expression,
            mode: profile.mode || prev.mode,
            latency_url: profile.latency_url || prev.latency_url,
            speed_url: profile.speed_url || prev.speed_url,
            timeout: profile.timeout || prev.timeout,
            latency_concurrency: profile.latency_concurrency || prev.latency_concurrency,
            speed_concurrency: profile.speed_concurrency || prev.speed_concurrency,
            include_handshake: typeof profile.include_handshake === 'boolean'
              ? profile.include_handshake
              : prev.include_handshake,
            detect_country: typeof profile.detect_country === 'boolean'
              ? profile.detect_country
              : prev.detect_country,
          }));
          if (profile.cron_expression || profile.schedule_cron) {
            setSelectedCronPreset(profile.cron_expression || profile.schedule_cron);
          }
        }
      }
      await speedtestApi.runTest(profileId);
      toast.success('全量测速任务已启动，请到任务管理查看进度');
      if (profileMode && profileMode !== 'speed') {
        toast.info('当前策略为“仅延迟”，不会产生下载速度。请在测速设置中切换为“延迟+速度”。');
      }
    } catch (error: any) {
      toast.error(error.response?.data?.error || '启动全量测速失败');
    } finally {
      setIsRunningFullTest(false);
    }
  };

  // 刷新延迟（仅延迟）
  const handleGlobalRefreshDelays = async () => {
    setIsRefreshingDelays(true);
    try {
      const response = await nodeApi.refreshAllDelays();
      toast.success(response.data.message || '延迟测试任务已启动');
      // 30秒后自动停止loading状态
      setTimeout(() => {
        setIsRefreshingDelays(false);
      }, 30000);
    } catch (error: any) {
      toast.error(error.response?.data?.error || '启动延迟测试失败');
      setIsRefreshingDelays(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold text-gray-800 dark:text-white">节点管理</h1>
        <div className="flex gap-2">
          <Dropdown>
            <DropdownTrigger>
              <Button
                color="success"
                variant="flat"
                startContent={(isRunningFullTest || isRefreshingDelays) ? <Spinner size="sm" /> : <Settings className="w-4 h-4" />}
                endContent={<ChevronDown className="w-4 h-4" />}
              >
                测速设置
              </Button>
            </DropdownTrigger>
            <DropdownMenu aria-label="测速操作">
              <DropdownSection title="快捷操作" showDivider>
                <DropdownItem
                  key="full-test"
                  description="延迟 + 下载速度测试"
                  startContent={<Zap className="w-4 h-4 text-success" />}
                  onPress={handleGlobalFullTest}
                  isDisabled={isRunningFullTest}
                >
                  全量测速
                </DropdownItem>
                <DropdownItem
                  key="refresh-delay"
                  description="仅测试延迟"
                  startContent={<RefreshCw className="w-4 h-4 text-secondary" />}
                  onPress={handleGlobalRefreshDelays}
                  isDisabled={isRefreshingDelays}
                >
                  刷新延迟
                </DropdownItem>
              </DropdownSection>
              <DropdownSection title="设置">
                <DropdownItem
                  key="settings"
                  description="定时测速、测速模式等"
                  startContent={<Settings className="w-4 h-4 text-primary" />}
                  onPress={handleOpenSpeedTestSettings}
                >
                  测速配置
                </DropdownItem>
              </DropdownSection>
            </DropdownMenu>
          </Dropdown>
          <Button
            color="secondary"
            variant="flat"
            startContent={<FilterIcon className="w-4 h-4" />}
            onPress={handleOpenAddFilter}
          >
            添加过滤器
          </Button>
          <Button
            color="primary"
            variant="flat"
            startContent={<Plus className="w-4 h-4" />}
            onPress={handleOpenAddNode}
          >
            添加节点
          </Button>
          <Button
            color="primary"
            startContent={<Plus className="w-4 h-4" />}
            onPress={handleOpenAddSubscription}
          >
            添加订阅
          </Button>
        </div>
      </div>

      <Tabs aria-label="节点管理">
        <Tab key="subscriptions" title="订阅管理">
          {subscriptions.length === 0 ? (
            <Card className="mt-4">
              <CardBody className="py-12 text-center">
                <Globe className="w-12 h-12 mx-auto text-gray-300 mb-4" />
                <p className="text-gray-500">暂无订阅，点击上方按钮添加</p>
              </CardBody>
            </Card>
          ) : (
            <div className="space-y-4 mt-4">
              {subscriptions.map((sub) => (
                <SubscriptionCard
                  key={sub.id}
                  subscription={sub}
                  onRefresh={() => handleRefresh(sub.id)}
                  onEdit={() => handleOpenEditSubscription(sub)}
                  onDelete={() => handleDeleteSubscription(sub.id)}
                  onToggle={() => handleToggleSubscription(sub)}
                  loading={loading}
                />
              ))}
            </div>
          )}
        </Tab>

        <Tab key="manual" title="手动节点">
          {manualNodes.length === 0 ? (
            <Card className="mt-4">
              <CardBody className="py-12 text-center">
                <Server className="w-12 h-12 mx-auto text-gray-300 mb-4" />
                <p className="text-gray-500">暂无手动节点，点击上方按钮添加</p>
              </CardBody>
            </Card>
          ) : (
            <div className="space-y-3 mt-4">
              {manualNodes.map((mn) => (
                <Card key={mn.id}>
                  <CardBody className="flex flex-row items-center justify-between">
                    <div className="flex items-center gap-3">
                      <span className="text-xl">{mn.node.country_emoji || '🌐'}</span>
                      <div>
                        <h3 className="font-medium">{mn.node.tag}</h3>
                        <p className="text-sm text-gray-500">
                          {mn.node.type} · {mn.node.server}:{mn.node.server_port}
                        </p>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <Button
                        isIconOnly
                        size="sm"
                        variant="light"
                        onPress={() => handleOpenEditNode(mn)}
                      >
                        <Pencil className="w-4 h-4" />
                      </Button>
                      <Button
                        isIconOnly
                        size="sm"
                        variant="light"
                        color="danger"
                        onPress={() => handleDeleteNode(mn.id)}
                      >
                        <Trash2 className="w-4 h-4" />
                      </Button>
                      <Switch
                        isSelected={mn.enabled}
                        onValueChange={() => handleToggleNode(mn)}
                      />
                    </div>
                  </CardBody>
                </Card>
              ))}
            </div>
          )}
        </Tab>

        <Tab key="filters" title="过滤器">
          {filters.length === 0 ? (
            <Card className="mt-4">
              <CardBody className="py-12 text-center">
                <FilterIcon className="w-12 h-12 mx-auto text-gray-300 mb-4" />
                <p className="text-gray-500">暂无过滤器，点击上方按钮添加</p>
                <p className="text-xs text-gray-400 mt-2">
                  过滤器可以根据国家或关键字筛选节点，创建自定义节点分组
                </p>
              </CardBody>
            </Card>
          ) : (
            <div className="space-y-3 mt-4">
              {filters.map((filter) => (
                <Card key={filter.id}>
                  <CardBody className="flex flex-row items-center justify-between">
                    <div className="flex items-center gap-3">
                      <FilterIcon className="w-5 h-5 text-secondary" />
                      <div>
                        <h3 className="font-medium">{filter.name}</h3>
                        <div className="flex flex-wrap gap-1 mt-1">
                          {filter.include_countries?.length > 0 && (
                            <Chip size="sm" variant="flat" color="success">
                              {filter.include_countries.map(code =>
                                countryOptions.find(c => c.code === code)?.emoji || code
                              ).join(' ')} 包含
                            </Chip>
                          )}
                          {filter.exclude_countries?.length > 0 && (
                            <Chip size="sm" variant="flat" color="danger">
                              {filter.exclude_countries.map(code =>
                                countryOptions.find(c => c.code === code)?.emoji || code
                              ).join(' ')} 排除
                            </Chip>
                          )}
                          {filter.include?.length > 0 && (
                            <Chip size="sm" variant="flat">
                              关键字: {filter.include.join('|')}
                            </Chip>
                          )}
                          <Chip size="sm" variant="flat" color="secondary">
                            {filter.mode === 'urltest' ? '自动测速' : '手动选择'}
                          </Chip>
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <Button
                        isIconOnly
                        size="sm"
                        variant="light"
                        onPress={() => handleOpenEditFilter(filter)}
                      >
                        <Pencil className="w-4 h-4" />
                      </Button>
                      <Button
                        isIconOnly
                        size="sm"
                        variant="light"
                        color="danger"
                        onPress={() => handleDeleteFilter(filter.id)}
                      >
                        <Trash2 className="w-4 h-4" />
                      </Button>
                      <Switch
                        isSelected={filter.enabled}
                        onValueChange={() => handleToggleFilter(filter)}
                      />
                    </div>
                  </CardBody>
                </Card>
              ))}
            </div>
          )}
        </Tab>

        <Tab key="countries" title="按国家/地区">
          {countryGroups.length === 0 ? (
            <Card className="mt-4">
              <CardBody className="py-12 text-center">
                <Globe className="w-12 h-12 mx-auto text-gray-300 mb-4" />
                <p className="text-gray-500">暂无节点，请先添加订阅或手动添加节点</p>
              </CardBody>
            </Card>
          ) : (
            <CountryNodesList countryGroups={countryGroups} />
          )}
        </Tab>
      </Tabs>

      {/* 添加/编辑订阅弹窗 */}
      <Modal isOpen={isSubOpen} onClose={onSubClose}>
        <ModalContent>
          <ModalHeader>{editingSubscription ? '编辑订阅' : '添加订阅'}</ModalHeader>
          <ModalBody className="space-y-4">
            <Input
              label="订阅名称"
              placeholder="输入订阅名称"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
            <Input
              label="订阅地址"
              placeholder="输入订阅 URL"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
            />
            <Divider />
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Timer className="w-4 h-4 text-gray-500" />
                <span className="text-sm">自动更新</span>
              </div>
              <Switch
                isSelected={autoUpdate}
                onValueChange={setAutoUpdate}
                size="sm"
              />
            </div>
            {autoUpdate && (
              <Select
                label="更新间隔"
                selectedKeys={[String(updateInterval)]}
                onChange={(e) => setUpdateInterval(Number(e.target.value))}
                size="sm"
              >
                <SelectItem key="30">每 30 分钟</SelectItem>
                <SelectItem key="60">每 1 小时</SelectItem>
                <SelectItem key="180">每 3 小时</SelectItem>
                <SelectItem key="360">每 6 小时</SelectItem>
                <SelectItem key="720">每 12 小时</SelectItem>
                <SelectItem key="1440">每天</SelectItem>
              </Select>
            )}
          </ModalBody>
          <ModalFooter>
            <Button variant="flat" onPress={onSubClose}>
              取消
            </Button>
            <Button
              color="primary"
              onPress={handleSaveSubscription}
              isLoading={isSubmitting}
              isDisabled={!name || !url}
            >
              {editingSubscription ? '保存' : '添加'}
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      {/* 添加/编辑节点弹窗 */}
      <Modal isOpen={isNodeOpen} onClose={onNodeClose} size="lg">
        <ModalContent>
          <ModalHeader>{editingNode ? '编辑节点' : '添加节点'}</ModalHeader>
          <ModalBody>
            <div className="space-y-4">
              {/* 节点链接输入 - 仅在添加模式显示 */}
              {!editingNode && (
                <div className="space-y-2">
                  <div className="flex gap-2">
                    <Input
                      label="节点链接"
                      placeholder="粘贴节点链接，如 hysteria2://... vmess://... ss://... socks://..."
                      value={nodeUrl}
                      onChange={(e) => setNodeUrl(e.target.value)}
                      startContent={<Link className="w-4 h-4 text-gray-400" />}
                      className="flex-1"
                    />
                    <Button
                      color="primary"
                      variant="flat"
                      onPress={handleParseUrl}
                      isLoading={isParsing}
                      isDisabled={!nodeUrl.trim()}
                      className="self-end"
                    >
                      解析
                    </Button>
                  </div>
                  {parseError && (
                    <p className="text-sm text-danger">{parseError}</p>
                  )}
                  <p className="text-xs text-gray-400">
                    支持的协议: ss://, vmess://, vless://, trojan://, hysteria2://, tuic://, socks://
                  </p>
                </div>
              )}

              {/* 解析后显示节点信息 */}
              {nodeForm.tag && (
                <Card className="bg-default-100">
                  <CardBody className="py-3">
                    <div className="flex items-center gap-3">
                      <span className="text-2xl">{nodeForm.country_emoji || '🌐'}</span>
                      <div className="flex-1">
                        <h4 className="font-medium">{nodeForm.tag}</h4>
                        <p className="text-sm text-gray-500">
                          {nodeForm.type} · {nodeForm.server}:{nodeForm.server_port}
                        </p>
                      </div>
                      <Chip size="sm" variant="flat" color="success">已解析</Chip>
                    </div>
                  </CardBody>
                </Card>
              )}

              {/* 手动编辑区域 - 可折叠 */}
              <Accordion variant="bordered" selectionMode="multiple">
                <AccordionItem key="manual" aria-label="手动编辑" title="手动编辑节点信息">
                  <div className="space-y-4 pb-2">
                    <Input
                      label="节点名称"
                      placeholder="例如：香港-01"
                      value={nodeForm.tag}
                      onChange={(e) => setNodeForm({ ...nodeForm, tag: e.target.value })}
                    />

                    <div className="grid grid-cols-2 gap-4">
                      <Select
                        label="节点类型"
                        selectedKeys={[nodeForm.type]}
                        onChange={(e) => setNodeForm({ ...nodeForm, type: e.target.value })}
                      >
                        {nodeTypeOptions.map((opt) => (
                          <SelectItem key={opt.value} value={opt.value}>
                            {opt.label}
                          </SelectItem>
                        ))}
                      </Select>

                      <Select
                        label="国家/地区"
                        selectedKeys={nodeForm.country ? new Set([nodeForm.country]) : new Set(['HK'])}
                        onSelectionChange={(keys) => {
                          const selected = Array.from(keys)[0] as string;
                          const country = countryOptions.find(c => c.code === selected);
                          setNodeForm({
                            ...nodeForm,
                            country: selected,
                            country_emoji: country?.emoji || '🌐',
                          });
                        }}
                      >
                        {countryOptions.map((opt) => (
                          <SelectItem key={opt.code}>
                            {opt.emoji} {opt.name}
                          </SelectItem>
                        ))}
                      </Select>
                    </div>

                    <div className="grid grid-cols-2 gap-4">
                      <Input
                        label="服务器地址"
                        placeholder="example.com"
                        value={nodeForm.server}
                        onChange={(e) => setNodeForm({ ...nodeForm, server: e.target.value })}
                      />

                      <Input
                        type="number"
                        label="端口"
                        placeholder="443"
                        value={String(nodeForm.server_port)}
                        onChange={(e) => setNodeForm({ ...nodeForm, server_port: parseInt(e.target.value) || 443 })}
                      />
                    </div>
                  </div>
                </AccordionItem>
              </Accordion>

              <div className="flex items-center justify-between">
                <span>启用节点</span>
                <Switch
                  isSelected={nodeEnabled}
                  onValueChange={setNodeEnabled}
                />
              </div>
            </div>
          </ModalBody>
          <ModalFooter>
            <Button variant="flat" onPress={onNodeClose}>
              取消
            </Button>
            <Button
              color="primary"
              onPress={handleSaveNode}
              isLoading={isSubmitting}
              isDisabled={!nodeForm.tag || !nodeForm.server}
            >
              {editingNode ? '保存' : '添加'}
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      {/* 添加/编辑过滤器弹窗 */}
      <Modal isOpen={isFilterOpen} onClose={onFilterClose} size="2xl">
        <ModalContent>
          <ModalHeader>{editingFilter ? '编辑过滤器' : '添加过滤器'}</ModalHeader>
          <ModalBody>
            <div className="space-y-4">
              {/* 过滤器名称 */}
              <Input
                label="过滤器名称"
                placeholder="例如：日本高速节点、TikTok专用"
                value={filterForm.name}
                onChange={(e) => setFilterForm({ ...filterForm, name: e.target.value })}
                isRequired
              />
              {/* 包含国家 */}
              <Select
                label="包含国家"
                placeholder="选择要包含的国家（可多选）"
                selectionMode="multiple"
                selectedKeys={filterForm.include_countries}
                onSelectionChange={(keys) => {
                  setFilterForm({
                    ...filterForm,
                    include_countries: Array.from(keys) as string[]
                  })
                }}
              >
                {countryOptions.map((opt) => (
                  <SelectItem key={opt.code} value={opt.code}>
                    {opt.name}
                  </SelectItem>
                ))}
              </Select>

              {/* 排除国家 */}
              <Select
                label="排除国家"
                placeholder="选择要排除的国家（可多选）"
                selectionMode="multiple"
                selectedKeys={filterForm.exclude_countries}
                onSelectionChange={(keys) => setFilterForm({
                  ...filterForm,
                  exclude_countries: Array.from(keys) as string[]
                })}
              >
                {countryOptions.map((opt) => (
                  <SelectItem key={opt.code} value={opt.code}>
                    {opt.name}
                  </SelectItem>
                ))}
              </Select>

              {/* 包含关键字 */}
              <Input
                label="包含关键字"
                placeholder="用 | 分隔，如：高速|IPLC|专线"
                value={filterForm.include.join('|')}
                onChange={(e) => setFilterForm({
                  ...filterForm,
                  include: e.target.value ? e.target.value.split('|').filter(Boolean) : []
                })}
              />

              {/* 排除关键字 */}
              <Input
                label="排除关键字"
                placeholder="用 | 分隔，如：过期|维护|低速"
                value={filterForm.exclude.join('|')}
                onChange={(e) => setFilterForm({
                  ...filterForm,
                  exclude: e.target.value ? e.target.value.split('|').filter(Boolean) : []
                })}
              />

              {/* 全部节点开关 */}
              <div className="flex items-center justify-between">
                <div>
                  <span className="font-medium">应用于全部节点</span>
                  <p className="text-xs text-gray-400">启用后将匹配所有订阅的节点</p>
                </div>
                <Switch
                  isSelected={filterForm.all_nodes}
                  onValueChange={(checked) => setFilterForm({ ...filterForm, all_nodes: checked })}
                />
              </div>

              {/* 模式选择 */}
              <Select
                label="模式"
                selectedKeys={[filterForm.mode]}
                onChange={(e) => setFilterForm({ ...filterForm, mode: e.target.value })}
              >
                <SelectItem key="urltest" value="urltest">
                  自动测速 (urltest)
                </SelectItem>
                <SelectItem key="selector" value="selector">
                  手动选择 (selector)
                </SelectItem>
              </Select>

              {/* urltest 配置 */}
              {filterForm.mode === 'urltest' && (
                <Card className="bg-default-50">
                  <CardBody className="space-y-3">
                    <h4 className="font-medium text-sm">测速配置</h4>
                    <Input
                      label="测速 URL"
                      placeholder="https://www.gstatic.com/generate_204"
                      value={filterForm.urltest_config?.url || ''}
                      onChange={(e) => setFilterForm({
                        ...filterForm,
                        urltest_config: { ...filterForm.urltest_config!, url: e.target.value }
                      })}
                      size="sm"
                    />
                    <div className="grid grid-cols-2 gap-3">
                      <Input
                        label="测速间隔"
                        placeholder="5m"
                        value={filterForm.urltest_config?.interval || ''}
                        onChange={(e) => setFilterForm({
                          ...filterForm,
                          urltest_config: { ...filterForm.urltest_config!, interval: e.target.value }
                        })}
                        size="sm"
                      />
                      <Input
                        type="number"
                        label="容差阈值 (ms)"
                        placeholder="50"
                        value={String(filterForm.urltest_config?.tolerance || 50)}
                        onChange={(e) => setFilterForm({
                          ...filterForm,
                          urltest_config: { ...filterForm.urltest_config!, tolerance: parseInt(e.target.value) || 50 }
                        })}
                        size="sm"
                      />
                    </div>
                  </CardBody>
                </Card>
              )}

              {/* 启用开关 */}
              <div className="flex items-center justify-between">
                <span>启用过滤器</span>
                <Switch
                  isSelected={filterForm.enabled}
                  onValueChange={(checked) => setFilterForm({ ...filterForm, enabled: checked })}
                />
              </div>
            </div>
          </ModalBody>
          <ModalFooter>
            <Button variant="flat" onPress={onFilterClose}>
              取消
            </Button>
            <Button
              color="primary"
              onPress={handleSaveFilter}
              isLoading={isSubmitting}
              isDisabled={!filterForm.name}
            >
              {editingFilter ? '保存' : '添加'}
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      {/* 测速设置弹窗 */}
      <Modal isOpen={isSpeedTestOpen} onClose={onSpeedTestClose} size="lg" scrollBehavior="inside">
        <ModalContent>
          <ModalHeader className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Zap className="w-5 h-5 text-success" />
              <span>测速设置</span>
            </div>
            <Button
              color="success"
              size="sm"
              startContent={<Zap className="w-4 h-4" />}
              onPress={handleRunSpeedTest}
            >
              立即测速
            </Button>
          </ModalHeader>
          <ModalBody>
            {isLoadingProfiles ? (
              <div className="flex justify-center py-8">
                <Spinner size="lg" />
              </div>
            ) : (
              <div className="space-y-6">
                {/* 定时测速设置 */}
                <Accordion variant="bordered" defaultExpandedKeys={['timing']}>
                  <AccordionItem
                    key="timing"
                    aria-label="定时测速"
                    title={
                      <div className="flex items-center gap-2">
                        <Timer className="w-4 h-4 text-primary" />
                        <span>定时测速</span>
                      </div>
                    }
                  >
                    <div className="space-y-4 pb-2">
                      <div className="flex items-center justify-between">
                        <span>启用自动测速</span>
                        <Switch
                          isSelected={speedTestProfile.enabled}
                          onValueChange={(checked) =>
                            setSpeedTestProfile({ ...speedTestProfile, enabled: checked })
                          }
                        />
                      </div>

                      {speedTestProfile.enabled && (
                        <>
                          <div>
                            <p className="text-sm text-gray-500 mb-2">定时测速设置</p>
                            <div className="flex flex-wrap gap-2">
                              {cronPresets.map((preset) => (
                                <Chip
                                  key={preset.value}
                                  variant={selectedCronPreset === preset.value ? 'solid' : 'flat'}
                                  color={selectedCronPreset === preset.value ? 'primary' : 'default'}
                                  className="cursor-pointer"
                                  onClick={() => setSelectedCronPreset(preset.value)}
                                >
                                  {preset.label}
                                </Chip>
                              ))}
                            </div>
                          </div>
                          <div className="p-3 bg-default-100 rounded-lg">
                            <p className="text-xs text-gray-500">当前表达式</p>
                            <p className="text-sm font-mono">{selectedCronPreset}</p>
                          </div>
                        </>
                      )}
                    </div>
                  </AccordionItem>
                </Accordion>

                {/* 测速模式设置 */}
                <Accordion variant="bordered" defaultExpandedKeys={['mode']}>
                  <AccordionItem
                    key="mode"
                    aria-label="测速模式"
                    title={
                      <div className="flex items-center gap-2">
                        <Settings className="w-4 h-4 text-secondary" />
                        <span>测速模式</span>
                      </div>
                    }
                  >
                    <div className="space-y-4 pb-2">
                      <Select
                        label="测速模式"
                        selectedKeys={[speedTestProfile.mode]}
                        onChange={(e) =>
                          setSpeedTestProfile({ ...speedTestProfile, mode: e.target.value })
                        }
                      >
                        <SelectItem key="speed" value="speed">
                          延迟 + 下载速度测试
                        </SelectItem>
                        <SelectItem key="delay" value="delay">
                          仅延迟测试
                        </SelectItem>
                      </Select>

                      <Input
                        label="延迟测试 URL"
                        placeholder="https://cp.cloudflare.com/generate_204"
                        value={speedTestProfile.latency_url}
                        onChange={(e) =>
                          setSpeedTestProfile({ ...speedTestProfile, latency_url: e.target.value })
                        }
                      />

                      {speedTestProfile.mode === 'speed' && (
                        <Input
                          label="下载测速 URL"
                          placeholder="https://speed.cloudflare.com/__down?bytes=5000000"
                          value={speedTestProfile.speed_url}
                          onChange={(e) =>
                            setSpeedTestProfile({ ...speedTestProfile, speed_url: e.target.value })
                          }
                        />
                      )}

                      <Input
                        type="number"
                        label="超时时间 (秒)"
                        value={String(speedTestProfile.timeout)}
                        onChange={(e) =>
                          setSpeedTestProfile({
                            ...speedTestProfile,
                            timeout: parseInt(e.target.value) || 7,
                          })
                        }
                      />

                      <div className="grid grid-cols-2 gap-4">
                        <Input
                          type="number"
                          label="延迟并发数"
                          value={String(speedTestProfile.latency_concurrency)}
                          onChange={(e) =>
                            setSpeedTestProfile({
                              ...speedTestProfile,
                              latency_concurrency: parseInt(e.target.value) || 50,
                            })
                          }
                        />
                        {speedTestProfile.mode === 'speed' && (
                          <Input
                            type="number"
                            label="速度并发数"
                            value={String(speedTestProfile.speed_concurrency)}
                            onChange={(e) =>
                              setSpeedTestProfile({
                                ...speedTestProfile,
                                speed_concurrency: parseInt(e.target.value) || 5,
                              })
                            }
                          />
                        )}
                      </div>

                      <Divider />

                      <div className="space-y-3">
                        <div className="flex items-center justify-between">
                          <div>
                            <span className="text-sm">包含握手时间</span>
                            <p className="text-xs text-gray-400">延迟结果将包含 TLS 握手时间</p>
                          </div>
                          <Switch
                            isSelected={speedTestProfile.include_handshake}
                            onValueChange={(checked) =>
                              setSpeedTestProfile({ ...speedTestProfile, include_handshake: checked })
                            }
                          />
                        </div>

                        <div className="flex items-center justify-between">
                          <div>
                            <span className="text-sm">检测落地国家</span>
                            <p className="text-xs text-gray-400">获取代理的实际出口 IP 归属地</p>
                          </div>
                          <Switch
                            isSelected={speedTestProfile.detect_country}
                            onValueChange={(checked) =>
                              setSpeedTestProfile({ ...speedTestProfile, detect_country: checked })
                            }
                          />
                        </div>
                      </div>
                    </div>
                  </AccordionItem>
                </Accordion>
              </div>
            )}
          </ModalBody>
          <ModalFooter>
            <Button variant="flat" onPress={onSpeedTestClose}>
              取消
            </Button>
            <Button
              color="primary"
              onPress={handleSaveSpeedTestProfile}
              isLoading={isSubmitting}
            >
              保存设置
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </div>
  );
}

interface SubscriptionCardProps {
  subscription: Subscription;
  onRefresh: () => void;
  onEdit: () => void;
  onDelete: () => void;
  onToggle: () => void;
  loading: boolean;
}

function SubscriptionCard({ subscription: sub, onRefresh, onEdit, onDelete, onToggle, loading }: SubscriptionCardProps) {
  const [isExpanded, setIsExpanded] = useState(false);
  const [nodeSpeedInfos, setNodeSpeedInfos] = useState<Record<string, { delay: number; speed: number }>>({});
  const [testingNode, setTestingNode] = useState<string | null>(null);

  // 确保 nodes 是数组，处理 null 或 undefined 情况
  const nodes = sub.nodes || [];

  // 获取所有节点的测速信息（延迟和速度）
  const fetchDelays = async () => {
    try {
      const response = await nodeApi.getDelays();
      setNodeSpeedInfos(response.data.data || {});
    } catch (error) {
      console.error('获取测速信息失败:', error);
    }
  };

  // 测试单个节点延迟
  const testNodeDelay = async (tag: string) => {
    setTestingNode(tag);
    try {
      const response = await nodeApi.testDelay(tag);
      const { delay } = response.data.data;
      setNodeSpeedInfos(prev => ({
        ...prev,
        [tag]: { ...prev[tag], delay }
      }));
    } catch (error) {
      console.error('测速失败:', error);
    } finally {
      setTestingNode(null);
    }
  };

  // 展开时获取测速信息
  useEffect(() => {
    if (isExpanded && Object.keys(nodeSpeedInfos).length === 0) {
      fetchDelays();
    }
  }, [isExpanded]);

  // 按国家分组节点
  const nodesByCountry = nodes.reduce((acc, node) => {
    const country = node.country || 'OTHER';
    if (!acc[country]) {
      acc[country] = {
        emoji: node.country_emoji || '🌐',
        nodes: [],
      };
    }
    acc[country].nodes.push(node);
    return acc;
  }, {} as Record<string, { emoji: string; nodes: Node[] }>);

  // 格式化延迟显示
  const formatDelay = (info: { delay: number; speed: number } | undefined) => {
    if (!info || info.delay === 0) return null;
    if (info.delay < 0) return '超时';
    return `${info.delay}ms`;
  };

  // 格式化速度显示
  const formatSpeed = (info: { delay: number; speed: number } | undefined) => {
    if (!info || !info.speed || info.speed <= 0) return null;
    // backend stores speed in MB/s
    if (info.speed < 1) return `${(info.speed * 1024).toFixed(0)} KB/s`;
    return `${info.speed.toFixed(2)} MB/s`;
  };

  // 延迟颜色
  const getDelayColor = (info: { delay: number; speed: number } | undefined): 'success' | 'warning' | 'danger' | 'default' => {
    if (!info || info.delay === 0) return 'default';
    if (info.delay < 0) return 'danger';
    if (info.delay < 200) return 'success';
    if (info.delay < 500) return 'warning';
    return 'danger';
  };

  return (
    <Card>
      <CardHeader
        className="flex justify-between items-start cursor-pointer"
        onClick={(e) => {
          // 如果点击的是按钮区域，不触发展开
          if ((e.target as HTMLElement).closest('button')) return;
          setIsExpanded(!isExpanded);
        }}
      >
        <div className="flex items-center gap-3">
          <Chip
            color={sub.enabled ? 'success' : 'default'}
            variant="flat"
            size="sm"
          >
            {sub.enabled ? '已启用' : '已禁用'}
          </Chip>
          <div>
            <h3 className="text-lg font-semibold">{sub.name}</h3>
            <p className="text-sm text-gray-500">
              {sub.node_count} 个节点 · 更新于 {new Date(sub.updated_at).toLocaleString()}
            </p>
            {sub.traffic && (
              <p className="text-sm text-gray-500">
                已用: {formatBytes(sub.traffic.used)}　剩余: {formatBytes(sub.traffic.remaining)}　总计: {formatBytes(sub.traffic.total)}
                {sub.expire_at && `　到期: ${new Date(sub.expire_at).toLocaleDateString()}`}
              </p>
            )}
          </div>
        </div>
        <div className="flex gap-2 items-center">
          <Button
            size="sm"
            variant="flat"
            startContent={loading ? <Spinner size="sm" /> : <RefreshCw className="w-4 h-4" />}
            onPress={onRefresh}
            isDisabled={loading}
          >
            刷新
          </Button>
          <Button
            size="sm"
            variant="flat"
            startContent={<Pencil className="w-4 h-4" />}
            onPress={onEdit}
          >
            编辑
          </Button>
          <Button
            size="sm"
            variant="flat"
            color="danger"
            startContent={<Trash2 className="w-4 h-4" />}
            onPress={onDelete}
          >
            删除
          </Button>
          <Button
            isIconOnly
            size="sm"
            variant="light"
            onPress={() => setIsExpanded(!isExpanded)}
          >
            {isExpanded ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
          </Button>
          <Switch
            isSelected={sub.enabled}
            onValueChange={onToggle}
          />
        </div>
      </CardHeader>

      {isExpanded && (
        <CardBody className="pt-0">
          {/* 按国家分组的节点列表 */}
          <Accordion variant="bordered" selectionMode="multiple">
            {Object.entries(nodesByCountry).map(([country, data]) => (
              <AccordionItem
                key={country}
                aria-label={country}
                title={
                  <div className="flex items-center gap-2">
                    <span>{data.emoji}</span>
                    <span>{country}</span>
                    <Chip size="sm" variant="flat">{data.nodes.length}</Chip>
                  </div>
                }
              >
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-2">
                  {data.nodes.map((node, idx) => {
                    const speedInfo = nodeSpeedInfos[node.tag];
                    const delayText = formatDelay(speedInfo);
                    const speedText = formatSpeed(speedInfo);
                    return (
                      <div
                        key={idx}
                        className="flex items-center gap-2 p-2 bg-gray-50 dark:bg-gray-800 rounded text-sm group"
                      >
                        <span className="truncate flex-1">{node.tag}</span>
                        {delayText && (
                          <Chip size="sm" variant="flat" color={getDelayColor(speedInfo)}>
                            {delayText}
                          </Chip>
                        )}
                        {speedText && (
                          <Chip size="sm" variant="flat" color="primary">
                            {speedText}
                          </Chip>
                        )}
                        <Chip size="sm" variant="flat">
                          {node.type}
                        </Chip>
                        <Button
                          isIconOnly
                          size="sm"
                          variant="light"
                          className="opacity-0 group-hover:opacity-100 transition-opacity"
                          onPress={() => testNodeDelay(node.tag)}
                          isLoading={testingNode === node.tag}
                        >
                          <RefreshCw className="w-3 h-3" />
                        </Button>
                      </div>
                    );
                  })}
                </div>
              </AccordionItem>
            ))}
          </Accordion>
        </CardBody>
      )}
    </Card>
  );
}

// 按国家/地区显示节点的组件
interface CountryNodesListProps {
  countryGroups: { code: string; name: string; emoji: string; node_count: number }[];
}

function CountryNodesList({ countryGroups }: CountryNodesListProps) {
  const { subscriptions, manualNodes } = useStore();
  const [expandedCountries, setExpandedCountries] = useState<Set<string>>(new Set());
  const [countryNodes, setCountryNodes] = useState<Record<string, Node[]>>({});
  const [loadingCountries, setLoadingCountries] = useState<Set<string>>(new Set());
  const [nodeSpeedInfos, setNodeSpeedInfos] = useState<Record<string, { delay: number; speed: number }>>({});
  const [testingNode, setTestingNode] = useState<string | null>(null);
  const [searchText, setSearchText] = useState('');

  // 获取所有节点的测速信息
  const fetchSpeedInfos = useCallback(async () => {
    try {
      const response = await nodeApi.getDelays();
      setNodeSpeedInfos(response.data.data || {});
    } catch (error) {
      console.error('获取测速信息失败:', error);
    }
  }, []);

  // 初始化时加载测速信息
  useEffect(() => {
    fetchSpeedInfos();
  }, [fetchSpeedInfos]);

  // 加载指定国家的节点
  const loadCountryNodes = async (code: string) => {
    if (countryNodes[code]) return; // 已加载

    setLoadingCountries(prev => new Set(prev).add(code));
    try {
      const response = await nodeApi.getByCountry(code);
      setCountryNodes(prev => ({ ...prev, [code]: response.data.data || [] }));
    } catch (error) {
      console.error(`加载 ${code} 节点失败:`, error);
    } finally {
      setLoadingCountries(prev => {
        const next = new Set(prev);
        next.delete(code);
        return next;
      });
    }
  };

  // 切换国家展开状态
  const toggleCountry = (code: string) => {
    const next = new Set(expandedCountries);
    if (next.has(code)) {
      next.delete(code);
    } else {
      next.add(code);
      loadCountryNodes(code);
    }
    setExpandedCountries(next);
  };

  // 测试单个节点延迟
  const testNodeDelay = async (tag: string) => {
    setTestingNode(tag);
    try {
      const response = await nodeApi.testDelay(tag);
      const { delay } = response.data.data;
      setNodeSpeedInfos(prev => ({
        ...prev,
        [tag]: { ...prev[tag], delay }
      }));
    } catch (error) {
      console.error('测速失败:', error);
    } finally {
      setTestingNode(null);
    }
  };

  // 格式化延迟显示
  const formatDelay = (info: { delay: number; speed: number } | undefined) => {
    if (!info || info.delay === 0) return null;
    if (info.delay < 0) return '超时';
    return `${info.delay}ms`;
  };

  // 格式化速度显示
  const formatSpeed = (info: { delay: number; speed: number } | undefined) => {
    if (!info || !info.speed || info.speed <= 0) return null;
    // backend stores speed in MB/s
    if (info.speed < 1) return `${(info.speed * 1024).toFixed(0)} KB/s`;
    return `${info.speed.toFixed(2)} MB/s`;
  };

  // 延迟颜色
  const getDelayColor = (info: { delay: number; speed: number } | undefined): 'success' | 'warning' | 'danger' | 'default' => {
    if (!info || info.delay === 0) return 'default';
    if (info.delay < 0) return 'danger';
    if (info.delay < 200) return 'success';
    if (info.delay < 500) return 'warning';
    return 'danger';
  };

  // 搜索过滤逻辑
  const filteredGroups = useMemo(() => {
    if (!searchText.trim()) return countryGroups;

    const lowerText = searchText.toLowerCase();
    // 收集所有节点
    const allNodes = [
      ...subscriptions.flatMap(s => s.nodes || []),
      ...manualNodes.map(m => m.node)
    ];

    // 过滤匹配搜索词的节点
    const filteredNodes = allNodes.filter(n =>
      n.tag.toLowerCase().includes(lowerText) ||
      n.server.toLowerCase().includes(lowerText)
    );

    // 按国家分组
    const groups: Record<string, Node[]> = {};
    filteredNodes.forEach(n => {
      const country = n.country || 'OTHER';
      if (!groups[country]) groups[country] = [];
      groups[country].push(n);
    });

    return Object.entries(groups).map(([code, nodes]) => {
      const existing = countryGroups.find(g => g.code === code);
      return {
        code,
        name: existing?.name || code,
        emoji: existing?.emoji || '🌐',
        node_count: nodes.length,
        _searchNodes: nodes // 搜索模式下直接携带节点数据
      };
    });
  }, [searchText, countryGroups, subscriptions, manualNodes]);

  return (
    <div className="space-y-4 mt-4">
      <Input
        placeholder="搜索节点名称或服务器地址..."
        value={searchText}
        onChange={(e) => setSearchText(e.target.value)}
        startContent={<Search className="w-4 h-4 text-gray-400" />}
        isClearable
        onClear={() => setSearchText('')}
        size="sm"
      />

      <div className="space-y-2">
        {filteredGroups.map((group) => {
          const isSearchMode = !!searchText.trim();
          const isExpanded = isSearchMode || expandedCountries.has(group.code);
          const isLoading = !isSearchMode && loadingCountries.has(group.code);
          // 搜索模式下使用 _searchNodes，普通模式使用懒加载的 countryNodes
          const nodes = isSearchMode ? ((group as any)._searchNodes || []) : (countryNodes[group.code] || []);

        return (
          <Card key={group.code}>
            <CardHeader
              className={`flex justify-between items-center py-3 ${!isSearchMode ? 'cursor-pointer' : ''}`}
              onClick={() => !isSearchMode && toggleCountry(group.code)}
            >
              <div className="flex items-center gap-3">
                <span className="text-2xl">{group.emoji}</span>
                <div>
                  <h3 className="font-semibold">{group.code}</h3>
                  <p className="text-xs text-gray-500">{group.name}</p>
                </div>
                <Chip size="sm" variant="flat">{group.node_count}</Chip>
              </div>
              {!isSearchMode && (
                <Button
                  isIconOnly
                  size="sm"
                  variant="light"
                >
                  {isExpanded ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
                </Button>
              )}
            </CardHeader>

            {isExpanded && (
              <CardBody className="pt-0">
                {isLoading ? (
                  <div className="flex justify-center py-4">
                    <Spinner size="sm" />
                  </div>
                ) : (
                  <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-2">
                    {nodes.map((node: Node, idx: number) => {
                      const speedInfo = nodeSpeedInfos[node.tag];
                      const delayText = formatDelay(speedInfo);
                      const speedText = formatSpeed(speedInfo);
                      return (
                        <div
                          key={idx}
                          className="flex items-center gap-2 p-2 bg-gray-50 dark:bg-gray-800 rounded text-sm group"
                        >
                          <span className="truncate flex-1">{node.tag}</span>
                          {delayText && (
                            <Chip size="sm" variant="flat" color={getDelayColor(speedInfo)}>
                              {delayText}
                            </Chip>
                          )}
                          {speedText && (
                            <Chip size="sm" variant="flat" color="primary">
                              {speedText}
                            </Chip>
                          )}
                          <Chip size="sm" variant="flat">
                            {node.type}
                          </Chip>
                          <Button
                            isIconOnly
                            size="sm"
                            variant="light"
                            className="opacity-0 group-hover:opacity-100 transition-opacity"
                            onPress={() => testNodeDelay(node.tag)}
                            isLoading={testingNode === node.tag}
                          >
                            <RefreshCw className="w-3 h-3" />
                          </Button>
                        </div>
                      );
                    })}
                  </div>
                )}
              </CardBody>
            )}
          </Card>
        );
      })}
      </div>
    </div>
  );
}
