import { useEffect, useState, useCallback } from 'react';
import {
  Card,
  CardBody,
  CardHeader,
  Button,
  Chip,
  Modal,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  useDisclosure,
  Input,
  Select,
  SelectItem,
  Switch,
  Progress,
  Table,
  TableHeader,
  TableColumn,
  TableBody,
  TableRow,
  TableCell,
  Tabs,
  Tab,
  Tooltip,
  Spinner,
  Accordion,
  AccordionItem,
} from '@nextui-org/react';
import {
  Play,
  Pause,
  Settings,
  Plus,
  Pencil,
  Trash2,
  Timer,
  Zap,
  RefreshCw,
  Activity,
  Clock,
  ChevronDown,
  ChevronUp,
} from 'lucide-react';
import { speedtestApi } from '../api';
import { toast } from '../components/Toast';

// 类型定义
interface SpeedTestProfile {
  ID: number;
  name: string;
  enabled: boolean;
  is_default: boolean;
  auto_test: boolean;
  schedule_type: string;
  schedule_interval: number;
  schedule_cron: string;
  mode: string;
  latency_url: string;
  speed_url: string;
  timeout: number;
  latency_concurrency: number;
  speed_concurrency: number;
  include_handshake: boolean;
  detect_country: boolean;
  landing_ip_url: string;
  speed_record_mode: string;
  peak_sample_interval: number;
  source_filter: string[];
  country_filter: string[];
  tag_filter: string[];
  last_run_at: string;
  next_run_at: string;
}

interface SpeedTestTask {
  ID: string;
  profile_id: number;
  profile_name: string;
  status: string;
  trigger_type: string;
  total: number;
  completed: number;
  success: number;
  failed: number;
  current_node: string;
  error: string;
  started_at: string;
  finished_at: string;
}

interface SpeedTestHistory {
  id: number;
  task_id: string;
  node_id: number;
  delay: number;
  speed: number;
  status: string;
  landing_ip: string;
  tested_at: string;
}

// 默认策略表单
const defaultProfileForm = {
  name: '',
  enabled: false,
  auto_test: false,
  schedule_type: 'interval',
  schedule_interval: 60,
  schedule_cron: '',
  mode: 'delay',
  latency_url: 'https://cp.cloudflare.com/generate_204',
  speed_url: 'https://speed.cloudflare.com/__down?bytes=5000000',
  timeout: 7,
  latency_concurrency: 50,
  speed_concurrency: 5,
  include_handshake: true,
  detect_country: false,
  landing_ip_url: 'https://api.ipify.org',
  speed_record_mode: 'average',
  peak_sample_interval: 100,
  source_filter: [] as string[],
  country_filter: [] as string[],
  tag_filter: [] as string[],
};

// 延迟测试 URL 选项
const latencyUrlOptions = [
  { value: 'https://cp.cloudflare.com/generate_204', label: 'Cloudflare 204' },
  { value: 'https://www.gstatic.com/generate_204', label: 'Google 204' },
  { value: 'http://www.msftconnecttest.com/connecttest.txt', label: 'Microsoft' },
];

// 速度测试 URL 选项
const speedUrlOptions = [
  { value: 'https://speed.cloudflare.com/__down?bytes=5000000', label: 'Cloudflare 5MB' },
  { value: 'https://speed.cloudflare.com/__down?bytes=25000000', label: 'Cloudflare 25MB' },
  { value: 'https://speed.cloudflare.com/__down?bytes=100000000', label: 'Cloudflare 100MB' },
];

// 落地IP检测接口选项
const landingIpOptions = [
  { value: 'https://api.ipify.org', label: 'ipify.org' },
  { value: 'https://api.ip.sb/ip', label: 'ip.sb' },
  { value: 'https://ifconfig.me/ip', label: 'ifconfig.me' },
];

export default function SpeedTest() {
  // 状态
  const [profiles, setProfiles] = useState<SpeedTestProfile[]>([]);
  const [tasks, setTasks] = useState<SpeedTestTask[]>([]);
  const [runningTasks, setRunningTasks] = useState<Record<string, boolean>>({});
  const [loading, setLoading] = useState(false);
  const [tasksLoading, setTasksLoading] = useState(false);
  const [activeTab, setActiveTab] = useState('profiles');

  // 弹窗状态
  const { isOpen, onOpen, onClose } = useDisclosure();
  const [editingProfile, setEditingProfile] = useState<SpeedTestProfile | null>(null);
  const [profileForm, setProfileForm] = useState(defaultProfileForm);
  const [isSubmitting, setIsSubmitting] = useState(false);

  // 任务详情弹窗
  const { isOpen: isTaskOpen, onOpen: onTaskOpen, onClose: onTaskClose } = useDisclosure();
  const [selectedTask, setSelectedTask] = useState<SpeedTestTask | null>(null);
  const [taskHistory, setTaskHistory] = useState<SpeedTestHistory[]>([]);

  // 加载策略列表
  const loadProfiles = useCallback(async () => {
    setLoading(true);
    try {
      const response = await speedtestApi.getProfiles();
      setProfiles(response.data || []);
    } catch (error: any) {
      toast.error(error.response?.data?.error || '加载策略失败');
    } finally {
      setLoading(false);
    }
  }, []);

  // 加载任务列表
  const loadTasks = useCallback(async () => {
    setTasksLoading(true);
    try {
      const response = await speedtestApi.getTasks(50);
      const data = response.data;
      setTasks(data.tasks || []);
      setRunningTasks(data.running || {});
    } catch (error: any) {
      toast.error(error.response?.data?.error || '加载任务失败');
    } finally {
      setTasksLoading(false);
    }
  }, []);

  // 初始化加载
  useEffect(() => {
    loadProfiles();
    loadTasks();
  }, [loadProfiles, loadTasks]);

  // 定时刷新任务状态（当有运行中的任务时）
  useEffect(() => {
    const hasRunning = Object.keys(runningTasks).length > 0;
    if (!hasRunning) return;

    const interval = setInterval(() => {
      loadTasks();
    }, 3000);

    return () => clearInterval(interval);
  }, [runningTasks, loadTasks]);

  // 打开新建弹窗
  const handleCreate = () => {
    setEditingProfile(null);
    setProfileForm(defaultProfileForm);
    onOpen();
  };

  // 打开编辑弹窗
  const handleEdit = (profile: SpeedTestProfile) => {
    setEditingProfile(profile);
    setProfileForm({
      name: profile.name,
      enabled: profile.enabled,
      auto_test: profile.auto_test,
      schedule_type: profile.schedule_type || 'interval',
      schedule_interval: profile.schedule_interval || 60,
      schedule_cron: profile.schedule_cron || '',
      mode: profile.mode || 'delay',
      latency_url: profile.latency_url || defaultProfileForm.latency_url,
      speed_url: profile.speed_url || defaultProfileForm.speed_url,
      timeout: profile.timeout || 7,
      latency_concurrency: profile.latency_concurrency || 50,
      speed_concurrency: profile.speed_concurrency || 5,
      include_handshake: profile.include_handshake,
      detect_country: profile.detect_country,
      landing_ip_url: profile.landing_ip_url || defaultProfileForm.landing_ip_url,
      speed_record_mode: profile.speed_record_mode || 'average',
      peak_sample_interval: profile.peak_sample_interval || 100,
      source_filter: profile.source_filter || [],
      country_filter: profile.country_filter || [],
      tag_filter: profile.tag_filter || [],
    });
    onOpen();
  };

  // 保存策略
  const handleSave = async () => {
    if (!profileForm.name.trim()) {
      toast.error('请输入策略名称');
      return;
    }

    setIsSubmitting(true);
    try {
      if (editingProfile) {
        await speedtestApi.updateProfile(editingProfile.ID, profileForm);
        toast.success('更新成功');
      } else {
        await speedtestApi.createProfile(profileForm);
        toast.success('创建成功');
      }
      onClose();
      loadProfiles();
    } catch (error: any) {
      toast.error(error.response?.data?.error || '保存失败');
    } finally {
      setIsSubmitting(false);
    }
  };

  // 删除策略
  const handleDelete = async (profile: SpeedTestProfile) => {
    if (profile.is_default) {
      toast.error('不能删除默认策略');
      return;
    }
    if (!confirm(`确定要删除策略 "${profile.name}" 吗？`)) return;

    try {
      await speedtestApi.deleteProfile(profile.ID);
      toast.success('删除成功');
      loadProfiles();
    } catch (error: any) {
      toast.error(error.response?.data?.error || '删除失败');
    }
  };

  // 执行测速
  const handleRunTest = async (profileId?: number) => {
    try {
      await speedtestApi.runTest(profileId);
      toast.success('测速任务已启动');
      loadTasks();
    } catch (error: any) {
      toast.error(error.response?.data?.error || '启动测速失败');
    }
  };

  // 取消任务
  const handleCancelTask = async (taskId: string) => {
    try {
      await speedtestApi.cancelTask(taskId);
      toast.success('任务已取消');
      loadTasks();
    } catch (error: any) {
      toast.error(error.response?.data?.error || '取消失败');
    }
  };

  // 查看任务详情
  const handleViewTask = async (task: SpeedTestTask) => {
    setSelectedTask(task);
    try {
      const response = await speedtestApi.getTask(task.ID);
      const data = response.data;
      setSelectedTask(data.task);
      setTaskHistory(data.history || []);
    } catch (error) {
      console.error('加载任务详情失败:', error);
    }
    onTaskOpen();
  };

  // 格式化时间
  const formatTime = (timeStr: string | null) => {
    if (!timeStr) return '-';
    return new Date(timeStr).toLocaleString('zh-CN', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  // 格式化延迟
  const formatDelay = (delay: number) => {
    if (delay < 0) return '超时';
    if (delay === 0) return '-';
    return `${delay}ms`;
  };

  // 格式化速度
  const formatSpeed = (speed: number) => {
    if (speed <= 0) return '-';
    // backend stores speed in MB/s
    if (speed < 1) return `${(speed * 1024).toFixed(0)} KB/s`;
    return `${speed.toFixed(2)} MB/s`;
  };

  // 获取状态颜色
  const getStatusColor = (status: string): 'success' | 'warning' | 'danger' | 'default' | 'primary' => {
    switch (status) {
      case 'running':
        return 'primary';
      case 'completed':
        return 'success';
      case 'cancelled':
        return 'warning';
      case 'failed':
        return 'danger';
      default:
        return 'default';
    }
  };

  // 获取状态文本
  const getStatusText = (status: string) => {
    switch (status) {
      case 'pending':
        return '等待中';
      case 'running':
        return '运行中';
      case 'completed':
        return '已完成';
      case 'cancelled':
        return '已取消';
      case 'failed':
        return '失败';
      default:
        return status;
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-gray-800 dark:text-white">测速管理</h1>
          <p className="text-sm text-gray-500 mt-1">
            配置测速策略，执行节点延迟和速度测试
          </p>
        </div>
        <div className="flex gap-2">
          <Button
            color="success"
            startContent={<Play className="w-4 h-4" />}
            onPress={() => handleRunTest()}
          >
            快速测速
          </Button>
          <Button
            color="primary"
            startContent={<Plus className="w-4 h-4" />}
            onPress={handleCreate}
          >
            新建策略
          </Button>
        </div>
      </div>

      <Tabs
        selectedKey={activeTab}
        onSelectionChange={(key) => setActiveTab(key as string)}
        aria-label="测速管理"
      >
        {/* 策略管理 Tab */}
        <Tab key="profiles" title="测速策略">
          {loading ? (
            <div className="flex justify-center py-12">
              <Spinner size="lg" />
            </div>
          ) : profiles.length === 0 ? (
            <Card className="mt-4">
              <CardBody className="py-12 text-center">
                <Settings className="w-12 h-12 mx-auto text-gray-300 mb-4" />
                <p className="text-gray-500">暂无测速策略</p>
                <Button
                  color="primary"
                  variant="flat"
                  className="mt-4"
                  onPress={handleCreate}
                >
                  创建第一个策略
                </Button>
              </CardBody>
            </Card>
          ) : (
            <div className="grid gap-4 mt-4">
              {profiles.map((profile) => (
                <ProfileCard
                  key={profile.ID}
                  profile={profile}
                  onEdit={() => handleEdit(profile)}
                  onDelete={() => handleDelete(profile)}
                  onRun={() => handleRunTest(profile.ID)}
                  formatTime={formatTime}
                />
              ))}
            </div>
          )}
        </Tab>

        {/* 任务历史 Tab */}
        <Tab key="tasks" title="任务历史">
          <div className="mt-4">
            <div className="flex justify-end mb-4">
              <Button
                size="sm"
                variant="flat"
                startContent={tasksLoading ? <Spinner size="sm" /> : <RefreshCw className="w-4 h-4" />}
                onPress={loadTasks}
                isDisabled={tasksLoading}
              >
                刷新
              </Button>
            </div>

            {tasksLoading && tasks.length === 0 ? (
              <div className="flex justify-center py-12">
                <Spinner size="lg" />
              </div>
            ) : tasks.length === 0 ? (
              <Card>
                <CardBody className="py-12 text-center">
                  <Activity className="w-12 h-12 mx-auto text-gray-300 mb-4" />
                  <p className="text-gray-500">暂无测速任务</p>
                </CardBody>
              </Card>
            ) : (
              <Table aria-label="测速任务列表">
                <TableHeader>
                  <TableColumn>策略</TableColumn>
                  <TableColumn>状态</TableColumn>
                  <TableColumn>进度</TableColumn>
                  <TableColumn>触发方式</TableColumn>
                  <TableColumn>开始时间</TableColumn>
                  <TableColumn>操作</TableColumn>
                </TableHeader>
                <TableBody>
                  {tasks.map((task) => (
                    <TableRow key={task.ID}>
                      <TableCell>{task.profile_name || '默认策略'}</TableCell>
                      <TableCell>
                        <Chip size="sm" color={getStatusColor(task.status)} variant="flat">
                          {getStatusText(task.status)}
                        </Chip>
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-2 min-w-[120px]">
                          {task.status === 'running' ? (
                            <Progress
                              size="sm"
                              value={(task.completed / task.total) * 100}
                              className="max-w-[100px]"
                            />
                          ) : null}
                          <span className="text-sm text-gray-500">
                            {task.completed}/{task.total}
                          </span>
                        </div>
                      </TableCell>
                      <TableCell>
                        <Chip size="sm" variant="flat">
                          {task.trigger_type === 'manual' ? '手动' : '定时'}
                        </Chip>
                      </TableCell>
                      <TableCell>{formatTime(task.started_at)}</TableCell>
                      <TableCell>
                        <div className="flex gap-1">
                          <Button
                            size="sm"
                            variant="light"
                            onPress={() => handleViewTask(task)}
                          >
                            详情
                          </Button>
                          {runningTasks[task.ID] && (
                            <Button
                              size="sm"
                              color="danger"
                              variant="light"
                              startContent={<Pause className="w-3 h-3" />}
                              onPress={() => handleCancelTask(task.ID)}
                            >
                              取消
                            </Button>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </div>
        </Tab>
      </Tabs>

      {/* 策略编辑弹窗 */}
      <Modal isOpen={isOpen} onClose={onClose} size="2xl" scrollBehavior="inside">
        <ModalContent>
          <ModalHeader>
            {editingProfile ? '编辑测速策略' : '新建测速策略'}
          </ModalHeader>
          <ModalBody>
            <div className="space-y-6">
              {/* 基本信息 */}
              <Input
                label="策略名称"
                placeholder="例如：每日全量测速"
                value={profileForm.name}
                onChange={(e) => setProfileForm({ ...profileForm, name: e.target.value })}
                isRequired
              />

              {/* 定时配置 */}
              <Accordion variant="bordered" selectionMode="multiple" defaultExpandedKeys={['schedule']}>
                <AccordionItem key="schedule" aria-label="定时配置" title="定时配置">
                  <div className="space-y-4 pb-2">
                    <div className="flex items-center justify-between">
                      <div>
                        <span className="font-medium">启用定时测速</span>
                        <p className="text-xs text-gray-400">按计划自动执行测速任务</p>
                      </div>
                      <Switch
                        isSelected={profileForm.auto_test && profileForm.enabled}
                        onValueChange={(checked) =>
                          setProfileForm({ ...profileForm, auto_test: checked, enabled: checked })
                        }
                      />
                    </div>

                    {profileForm.auto_test && (
                      <>
                        <Select
                          label="调度类型"
                          selectedKeys={[profileForm.schedule_type]}
                          onChange={(e) =>
                            setProfileForm({ ...profileForm, schedule_type: e.target.value })
                          }
                        >
                          <SelectItem key="interval" value="interval">
                            固定间隔
                          </SelectItem>
                          <SelectItem key="cron" value="cron">
                            Cron 表达式
                          </SelectItem>
                        </Select>

                        {profileForm.schedule_type === 'interval' ? (
                          <Input
                            type="number"
                            label="间隔时间（分钟）"
                            value={String(profileForm.schedule_interval)}
                            onChange={(e) =>
                              setProfileForm({
                                ...profileForm,
                                schedule_interval: parseInt(e.target.value) || 60,
                              })
                            }
                          />
                        ) : (
                          <Input
                            label="Cron 表达式"
                            placeholder="0 0 * * *"
                            value={profileForm.schedule_cron}
                            onChange={(e) =>
                              setProfileForm({ ...profileForm, schedule_cron: e.target.value })
                            }
                            description="例如: 0 0 * * * 表示每天 0 点执行"
                          />
                        )}
                      </>
                    )}
                  </div>
                </AccordionItem>

                {/* 测速模式 */}
                <AccordionItem key="mode" aria-label="测速模式" title="测速模式">
                  <div className="space-y-4 pb-2">
                    <Select
                      label="测速模式"
                      selectedKeys={[profileForm.mode]}
                      onChange={(e) => setProfileForm({ ...profileForm, mode: e.target.value })}
                      description={
                        profileForm.mode === 'speed'
                          ? '两阶段测试：先并发测延迟，再低并发测下载速度'
                          : '仅测试延迟，速度更快，适合快速筛选可用节点'
                      }
                    >
                      <SelectItem key="delay" value="delay">
                        仅延迟测试（更快）
                      </SelectItem>
                      <SelectItem key="speed" value="speed">
                        延迟 + 下载速度测试
                      </SelectItem>
                    </Select>

                    <Select
                      label="延迟测试 URL"
                      selectedKeys={[profileForm.latency_url]}
                      onChange={(e) =>
                        setProfileForm({ ...profileForm, latency_url: e.target.value })
                      }
                    >
                      {latencyUrlOptions.map((opt) => (
                        <SelectItem key={opt.value} value={opt.value}>
                          {opt.label}
                        </SelectItem>
                      ))}
                    </Select>

                    {profileForm.mode === 'speed' && (
                      <Select
                        label="速度测试 URL"
                        selectedKeys={[profileForm.speed_url]}
                        onChange={(e) =>
                          setProfileForm({ ...profileForm, speed_url: e.target.value })
                        }
                      >
                        {speedUrlOptions.map((opt) => (
                          <SelectItem key={opt.value} value={opt.value}>
                            {opt.label}
                          </SelectItem>
                        ))}
                      </Select>
                    )}

                    <Input
                      type="number"
                      label="超时时间（秒）"
                      value={String(profileForm.timeout)}
                      onChange={(e) =>
                        setProfileForm({ ...profileForm, timeout: parseInt(e.target.value) || 7 })
                      }
                    />

                    {profileForm.mode === 'speed' && (
                      <Select
                        label="速度记录模式"
                        selectedKeys={[profileForm.speed_record_mode]}
                        onChange={(e) =>
                          setProfileForm({ ...profileForm, speed_record_mode: e.target.value })
                        }
                      >
                        <SelectItem key="average" value="average">
                          平均速度（推荐）
                        </SelectItem>
                        <SelectItem key="peak" value="peak">
                          峰值速度
                        </SelectItem>
                      </Select>
                    )}
                  </div>
                </AccordionItem>

                {/* 性能参数 */}
                <AccordionItem key="performance" aria-label="性能参数" title="性能参数">
                  <div className="space-y-4 pb-2">
                    <div className="flex items-center justify-between">
                      <div>
                        <span className="font-medium">延迟包含握手时间</span>
                        <p className="text-xs text-gray-400">
                          {profileForm.include_handshake
                            ? '测量完整连接时间，反映真实体验'
                            : '排除握手开销，精确评估线路质量'}
                        </p>
                      </div>
                      <Switch
                        isSelected={profileForm.include_handshake}
                        onValueChange={(checked) =>
                          setProfileForm({ ...profileForm, include_handshake: checked })
                        }
                      />
                    </div>

                    <div className="grid grid-cols-2 gap-4">
                      <Input
                        type="number"
                        label="延迟测试并发"
                        value={String(profileForm.latency_concurrency)}
                        onChange={(e) =>
                          setProfileForm({
                            ...profileForm,
                            latency_concurrency: parseInt(e.target.value) || 50,
                          })
                        }
                        description="0 = 智能动态"
                      />
                      <Input
                        type="number"
                        label="速度测试并发"
                        value={String(profileForm.speed_concurrency)}
                        onChange={(e) =>
                          setProfileForm({
                            ...profileForm,
                            speed_concurrency: parseInt(e.target.value) || 5,
                          })
                        }
                        description="0 = 智能动态"
                      />
                    </div>

                    <div className="flex items-center justify-between">
                      <div>
                        <span className="font-medium">检测落地 IP 国家</span>
                        <p className="text-xs text-gray-400">测速时获取节点出口国家</p>
                      </div>
                      <Switch
                        isSelected={profileForm.detect_country}
                        onValueChange={(checked) =>
                          setProfileForm({ ...profileForm, detect_country: checked })
                        }
                      />
                    </div>

                    {profileForm.detect_country && (
                      <Select
                        label="IP 查询接口"
                        selectedKeys={[profileForm.landing_ip_url]}
                        onChange={(e) =>
                          setProfileForm({ ...profileForm, landing_ip_url: e.target.value })
                        }
                      >
                        {landingIpOptions.map((opt) => (
                          <SelectItem key={opt.value} value={opt.value}>
                            {opt.label}
                          </SelectItem>
                        ))}
                      </Select>
                    )}
                  </div>
                </AccordionItem>
              </Accordion>
            </div>
          </ModalBody>
          <ModalFooter>
            <Button variant="flat" onPress={onClose}>
              取消
            </Button>
            <Button
              color="primary"
              onPress={handleSave}
              isLoading={isSubmitting}
              isDisabled={!profileForm.name.trim()}
            >
              {editingProfile ? '保存' : '创建'}
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      {/* 任务详情弹窗 */}
      <Modal isOpen={isTaskOpen} onClose={onTaskClose} size="3xl" scrollBehavior="inside">
        <ModalContent>
          <ModalHeader>
            <div className="flex items-center gap-2">
              <Activity className="w-5 h-5" />
              任务详情
            </div>
          </ModalHeader>
          <ModalBody>
            {selectedTask && (
              <div className="space-y-4">
                {/* 任务信息 */}
                <Card>
                  <CardBody>
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                      <div>
                        <p className="text-xs text-gray-500">策略</p>
                        <p className="font-medium">{selectedTask.profile_name || '默认策略'}</p>
                      </div>
                      <div>
                        <p className="text-xs text-gray-500">状态</p>
                        <Chip size="sm" color={getStatusColor(selectedTask.status)} variant="flat">
                          {getStatusText(selectedTask.status)}
                        </Chip>
                      </div>
                      <div>
                        <p className="text-xs text-gray-500">进度</p>
                        <p className="font-medium">
                          {selectedTask.completed}/{selectedTask.total}
                          {selectedTask.status === 'running' && (
                            <span className="text-gray-400 ml-2">
                              ({Math.round((selectedTask.completed / selectedTask.total) * 100)}%)
                            </span>
                          )}
                        </p>
                      </div>
                      <div>
                        <p className="text-xs text-gray-500">成功/失败</p>
                        <p className="font-medium">
                          <span className="text-success">{selectedTask.success}</span>
                          {' / '}
                          <span className="text-danger">{selectedTask.failed}</span>
                        </p>
                      </div>
                    </div>

                    {selectedTask.status === 'running' && (
                      <div className="mt-4">
                        <Progress
                          size="sm"
                          value={(selectedTask.completed / selectedTask.total) * 100}
                          color="primary"
                          className="mb-2"
                        />
                        {selectedTask.current_node && (
                          <p className="text-sm text-gray-500">
                            正在测试: {selectedTask.current_node}
                          </p>
                        )}
                      </div>
                    )}

                    {selectedTask.error && (
                      <div className="mt-4 p-3 bg-danger-50 text-danger rounded-lg text-sm">
                        {selectedTask.error}
                      </div>
                    )}
                  </CardBody>
                </Card>

                {/* 测速结果 */}
                {taskHistory.length > 0 && (
                  <div>
                    <h4 className="font-medium mb-2">测速结果</h4>
                    <Table aria-label="测速结果" className="max-h-[400px]">
                      <TableHeader>
                        <TableColumn>节点 ID</TableColumn>
                        <TableColumn>延迟</TableColumn>
                        <TableColumn>速度</TableColumn>
                        <TableColumn>状态</TableColumn>
                        <TableColumn>落地 IP</TableColumn>
                        <TableColumn>测试时间</TableColumn>
                      </TableHeader>
                      <TableBody>
                        {taskHistory.map((item) => (
                          <TableRow key={item.id}>
                            <TableCell>{item.node_id}</TableCell>
                            <TableCell>
                              <span
                                className={
                                  item.delay < 0
                                    ? 'text-danger'
                                    : item.delay < 200
                                    ? 'text-success'
                                    : item.delay < 500
                                    ? 'text-warning'
                                    : 'text-danger'
                                }
                              >
                                {formatDelay(item.delay)}
                              </span>
                            </TableCell>
                            <TableCell>{formatSpeed(item.speed)}</TableCell>
                            <TableCell>
                              <Chip
                                size="sm"
                                color={item.status === 'success' ? 'success' : 'danger'}
                                variant="flat"
                              >
                                {item.status === 'success' ? '成功' : item.status}
                              </Chip>
                            </TableCell>
                            <TableCell>{item.landing_ip || '-'}</TableCell>
                            <TableCell>{formatTime(item.tested_at)}</TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                )}
              </div>
            )}
          </ModalBody>
          <ModalFooter>
            <Button variant="flat" onPress={onTaskClose}>
              关闭
            </Button>
            {selectedTask && runningTasks[selectedTask.ID] && (
              <Button
                color="danger"
                startContent={<Pause className="w-4 h-4" />}
                onPress={() => handleCancelTask(selectedTask.ID)}
              >
                取消任务
              </Button>
            )}
          </ModalFooter>
        </ModalContent>
      </Modal>
    </div>
  );
}

// 策略卡片组件
interface ProfileCardProps {
  profile: SpeedTestProfile;
  onEdit: () => void;
  onDelete: () => void;
  onRun: () => void;
  formatTime: (time: string | null) => string;
}

function ProfileCard({ profile, onEdit, onDelete, onRun, formatTime }: ProfileCardProps) {
  const [isExpanded, setIsExpanded] = useState(false);

  return (
    <Card>
      <CardHeader className="flex justify-between items-start">
        <div className="flex items-center gap-3">
          <div
            className={`w-10 h-10 rounded-lg flex items-center justify-center ${
              profile.mode === 'speed' ? 'bg-success-100' : 'bg-primary-100'
            }`}
          >
            {profile.mode === 'speed' ? (
              <Zap className="w-5 h-5 text-success" />
            ) : (
              <Timer className="w-5 h-5 text-primary" />
            )}
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h3 className="font-semibold">{profile.name}</h3>
              {profile.is_default && (
                <Chip size="sm" variant="flat" color="secondary">
                  默认
                </Chip>
              )}
            </div>
            <div className="flex items-center gap-2 mt-1">
              <Chip size="sm" variant="flat" color={profile.mode === 'speed' ? 'success' : 'primary'}>
                {profile.mode === 'speed' ? '延迟+速度' : '仅延迟'}
              </Chip>
              {profile.auto_test && profile.enabled && (
                <Chip size="sm" variant="flat" color="warning">
                  <Clock className="w-3 h-3 mr-1" />
                  定时启用
                </Chip>
              )}
            </div>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Tooltip content="立即执行">
            <Button
              isIconOnly
              size="sm"
              color="success"
              variant="flat"
              onPress={onRun}
            >
              <Play className="w-4 h-4" />
            </Button>
          </Tooltip>
          <Tooltip content="编辑">
            <Button isIconOnly size="sm" variant="flat" onPress={onEdit}>
              <Pencil className="w-4 h-4" />
            </Button>
          </Tooltip>
          {!profile.is_default && (
            <Tooltip content="删除">
              <Button
                isIconOnly
                size="sm"
                color="danger"
                variant="flat"
                onPress={onDelete}
              >
                <Trash2 className="w-4 h-4" />
              </Button>
            </Tooltip>
          )}
          <Button
            isIconOnly
            size="sm"
            variant="light"
            onPress={() => setIsExpanded(!isExpanded)}
          >
            {isExpanded ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
          </Button>
        </div>
      </CardHeader>

      {isExpanded && (
        <CardBody className="pt-0">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
            <div>
              <p className="text-gray-500">超时时间</p>
              <p className="font-medium">{profile.timeout}s</p>
            </div>
            <div>
              <p className="text-gray-500">延迟并发</p>
              <p className="font-medium">{profile.latency_concurrency || '智能'}</p>
            </div>
            <div>
              <p className="text-gray-500">速度并发</p>
              <p className="font-medium">{profile.speed_concurrency || '智能'}</p>
            </div>
            <div>
              <p className="text-gray-500">握手时间</p>
              <p className="font-medium">{profile.include_handshake ? '包含' : '排除'}</p>
            </div>
            <div>
              <p className="text-gray-500">上次执行</p>
              <p className="font-medium">{formatTime(profile.last_run_at)}</p>
            </div>
            {profile.auto_test && profile.enabled && (
              <div>
                <p className="text-gray-500">下次执行</p>
                <p className="font-medium">{formatTime(profile.next_run_at)}</p>
              </div>
            )}
            <div>
              <p className="text-gray-500">落地检测</p>
              <p className="font-medium">{profile.detect_country ? '启用' : '禁用'}</p>
            </div>
            <div>
              <p className="text-gray-500">速度记录</p>
              <p className="font-medium">
                {profile.speed_record_mode === 'peak' ? '峰值' : '平均'}
              </p>
            </div>
          </div>
        </CardBody>
      )}
    </Card>
  );
}
