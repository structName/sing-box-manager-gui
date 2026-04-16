import { useEffect, useState, useMemo } from 'react';
import { Card, CardBody, CardHeader, Button, Chip, Modal, ModalContent, ModalHeader, ModalBody, ModalFooter, Tooltip, Progress } from '@nextui-org/react';
import { Play, Square, RefreshCw, Cpu, HardDrive, Wifi, Info, Activity, Network, Link2, Sparkles, TrendingUp, Zap, Timer } from 'lucide-react';
import { useStore } from '../store';
import { serviceApi, configApi, inboundPortApi, proxyChainApi, nodeApi } from '../api';
import { toast } from '../components/Toast';

// 节点测速信息类型
interface NodeSpeedInfo {
  delay: number;
  speed: number;
}

// 入站端口类型
interface InboundPort {
  id: string;
  name: string;
  type: string;
  listen: string;
  port: number;
  outbound: string;
  enabled: boolean;
  auth?: { username: string; password: string };
}

// 代理链路类型
interface ProxyChain {
  id: string;
  name: string;
  enabled: boolean;
  nodes: string[];
}

// 问候语计算
const getGreeting = () => {
  const hour = new Date().getHours();
  const greetings = [
    { range: [5, 9], text: '早上好', emoji: '🌅', subText: '新的一天开始了' },
    { range: [9, 12], text: '上午好', emoji: '☀️', subText: '充满活力的上午' },
    { range: [12, 14], text: '中午好', emoji: '🌤️', subText: '记得休息一下' },
    { range: [14, 18], text: '下午好', emoji: '🌇', subText: '继续加油' },
    { range: [18, 23], text: '晚上好', emoji: '🌙', subText: '辛苦了一天' },
  ];

  const greeting = greetings.find(g => hour >= g.range[0] && hour < g.range[1]);
  return greeting || { text: '夜深了', emoji: '✨', subText: '注意休息' };
};

// 统计卡片配置类型
interface StatCardConfig {
  title: string;
  value: string | number;
  subValue?: string;
  icon: React.ElementType;
  gradient: string;
  iconBg: string;
  iconColor: string;
}

// 高级统计卡片组件
function PremiumStatCard({ config, index }: { config: StatCardConfig; index: number }) {
  const Icon = config.icon;

  return (
    <Card
      className={`relative overflow-hidden border-none transition-all duration-300 hover:-translate-y-1 hover:shadow-xl group`}
      style={{
        background: config.gradient,
        animationDelay: `${index * 0.1}s`
      }}
    >
      {/* 顶部彩色边框 */}
      <div className="absolute top-0 left-0 right-0 h-1 bg-gradient-to-r from-white/30 to-white/10" />

      {/* 背景装饰 */}
      <div className="absolute -top-8 -right-8 w-24 h-24 rounded-full bg-white/10 blur-xl" />
      <div className="absolute -bottom-4 -left-4 w-16 h-16 rounded-full bg-white/5 blur-lg" />

      <CardBody className="relative z-10 p-5">
        <div className="flex items-start justify-between">
          <div className="flex-1 min-w-0">
            {/* 标题 */}
            <div className="flex items-center gap-2 mb-2">
              <div className="w-1.5 h-1.5 rounded-full bg-white/60 animate-pulse" />
              <p className="text-xs font-medium text-white/70 uppercase tracking-wider">
                {config.title}
              </p>
            </div>

            {/* 数值 */}
            <p className="text-3xl font-bold text-white mb-1 transition-transform duration-300 group-hover:scale-105">
              {config.value}
            </p>

            {/* 副标题/趋势 */}
            <div className="flex items-center gap-1">
              {config.subValue ? (
                <span className="text-xs text-white/60">{config.subValue}</span>
              ) : (
                <>
                  <TrendingUp className="w-3 h-3 text-green-300" />
                  <span className="text-xs text-green-300 font-medium">运行中</span>
                </>
              )}
            </div>
          </div>

          {/* 图标 */}
          <div
            className={`w-14 h-14 rounded-xl flex items-center justify-center transition-transform duration-300 group-hover:rotate-6 group-hover:scale-110 ${config.iconBg}`}
          >
            <Icon className={`w-7 h-7 ${config.iconColor}`} />
          </div>
        </div>

        {/* 底部进度条装饰 */}
        <div className="mt-4">
          <Progress
            size="sm"
            value={100}
            classNames={{
              base: "h-1",
              track: "bg-white/10",
              indicator: "bg-white/40"
            }}
          />
        </div>
      </CardBody>
    </Card>
  );
}

// 欢迎横幅组件
function WelcomeBanner({ greeting }: { greeting: ReturnType<typeof getGreeting> }) {
  return (
    <Card className="relative overflow-hidden border-none bg-gradient-to-br from-indigo-500 via-purple-500 to-pink-500">
      {/* 背景装饰 */}
      <div className="absolute inset-0 opacity-30">
        <div className="absolute top-1/4 left-1/4 w-32 h-32 rounded-full bg-white/20 blur-3xl" />
        <div className="absolute bottom-1/4 right-1/4 w-40 h-40 rounded-full bg-white/10 blur-3xl" />
      </div>

      {/* 网格装饰 */}
      <div
        className="absolute inset-0 opacity-5"
        style={{
          backgroundImage: 'linear-gradient(to right, white 1px, transparent 1px), linear-gradient(to bottom, white 1px, transparent 1px)',
          backgroundSize: '40px 40px'
        }}
      />

      <CardBody className="relative z-10 py-8 px-6">
        <div className="flex items-center justify-between flex-wrap gap-4">
          <div>
            <div className="flex items-center gap-3 mb-2">
              <h1 className="text-3xl md:text-4xl font-bold text-white">
                {greeting.text}
              </h1>
              <span className="text-3xl md:text-4xl animate-bounce">
                {greeting.emoji}
              </span>
            </div>
            <p className="text-white/80 text-lg">
              欢迎使用 <span className="font-bold text-white">SingBox Manager</span>，{greeting.subText}
            </p>
          </div>

          {/* 装饰图标 */}
          <div className="hidden md:flex items-center justify-center w-20 h-20 rounded-full bg-white/10 backdrop-blur-sm border border-white/20 animate-pulse">
            <Sparkles className="w-10 h-10 text-white" />
          </div>
        </div>
      </CardBody>
    </Card>
  );
}

// 服务状态卡片组件
function ServiceStatusCard({
  serviceStatus,
  onStart,
  onStop,
  onRestart,
  onApplyConfig
}: {
  serviceStatus: any;
  onStart: () => void;
  onStop: () => void;
  onRestart: () => void;
  onApplyConfig: () => void;
}) {
  const isRunning = serviceStatus?.running;

  return (
    <Card className={`relative overflow-hidden border-none transition-all duration-500 ${
      isRunning
        ? 'bg-gradient-to-br from-emerald-500/10 via-green-500/5 to-teal-500/10 dark:from-emerald-900/30 dark:via-green-900/20 dark:to-teal-900/30'
        : 'bg-gradient-to-br from-red-500/10 via-orange-500/5 to-amber-500/10 dark:from-red-900/30 dark:via-orange-900/20 dark:to-amber-900/30'
    }`}>
      {/* 状态指示条 */}
      <div className={`absolute top-0 left-0 right-0 h-1 ${
        isRunning ? 'bg-gradient-to-r from-emerald-400 to-teal-400' : 'bg-gradient-to-r from-red-400 to-orange-400'
      }`} />

      {/* 背景光晕 */}
      <div className={`absolute -top-20 -right-20 w-40 h-40 rounded-full blur-3xl ${
        isRunning ? 'bg-emerald-500/20' : 'bg-red-500/20'
      }`} />

      <CardHeader className="relative z-10 flex justify-between items-center pb-2">
        <div className="flex items-center gap-3">
          <div className={`w-10 h-10 rounded-xl flex items-center justify-center ${
            isRunning ? 'bg-emerald-500/20' : 'bg-red-500/20'
          }`}>
            <Activity className={`w-5 h-5 ${isRunning ? 'text-emerald-500' : 'text-red-500'}`} />
          </div>
          <div>
            <h2 className="text-lg font-semibold text-gray-800 dark:text-white">sing-box 服务</h2>
            <div className="flex items-center gap-2 mt-0.5">
              <span className={`w-2 h-2 rounded-full ${isRunning ? 'bg-emerald-500 animate-pulse' : 'bg-red-500'}`} />
              <span className={`text-sm font-medium ${isRunning ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'}`}>
                {isRunning ? '运行中' : '已停止'}
              </span>
            </div>
          </div>
        </div>
        <div className="flex gap-2">
          {isRunning ? (
            <>
              <Button
                size="sm"
                color="danger"
                variant="flat"
                startContent={<Square className="w-4 h-4" />}
                onPress={onStop}
              >
                停止
              </Button>
              <Button
                size="sm"
                color="primary"
                variant="flat"
                startContent={<RefreshCw className="w-4 h-4" />}
                onPress={onRestart}
              >
                重启
              </Button>
            </>
          ) : (
            <Button
              size="sm"
              color="success"
              startContent={<Play className="w-4 h-4" />}
              onPress={onStart}
            >
              启动
            </Button>
          )}
          <Button
            size="sm"
            color="primary"
            onPress={onApplyConfig}
          >
            应用配置
          </Button>
        </div>
      </CardHeader>
      <CardBody className="relative z-10 pt-2">
        <div className="grid grid-cols-3 gap-4">
          <div className="p-3 rounded-xl bg-white/50 dark:bg-white/5 backdrop-blur-sm">
            <p className="text-xs text-gray-500 dark:text-gray-400 mb-1">版本</p>
            <div className="flex items-center gap-1">
              <p className="font-semibold text-gray-800 dark:text-white">
                {serviceStatus?.version?.match(/version\s+(\S+)/)?.[1] || serviceStatus?.version || '-'}
              </p>
              {serviceStatus?.version && (
                <Tooltip
                  content={
                    <div className="max-w-md whitespace-pre-wrap text-xs p-1 break-all">
                      {serviceStatus.version}
                    </div>
                  }
                  placement="bottom"
                >
                  <Info className="w-3.5 h-3.5 text-gray-400 cursor-help" />
                </Tooltip>
              )}
            </div>
          </div>
          <div className="p-3 rounded-xl bg-white/50 dark:bg-white/5 backdrop-blur-sm">
            <p className="text-xs text-gray-500 dark:text-gray-400 mb-1">进程 ID</p>
            <p className="font-semibold text-gray-800 dark:text-white">{serviceStatus?.pid || '-'}</p>
          </div>
          <div className="p-3 rounded-xl bg-white/50 dark:bg-white/5 backdrop-blur-sm">
            <p className="text-xs text-gray-500 dark:text-gray-400 mb-1">状态</p>
            <p className={`font-semibold ${isRunning ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-400'}`}>
              {isRunning ? '正常运行' : '未运行'}
            </p>
          </div>
        </div>
      </CardBody>
    </Card>
  );
}

// 概览列表卡片组件
function OverviewCard({
  title,
  icon: Icon,
  iconColor,
  items,
  emptyText,
  renderItem
}: {
  title: string;
  icon: React.ElementType;
  iconColor: string;
  items: any[];
  emptyText: string;
  renderItem: (item: any) => React.ReactNode;
}) {
  return (
    <Card className="border-none bg-white/70 dark:bg-gray-800/50 backdrop-blur-sm">
      <CardHeader className="pb-2">
        <div className="flex items-center gap-2">
          <div className={`w-8 h-8 rounded-lg flex items-center justify-center ${iconColor}`}>
            <Icon className="w-4 h-4 text-white" />
          </div>
          <h2 className="text-lg font-semibold text-gray-800 dark:text-white">{title}</h2>
          <Chip size="sm" variant="flat" className="ml-auto">
            {items.length}
          </Chip>
        </div>
      </CardHeader>
      <CardBody className="pt-2">
        {items.length === 0 ? (
          <p className="text-gray-500 text-center py-6">{emptyText}</p>
        ) : (
          <div className="space-y-2">
            {items.map(renderItem)}
          </div>
        )}
      </CardBody>
    </Card>
  );
}

export default function Dashboard() {
  const { serviceStatus, subscriptions, systemInfo, fetchServiceStatus, fetchSubscriptions, fetchSystemInfo } = useStore();

  // 入站端口和链路数据
  const [inboundPorts, setInboundPorts] = useState<InboundPort[]>([]);
  const [proxyChains, setProxyChains] = useState<ProxyChain[]>([]);
  // 测速数据
  const [speedInfos, setSpeedInfos] = useState<Record<string, NodeSpeedInfo>>({});

  // 错误模态框状态
  const [errorModal, setErrorModal] = useState<{
    isOpen: boolean;
    title: string;
    message: string;
  }>({
    isOpen: false,
    title: '',
    message: ''
  });

  const greeting = useMemo(() => getGreeting(), []);

  // 显示错误的辅助函数
  const showError = (title: string, error: any) => {
    const message = error.response?.data?.error || error.message || '操作失败';
    setErrorModal({
      isOpen: true,
      title,
      message
    });
  };

  useEffect(() => {
    fetchServiceStatus();
    fetchSubscriptions();
    fetchSystemInfo();
    fetchInboundPorts();
    fetchProxyChains();
    fetchSpeedInfos();

    // 每 5 秒刷新状态和系统信息
    const interval = setInterval(() => {
      fetchServiceStatus();
      fetchSystemInfo();
    }, 5000);
    return () => clearInterval(interval);
  }, []);

  const fetchInboundPorts = async () => {
    try {
      const res = await inboundPortApi.getAll();
      setInboundPorts(res.data.data || []);
    } catch (error) {
      console.error('获取入站端口失败:', error);
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

  const fetchSpeedInfos = async () => {
    try {
      const res = await nodeApi.getDelays();
      setSpeedInfos(res.data.data || {});
    } catch (error) {
      console.error('获取测速信息失败:', error);
    }
  };

  const handleStart = async () => {
    try {
      await serviceApi.start();
      await fetchServiceStatus();
      toast.success('服务已启动');
    } catch (error) {
      showError('启动失败', error);
    }
  };

  const handleStop = async () => {
    try {
      await serviceApi.stop();
      await fetchServiceStatus();
      toast.success('服务已停止');
    } catch (error) {
      showError('停止失败', error);
    }
  };

  const handleRestart = async () => {
    try {
      await serviceApi.restart();
      await fetchServiceStatus();
      toast.success('服务已重启');
    } catch (error) {
      showError('重启失败', error);
    }
  };

  const handleApplyConfig = async () => {
    try {
      await configApi.apply();
      await fetchServiceStatus();
      toast.success('配置已应用');
    } catch (error) {
      showError('应用配置失败', error);
    }
  };

  const totalNodes = subscriptions.reduce((sum, sub) => sum + sub.node_count, 0);
  const enabledSubs = subscriptions.filter(sub => sub.enabled).length;
  const enabledPorts = inboundPorts.filter(p => p.enabled).length;
  const enabledChains = proxyChains.filter(c => c.enabled).length;

  // 计算测速统计
  const speedStats = useMemo(() => {
    const entries = Object.entries(speedInfos);
    const testedNodes = entries.filter(([, info]) => info.delay > 0);
    const speedTestedNodes = entries.filter(([, info]) => info.speed > 0);

    let lowestDelay = { tag: '-', delay: 0 };
    let fastestSpeed = { tag: '-', speed: 0 };

    testedNodes.forEach(([tag, info]) => {
      if (lowestDelay.delay === 0 || info.delay < lowestDelay.delay) {
        lowestDelay = { tag, delay: info.delay };
      }
    });

    speedTestedNodes.forEach(([tag, info]) => {
      if (info.speed > fastestSpeed.speed) {
        fastestSpeed = { tag, speed: info.speed };
      }
    });

    return {
      testedCount: testedNodes.length,
      totalCount: entries.length,
      lowestDelay,
      fastestSpeed
    };
  }, [speedInfos]);

  // 统计卡片配置
  const statsConfig: StatCardConfig[] = [
    {
      title: '订阅数量',
      value: `${enabledSubs} / ${subscriptions.length}`,
      subValue: '已启用 / 总数',
      icon: Wifi,
      gradient: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
      iconBg: 'bg-white/20',
      iconColor: 'text-white'
    },
    {
      title: '代理链路',
      value: `${enabledChains} / ${proxyChains.length}`,
      subValue: '已启用 / 总数',
      icon: Link2,
      gradient: 'linear-gradient(135deg, #f093fb 0%, #f5576c 100%)',
      iconBg: 'bg-white/20',
      iconColor: 'text-white'
    },
    {
      title: '入站端口',
      value: `${enabledPorts} / ${inboundPorts.length}`,
      subValue: '已启用 / 总数',
      icon: Network,
      gradient: 'linear-gradient(135deg, #4facfe 0%, #00f2fe 100%)',
      iconBg: 'bg-white/20',
      iconColor: 'text-white'
    },
    {
      title: '节点总数',
      value: totalNodes,
      subValue: `已测速 ${speedStats.testedCount} 个`,
      icon: HardDrive,
      gradient: 'linear-gradient(135deg, #43e97b 0%, #38f9d7 100%)',
      iconBg: 'bg-white/20',
      iconColor: 'text-white'
    },
    {
      title: '最低延迟',
      value: speedStats.lowestDelay.delay > 0 ? `${speedStats.lowestDelay.delay}ms` : '-',
      subValue: speedStats.lowestDelay.tag !== '-' ? speedStats.lowestDelay.tag.slice(0, 20) : '暂无数据',
      icon: Timer,
      gradient: 'linear-gradient(135deg, #11998e 0%, #38ef7d 100%)',
      iconBg: 'bg-white/20',
      iconColor: 'text-white'
    },
    {
      title: '最快速度',
      value: speedStats.fastestSpeed.speed > 0 ? `${speedStats.fastestSpeed.speed.toFixed(1)}MB/s` : '-',
      subValue: speedStats.fastestSpeed.tag !== '-' ? speedStats.fastestSpeed.tag.slice(0, 20) : '暂无数据',
      icon: Zap,
      gradient: 'linear-gradient(135deg, #fc4a1a 0%, #f7b733 100%)',
      iconBg: 'bg-white/20',
      iconColor: 'text-white'
    },
    {
      title: 'SBM 资源',
      value: systemInfo?.sbm ? `${systemInfo.sbm.cpu_percent.toFixed(1)}%` : '-',
      subValue: systemInfo?.sbm ? `内存 ${systemInfo.sbm.memory_mb.toFixed(1)}MB` : 'CPU 使用率',
      icon: Cpu,
      gradient: 'linear-gradient(135deg, #fa709a 0%, #fee140 100%)',
      iconBg: 'bg-white/20',
      iconColor: 'text-white'
    },
    {
      title: 'sing-box 资源',
      value: serviceStatus?.running && systemInfo?.singbox ? `${systemInfo.singbox.cpu_percent.toFixed(1)}%` : '-',
      subValue: serviceStatus?.running && systemInfo?.singbox ? `内存 ${systemInfo.singbox.memory_mb.toFixed(1)}MB` : '未运行',
      icon: Activity,
      gradient: 'linear-gradient(135deg, #a8edea 0%, #fed6e3 100%)',
      iconBg: 'bg-white/20',
      iconColor: 'text-gray-700'
    }
  ];

  return (
    <div className="space-y-6">
      {/* 欢迎横幅 */}
      <WelcomeBanner greeting={greeting} />

      {/* 服务状态卡片 */}
      <ServiceStatusCard
        serviceStatus={serviceStatus}
        onStart={handleStart}
        onStop={handleStop}
        onRestart={handleRestart}
        onApplyConfig={handleApplyConfig}
      />

      {/* 统计卡片网格 */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {statsConfig.map((config, index) => (
          <PremiumStatCard key={config.title} config={config} index={index} />
        ))}
      </div>

      {/* 概览卡片网格 */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        {/* 订阅概览 */}
        <OverviewCard
          title="订阅概览"
          icon={Wifi}
          iconColor="bg-gradient-to-br from-indigo-500 to-purple-500"
          items={subscriptions}
          emptyText="暂无订阅，请前往节点页面添加"
          renderItem={(sub) => (
            <div
              key={sub.id}
              className="flex items-center justify-between p-3 bg-gray-50/80 dark:bg-gray-700/50 rounded-xl transition-all hover:bg-gray-100 dark:hover:bg-gray-700"
            >
              <div className="flex items-center gap-2 min-w-0">
                <Chip
                  size="sm"
                  color={sub.enabled ? 'success' : 'default'}
                  variant="dot"
                  className="shrink-0"
                >
                  <span className="truncate max-w-[100px]">{sub.name}</span>
                </Chip>
                <span className="text-xs text-gray-500 shrink-0">
                  {sub.node_count} 节点
                </span>
              </div>
            </div>
          )}
        />

        {/* 代理链路概览 */}
        <OverviewCard
          title="代理链路"
          icon={Link2}
          iconColor="bg-gradient-to-br from-pink-500 to-rose-500"
          items={proxyChains}
          emptyText="暂无代理链路，请前往链路页面添加"
          renderItem={(chain) => (
            <div
              key={chain.id}
              className="flex items-center justify-between p-3 bg-gray-50/80 dark:bg-gray-700/50 rounded-xl transition-all hover:bg-gray-100 dark:hover:bg-gray-700"
            >
              <div className="flex items-center gap-2">
                <Chip
                  size="sm"
                  color={chain.enabled ? 'success' : 'default'}
                  variant="dot"
                >
                  {chain.name}
                </Chip>
                <span className="text-xs text-gray-500">
                  {chain.nodes?.length || 0} 跳
                </span>
              </div>
            </div>
          )}
        />

        {/* 入站端口概览 */}
        <OverviewCard
          title="入站端口"
          icon={Network}
          iconColor="bg-gradient-to-br from-cyan-500 to-blue-500"
          items={inboundPorts}
          emptyText="暂无入站端口，请前往入站页面添加"
          renderItem={(port) => (
            <div
              key={port.id}
              className="flex items-center justify-between p-3 bg-gray-50/80 dark:bg-gray-700/50 rounded-xl transition-all hover:bg-gray-100 dark:hover:bg-gray-700"
            >
              <div className="flex items-center gap-2 min-w-0">
                <Chip
                  size="sm"
                  color={port.enabled ? 'success' : 'default'}
                  variant="dot"
                  className="shrink-0"
                >
                  <span className="truncate max-w-[80px]">{port.name}</span>
                </Chip>
                <Chip size="sm" variant="flat" className="shrink-0">{port.type}</Chip>
                <span className="text-xs text-gray-500 truncate">
                  :{port.port}
                </span>
              </div>
            </div>
          )}
        />
      </div>

      {/* 错误提示模态框 */}
      <Modal isOpen={errorModal.isOpen} onClose={() => setErrorModal({ ...errorModal, isOpen: false })}>
        <ModalContent>
          <ModalHeader className="text-danger">{errorModal.title}</ModalHeader>
          <ModalBody>
            <p className="whitespace-pre-wrap text-sm">{errorModal.message}</p>
          </ModalBody>
          <ModalFooter>
            <Button color="primary" onPress={() => setErrorModal({ ...errorModal, isOpen: false })}>
              确定
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </div>
  );
}
