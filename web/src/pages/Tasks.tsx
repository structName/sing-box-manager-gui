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
  Switch,
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
  Tag,
  Link,
  Settings,
  Trash2,
  Calendar,
  Timer,
} from 'lucide-react';
import { useTaskStore } from '../store/taskStore';
import { useSchedulerStore } from '../store/schedulerStore';
import { toast } from '../components/Toast';

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
  event: '事件触发',
};

// 任务类型映射
const typeConfig: Record<string, { label: string; icon: React.ReactNode; color: 'primary' | 'secondary' | 'success' | 'warning' }> = {
  speed_test: { label: '节点测速', icon: <Zap className="w-3 h-3" />, color: 'primary' },
  sub_update: { label: '订阅更新', icon: <RefreshCw className="w-3 h-3" />, color: 'secondary' },
  tag_rule: { label: '标签规则', icon: <Tag className="w-3 h-3" />, color: 'success' },
  chain_check: { label: '链路检测', icon: <Link className="w-3 h-3" />, color: 'warning' },
  config_apply: { label: '配置应用', icon: <Settings className="w-3 h-3" />, color: 'primary' },
  config_watch: { label: '配置监控', icon: <Settings className="w-3 h-3" />, color: 'secondary' },
};

export default function Tasks() {
  const { tasks, stats, loading, fetchTasks, fetchStats, cancelTask: cancelTaskAction, cleanupTasks, subscribeSSE, unsubscribeSSE } = useTaskStore();
  const { status: schedulerStatus, entries, loading: schedulerLoading, fetchStatus, fetchEntries, enableEntry, disableEntry, triggerEntry, pause, resume } = useSchedulerStore();
  const [mainTab, setMainTab] = useState('tasks');
  const [activeTab, setActiveTab] = useState('all');
  const [statusFilter, setStatusFilter] = useState<string>('all');

  // 加载任务列表
  const loadTasks = useCallback(async () => {
    await Promise.all([fetchTasks({ limit: 100 }), fetchStats()]);
  }, [fetchTasks, fetchStats]);

  // 加载调度数据
  const loadScheduler = useCallback(async () => {
    await Promise.all([fetchStatus(), fetchEntries()]);
  }, [fetchStatus, fetchEntries]);

  // 首次加载
  useEffect(() => {
    loadTasks();
    loadScheduler();
    subscribeSSE();
    return () => unsubscribeSSE();
  }, [loadTasks, loadScheduler, subscribeSSE, unsubscribeSSE]);

  // 取消任务
  const handleCancel = async (taskId: string) => {
    try {
      await cancelTaskAction(taskId);
      toast.success('任务已取消');
    } catch (error: any) {
      toast.error(error.message || '取消任务失败');
    }
  };

  // 清理历史任务
  const handleCleanup = async () => {
    try {
      await cleanupTasks(7);
      toast.success('历史任务已清理');
    } catch (error: any) {
      toast.error(error.message || '清理失败');
    }
  };

  // 切换调度条目状态
  const handleToggleEntry = async (key: string, enabled: boolean) => {
    try {
      if (enabled) {
        await disableEntry(key);
        toast.success('调度已禁用');
      } else {
        await enableEntry(key);
        toast.success('调度已启用');
      }
    } catch (error: any) {
      toast.error(error.message || '操作失败');
    }
  };

  // 立即触发调度
  const handleTrigger = async (key: string) => {
    try {
      await triggerEntry(key);
      toast.success('已触发执行');
      loadTasks();
    } catch (error: any) {
      toast.error(error.message || '触发失败');
    }
  };

  // 暂停/恢复调度器
  const handleToggleScheduler = async () => {
    try {
      if (schedulerStatus?.running) {
        await pause();
        toast.success('调度器已暂停');
      } else {
        await resume();
        toast.success('调度器已恢复');
      }
    } catch (error: any) {
      toast.error(error.message || '操作失败');
    }
  };

  // 过滤任务
  const filteredTasks = tasks.filter((task) => {
    // Tab 过滤
    const tabMatch = activeTab === 'all' ||
      (activeTab === 'running' && (task.status === 'running' || task.status === 'pending')) ||
      (activeTab === 'completed' && task.status === 'completed') ||
      (activeTab === 'failed' && (task.status === 'error' || task.status === 'cancelled'));

    // 状态过滤
    const statusMatch = statusFilter === 'all' || task.status === statusFilter;

    return tabMatch && statusMatch;
  });

  // 格式化时间
  const formatTime = (timeStr?: string | null) => {
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
  const formatDuration = (startedAt?: string, completedAt?: string) => {
    if (!startedAt) return '-';
    const start = new Date(startedAt).getTime();
    const end = completedAt ? new Date(completedAt).getTime() : Date.now();
    const duration = Math.floor((end - start) / 1000);

    if (duration < 60) return `${duration}秒`;
    if (duration < 3600) return `${Math.floor(duration / 60)}分${duration % 60}秒`;
    return `${Math.floor(duration / 3600)}时${Math.floor((duration % 3600) / 60)}分`;
  };

  // 格式化相对时间
  const formatRelativeTime = (timeStr?: string | null) => {
    if (!timeStr) return '-';
    const date = new Date(timeStr);
    const now = new Date();
    const diff = date.getTime() - now.getTime();

    if (diff < 0) {
      const absDiff = Math.abs(diff);
      if (absDiff < 60000) return '刚刚';
      if (absDiff < 3600000) return `${Math.floor(absDiff / 60000)}分钟前`;
      if (absDiff < 86400000) return `${Math.floor(absDiff / 3600000)}小时前`;
      return formatTime(timeStr);
    } else {
      if (diff < 60000) return '即将执行';
      if (diff < 3600000) return `${Math.floor(diff / 60000)}分钟后`;
      if (diff < 86400000) return `${Math.floor(diff / 3600000)}小时后`;
      return formatTime(timeStr);
    }
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
          onPress={() => mainTab === 'tasks' ? loadTasks() : loadScheduler()}
          isLoading={loading || schedulerLoading}
        >
          <RefreshCw className="w-4 h-4" />
        </Button>
      </div>

      {/* 主 Tab 切换 */}
      <Tabs
        selectedKey={mainTab}
        onSelectionChange={(key) => setMainTab(key as string)}
        variant="underlined"
        classNames={{
          tabList: "gap-6",
        }}
      >
        <Tab
          key="tasks"
          title={
            <div className="flex items-center gap-2">
              <Clock className="w-4 h-4" />
              <span>任务历史</span>
              {(stats?.running || 0) > 0 && (
                <Chip size="sm" color="primary" variant="flat">{stats?.running}</Chip>
              )}
            </div>
          }
        />
        <Tab
          key="scheduler"
          title={
            <div className="flex items-center gap-2">
              <Calendar className="w-4 h-4" />
              <span>定时调度</span>
              {schedulerStatus && (
                <Chip size="sm" color={schedulerStatus.running ? 'success' : 'default'} variant="flat">
                  {schedulerStatus.enabled}
                </Chip>
              )}
            </div>
          }
        />
      </Tabs>

      {mainTab === 'tasks' ? (
        <>
          {/* 统计卡片 */}
          <div className="grid grid-cols-4 gap-4">
            <Card className="bg-primary-50 dark:bg-primary-900/20">
              <CardBody className="flex flex-row items-center gap-3 py-3">
                <div className="p-2 bg-primary-100 dark:bg-primary-800 rounded-lg">
                  <Play className="w-5 h-5 text-primary" />
                </div>
                <div>
                  <p className="text-2xl font-bold text-primary">{stats?.running || 0}</p>
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
                  <p className="text-2xl font-bold">{stats?.pending || 0}</p>
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
                  <p className="text-2xl font-bold text-success">{stats?.completed || 0}</p>
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
                  <p className="text-2xl font-bold text-danger">{stats?.failed || 0}</p>
                  <p className="text-xs text-gray-500">失败</p>
                </div>
              </CardBody>
            </Card>
          </div>

          {/* 任务列表 */}
          <Card>
            <CardBody>
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
                    <SelectItem key="error">失败</SelectItem>
                  </Select>
                  <Tooltip content="清理7天前的历史任务">
                    <Button
                      size="sm"
                      variant="flat"
                      color="danger"
                      startContent={<Trash2 className="w-3 h-3" />}
                      onPress={handleCleanup}
                    >
                      清理
                    </Button>
                  </Tooltip>
                </div>
              </div>

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
                <Table aria-label="任务列表" removeWrapper selectionMode="none">
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
                      const taskType = typeConfig[task.type] || { label: task.type, icon: <Settings className="w-3 h-3" />, color: 'primary' as const };
                      const isRunning = task.status === 'running';
                      const progress = task.total > 0 ? Math.round((task.progress / task.total) * 100) : 0;

                      return (
                        <TableRow key={task.id}>
                          <TableCell>
                            <div>
                              <p className="font-medium">{task.name}</p>
                              {task.current_item && isRunning && (
                                <p className="text-xs text-gray-400 truncate max-w-[200px]">
                                  正在处理: {task.current_item}
                                </p>
                              )}
                              {task.message && task.status === 'error' && (
                                <Tooltip content={task.message}>
                                  <p className="text-xs text-danger truncate max-w-[200px]">
                                    {task.message}
                                  </p>
                                </Tooltip>
                              )}
                            </div>
                          </TableCell>
                          <TableCell>
                            <Chip
                              size="sm"
                              variant="flat"
                              color={taskType.color}
                              startContent={taskType.icon}
                            >
                              {taskType.label}
                            </Chip>
                          </TableCell>
                          <TableCell>
                            <Chip size="sm" variant="flat">
                              {triggerLabels[task.trigger] || task.trigger}
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
                              {isRunning && task.total > 0 ? (
                                <div className="space-y-1">
                                  <Progress
                                    size="sm"
                                    value={progress}
                                    color="primary"
                                    className="max-w-md"
                                  />
                                  <p className="text-xs text-gray-500">
                                    {task.progress}/{task.total}
                                  </p>
                                </div>
                              ) : task.total > 0 ? (
                                <span className="text-sm">
                                  {task.progress}/{task.total}
                                </span>
                              ) : (
                                <span className="text-sm text-gray-400">-</span>
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
                              {formatDuration(task.started_at, task.completed_at)}
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
        </>
      ) : (
        <>
          {/* 调度器状态卡片 */}
          <Card>
            <CardBody>
              <div className="flex justify-between items-center">
                <div className="flex items-center gap-4">
                  <div className={`p-3 rounded-lg ${schedulerStatus?.running ? 'bg-success-100 dark:bg-success-900/30' : 'bg-default-100 dark:bg-default-800'}`}>
                    <Timer className={`w-6 h-6 ${schedulerStatus?.running ? 'text-success' : 'text-default-500'}`} />
                  </div>
                  <div>
                    <h3 className="text-lg font-semibold">统一调度器</h3>
                    <p className="text-sm text-gray-500">
                      {schedulerStatus?.running ? '运行中' : '已暂停'} · {schedulerStatus?.enabled || 0} 个启用 / {schedulerStatus?.entry_count || 0} 个总计
                    </p>
                  </div>
                </div>
                <Button
                  color={schedulerStatus?.running ? 'warning' : 'success'}
                  variant="flat"
                  startContent={schedulerStatus?.running ? <Pause className="w-4 h-4" /> : <Play className="w-4 h-4" />}
                  onPress={handleToggleScheduler}
                >
                  {schedulerStatus?.running ? '暂停调度器' : '启动调度器'}
                </Button>
              </div>
            </CardBody>
          </Card>

          {/* 调度条目列表 */}
          <Card>
            <CardBody>
              <h3 className="text-lg font-semibold mb-4">调度条目</h3>
              {schedulerLoading && entries.length === 0 ? (
                <div className="flex justify-center py-12">
                  <Spinner size="lg" />
                </div>
              ) : entries.length === 0 ? (
                <div className="py-12 text-center">
                  <Calendar className="w-12 h-12 mx-auto text-gray-300 mb-4" />
                  <p className="text-gray-500">暂无调度条目</p>
                  <p className="text-xs text-gray-400 mt-2">
                    在订阅或测速策略中启用自动更新后，调度条目会显示在这里
                  </p>
                </div>
              ) : (
                <Table aria-label="调度条目列表" removeWrapper selectionMode="none">
                  <TableHeader>
                    <TableColumn>名称</TableColumn>
                    <TableColumn>类型</TableColumn>
                    <TableColumn>Cron 表达式</TableColumn>
                    <TableColumn>下次执行</TableColumn>
                    <TableColumn>上次执行</TableColumn>
                    <TableColumn>状态</TableColumn>
                    <TableColumn>操作</TableColumn>
                  </TableHeader>
                  <TableBody>
                    {entries.map((entry) => {
                      const entryType = typeConfig[entry.type] || { label: entry.type, icon: <Settings className="w-3 h-3" />, color: 'primary' as const };

                      return (
                        <TableRow key={entry.key}>
                          <TableCell>
                            <p className="font-medium">{entry.name}</p>
                            <p className="text-xs text-gray-400">{entry.key}</p>
                          </TableCell>
                          <TableCell>
                            <Chip
                              size="sm"
                              variant="flat"
                              color={entryType.color}
                              startContent={entryType.icon}
                            >
                              {entryType.label}
                            </Chip>
                          </TableCell>
                          <TableCell>
                            <code className="text-xs bg-default-100 dark:bg-default-800 px-2 py-1 rounded">
                              {entry.cron_expr}
                            </code>
                          </TableCell>
                          <TableCell>
                            <Tooltip content={formatTime(entry.next_run)}>
                              <span className="text-sm">
                                {entry.enabled ? formatRelativeTime(entry.next_run) : '-'}
                              </span>
                            </Tooltip>
                          </TableCell>
                          <TableCell>
                            <span className="text-sm text-gray-500">
                              {formatRelativeTime(entry.last_run)}
                            </span>
                          </TableCell>
                          <TableCell>
                            <div onClick={(e) => e.stopPropagation()}>
                              <Switch
                                size="sm"
                                isSelected={entry.enabled}
                                onValueChange={() => handleToggleEntry(entry.key, entry.enabled)}
                              />
                            </div>
                          </TableCell>
                          <TableCell>
                            <div onClick={(e) => e.stopPropagation()}>
                              <Tooltip content="立即执行">
                                <Button
                                  isIconOnly
                                  size="sm"
                                  variant="flat"
                                  color="primary"
                                  onPress={() => handleTrigger(entry.key)}
                                  isDisabled={!entry.enabled}
                                >
                                  <Play className="w-4 h-4" />
                                </Button>
                              </Tooltip>
                            </div>
                          </TableCell>
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              )}
            </CardBody>
          </Card>
        </>
      )}
    </div>
  );
}
