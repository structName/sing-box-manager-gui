import { useEffect, useState, useCallback } from 'react';
import {
  Card,
  CardBody,
  Button,
  Chip,
  Spinner,
  Table,
  TableHeader,
  TableColumn,
  TableBody,
  TableRow,
  TableCell,
  Tabs,
  Tab,
  Progress,
  Tooltip,
  Select,
  SelectItem,
} from '@nextui-org/react';
import {
  Play,
  Pause,
  RefreshCw,
  CheckCircle,
  XCircle,
  Clock,
  AlertCircle,
  ListFilter,
  Zap,
} from 'lucide-react';
import { speedtestApi } from '../api';
import { toast } from '../components/Toast';

// 任务类型定义
interface SpeedTestTask {
  id: string;
  profile_id?: number;
  profile_name: string;
  status: string;
  trigger_type: string;
  total: number;
  completed: number;
  success: number;
  failed: number;
  current_node?: string;
  error?: string;
  started_at?: string;
  finished_at?: string;
}

// 状态映射
const statusConfig: Record<string, { label: string; color: 'default' | 'primary' | 'success' | 'warning' | 'danger'; icon: React.ReactNode }> = {
  pending: { label: '待运行', color: 'default', icon: <Clock className="w-4 h-4" /> },
  running: { label: '运行中', color: 'primary', icon: <Play className="w-4 h-4" /> },
  completed: { label: '已完成', color: 'success', icon: <CheckCircle className="w-4 h-4" /> },
  cancelled: { label: '已取消', color: 'warning', icon: <Pause className="w-4 h-4" /> },
  failed: { label: '失败', color: 'danger', icon: <XCircle className="w-4 h-4" /> },
  error: { label: '错误', color: 'danger', icon: <AlertCircle className="w-4 h-4" /> },
};

// 触发方式映射
const triggerLabels: Record<string, string> = {
  manual: '手动触发',
  scheduled: '定时触发',
  auto: '自动触发',
};

export default function Tasks() {
  const [tasks, setTasks] = useState<SpeedTestTask[]>([]);
  const [runningIds, setRunningIds] = useState<Record<string, boolean>>({});
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState('all');
  const [statusFilter, setStatusFilter] = useState<string>('all');
  const [stats, setStats] = useState({
    running: 0,
    pending: 0,
    completed: 0,
    failed: 0,
  });

  // 加载任务列表
  const loadTasks = useCallback(async () => {
    setLoading(true);
    try {
      const response = await speedtestApi.getTasks(50);
      const data = response.data;
      setTasks(data.tasks || []);
      setRunningIds(data.running || {});

      // 计算统计
      const taskList = data.tasks || [];
      setStats({
        running: taskList.filter((t: SpeedTestTask) => t.status === 'running').length,
        pending: taskList.filter((t: SpeedTestTask) => t.status === 'pending').length,
        completed: taskList.filter((t: SpeedTestTask) => t.status === 'completed').length,
        failed: taskList.filter((t: SpeedTestTask) => t.status === 'failed' || t.status === 'error').length,
      });
    } catch (error: any) {
      toast.error(error.response?.data?.error || '加载任务列表失败');
    } finally {
      setLoading(false);
    }
  }, []);

  // 首次加载
  useEffect(() => {
    loadTasks();
  }, [loadTasks]);

  // 自动刷新运行中任务（使用 ref 避免循环依赖）
  useEffect(() => {
    const interval = setInterval(() => {
      const hasRunning = tasks.some(t => t.status === 'running' || t.status === 'pending');
      if (hasRunning) {
        loadTasks();
      }
    }, 3000);
    return () => clearInterval(interval);
  }, [tasks, loadTasks]);

  // 取消任务
  const handleCancel = async (taskId: string) => {
    try {
      await speedtestApi.cancelTask(taskId);
      toast.success('任务已取消');
      loadTasks();
    } catch (error: any) {
      toast.error(error.response?.data?.error || '取消任务失败');
    }
  };

  // 过滤任务
  const filteredTasks = tasks.filter((task) => {
    if (statusFilter !== 'all' && task.status !== statusFilter) {
      return false;
    }
    return true;
  });

  // 格式化时间
  const formatTime = (timeStr?: string) => {
    if (!timeStr) return '-';
    const date = new Date(timeStr);
    return date.toLocaleString('zh-CN', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  // 计算耗时
  const formatDuration = (startedAt?: string, finishedAt?: string) => {
    if (!startedAt) return '-';
    const start = new Date(startedAt).getTime();
    const end = finishedAt ? new Date(finishedAt).getTime() : Date.now();
    const duration = Math.floor((end - start) / 1000);

    if (duration < 60) return `${duration}秒`;
    if (duration < 3600) return `${Math.floor(duration / 60)}分${duration % 60}秒`;
    return `${Math.floor(duration / 3600)}时${Math.floor((duration % 3600) / 60)}分`;
  };

  return (
    <div className="space-y-6">
      {/* 页面标题和操作 */}
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-gray-800 dark:text-white">任务管理</h1>
          <p className="text-sm text-gray-500 mt-1">
            查看和管理定时订阅更新、节点测速、自动打标等后台任务
          </p>
        </div>
        <Button
          isIconOnly
          variant="flat"
          onPress={loadTasks}
          isLoading={loading}
        >
          <RefreshCw className="w-4 h-4" />
        </Button>
      </div>

      {/* 统计卡片 */}
      <div className="grid grid-cols-4 gap-4">
        <Card className="bg-primary-50 dark:bg-primary-900/20">
          <CardBody className="flex flex-row items-center gap-3 py-3">
            <div className="p-2 bg-primary-100 dark:bg-primary-800 rounded-lg">
              <Play className="w-5 h-5 text-primary" />
            </div>
            <div>
              <p className="text-2xl font-bold text-primary">{stats.running}</p>
              <p className="text-xs text-gray-500">运行中</p>
            </div>
          </CardBody>
        </Card>
        <Card className="bg-default-50 dark:bg-default-900/20">
          <CardBody className="flex flex-row items-center gap-3 py-3">
            <div className="p-2 bg-default-100 dark:bg-default-800 rounded-lg">
              <Clock className="w-5 h-5 text-default-500" />
            </div>
            <div>
              <p className="text-2xl font-bold">{stats.pending}</p>
              <p className="text-xs text-gray-500">待运行</p>
            </div>
          </CardBody>
        </Card>
        <Card className="bg-success-50 dark:bg-success-900/20">
          <CardBody className="flex flex-row items-center gap-3 py-3">
            <div className="p-2 bg-success-100 dark:bg-success-800 rounded-lg">
              <CheckCircle className="w-5 h-5 text-success" />
            </div>
            <div>
              <p className="text-2xl font-bold text-success">{stats.completed}</p>
              <p className="text-xs text-gray-500">已完成</p>
            </div>
          </CardBody>
        </Card>
        <Card className="bg-danger-50 dark:bg-danger-900/20">
          <CardBody className="flex flex-row items-center gap-3 py-3">
            <div className="p-2 bg-danger-100 dark:bg-danger-800 rounded-lg">
              <XCircle className="w-5 h-5 text-danger" />
            </div>
            <div>
              <p className="text-2xl font-bold text-danger">{stats.failed}</p>
              <p className="text-xs text-gray-500">失败/取消</p>
            </div>
          </CardBody>
        </Card>
      </div>

      {/* 过滤和任务列表 */}
      <Card>
        <CardBody>
          {/* 过滤器 */}
          <div className="flex justify-between items-center mb-4">
            <Tabs
              selectedKey={activeTab}
              onSelectionChange={(key) => setActiveTab(key as string)}
              size="sm"
            >
              <Tab key="all" title="全部" />
              <Tab key="running" title="运行中" />
              <Tab key="completed" title="已完成" />
              <Tab key="failed" title="失败" />
            </Tabs>
            <div className="flex items-center gap-2">
              <ListFilter className="w-4 h-4 text-gray-400" />
              <Select
                size="sm"
                className="w-32"
                selectedKeys={[statusFilter]}
                onChange={(e) => setStatusFilter(e.target.value)}
                aria-label="状态筛选"
              >
                <SelectItem key="all">全部状态</SelectItem>
                <SelectItem key="running">运行中</SelectItem>
                <SelectItem key="pending">待运行</SelectItem>
                <SelectItem key="completed">已完成</SelectItem>
                <SelectItem key="cancelled">已取消</SelectItem>
                <SelectItem key="failed">失败</SelectItem>
              </Select>
            </div>
          </div>

          {/* 任务列表 */}
          {loading && tasks.length === 0 ? (
            <div className="flex justify-center py-12">
              <Spinner size="lg" />
            </div>
          ) : filteredTasks.length === 0 ? (
            <div className="py-12 text-center">
              <Clock className="w-12 h-12 mx-auto text-gray-300 mb-4" />
              <p className="text-gray-500">暂无任务记录</p>
              <p className="text-xs text-gray-400 mt-2">
                执行测速或应用标签规则后，任务会显示在这里
              </p>
            </div>
          ) : (
            <Table aria-label="任务列表" removeWrapper>
              <TableHeader>
                <TableColumn>任务名称</TableColumn>
                <TableColumn>类型</TableColumn>
                <TableColumn>触发方式</TableColumn>
                <TableColumn>状态</TableColumn>
                <TableColumn>进度</TableColumn>
                <TableColumn>创建时间</TableColumn>
                <TableColumn>耗时</TableColumn>
                <TableColumn>操作</TableColumn>
              </TableHeader>
              <TableBody>
                {filteredTasks.map((task) => {
                  const status = statusConfig[task.status] || statusConfig.pending;
                  const isRunning = runningIds[task.id] || task.status === 'running';
                  const progress = task.total > 0 ? Math.round((task.completed / task.total) * 100) : 0;

                  return (
                    <TableRow key={task.id}>
                      <TableCell>
                        <div>
                          <p className="font-medium">{task.profile_name || '快速测速'}</p>
                          {task.current_node && isRunning && (
                            <p className="text-xs text-gray-400 truncate max-w-[200px]">
                              正在处理: {task.current_node}
                            </p>
                          )}
                          {task.error && (
                            <Tooltip content={task.error}>
                              <p className="text-xs text-danger truncate max-w-[200px]">
                                {task.error}
                              </p>
                            </Tooltip>
                          )}
                        </div>
                      </TableCell>
                      <TableCell>
                        <Chip
                          size="sm"
                          variant="flat"
                          color="primary"
                          startContent={<Zap className="w-3 h-3" />}
                        >
                          节点测速
                        </Chip>
                      </TableCell>
                      <TableCell>
                        <Chip size="sm" variant="flat">
                          {triggerLabels[task.trigger_type] || task.trigger_type}
                        </Chip>
                      </TableCell>
                      <TableCell>
                        <Chip
                          size="sm"
                          variant="flat"
                          color={status.color}
                          startContent={status.icon}
                        >
                          {status.label}
                        </Chip>
                      </TableCell>
                      <TableCell>
                        <div className="w-32">
                          {isRunning ? (
                            <div className="space-y-1">
                              <Progress
                                size="sm"
                                value={progress}
                                color="primary"
                                className="max-w-md"
                              />
                              <p className="text-xs text-gray-500">
                                {task.completed}/{task.total}
                              </p>
                            </div>
                          ) : (
                            <span className="text-sm">
                              {task.success}/{task.total}
                              {task.failed > 0 && (
                                <span className="text-danger ml-1">
                                  ({task.failed} 失败)
                                </span>
                              )}
                            </span>
                          )}
                        </div>
                      </TableCell>
                      <TableCell>
                        <span className="text-sm text-gray-500">
                          {formatTime(task.started_at)}
                        </span>
                      </TableCell>
                      <TableCell>
                        <span className="text-sm">
                          {formatDuration(task.started_at, task.finished_at)}
                        </span>
                      </TableCell>
                      <TableCell>
                        {isRunning ? (
                          <Button
                            size="sm"
                            color="danger"
                            variant="flat"
                            startContent={<Pause className="w-3 h-3" />}
                            onPress={() => handleCancel(task.id)}
                          >
                            取消
                          </Button>
                        ) : (
                          <Tooltip content="查看详情">
                            <Button
                              isIconOnly
                              size="sm"
                              variant="light"
                              isDisabled
                            >
                              <AlertCircle className="w-4 h-4" />
                            </Button>
                          </Tooltip>
                        )}
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          )}
        </CardBody>
      </Card>
    </div>
  );
}
