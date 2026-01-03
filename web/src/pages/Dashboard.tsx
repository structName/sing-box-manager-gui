import { useEffect, useState } from 'react';
import { Card, CardBody, CardHeader, Button, Chip, Modal, ModalContent, ModalHeader, ModalBody, ModalFooter, Tooltip } from '@nextui-org/react';
import { Play, Square, RefreshCw, Cpu, HardDrive, Wifi, Info, Activity, Network, Link2 } from 'lucide-react';
import { useStore } from '../store';
import { serviceApi, configApi, inboundPortApi, proxyChainApi } from '../api';
import { toast } from '../components/Toast';

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

export default function Dashboard() {
  const { serviceStatus, subscriptions, systemInfo, fetchServiceStatus, fetchSubscriptions, fetchSystemInfo } = useStore();

  // 入站端口和链路数据
  const [inboundPorts, setInboundPorts] = useState<InboundPort[]>([]);
  const [proxyChains, setProxyChains] = useState<ProxyChain[]>([]);

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

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-gray-800 dark:text-white">仪表盘</h1>

      {/* 服务状态卡片 */}
      <Card>
        <CardHeader className="flex justify-between items-center">
          <div className="flex items-center gap-3">
            <h2 className="text-lg font-semibold">sing-box 服务</h2>
            <Chip
              color={serviceStatus?.running ? 'success' : 'danger'}
              variant="flat"
              size="sm"
            >
              {serviceStatus?.running ? '运行中' : '已停止'}
            </Chip>
          </div>
          <div className="flex gap-2">
            {serviceStatus?.running ? (
              <>
                <Button
                  size="sm"
                  color="danger"
                  variant="flat"
                  startContent={<Square className="w-4 h-4" />}
                  onPress={handleStop}
                >
                  停止
                </Button>
                <Button
                  size="sm"
                  color="primary"
                  variant="flat"
                  startContent={<RefreshCw className="w-4 h-4" />}
                  onPress={handleRestart}
                >
                  重启
                </Button>
              </>
            ) : (
              <Button
                size="sm"
                color="success"
                startContent={<Play className="w-4 h-4" />}
                onPress={handleStart}
              >
                启动
              </Button>
            )}
            <Button
              size="sm"
              color="primary"
              onPress={handleApplyConfig}
            >
              应用配置
            </Button>
          </div>
        </CardHeader>
        <CardBody>
          <div className="grid grid-cols-3 gap-4">
            <div>
              <p className="text-sm text-gray-500">版本</p>
              <div className="flex items-center gap-1">
                <p className="font-medium">
                  {serviceStatus?.version?.match(/version\s+([\d.]+)/)?.[1] || serviceStatus?.version || '-'}
                </p>
                {serviceStatus?.version && (
                  <Tooltip
                    content={
                      <div className="max-w-xs whitespace-pre-wrap text-xs p-1">
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
            <div>
              <p className="text-sm text-gray-500">进程 ID</p>
              <p className="font-medium">{serviceStatus?.pid || '-'}</p>
            </div>
            <div>
              <p className="text-sm text-gray-500">状态</p>
              <p className="font-medium">
                {serviceStatus?.running ? '正常运行' : '未运行'}
              </p>
            </div>
          </div>
        </CardBody>
      </Card>

      {/* 统计卡片 - 第一行：订阅、代理链路、入站端口 */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card>
          <CardBody className="flex flex-row items-center gap-4">
            <div className="p-3 bg-blue-100 dark:bg-blue-900 rounded-lg">
              <Wifi className="w-6 h-6 text-blue-600 dark:text-blue-300" />
            </div>
            <div>
              <p className="text-sm text-gray-500">订阅数量</p>
              <p className="text-2xl font-bold">{enabledSubs} / {subscriptions.length}</p>
            </div>
          </CardBody>
        </Card>

        <Card>
          <CardBody className="flex flex-row items-center gap-4">
            <div className="p-3 bg-indigo-100 dark:bg-indigo-900 rounded-lg">
              <Link2 className="w-6 h-6 text-indigo-600 dark:text-indigo-300" />
            </div>
            <div>
              <p className="text-sm text-gray-500">代理链路</p>
              <p className="text-2xl font-bold">{enabledChains} / {proxyChains.length}</p>
            </div>
          </CardBody>
        </Card>

        <Card>
          <CardBody className="flex flex-row items-center gap-4">
            <div className="p-3 bg-cyan-100 dark:bg-cyan-900 rounded-lg">
              <Network className="w-6 h-6 text-cyan-600 dark:text-cyan-300" />
            </div>
            <div>
              <p className="text-sm text-gray-500">入站端口</p>
              <p className="text-2xl font-bold">{enabledPorts} / {inboundPorts.length}</p>
            </div>
          </CardBody>
        </Card>
      </div>

      {/* 统计卡片 - 第二行：节点总数、sbm资源、sing-box资源 */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card>
          <CardBody className="flex flex-row items-center gap-4">
            <div className="p-3 bg-green-100 dark:bg-green-900 rounded-lg">
              <HardDrive className="w-6 h-6 text-green-600 dark:text-green-300" />
            </div>
            <div>
              <p className="text-sm text-gray-500">节点总数</p>
              <p className="text-2xl font-bold">{totalNodes}</p>
            </div>
          </CardBody>
        </Card>

        <Card>
          <CardBody className="flex flex-row items-center gap-4">
            <div className="p-3 bg-purple-100 dark:bg-purple-900 rounded-lg">
              <Cpu className="w-6 h-6 text-purple-600 dark:text-purple-300" />
            </div>
            <div>
              <p className="text-sm text-gray-500">sbm 资源</p>
              <p className="text-lg font-bold">
                {systemInfo?.sbm ? (
                  <>
                    <span className="text-sm font-normal text-gray-500">CPU </span>
                    {systemInfo.sbm.cpu_percent.toFixed(1)}%
                    <span className="text-sm font-normal text-gray-500 ml-2">内存 </span>
                    {systemInfo.sbm.memory_mb.toFixed(1)}MB
                  </>
                ) : '-'}
              </p>
            </div>
          </CardBody>
        </Card>

        <Card>
          <CardBody className="flex flex-row items-center gap-4">
            <div className="p-3 bg-orange-100 dark:bg-orange-900 rounded-lg">
              <Activity className="w-6 h-6 text-orange-600 dark:text-orange-300" />
            </div>
            <div>
              <p className="text-sm text-gray-500">sing-box 资源</p>
              <p className="text-lg font-bold">
                {serviceStatus?.running && systemInfo?.singbox ? (
                  <>
                    <span className="text-sm font-normal text-gray-500">CPU </span>
                    {systemInfo.singbox.cpu_percent.toFixed(1)}%
                    <span className="text-sm font-normal text-gray-500 ml-2">内存 </span>
                    {systemInfo.singbox.memory_mb.toFixed(1)}MB
                  </>
                ) : (
                  <span className="text-gray-400">未运行</span>
                )}
              </p>
            </div>
          </CardBody>
        </Card>
      </div>

      {/* 订阅概览 */}
      <Card>
        <CardHeader>
          <h2 className="text-lg font-semibold">订阅概览</h2>
        </CardHeader>
        <CardBody>
          {subscriptions.length === 0 ? (
            <p className="text-gray-500 text-center py-4">暂无订阅，请前往节点页面添加</p>
          ) : (
            <div className="space-y-3">
              {subscriptions.map((sub) => (
                <div
                  key={sub.id}
                  className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-800 rounded-lg"
                >
                  <div className="flex items-center gap-3">
                    <Chip
                      size="sm"
                      color={sub.enabled ? 'success' : 'default'}
                      variant="dot"
                    >
                      {sub.name}
                    </Chip>
                    <span className="text-sm text-gray-500">
                      {sub.node_count} 个节点
                    </span>
                  </div>
                  <span className="text-sm text-gray-400">
                    更新于 {new Date(sub.updated_at).toLocaleString()}
                  </span>
                </div>
              ))}
            </div>
          )}
        </CardBody>
      </Card>

      {/* 代理链路概览 */}
      <Card>
        <CardHeader>
          <h2 className="text-lg font-semibold">代理链路概览</h2>
        </CardHeader>
        <CardBody>
          {proxyChains.length === 0 ? (
            <p className="text-gray-500 text-center py-4">暂无代理链路，请前往链路页面添加</p>
          ) : (
            <div className="space-y-3">
              {proxyChains.map((chain) => (
                <div
                  key={chain.id}
                  className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-800 rounded-lg"
                >
                  <div className="flex items-center gap-3">
                    <Chip
                      size="sm"
                      color={chain.enabled ? 'success' : 'default'}
                      variant="dot"
                    >
                      {chain.name}
                    </Chip>
                    <span className="text-sm text-gray-500">
                      {chain.nodes?.length || 0} 跳
                    </span>
                  </div>
                  <Chip size="sm" variant="flat">
                    {chain.enabled ? '已启用' : '已禁用'}
                  </Chip>
                </div>
              ))}
            </div>
          )}
        </CardBody>
      </Card>

      {/* 入站端口概览 */}
      <Card>
        <CardHeader>
          <h2 className="text-lg font-semibold">入站端口概览</h2>
        </CardHeader>
        <CardBody>
          {inboundPorts.length === 0 ? (
            <p className="text-gray-500 text-center py-4">暂无入站端口，请前往入站页面添加</p>
          ) : (
            <div className="space-y-3">
              {inboundPorts.map((port) => (
                <div
                  key={port.id}
                  className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-800 rounded-lg"
                >
                  <div className="flex items-center gap-3">
                    <Chip
                      size="sm"
                      color={port.enabled ? 'success' : 'default'}
                      variant="dot"
                    >
                      {port.name}
                    </Chip>
                    <Chip size="sm" variant="flat">{port.type}</Chip>
                    <span className="text-sm text-gray-500">
                      {port.listen}:{port.port}
                    </span>
                    {port.auth && (
                      <Chip size="sm" color="warning" variant="flat">需认证</Chip>
                    )}
                  </div>
                  <span className="text-sm text-gray-400">
                    → {port.outbound}
                  </span>
                </div>
              ))}
            </div>
          )}
        </CardBody>
      </Card>

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
