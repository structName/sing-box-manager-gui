import { type Key, useEffect, useMemo, useRef, useState } from 'react';
import {
  Button,
  Card,
  CardBody,
  CardHeader,
  Chip,
  Input,
  Select,
  SelectItem,
  Switch,
  Tab,
  Table,
  TableBody,
  TableCell,
  TableColumn,
  TableHeader,
  TableRow,
  Tabs,
  Textarea,
} from '@nextui-org/react';
import { AlertTriangle, CheckCircle2, CircleDot, Clock3, Copy, DownloadCloud, Import, KeyRound, Network, Play, RefreshCw, Rocket, ServerCog, ShieldCheck, Terminal, UploadCloud } from 'lucide-react';
import { deploymentApi } from '../api';
import { toast } from '../components/Toast';

interface DeploymentTemplate {
  name: string;
  version: string;
  checksum: string;
  description: string;
  user_selectable: boolean;
  parameters?: Array<{
    name: string;
    label: string;
    type: string;
    required: boolean;
    default?: string;
    options?: string[];
    secret?: boolean;
    advanced?: boolean;
    description?: string;
  }>;
  runtime: {
    name: string;
    version: string;
    channel: string;
  };
}

interface DeploymentRun {
  id: string;
  task_id?: string;
  template_name: string;
  template_version: string;
  template_checksum: string;
  status: string;
  dry_run: boolean;
  ssh_host: string;
  ssh_port: number;
  ssh_user: string;
  auth_method: string;
  node_server: string;
  proxy_port: number;
  connection_mode: string;
  proxy_type?: string;
  proxy_host?: string;
  proxy_connection_port?: number;
  redacted_params?: Record<string, unknown>;
  progress_markers?: Record<string, unknown>;
  probe_result?: Record<string, unknown>;
  generated_node?: Record<string, unknown>;
  stdout?: string;
  stderr?: string;
  exit_code?: number;
  imported_node_id?: number;
  started_at?: string;
  created_at: string;
  completed_at?: string;
}

interface DeploymentConnectionTestResult {
  ok: boolean;
  message: string;
  host: string;
  port: number;
  user: string;
  auth_method: string;
  duration_ms: number;
  remote?: Record<string, string>;
}

interface DeploymentConnectionCandidate {
  id: string;
  kind: string;
  display_name: string;
  source?: string;
  source_name?: string;
  node_tag?: string;
  chain_id?: string;
  outbound?: string;
  local_endpoint?: string;
  requires_service_running: boolean;
  requires_temporary_entrypoint: boolean;
  available: boolean;
  unavailable_reason?: string;
}

interface DeploymentRuntimeArchive {
  runtime_name?: string;
  version?: string;
  os?: string;
  arch?: string;
  path?: string;
  checksum?: string;
  expected_checksum?: string;
  status?: string;
  downloadable?: boolean;
  download_url?: string;
}

interface DeploymentGeneratedNodeReview {
  run_id: string;
  generated_node: Record<string, unknown>;
  share_link?: string;
  default_tag: string;
  can_import: boolean;
  imported_node_id?: number;
}

const statusColor: Record<string, 'default' | 'primary' | 'success' | 'warning' | 'danger'> = {
  pending: 'default',
  running: 'primary',
  success: 'success',
  failed: 'danger',
  cancelled: 'warning',
};

const runtimeStatusColor: Record<string, 'default' | 'success' | 'warning' | 'danger'> = {
  valid: 'success',
  present: 'success',
  missing: 'warning',
  invalid: 'danger',
};

const statusLabel: Record<string, string> = {
  pending: '等待中',
  running: '部署中',
  success: '已完成',
  failed: '失败',
  cancelled: '已取消',
};

const wizardSteps = [
  { key: 'target', label: '目标与认证', description: '选择模板并验证 SSH' },
  { key: 'node', label: '参数与链路', description: '配置节点、运行时和出口' },
  { key: 'review', label: '确认并部署', description: '复核风险和执行模式' },
];

function isUsableConnectionCandidate(candidate?: DeploymentConnectionCandidate) {
  return Boolean(candidate?.available && (candidate.local_endpoint || candidate.requires_temporary_entrypoint));
}

function firstUsableConnectionCandidate(candidates: DeploymentConnectionCandidate[]) {
  return candidates.find(isUsableConnectionCandidate);
}

function selectedKeyFrom(keys: 'all' | Set<Key>) {
  if (keys === 'all') return '';
  const [key] = Array.from(keys);
  return key === undefined ? '' : String(key);
}

function apiErrorMessage(error: unknown, fallback: string) {
  const response = (error as { response?: { data?: { error?: unknown } } })?.response;
  return typeof response?.data?.error === 'string' ? response.data.error : fallback;
}

export default function Deployments() {
  const [templates, setTemplates] = useState<DeploymentTemplate[]>([]);
  const [connectionCandidates, setConnectionCandidates] = useState<DeploymentConnectionCandidate[]>([]);
  const [runtimeArchives, setRuntimeArchives] = useState<DeploymentRuntimeArchive[]>([]);
  const [runs, setRuns] = useState<DeploymentRun[]>([]);
  const [selectedRunId, setSelectedRunId] = useState<string>('');
  const [activeTab, setActiveTab] = useState('create');
  const [wizardStep, setWizardStep] = useState('target');
  const [selectedArtifactKey, setSelectedArtifactKey] = useState<string>('generated_node');
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [testingConnection, setTestingConnection] = useState(false);
  const [connectionResult, setConnectionResult] = useState<DeploymentConnectionTestResult | null>(null);
  const [generatedNodeReview, setGeneratedNodeReview] = useState<DeploymentGeneratedNodeReview | null>(null);
  const [importing, setImporting] = useState(false);
  const [uploadingRuntime, setUploadingRuntime] = useState(false);
  const previousImportRunIdRef = useRef<string>('');
  const connectionModeTouchedRef = useRef(false);
  const [form, setForm] = useState({
    template_name: 'singbox-vless-reality',
    ssh_host: '',
    ssh_port: '22',
    ssh_user: 'root',
    auth_method: 'password',
    ssh_password: '',
    ssh_private_key: '',
    ssh_private_key_passphrase: '',
    node_server: '',
    node_name: 'deployed-vless-reality',
    node_proxy_port: '443',
    connection_mode: 'direct',
    managed_candidate_id: '',
    proxy_type: 'socks5',
    proxy_host: '',
    connection_proxy_port: '1080',
    proxy_username: '',
    proxy_password: '',
	    runtime_source: 'remote',
	    runtime_arch: 'amd64',
	    security_install_base_packages: false,
	    security_base_packages: 'curl ca-certificates tar gzip unzip',
	    security_firewall_mode: 'inspect_only',
	    debug_preserve_remote_run_dir: false,
	    dry_run: true,
	  });
  const [importForm, setImportForm] = useState({ tag: '', sourceName: '手动添加', enabled: true, advancedProtocolEdit: false, protocolOverrides: '' });

  const selectedRun = useMemo(
    () => runs.find((run) => run.id === selectedRunId) || runs[0],
    [runs, selectedRunId],
  );
	  const selectableTemplates = templates.filter((template) => template.user_selectable);
	  const selectedTemplate = templates.find((template) => template.name === form.template_name);
	  const isProxyDeploymentTemplate = form.template_name === 'singbox-vless-reality';
	  const isSecurityBasicTemplate = form.template_name === 'security-basic';
	  const selectedTemplateHasRuntime = Boolean(selectedTemplate?.runtime?.name);
  const hasActiveRun = runs.some((run) => run.status === 'running' || run.status === 'pending');
  const successfulRunCount = runs.filter((run) => run.status === 'success').length;
  const failedRunCount = runs.filter((run) => run.status === 'failed').length;
  const importableRunCount = runs.filter((run) => run.status === 'success' && !run.dry_run && run.generated_node && !run.imported_node_id).length;
  const currentWizardStepIndex = Math.max(0, wizardSteps.findIndex((step) => step.key === wizardStep));
  const usableConnectionCandidates = connectionCandidates.filter(isUsableConnectionCandidate);
  const selectedConnectionCandidate = connectionCandidates.find((candidate) => candidate.id === form.managed_candidate_id);
  const generatedNodeForReview = generatedNodeReview?.generated_node || selectedRun?.generated_node;
  const hasGeneratedNodeForReview = Boolean(generatedNodeForReview && Object.keys(generatedNodeForReview).length > 0);
  const canImportSelectedRun = Boolean(
    selectedRun
      && selectedRun.status === 'success'
      && !selectedRun.dry_run
      && hasGeneratedNodeForReview
      && generatedNodeReview?.can_import
      && !selectedRun.imported_node_id,
  );
  const selectedImportActionLabel = selectedRun?.dry_run
    ? 'Dry run 不可导入'
    : selectedRun?.imported_node_id
      ? `已导入 #${selectedRun.imported_node_id}`
      : '导入手动节点';

  const deploymentRunImportStatus = (run: DeploymentRun): { label: string; color: 'default' | 'success' | 'warning' } => {
    if (run.dry_run) return { label: 'Dry run 不导入', color: 'default' };
    if (run.imported_node_id) return { label: `已导入 #${run.imported_node_id}`, color: 'success' };
    if (run.status === 'success' && run.generated_node) return { label: '未导入', color: 'warning' };
    if (run.status === 'success') return { label: '无生成节点', color: 'default' };
    return { label: '等待部署', color: 'default' };
  };

  useEffect(() => {
    void loadAll();
  }, []);

  useEffect(() => {
    if (!hasActiveRun) return;
    const timer = window.setInterval(() => {
      void loadRuns();
    }, 2500);
    return () => window.clearInterval(timer);
  }, [hasActiveRun]);

  useEffect(() => {
    const runID = selectedRun?.id || '';
    if (runID !== previousImportRunIdRef.current) {
      previousImportRunIdRef.current = runID;
      setGeneratedNodeReview(null);
      setImportForm({ tag: String(selectedRun?.generated_node?.tag || ''), sourceName: '手动添加', enabled: true, advancedProtocolEdit: false, protocolOverrides: '' });
      return;
    }
    if (selectedRun?.generated_node?.tag) {
      setImportForm((current) => (current.tag ? current : { ...current, tag: String(selectedRun.generated_node?.tag || '') }));
    }
  }, [selectedRun?.generated_node?.tag, selectedRun?.id]);

  useEffect(() => {
    if (!selectedRun || selectedRun.status !== 'success' || selectedRun.dry_run || !selectedRun.generated_node) {
      setGeneratedNodeReview(null);
      return;
    }
    let cancelled = false;
    deploymentApi.getGeneratedNode(selectedRun.id)
      .then((res) => {
        if (cancelled) return;
        const review = res.data.data as DeploymentGeneratedNodeReview;
        setGeneratedNodeReview(review);
        if (review.default_tag) {
          setImportForm((current) => ({ ...current, tag: current.tag || review.default_tag }));
        }
      })
      .catch(() => {
        if (!cancelled) setGeneratedNodeReview(null);
      });
    return () => {
      cancelled = true;
    };
  }, [selectedRun?.dry_run, selectedRun?.generated_node, selectedRun?.id, selectedRun?.status]);

  useEffect(() => {
    if (selectedTemplate?.runtime?.name === 'sing-box' && selectedTemplate.runtime.version) {
      void refreshRuntimeCache(selectedTemplate.runtime.version);
    }
  }, [selectedTemplate?.runtime?.name, selectedTemplate?.runtime?.version]);

  const refreshRuntimeCache = async (version: string) => {
    const runtimeRes = await deploymentApi.getRuntimeCache({ runtime_name: 'sing-box', version });
    setRuntimeArchives(runtimeRes.data.data || []);
  };

  const loadAll = async () => {
    setLoading(true);
    try {
      const [templateRes, runRes] = await Promise.all([
        deploymentApi.getTemplates(),
        deploymentApi.getRuns({ limit: 50 }),
      ]);
      const nextTemplates = templateRes.data.data || [];
      setTemplates(nextTemplates);
      if (nextTemplates.length > 0 && !nextTemplates.some((template: DeploymentTemplate) => template.name === form.template_name)) {
        setForm((current) => ({ ...current, template_name: nextTemplates[0].name }));
      }
      const nextRuns = runRes.data.data || [];
      setRuns(nextRuns);
      const selectedOrFirst = nextTemplates.find((template: DeploymentTemplate) => template.name === form.template_name) || nextTemplates[0];
      if (selectedOrFirst?.runtime?.name === 'sing-box' && selectedOrFirst.runtime.version) {
        await refreshRuntimeCache(selectedOrFirst.runtime.version);
      }
      if (!selectedRunId && nextRuns.length > 0) {
        setSelectedRunId(nextRuns[0].id);
      }
      const candidateRes = await deploymentApi.getConnectionCandidates();
      const nextCandidates = candidateRes.data.data || [];
      setConnectionCandidates(nextCandidates);
      const firstUsableCandidate = firstUsableConnectionCandidate(nextCandidates);
      setForm((current) => {
        const selectedCandidateIsUsable = nextCandidates.some((candidate: DeploymentConnectionCandidate) => (
          candidate.id === current.managed_candidate_id && isUsableConnectionCandidate(candidate)
        ));
        const managedCandidateID = selectedCandidateIsUsable ? current.managed_candidate_id : firstUsableCandidate?.id || '';
        const shouldAutoUseManagedProxy = !connectionModeTouchedRef.current
          && current.connection_mode === 'direct'
          && Boolean(firstUsableCandidate);

        if (current.managed_candidate_id === managedCandidateID && !shouldAutoUseManagedProxy) {
          return current;
        }

        return {
          ...current,
          connection_mode: shouldAutoUseManagedProxy ? 'managed_proxy' : current.connection_mode,
          managed_candidate_id: managedCandidateID,
        };
      });
    } catch (error: unknown) {
      toast.error(apiErrorMessage(error, '加载部署数据失败'));
    } finally {
      setLoading(false);
    }
  };

  const loadRuns = async () => {
    const runRes = await deploymentApi.getRuns({ limit: 50 });
    const nextRuns = runRes.data.data || [];
    setRuns(nextRuns);
    if (!selectedRunId && nextRuns.length > 0) {
      setSelectedRunId(nextRuns[0].id);
    }
  };

	  const buildDeploymentPayload = () => {
	    const nodeServer = form.node_server.trim() || form.ssh_host.trim();
	    const parameters = isSecurityBasicTemplate
	      ? {
	          security_install_base_packages: form.security_install_base_packages,
	          security_base_packages: form.security_base_packages.trim() || 'curl ca-certificates tar gzip unzip',
	          security_firewall_mode: form.security_firewall_mode,
	        }
	      : {
	          node_name: form.node_name.trim() || 'deployed-vless-reality',
	          node_server: nodeServer,
	          proxy_port: Number(form.node_proxy_port) || 443,
	          runtime_source: form.runtime_source,
	          runtime_os: 'linux',
	          runtime_arch: form.runtime_arch,
	          debug_preserve_remote_run_dir: form.debug_preserve_remote_run_dir,
	        };
	    return {
	      template_name: form.template_name,
      dry_run: form.dry_run,
      ssh: {
        host: form.ssh_host.trim(),
        port: Number(form.ssh_port) || 22,
        user: form.ssh_user.trim() || 'root',
        auth_method: form.auth_method,
        password: form.auth_method === 'password' ? form.ssh_password : '',
        private_key: form.auth_method === 'private_key' ? form.ssh_private_key : '',
        private_key_passphrase: form.auth_method === 'private_key' ? form.ssh_private_key_passphrase : '',
      },
      connection: {
        mode: form.connection_mode,
        managed_candidate_id: form.connection_mode === 'managed_proxy' ? form.managed_candidate_id : '',
        proxy_type: form.connection_mode === 'custom_proxy' ? form.proxy_type : '',
        proxy_host: form.connection_mode === 'custom_proxy' ? form.proxy_host.trim() : '',
        proxy_port: form.connection_mode === 'custom_proxy' ? Number(form.connection_proxy_port) || 0 : 0,
        proxy_username: form.connection_mode === 'custom_proxy' ? form.proxy_username : '',
        proxy_password: form.connection_mode === 'custom_proxy' ? form.proxy_password : '',
      },
	      parameters,
	    };
	  };

  const startDeployment = async () => {
    if (!validateConnectionForm(true)) {
      return;
    }
    setSubmitting(true);
    try {
      const res = await deploymentApi.createRun(buildDeploymentPayload());
      const run = res.data.data;
      setRuns((current) => [run, ...current.filter((item) => item.id !== run.id)]);
      setSelectedRunId(run.id);
      setImportForm({ tag: String(run.generated_node?.tag || form.node_name), sourceName: '手动添加', enabled: true, advancedProtocolEdit: false, protocolOverrides: '' });
      setActiveTab('runs');
      if (run.status === 'running' || run.status === 'pending') {
        toast.success('部署已启动');
        void loadRuns();
      } else {
        toast.success('部署运行已完成');
      }
    } catch (error: unknown) {
      toast.error(apiErrorMessage(error, '启动部署失败'));
    } finally {
      setSubmitting(false);
    }
  };

  const testConnection = async () => {
    if (!validateConnectionForm()) {
      return;
    }
    setTestingConnection(true);
    setConnectionResult(null);
    try {
      const payload = buildDeploymentPayload();
      const res = await deploymentApi.testConnection({
        ssh: payload.ssh,
        connection: payload.connection,
      });
      const result = res.data.data as DeploymentConnectionTestResult;
      setConnectionResult(result);
      if (result.ok) {
        toast.success(result.message || 'SSH 连接成功');
      } else {
        toast.error(result.message || 'SSH 连接失败');
      }
    } catch (error: unknown) {
      toast.error(apiErrorMessage(error, 'SSH 连接测试失败'));
    } finally {
      setTestingConnection(false);
    }
  };

  const importGeneratedNode = async () => {
    if (!selectedRun) return;
    if (!importForm.tag.trim()) {
      toast.error('节点名称为必填');
      return;
    }
    let generatedNodeOverrides: Record<string, unknown> | undefined;
    if (importForm.advancedProtocolEdit) {
      const text = importForm.protocolOverrides.trim();
      if (text) {
        try {
          const parsed = JSON.parse(text);
          if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
            toast.error('协议覆盖必须是 JSON 对象');
            return;
          }
          generatedNodeOverrides = parsed;
        } catch {
          toast.error('协议覆盖 JSON 无法解析');
          return;
        }
      }
    }
    setImporting(true);
    try {
      await deploymentApi.importNode(selectedRun.id, {
        tag: importForm.tag.trim(),
        source_name: importForm.sourceName.trim() || '手动添加',
        enabled: importForm.enabled,
        advanced_protocol_edit: importForm.advancedProtocolEdit,
        generated_node_overrides: generatedNodeOverrides,
      });
      toast.success('生成节点已导入手动节点');
      await loadAll();
    } catch (error: unknown) {
      toast.error(apiErrorMessage(error, '导入节点失败'));
    } finally {
      setImporting(false);
    }
  };

  const copyGeneratedNodeLink = async (run?: DeploymentRun) => {
    const runID = run?.id || selectedRun?.id || '';
    if (!runID) {
      toast.error('请先选择部署记录');
      return;
    }

    let link = !run || run.id === selectedRun?.id ? generatedNodeReview?.share_link || '' : '';
    if (!link) {
      try {
        const res = await deploymentApi.getGeneratedNode(runID);
        const review = res.data.data as DeploymentGeneratedNodeReview;
        link = review.share_link || '';
        if (runID === selectedRun?.id) {
          setGeneratedNodeReview(review);
        }
      } catch (error: unknown) {
        toast.error(apiErrorMessage(error, '获取节点链接失败'));
        return;
      }
    }
    if (!link) {
      toast.error('暂无可复制的节点链接');
      return;
    }
    try {
      await navigator.clipboard.writeText(link);
      toast.success('节点链接已复制');
    } catch (error: unknown) {
      toast.error(error instanceof Error ? error.message : '复制节点链接失败');
    }
  };

  const cancelDeploymentRun = async (run: DeploymentRun) => {
    try {
      await deploymentApi.cancelRun(run.id);
      toast.success('部署任务已取消');
      await loadRuns();
    } catch (error: unknown) {
      toast.error(apiErrorMessage(error, '取消部署失败'));
    }
  };

  const uploadRuntimeArchive = async (file: File | null) => {
    if (!file) return;
    setUploadingRuntime(true);
    try {
      const version = selectedTemplate?.runtime?.version || '1.13.13';
      await deploymentApi.uploadRuntimeCache({
        runtime_name: 'sing-box',
        version,
        os: 'linux',
        arch: form.runtime_arch,
        file,
      });
      toast.success('运行时缓存已上传');
      const runtimeRes = await deploymentApi.getRuntimeCache({ runtime_name: 'sing-box', version });
      const archives = runtimeRes.data.data || [];
      setRuntimeArchives(archives);
      const uploadedArchive = archives.find((archive: DeploymentRuntimeArchive) => archive.os === 'linux' && archive.arch === form.runtime_arch);
      if (uploadedArchive?.status === 'valid') {
        setForm((current) => ({ ...current, runtime_source: 'cache' }));
      } else {
        toast.error('运行时缓存校验未通过，保持 Remote download');
      }
    } catch (error: unknown) {
      toast.error(apiErrorMessage(error, '上传运行时缓存失败'));
    } finally {
      setUploadingRuntime(false);
    }
  };

  const validateConnectionForm = (includeDeploymentFields = false) => {
    if (!form.ssh_host.trim()) {
      toast.error('SSH Host 为必填');
      return false;
    }
    const sshPort = Number(form.ssh_port);
    if (!Number.isInteger(sshPort) || sshPort < 1 || sshPort > 65535) {
      toast.error('SSH Port 必须在 1-65535 之间');
      return false;
    }
    if (!form.ssh_user.trim()) {
      toast.error('SSH User 为必填');
      return false;
    }
    if (form.auth_method === 'password' && !form.ssh_password) {
      toast.error('SSH Password 为必填');
      return false;
    }
    if (form.auth_method === 'private_key' && !form.ssh_private_key.trim()) {
      toast.error('SSH Private Key 为必填');
      return false;
    }
    if (form.connection_mode === 'managed_proxy' && !isUsableConnectionCandidate(selectedConnectionCandidate)) {
      toast.error('请选择可用的托管连接路线');
      return false;
    }
    if (form.connection_mode === 'custom_proxy') {
      if (!form.proxy_host.trim()) {
        toast.error('Proxy Host 为必填');
        return false;
      }
      const proxyPort = Number(form.connection_proxy_port);
      if (!Number.isInteger(proxyPort) || proxyPort < 1 || proxyPort > 65535) {
        toast.error('Proxy Port 必须在 1-65535 之间');
        return false;
      }
    }
	    if (includeDeploymentFields && isProxyDeploymentTemplate) {
	      const nodeProxyPort = Number(form.node_proxy_port);
	      if (!Number.isInteger(nodeProxyPort) || nodeProxyPort < 1 || nodeProxyPort > 65535) {
	        toast.error('Proxy Port 必须在 1-65535 之间');
	        return false;
	      }
	    }
	    if (includeDeploymentFields && isSecurityBasicTemplate && form.security_firewall_mode !== 'inspect_only') {
	      toast.error('Firewall Mode 仅支持 inspect_only');
	      return false;
	    }
    return true;
  };

  const selectedTemplateKeys = selectableTemplates.some((template) => template.name === form.template_name) ? [form.template_name] : [];
  const selectedCandidateKeys = isUsableConnectionCandidate(selectedConnectionCandidate) ? [form.managed_candidate_id] : [];
  const selectedRunKeys = selectedRun ? [selectedRun.id] : [];
  const artifactRows = useMemo(() => {
    if (!selectedRun) return [];
    const rows = [
      {
        key: 'run_summary',
        label: 'Run Summary',
        kind: 'JSON',
        status: 'ready',
        value: JSON.stringify({
          id: selectedRun.id,
          task_id: selectedRun.task_id,
          status: selectedRun.status,
          dry_run: selectedRun.dry_run,
          template: {
            name: selectedRun.template_name,
            version: selectedRun.template_version,
            checksum: selectedRun.template_checksum,
          },
          ssh: {
            host: selectedRun.ssh_host,
            port: selectedRun.ssh_port,
            user: selectedRun.ssh_user,
            auth_method: selectedRun.auth_method,
          },
          node: {
            server: selectedRun.node_server,
            proxy_port: selectedRun.proxy_port,
          },
          connection: {
            mode: selectedRun.connection_mode,
            proxy_type: selectedRun.proxy_type,
            proxy_host: selectedRun.proxy_host,
            proxy_port: selectedRun.proxy_connection_port,
          },
          exit_code: selectedRun.exit_code ?? null,
          imported_node_id: selectedRun.imported_node_id ?? null,
          created_at: selectedRun.created_at,
          started_at: selectedRun.started_at || null,
          completed_at: selectedRun.completed_at || null,
        }, null, 2),
      },
      {
        key: 'generated_node',
        label: generatedNodeReview ? 'Generated Node Review' : 'Generated Node (Redacted)',
        kind: 'JSON',
        status: hasGeneratedNodeForReview ? 'ready' : 'empty',
        value: JSON.stringify(generatedNodeForReview || {}, null, 2),
      },
      {
        key: 'probe_result',
        label: 'Probe Result',
        kind: 'JSON',
        status: selectedRun.probe_result && Object.keys(selectedRun.probe_result).length > 0 ? 'ready' : 'empty',
        value: JSON.stringify(selectedRun.probe_result || {}, null, 2),
      },
      {
        key: 'progress_markers',
        label: 'Progress Markers',
        kind: 'JSON',
        status: selectedRun.progress_markers && Object.keys(selectedRun.progress_markers).length > 0 ? 'ready' : 'empty',
        value: JSON.stringify(selectedRun.progress_markers || {}, null, 2),
      },
      {
        key: 'redacted_params',
        label: 'Redacted Parameters',
        kind: 'JSON',
        status: selectedRun.redacted_params && Object.keys(selectedRun.redacted_params).length > 0 ? 'ready' : 'empty',
        value: JSON.stringify(selectedRun.redacted_params || {}, null, 2),
      },
      {
        key: 'stdout',
        label: 'Stdout',
        kind: 'Log',
        status: selectedRun.stdout ? 'ready' : 'empty',
        value: selectedRun.stdout || '',
      },
    ];
    if (generatedNodeReview?.share_link) {
      rows.splice(2, 0, {
        key: 'generated_node_link',
        label: '节点链接',
        kind: 'URI',
        status: 'ready',
        value: generatedNodeReview.share_link,
      });
    }
    if (selectedRun.stderr) {
      rows.push({
        key: 'stderr',
        label: 'Stderr',
        kind: 'Log',
        status: 'ready',
        value: selectedRun.stderr,
      });
    }
    return rows;
  }, [generatedNodeForReview, generatedNodeReview, hasGeneratedNodeForReview, selectedRun]);
  const selectedArtifact = artifactRows.find((row) => row.key === selectedArtifactKey) || artifactRows[0];
  const generatedNodeArtifact = artifactRows.find((row) => row.key === 'generated_node');
  const stdoutArtifact = artifactRows.find((row) => row.key === 'stdout');
  const stderrArtifact = artifactRows.find((row) => row.key === 'stderr');
  const diagnosticRows = artifactRows.filter((row) => !['generated_node', 'generated_node_link', 'stdout', 'stderr'].includes(row.key));
  const selectedImportStatus = selectedRun ? deploymentRunImportStatus(selectedRun) : { label: '无记录', color: 'default' as const };
  const externalReachability = selectedRun?.progress_markers?.external_reachability as Record<string, unknown> | undefined;
  const externalReachabilityStatus = typeof externalReachability?.status === 'string' ? externalReachability.status : '';

  const previewText = (value: string) => {
    const singleLine = value.replace(/\s+/g, ' ').trim();
    if (!singleLine) return '无内容';
    return singleLine.length > 120 ? `${singleLine.slice(0, 120)}...` : singleLine;
  };

  return (
    <div className="space-y-6">
      <section className="overflow-hidden rounded-2xl border border-primary-100 bg-gradient-to-br from-primary-50 via-white to-cyan-50 shadow-sm dark:border-primary-900/60 dark:from-primary-950/40 dark:via-gray-900 dark:to-cyan-950/30">
        <div className="flex flex-col gap-5 p-5 sm:p-6 xl:flex-row xl:items-center xl:justify-between">
          <div className="max-w-2xl">
            <div className="mb-3 flex items-center gap-2 text-primary">
              <div className="rounded-xl bg-primary-100 p-2 dark:bg-primary-900/60">
                <Rocket className="h-5 w-5" />
              </div>
              <span className="text-sm font-semibold">远程代理节点交付工作台</span>
            </div>
            <h1 className="text-2xl font-bold tracking-tight text-gray-900 dark:text-white sm:text-3xl">代理节点部署</h1>
            <p className="mt-2 text-sm leading-6 text-gray-600 dark:text-gray-300">
              从 SSH 连通性检查、模板执行到节点导入，在同一条部署链路中完成并保留可审计记录。
            </p>
          </div>
          <Button className="self-start xl:self-auto" color="primary" variant="flat" startContent={<RefreshCw className="h-4 w-4" />} isLoading={loading} onPress={() => void loadAll()}>
            刷新数据
          </Button>
        </div>
        <div className="grid border-t border-primary-100 bg-white/70 sm:grid-cols-2 xl:grid-cols-4 dark:border-primary-900/60 dark:bg-gray-950/20">
          {[
            { label: '全部运行', value: runs.length, hint: hasActiveRun ? '正在自动刷新' : '历史部署记录', color: 'text-gray-900 dark:text-white' },
            { label: '进行中', value: runs.filter((run) => run.status === 'running' || run.status === 'pending').length, hint: '等待或正在执行', color: 'text-primary' },
            { label: '已完成', value: successfulRunCount, hint: failedRunCount > 0 ? `${failedRunCount} 次失败` : '暂无失败记录', color: 'text-success' },
            { label: '待导入节点', value: importableRunCount, hint: '可进入产物页处理', color: 'text-warning' },
          ].map((item) => (
            <div key={item.label} className="border-b border-primary-100 px-5 py-4 last:border-b-0 sm:[&:nth-child(odd)]:border-r xl:border-b-0 xl:border-r xl:last:border-r-0 dark:border-primary-900/60">
              <p className="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{item.label}</p>
              <div className="mt-1 flex items-end justify-between gap-3">
                <p className={`text-2xl font-bold ${item.color}`}>{item.value}</p>
                <p className="text-right text-xs text-gray-500 dark:text-gray-400">{item.hint}</p>
              </div>
            </div>
          ))}
        </div>
      </section>

      <Tabs
        aria-label="部署工作区"
        classNames={{
          tabList: 'w-full gap-1 rounded-xl bg-gray-100 p-1 dark:bg-gray-800',
          cursor: 'rounded-lg shadow-sm',
          tab: 'h-11 px-3 sm:px-5',
          panel: 'px-0 pt-4',
        }}
        color="primary"
        selectedKey={activeTab}
        onSelectionChange={(key) => setActiveTab(String(key))}
        variant="solid"
      >
        <Tab key="create" title={<div className="flex items-center gap-2"><ServerCog className="h-4 w-4" /><span>新建部署</span></div>}>
          <Card className="border border-gray-200 shadow-sm dark:border-gray-700">
            <CardHeader className="flex flex-col items-start gap-2 sm:flex-row sm:items-center sm:justify-between">
              <div className="flex items-center gap-2">
                <ServerCog className="h-5 w-5 text-primary" />
                <span className="font-semibold">新建部署向导</span>
              </div>
              <Chip size="sm" variant="flat" color={form.dry_run ? 'warning' : 'primary'}>
                {form.dry_run ? 'Dry run' : 'Real run'}
              </Chip>
            </CardHeader>
            <CardBody className="gap-5">
              <div className="grid gap-2 md:grid-cols-3">
                {wizardSteps.map((step, index) => {
                  const isCurrent = step.key === wizardStep;
                  const isComplete = index < currentWizardStepIndex;
                  return (
                    <button
                      key={step.key}
                      type="button"
                      className={`flex items-start gap-3 rounded-xl border p-3 text-left transition ${
                        isCurrent
                          ? 'border-primary bg-primary-50 shadow-sm dark:bg-primary-950/30'
                          : 'border-gray-200 hover:border-primary-300 hover:bg-gray-50 dark:border-gray-700 dark:hover:bg-gray-800'
                      }`}
                      onClick={() => setWizardStep(step.key)}
                    >
                      <span className={`flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-xs font-bold ${isCurrent || isComplete ? 'bg-primary text-white' : 'bg-gray-100 text-gray-500 dark:bg-gray-700 dark:text-gray-300'}`}>
                        {isComplete ? <CheckCircle2 className="h-4 w-4" /> : index + 1}
                      </span>
                      <span>
                        <span className="block text-sm font-semibold text-gray-800 dark:text-gray-100">{step.label}</span>
                        <span className="mt-0.5 block text-xs text-gray-500 dark:text-gray-400">{step.description}</span>
                      </span>
                    </button>
                  );
                })}
              </div>
              <Tabs classNames={{ tabList: 'hidden', panel: 'p-0' }} selectedKey={wizardStep} onSelectionChange={(key) => setWizardStep(String(key))}>
                <Tab key="target" title="1. 目标与认证">
                  <div className="space-y-5 pt-4">
                    <div className="space-y-3 rounded-md border border-gray-200 p-4 dark:border-gray-700">
                      <div className="flex items-center gap-2 text-sm font-semibold text-gray-700 dark:text-gray-200">
                        <ShieldCheck className="h-4 w-4 text-primary" />
                        <span>模板</span>
                      </div>
                      <Select
                        id="deployment-template-name"
                        name="template_name"
                        aria-label="Deployment template"
                        label="模板"
                        selectedKeys={selectedTemplateKeys}
                        onSelectionChange={(keys) => {
                          const next = selectedKeyFrom(keys);
                          if (next) setForm((current) => ({ ...current, template_name: next }));
                        }}
                      >
                        {selectableTemplates.map((template) => (
                          <SelectItem key={template.name} textValue={template.name}>
                            {template.name}
                          </SelectItem>
                        ))}
                      </Select>
                      {selectedTemplate && (
                        <div className="rounded-md bg-gray-50 p-3 text-xs text-gray-600 dark:bg-gray-900/40 dark:text-gray-300">
	                          <div className="flex flex-wrap items-center gap-2">
	                            <span>{selectedTemplate.version}</span>
	                            {selectedTemplateHasRuntime && <span>{selectedTemplate.runtime.name} {selectedTemplate.runtime.version}</span>}
	                          </div>
                          <p className="mt-2 break-all">sha256 {selectedTemplate.checksum}</p>
                        </div>
                      )}
                    </div>

                    <form className="space-y-3 rounded-md border border-gray-200 p-4 dark:border-gray-700" onSubmit={(event) => event.preventDefault()}>
                      <div className="flex items-center gap-2 text-sm font-semibold text-gray-700 dark:text-gray-200">
                        <Terminal className="h-4 w-4 text-primary" />
                        <span>SSH</span>
                      </div>
                      <div className="grid grid-cols-1 gap-3 md:grid-cols-4">
                        <Input
                          className="md:col-span-2"
                          id="deployment-ssh-host"
                          name="ssh_host"
                          label="SSH Host"
                          value={form.ssh_host}
                          onValueChange={(value) => {
                            setConnectionResult(null);
                            setForm((current) => ({
                              ...current,
                              ssh_host: value,
                              node_server: !current.node_server || current.node_server === current.ssh_host ? value : current.node_server,
                            }));
                          }}
                        />
                        <Input
                          id="deployment-ssh-port"
                          name="ssh_port"
                          label="SSH Port"
                          type="number"
                          min={1}
                          max={65535}
                          value={form.ssh_port}
                          onValueChange={(value) => {
                            setConnectionResult(null);
                            setForm({ ...form, ssh_port: value });
                          }}
                        />
                        <Input
                          id="deployment-ssh-user"
                          name="ssh_user"
                          label="SSH User"
                          value={form.ssh_user}
                          onValueChange={(value) => {
                            setConnectionResult(null);
                            setForm({ ...form, ssh_user: value });
                          }}
                        />
                      </div>
                      <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                        <div className="grid grid-cols-2 gap-2 rounded-md bg-gray-100 p-1 dark:bg-gray-900">
                          <Button
                            type="button"
                            color={form.auth_method === 'password' ? 'primary' : 'default'}
                            variant={form.auth_method === 'password' ? 'solid' : 'light'}
                            onPress={() => {
                              setForm({ ...form, auth_method: 'password' });
                              setConnectionResult(null);
                            }}
                          >
                            Password
                          </Button>
                          <Button
                            type="button"
                            color={form.auth_method === 'private_key' ? 'primary' : 'default'}
                            variant={form.auth_method === 'private_key' ? 'solid' : 'light'}
                            onPress={() => {
                              setForm({ ...form, auth_method: 'private_key' });
                              setConnectionResult(null);
                            }}
                          >
                            Private key
                          </Button>
                        </div>
                        {form.auth_method === 'password' && (
                          <Input
                            id="deployment-ssh-password"
                            name="ssh_password"
                            label="SSH Password"
                            type="password"
                            autoComplete="off"
                            value={form.ssh_password}
                            onValueChange={(value) => {
                              setConnectionResult(null);
                              setForm({ ...form, ssh_password: value });
                            }}
                          />
                        )}
                        {form.auth_method === 'private_key' && (
                          <Input
                            id="deployment-ssh-private-key-passphrase"
                            name="ssh_private_key_passphrase"
                            label="Private Key Passphrase"
                            type="password"
                            autoComplete="off"
                            value={form.ssh_private_key_passphrase}
                            onValueChange={(value) => {
                              setConnectionResult(null);
                              setForm({ ...form, ssh_private_key_passphrase: value });
                            }}
                          />
                        )}
                      </div>
                      {form.auth_method === 'private_key' && (
                        <Textarea
                          id="deployment-ssh-private-key"
                          name="ssh_private_key"
                          label="SSH Private Key"
                          minRows={6}
                          value={form.ssh_private_key}
                          onValueChange={(value) => {
                            setConnectionResult(null);
                            setForm({ ...form, ssh_private_key: value });
                          }}
                        />
                      )}
                    </form>

                    <div className="flex justify-end gap-3">
                      <Button color="primary" onPress={() => setWizardStep('node')}>下一步</Button>
                    </div>
                  </div>
                </Tab>

                <Tab key="node" title="2. 节点与路径">
                  <div className="space-y-5 pt-4">
	                    {isProxyDeploymentTemplate && (
	                      <>
	                        <div className="space-y-3 rounded-md border border-gray-200 p-4 dark:border-gray-700">
	                          <div className="flex items-center gap-2 text-sm font-semibold text-gray-700 dark:text-gray-200">
	                            <KeyRound className="h-4 w-4 text-primary" />
	                            <span>节点参数</span>
	                          </div>
	                          <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
	                            <Input id="deployment-node-server" name="node_server" label="Node Server" value={form.node_server} onValueChange={(value) => setForm({ ...form, node_server: value })} />
	                            <Input id="deployment-node-proxy-port" name="node_proxy_port" label="Proxy Port" type="number" min={1} max={65535} value={form.node_proxy_port} onValueChange={(value) => setForm({ ...form, node_proxy_port: value })} />
	                            <Input id="deployment-node-name" name="node_name" label="Node Name" value={form.node_name} onValueChange={(value) => setForm({ ...form, node_name: value })} />
	                          </div>
	                        </div>

	                        <div className="space-y-3 rounded-md border border-gray-200 p-4 dark:border-gray-700">
	                          <div className="flex items-center gap-2 text-sm font-semibold text-gray-700 dark:text-gray-200">
	                            <DownloadCloud className="h-4 w-4 text-primary" />
	                            <span>运行时</span>
	                          </div>
	                          <div className="grid grid-cols-1 gap-3 md:grid-cols-[1fr_180px_180px]">
	                            <Select
	                              id="deployment-runtime-source"
	                              name="runtime_source"
	                              aria-label="Runtime source"
	                              label="Runtime Source"
	                              selectedKeys={[form.runtime_source]}
	                              onSelectionChange={(keys) => {
	                                setForm((current) => ({ ...current, runtime_source: selectedKeyFrom(keys) || 'remote' }));
	                              }}
	                            >
	                              <SelectItem key="remote">Remote download</SelectItem>
	                              <SelectItem key="cache">Local cache</SelectItem>
	                            </Select>
	                            <Select
	                              id="deployment-runtime-arch"
	                              name="runtime_arch"
	                              aria-label="Runtime architecture"
	                              label="Linux Arch"
	                              selectedKeys={[form.runtime_arch]}
	                              onSelectionChange={(keys) => {
	                                setForm((current) => ({ ...current, runtime_arch: selectedKeyFrom(keys) || 'amd64' }));
	                              }}
	                            >
	                              <SelectItem key="amd64">amd64</SelectItem>
	                              <SelectItem key="arm64">arm64</SelectItem>
	                            </Select>
	                            <Button
	                              as="label"
	                              variant="flat"
	                              color="primary"
	                              startContent={<UploadCloud className="h-4 w-4" />}
	                              isLoading={uploadingRuntime}
	                            >
	                              上传 tar.gz
	                              <input
	                                id="deployment-runtime-archive"
	                                name="runtime_archive"
	                                type="file"
	                                accept=".gz,.tgz,application/gzip"
	                                className="hidden"
	                                onChange={(event) => {
	                                  const file = event.target.files?.[0] || null;
	                                  event.target.value = '';
	                                  void uploadRuntimeArchive(file);
	                                }}
	                              />
	                            </Button>
	                          </div>
	                          <div className="grid grid-cols-1 gap-2 lg:grid-cols-2">
	                            {runtimeArchives.map((archive) => (
	                              <div key={`${archive.os || 'unknown'}-${archive.arch || archive.path}`} className="rounded-md border border-gray-200 p-3 text-xs dark:border-gray-700">
	                                <div className="flex items-center justify-between gap-2">
	                                  <div className="font-medium text-gray-700 dark:text-gray-200">{archive.os || 'linux'}/{archive.arch || archive.path?.split('/').slice(-1)[0]}</div>
	                                  <Chip size="sm" color={runtimeStatusColor[archive.status || ''] || 'default'} variant="flat">
	                                    {archive.status || 'present'}
	                                  </Chip>
	                                </div>
	                                {archive.checksum && <div className="mt-1 break-all text-gray-500 dark:text-gray-400">sha256 {archive.checksum}</div>}
	                                {archive.expected_checksum && archive.status !== 'valid' && (
	                                  <div className="mt-1 break-all text-gray-400 dark:text-gray-500">expected {archive.expected_checksum}</div>
	                                )}
	                                {archive.download_url && archive.status === 'missing' && (
	                                  <div className="mt-1 break-all text-gray-400 dark:text-gray-500">{archive.download_url}</div>
	                                )}
	                              </div>
	                            ))}
	                            {runtimeArchives.length === 0 && (
	                              <div className="rounded-md border border-dashed border-gray-300 p-3 text-sm text-gray-500 dark:border-gray-700">
	                                本地缓存为空，真实部署会从官方 Release 下载。
	                              </div>
	                            )}
	                          </div>
	                        </div>
	                      </>
	                    )}

	                    {isSecurityBasicTemplate && (
	                      <div className="space-y-3 rounded-md border border-gray-200 p-4 dark:border-gray-700">
	                        <div className="flex items-center gap-2 text-sm font-semibold text-gray-700 dark:text-gray-200">
	                          <ShieldCheck className="h-4 w-4 text-primary" />
	                          <span>安全维护</span>
	                        </div>
	                        <div className="grid grid-cols-1 gap-3 md:grid-cols-[220px_1fr_180px]">
	                          <div className="flex items-center justify-between rounded-md bg-gray-50 p-3 dark:bg-gray-900/40">
	                            <span className="text-sm text-gray-700 dark:text-gray-200">安装基础包</span>
	                            <Switch aria-label="安装基础包" isSelected={form.security_install_base_packages} onValueChange={(value) => setForm({ ...form, security_install_base_packages: value })} />
	                          </div>
	                          <Input
	                            id="deployment-security-base-packages"
	                            name="security_base_packages"
	                            label="Base Packages"
	                            value={form.security_base_packages}
	                            isDisabled={!form.security_install_base_packages}
	                            onValueChange={(value) => setForm({ ...form, security_base_packages: value })}
	                          />
	                          <Select
	                            id="deployment-security-firewall-mode"
	                            name="security_firewall_mode"
	                            aria-label="Security firewall mode"
	                            label="Firewall"
	                            selectedKeys={[form.security_firewall_mode]}
	                            onSelectionChange={(keys) => setForm((current) => ({ ...current, security_firewall_mode: selectedKeyFrom(keys) || 'inspect_only' }))}
	                          >
	                            <SelectItem key="inspect_only">Inspect only</SelectItem>
	                          </Select>
	                        </div>
	                      </div>
	                    )}

                    <div className="space-y-3 rounded-md border border-gray-200 p-4 dark:border-gray-700">
                      <div className="flex items-center gap-2 text-sm font-semibold text-gray-700 dark:text-gray-200">
                        <Network className="h-4 w-4 text-primary" />
                        <span>连接路径</span>
                      </div>
                      <Select
                        id="deployment-connection-mode"
                        name="connection_mode"
                        aria-label="Deployment connection mode"
                        label="Connection"
                        selectedKeys={[form.connection_mode]}
                        onSelectionChange={(keys) => {
                          const connectionMode = selectedKeyFrom(keys) || 'direct';
                          connectionModeTouchedRef.current = true;
                          setConnectionResult(null);
                          setForm((current) => ({ ...current, connection_mode: connectionMode }));
                        }}
                      >
                        <SelectItem key="direct">Direct</SelectItem>
                        <SelectItem key="managed_proxy">Managed proxy</SelectItem>
                        <SelectItem key="custom_proxy">Custom proxy</SelectItem>
                      </Select>

                      {form.connection_mode === 'managed_proxy' && (
                        <div className="space-y-3">
                          <div className="flex items-center justify-between gap-3">
                            <Select
                              id="deployment-managed-candidate"
                              name="managed_candidate_id"
                              aria-label="Managed deployment route"
                              label="Managed route"
                              selectedKeys={selectedCandidateKeys}
                              onSelectionChange={(keys) => {
                                const candidateID = selectedKeyFrom(keys);
                                setConnectionResult(null);
                                setForm((current) => ({ ...current, managed_candidate_id: candidateID }));
                              }}
                            >
                              {connectionCandidates.map((candidate) => (
                                <SelectItem key={candidate.id} textValue={candidate.display_name} isDisabled={!isUsableConnectionCandidate(candidate)}>
                                  {candidate.display_name}
                                </SelectItem>
                              ))}
                            </Select>
                            <Button
                              isIconOnly
                              variant="flat"
                              aria-label="刷新连接候选"
                              onPress={() => void loadAll()}
                            >
                              <RefreshCw className="h-4 w-4" />
                            </Button>
                          </div>
                          <div className="grid grid-cols-1 gap-2 lg:grid-cols-2">
                            {connectionCandidates.map((candidate) => (
                              <button
                                key={candidate.id}
                                type="button"
                                disabled={!isUsableConnectionCandidate(candidate)}
                                className={`rounded-md border p-3 text-left text-sm transition ${
                                  form.managed_candidate_id === candidate.id
                                    ? 'border-primary bg-primary-50 dark:bg-primary-950/30'
                                    : isUsableConnectionCandidate(candidate)
                                      ? 'border-gray-200 hover:border-primary/60 dark:border-gray-700'
                                      : 'border-gray-200 opacity-75 dark:border-gray-700'
                                }`}
                                onClick={() => {
                                  if (!isUsableConnectionCandidate(candidate)) return;
                                  setConnectionResult(null);
                                  setForm({ ...form, managed_candidate_id: candidate.id });
                                }}
                              >
                                <div className="flex items-center justify-between gap-2">
                                  <span className="font-medium text-gray-800 dark:text-gray-100">{candidate.display_name}</span>
                                  <Chip size="sm" color={candidate.available ? 'success' : 'warning'} variant="flat">
                                    {candidate.available ? '可用' : '需处理'}
                                  </Chip>
                                </div>
                                <div className="mt-2 flex flex-wrap gap-2 text-xs text-gray-500 dark:text-gray-400">
                                  <span>{candidate.kind}</span>
                                  {candidate.local_endpoint && <span>{candidate.local_endpoint}</span>}
                                  {candidate.requires_temporary_entrypoint && <span>临时入口</span>}
                                  {candidate.outbound && <span>{candidate.outbound}</span>}
                                </div>
                                {candidate.unavailable_reason && (
                                  <p className="mt-2 text-xs text-warning-700 dark:text-warning-300">{candidate.unavailable_reason}</p>
                                )}
                              </button>
                            ))}
                            {connectionCandidates.length === 0 && (
                              <div className="rounded-md border border-dashed border-gray-300 p-4 text-sm text-gray-500 dark:border-gray-700">
                                暂无可用托管路线。可以先添加节点、代理链路或 mixed/socks 入站端口。
                              </div>
                            )}
                            {connectionCandidates.length > 0 && usableConnectionCandidates.length === 0 && (
                              <div className="rounded-md border border-dashed border-warning-300 p-4 text-sm text-warning-700 dark:border-warning-700 dark:text-warning-300">
                                当前没有可用托管路线。请启用代理链路、节点或 mixed/socks 入站后重试。
                              </div>
                            )}
                          </div>
                        </div>
                      )}

                      {form.connection_mode === 'custom_proxy' && (
                        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                          <Select
                            id="deployment-proxy-type"
                            name="proxy_type"
                            aria-label="Proxy type"
                            label="Proxy Type"
                            selectedKeys={[form.proxy_type]}
                            onSelectionChange={(keys) => {
                              setConnectionResult(null);
                              setForm((current) => ({ ...current, proxy_type: selectedKeyFrom(keys) || 'socks5' }));
                            }}
                          >
                            <SelectItem key="socks5">SOCKS5</SelectItem>
                            <SelectItem key="http_connect">HTTP CONNECT</SelectItem>
                          </Select>
                          <Input
                            id="deployment-proxy-host"
                            name="proxy_host"
                            label="Proxy Host"
                            value={form.proxy_host}
                            onValueChange={(value) => {
                              setConnectionResult(null);
                              setForm({ ...form, proxy_host: value });
                            }}
                          />
                          <Input
                            id="deployment-connection-proxy-port"
                            name="connection_proxy_port"
                            label="Proxy Port"
                            type="number"
                            min={1}
                            max={65535}
                            value={form.connection_proxy_port}
                            onValueChange={(value) => {
                              setConnectionResult(null);
                              setForm({ ...form, connection_proxy_port: value });
                            }}
                          />
                          <Input
                            id="deployment-proxy-username"
                            name="proxy_username"
                            label="Proxy User"
                            value={form.proxy_username}
                            onValueChange={(value) => {
                              setConnectionResult(null);
                              setForm({ ...form, proxy_username: value });
                            }}
                          />
                          <form className="contents" onSubmit={(event) => event.preventDefault()}>
                            <Input
                              id="deployment-proxy-password"
                              name="proxy_password"
                              label="Proxy Password"
                              type="password"
                              autoComplete="off"
                              value={form.proxy_password}
                              onValueChange={(value) => {
                                setConnectionResult(null);
                                setForm({ ...form, proxy_password: value });
                              }}
                            />
                          </form>
                        </div>
                      )}
                      <div className="flex flex-col gap-3 rounded-md bg-gray-50 p-3 dark:bg-gray-900/40">
                        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                          <span className="text-sm text-gray-600 dark:text-gray-300">当前测试会使用上面选择的连接路径</span>
                          <Button variant="flat" color="primary" startContent={<Terminal className="h-4 w-4" />} isLoading={testingConnection} onPress={() => void testConnection()}>
                            测试 SSH 连接
                          </Button>
                        </div>
                        {connectionResult && (
                          <div className={`rounded-md border p-3 text-sm ${connectionResult.ok ? 'border-success-200 bg-success-50 text-success-700 dark:border-success-900 dark:bg-success-950/30 dark:text-success-300' : 'border-danger-200 bg-danger-50 text-danger-700 dark:border-danger-900 dark:bg-danger-950/30 dark:text-danger-300'}`}>
                            <div className="flex flex-wrap items-center gap-2 font-medium">
                              {connectionResult.ok ? <CheckCircle2 className="h-4 w-4" /> : <AlertTriangle className="h-4 w-4" />}
                              <span>{connectionResult.message}</span>
                              <span className="text-xs opacity-75">{connectionResult.duration_ms} ms</span>
                            </div>
                            {connectionResult.remote && Object.keys(connectionResult.remote).length > 0 && (
                              <div className="mt-2 grid grid-cols-1 gap-2 text-xs md:grid-cols-2">
                                {Object.entries(connectionResult.remote).map(([key, value]) => (
                                  <div key={key} className="rounded bg-white/70 px-2 py-1 dark:bg-black/20">
                                    <span className="font-medium">{key}</span>: {value}
                                  </div>
                                ))}
                              </div>
                            )}
                          </div>
                        )}
                      </div>
                    </div>

                    <div className="flex items-center justify-between rounded-md border border-gray-200 p-4 dark:border-gray-700">
                      <span className="text-sm text-gray-700 dark:text-gray-200">Dry run</span>
	                      <Switch aria-label="Dry run" isSelected={form.dry_run} onValueChange={(value) => setForm({ ...form, dry_run: value })} />
                    </div>
                    <div className="flex items-center justify-between rounded-md border border-gray-200 p-4 dark:border-gray-700">
                      <span className="text-sm text-gray-700 dark:text-gray-200">保留远端运行目录</span>
	                      <Switch aria-label="保留远端运行目录" isSelected={form.debug_preserve_remote_run_dir} onValueChange={(value) => setForm({ ...form, debug_preserve_remote_run_dir: value })} />
                    </div>
                    <div className="flex justify-between gap-3">
                      <Button variant="flat" onPress={() => setWizardStep('target')}>上一步</Button>
                      <Button color="primary" onPress={() => setWizardStep('review')}>下一步</Button>
                    </div>
                  </div>
                </Tab>

                <Tab key="review" title="3. 确认">
                  <div className="space-y-5 pt-4">
                    <Table aria-label="Deployment request summary" hideHeader>
                      <TableHeader>
                        <TableColumn>字段</TableColumn>
                        <TableColumn>值</TableColumn>
                      </TableHeader>
                      <TableBody>
                        <TableRow key="target">
                          <TableCell>目标</TableCell>
                          <TableCell>{form.ssh_user || 'root'}@{form.ssh_host || '-'}:{form.ssh_port || '22'}</TableCell>
                        </TableRow>
                        <TableRow key="auth">
                          <TableCell>认证</TableCell>
                          <TableCell>{form.auth_method === 'private_key' ? 'Private key' : 'Password'}</TableCell>
                        </TableRow>
	                        <TableRow key="template-params">
	                          <TableCell>{isSecurityBasicTemplate ? '安全维护' : '节点'}</TableCell>
	                          <TableCell>
	                            {isSecurityBasicTemplate
	                              ? (form.security_install_base_packages ? `Install ${form.security_base_packages || 'base packages'}` : 'Inspect only')
	                              : `${form.node_name || '-'} / ${form.node_server || form.ssh_host || '-'}:${form.node_proxy_port || '443'}`}
	                          </TableCell>
	                        </TableRow>
	                        <TableRow key="connection">
	                          <TableCell>连接路径</TableCell>
	                          <TableCell>{form.connection_mode}</TableCell>
	                        </TableRow>
	                        <TableRow key="runtime">
	                          <TableCell>运行时</TableCell>
	                          <TableCell>{isProxyDeploymentTemplate ? (form.runtime_source === 'cache' ? `Local cache linux/${form.runtime_arch}` : 'Remote download') : 'Not required'}</TableCell>
	                        </TableRow>
                        <TableRow key="mode">
                          <TableCell>运行模式</TableCell>
                          <TableCell>{form.dry_run ? 'Dry run' : form.debug_preserve_remote_run_dir ? 'Real run / Preserve remote dir' : 'Real run'}</TableCell>
                        </TableRow>
                      </TableBody>
                    </Table>
                    <div className="flex justify-between gap-3">
                      <Button variant="flat" onPress={() => setWizardStep('node')}>上一步</Button>
                      <Button color="primary" startContent={<Play className="h-4 w-4" />} isLoading={submitting} onPress={() => void startDeployment()}>
                        启动部署
                      </Button>
                    </div>
                  </div>
                </Tab>
              </Tabs>
            </CardBody>
          </Card>
        </Tab>

        <Tab key="runs" title={<div className="flex items-center gap-2"><DownloadCloud className="h-4 w-4" /><span>运行记录</span>{runs.length > 0 && <Chip size="sm" variant="flat">{runs.length}</Chip>}</div>}>
          <Card className="border border-gray-200 shadow-sm dark:border-gray-700">
            <CardHeader className="flex flex-col items-start gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <div className="flex items-center gap-2">
                  <DownloadCloud className="h-5 w-5 text-primary" />
                  <span className="font-semibold">部署运行记录</span>
                </div>
                <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">选择一条记录可查看完整产物；运行中的任务每 2.5 秒自动刷新。</p>
              </div>
              {hasActiveRun && <Chip color="primary" startContent={<CircleDot className="h-3 w-3" />} variant="flat">自动刷新中</Chip>}
            </CardHeader>
            <CardBody className="pt-0">
              <Table
                aria-label="Deployment runs"
                classNames={{ wrapper: 'max-h-[620px]', table: 'min-w-[980px]', th: 'bg-gray-50 text-xs dark:bg-gray-800/80' }}
                selectionMode="single"
                selectedKeys={selectedRunKeys}
                onSelectionChange={(keys) => setSelectedRunId(String(Array.from(keys)[0] || ''))}
              >
	                <TableHeader>
	                  <TableColumn>状态</TableColumn>
	                  <TableColumn>目标</TableColumn>
	                  <TableColumn>模板</TableColumn>
	                  <TableColumn>执行信息</TableColumn>
	                  <TableColumn>产物</TableColumn>
	                  <TableColumn>时间</TableColumn>
	                  <TableColumn>操作</TableColumn>
                </TableHeader>
                <TableBody emptyContent="暂无部署记录">
                  {runs.map((run) => (
	                    <TableRow key={run.id}>
	                      <TableCell><Chip size="sm" color={statusColor[run.status] || 'default'} variant="flat">{statusLabel[run.status] || run.status}</Chip></TableCell>
	                      <TableCell>
                            <div className="min-w-[180px]">
                              <p className="font-medium text-gray-800 dark:text-gray-100">{run.ssh_user}@{run.ssh_host}</p>
                              <p className="text-xs text-gray-500 dark:text-gray-400">SSH :{run.ssh_port}</p>
                            </div>
                          </TableCell>
	                      <TableCell>
                            <div className="min-w-[150px]">
                              <p className="font-medium text-gray-800 dark:text-gray-100">{run.template_name}</p>
                              <p className="text-xs text-gray-500 dark:text-gray-400">{run.template_version}</p>
                            </div>
                          </TableCell>
	                      <TableCell>
                            <div className="flex min-w-[170px] flex-wrap gap-1.5">
                              <Chip size="sm" variant="flat" color={run.dry_run ? 'warning' : 'primary'}>{run.dry_run ? 'Dry run' : 'Real run'}</Chip>
                              <Chip size="sm" variant="bordered">{run.connection_mode}</Chip>
                            </div>
                          </TableCell>
	                      <TableCell>
                            <div className="min-w-[150px] space-y-1.5">
                              <p className="text-sm text-gray-700 dark:text-gray-200">{run.generated_node?.tag ? String(run.generated_node.tag) : '暂无节点'}</p>
                              <Chip size="sm" variant="flat" color={deploymentRunImportStatus(run).color}>{deploymentRunImportStatus(run).label}</Chip>
                            </div>
                          </TableCell>
	                      <TableCell>
                            <div className="min-w-[130px] text-xs text-gray-600 dark:text-gray-300">
                              <p>{new Date(run.created_at).toLocaleDateString()}</p>
                              <p className="mt-0.5 text-gray-400">{new Date(run.created_at).toLocaleTimeString()}</p>
                            </div>
                          </TableCell>
                      <TableCell>
                        <div className="flex min-w-[190px] gap-2">
	                          <Button size="sm" variant="flat" onPress={() => {
	                            setSelectedRunId(run.id);
	                            setSelectedArtifactKey('generated_node');
	                            setActiveTab('artifacts');
	                          }}>
	                            查看
	                          </Button>
	                          <Button size="sm" variant="flat" startContent={<Copy className="h-4 w-4" />} isDisabled={run.status !== 'success' || run.dry_run} onPress={() => void copyGeneratedNodeLink(run)}>
	                            复制链接
	                          </Button>
	                          {(run.status === 'running' || run.status === 'pending') && (
	                            <Button size="sm" color="danger" variant="flat" onPress={() => void cancelDeploymentRun(run)}>
	                              取消
                            </Button>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardBody>
          </Card>
        </Tab>

        <Tab key="artifacts" title={<div className="flex items-center gap-2"><Import className="h-4 w-4" /><span>节点与日志</span>{importableRunCount > 0 && <Chip size="sm" color="warning" variant="flat">{importableRunCount}</Chip>}</div>}>
          <Card className="border border-gray-200 shadow-sm dark:border-gray-700">
            <CardHeader className="flex flex-col items-start gap-2 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <div className="flex items-center gap-2">
                  <Import className="h-5 w-5 text-primary" />
                  <span className="font-semibold">生成节点与运行诊断</span>
                </div>
                <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">审阅部署结果、复制分享链接，并将成功产物导入手动节点。</p>
              </div>
              {selectedRun?.imported_node_id && <Chip color="success" size="sm">已导入 #{selectedRun.imported_node_id}</Chip>}
            </CardHeader>
	            <CardBody className="space-y-5">
	              {selectedRun ? (
	                <>
	                  <div className="flex flex-col gap-3 rounded-xl border border-gray-200 bg-gray-50 p-4 sm:flex-row sm:items-center sm:justify-between dark:border-gray-700 dark:bg-gray-800/50">
                        <div>
                          <div className="flex flex-wrap items-center gap-2">
                            <Chip size="sm" color={statusColor[selectedRun.status] || 'default'} variant="flat">{statusLabel[selectedRun.status] || selectedRun.status}</Chip>
                            <span className="font-semibold text-gray-800 dark:text-gray-100">{selectedRun.ssh_user}@{selectedRun.ssh_host}:{selectedRun.ssh_port}</span>
                          </div>
                          <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">{selectedRun.template_name} {selectedRun.template_version} · Run {selectedRun.id}</p>
                        </div>
                        <Button size="sm" variant="flat" startContent={<Clock3 className="h-4 w-4" />} onPress={() => setActiveTab('runs')}>返回运行记录</Button>
	                  </div>
	                  <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
	                    <div className="rounded-md border border-gray-200 p-3 dark:border-gray-700">
	                      <p className="text-xs text-gray-500 dark:text-gray-400">运行模式</p>
	                      <Chip size="sm" variant="flat" color={selectedRun.dry_run ? 'warning' : 'primary'}>{selectedRun.dry_run ? 'Dry run' : 'Real run'}</Chip>
	                    </div>
	                    <div className="rounded-md border border-gray-200 p-3 dark:border-gray-700">
	                      <p className="text-xs text-gray-500 dark:text-gray-400">导入状态</p>
	                      <Chip size="sm" variant="flat" color={selectedImportStatus.color}>{selectedImportStatus.label}</Chip>
	                    </div>
	                    <div className="rounded-md border border-gray-200 p-3 dark:border-gray-700">
	                      <p className="text-xs text-gray-500 dark:text-gray-400">外部可达性</p>
	                      <Chip size="sm" variant="flat" color={externalReachabilityStatus === 'success' ? 'success' : externalReachabilityStatus === 'failed' ? 'danger' : 'default'}>
	                        {externalReachabilityStatus || '未检查'}
	                      </Chip>
	                    </div>
	                    <div className="rounded-md border border-gray-200 p-3 dark:border-gray-700">
	                      <p className="text-xs text-gray-500 dark:text-gray-400">退出码</p>
	                      <Chip size="sm" variant="flat" color={selectedRun.exit_code === 0 ? 'success' : selectedRun.exit_code === undefined ? 'default' : 'danger'}>
	                        {selectedRun.exit_code ?? '未完成'}
	                      </Chip>
	                    </div>
	                  </div>

	                  <Tabs
	                    aria-label="部署产物详情"
	                    classNames={{
	                      tabList: 'w-full justify-start gap-2 rounded-xl bg-gray-100 p-1 dark:bg-gray-800',
	                      cursor: 'rounded-lg shadow-sm',
	                      tab: 'h-10 px-4',
	                      panel: 'px-0 pt-4',
	                    }}
	                    color="primary"
	                    variant="solid"
	                  >
	                    <Tab key="node" title={<div className="flex items-center gap-2"><Import className="h-4 w-4" /><span>节点产物</span></div>}>
	                      <div className="grid gap-4 xl:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
	                        <div className="space-y-4 rounded-xl border border-gray-200 p-4 dark:border-gray-700">
	                          <div>
	                            <p className="font-semibold text-gray-800 dark:text-gray-100">导入手动节点</p>
	                            <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">确认节点名称和来源后完成导入。</p>
	                          </div>
	                          <div className="grid gap-3 sm:grid-cols-2">
	                            <Input id="deployment-import-tag" name="deployment_import_tag" label="节点名称" value={importForm.tag} isDisabled={Boolean(selectedRun.imported_node_id)} onValueChange={(value) => setImportForm({ ...importForm, tag: value })} />
	                            <Input id="deployment-import-source-name" name="deployment_import_source_name" label="来源名称" value={importForm.sourceName} isDisabled={Boolean(selectedRun.imported_node_id)} onValueChange={(value) => setImportForm({ ...importForm, sourceName: value })} />
	                          </div>
	                          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
	                            <Switch isSelected={importForm.enabled} isDisabled={Boolean(selectedRun.imported_node_id)} onValueChange={(value) => setImportForm({ ...importForm, enabled: value })}>导入后启用</Switch>
	                            <Button color="primary" startContent={<Import className="h-4 w-4" />} isDisabled={!canImportSelectedRun} isLoading={importing} onPress={() => void importGeneratedNode()}>
	                              {selectedImportActionLabel}
	                            </Button>
	                          </div>
	                          {generatedNodeReview?.share_link && (
	                            <div className="grid gap-2 sm:grid-cols-[1fr_auto]">
	                              <Input id="deployment-generated-node-link" name="generated_node_link" label="节点链接" value={generatedNodeReview.share_link} readOnly />
	                              <Button className="sm:self-end" variant="flat" startContent={<Copy className="h-4 w-4" />} onPress={() => void copyGeneratedNodeLink()}>复制</Button>
	                            </div>
	                          )}
	                          <div className="border-t border-gray-200 pt-3 dark:border-gray-700">
	                            <Switch
	                              isSelected={importForm.advancedProtocolEdit}
	                              isDisabled={Boolean(selectedRun.imported_node_id)}
	                              onValueChange={(value) => setImportForm({ ...importForm, advancedProtocolEdit: value })}
	                            >
	                              高级协议编辑
	                            </Switch>
	                            {importForm.advancedProtocolEdit && (
	                              <Textarea
	                                className="mt-3"
	                                id="deployment-generated-node-overrides"
	                                name="generated_node_overrides"
	                                label="协议覆盖参数"
	                                minRows={4}
	                                placeholder='{"server":"override.example.com","server_port":8443,"extra":{"flow":"xtls-rprx-vision"}}'
	                                value={importForm.protocolOverrides}
	                                isDisabled={Boolean(selectedRun.imported_node_id)}
	                                onValueChange={(value) => setImportForm({ ...importForm, protocolOverrides: value })}
	                              />
	                            )}
	                          </div>
	                        </div>

	                        <div className="space-y-3 rounded-xl border border-gray-200 p-4 dark:border-gray-700">
	                          <div className="flex items-center justify-between gap-3">
	                            <div>
	                              <p className="font-semibold text-gray-800 dark:text-gray-100">生成节点配置</p>
	                              <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">{generatedNodeReview ? '完整节点仅在详情接口返回' : '历史列表保持脱敏'}</p>
	                            </div>
	                            <Chip size="sm" color={hasGeneratedNodeForReview ? 'success' : 'default'} variant="flat">{hasGeneratedNodeForReview ? '已生成' : '无产物'}</Chip>
	                          </div>
	                          <Textarea label={generatedNodeArtifact?.label || 'Generated Node'} minRows={14} value={generatedNodeArtifact?.value || '{}'} readOnly />
	                        </div>
	                      </div>
	                    </Tab>

	                    <Tab key="diagnostics" title={<div className="flex items-center gap-2"><ShieldCheck className="h-4 w-4" /><span>运行诊断</span><Chip size="sm" variant="flat">{diagnosticRows.length}</Chip></div>}>
	                      <div className="grid gap-4 xl:grid-cols-[minmax(360px,0.8fr)_minmax(0,1.2fr)]">
	                        <div className="rounded-xl border border-gray-200 p-4 dark:border-gray-700">
	                          <Table
	                            aria-label="Deployment diagnostic artifacts"
	                            selectionMode="single"
	                            selectedKeys={selectedArtifact ? [selectedArtifact.key] : []}
	                            onSelectionChange={(keys) => setSelectedArtifactKey(String(Array.from(keys)[0] || 'run_summary'))}
	                          >
	                            <TableHeader>
	                              <TableColumn>项目</TableColumn>
	                              <TableColumn>状态</TableColumn>
	                            </TableHeader>
	                            <TableBody emptyContent="暂无诊断内容">
	                              {diagnosticRows.map((row) => (
	                                <TableRow key={row.key}>
	                                  <TableCell><div><p className="font-medium">{row.label}</p><p className="mt-1 max-w-[260px] truncate text-xs text-gray-400">{previewText(row.value)}</p></div></TableCell>
	                                  <TableCell><Chip size="sm" color={row.status === 'ready' ? 'success' : 'default'} variant="flat">{row.status}</Chip></TableCell>
	                                </TableRow>
	                              ))}
	                            </TableBody>
	                          </Table>
	                        </div>
	                        <div className="rounded-xl border border-gray-200 p-4 dark:border-gray-700">
	                          {selectedArtifact && !['generated_node', 'generated_node_link', 'stdout', 'stderr'].includes(selectedArtifact.key) ? (
	                            <Textarea label={selectedArtifact.label} minRows={14} value={selectedArtifact.value} readOnly />
	                          ) : (
	                            <div className="flex min-h-64 items-center justify-center text-sm text-gray-400">选择左侧诊断项目查看详情</div>
	                          )}
	                        </div>
	                      </div>
	                    </Tab>

	                    <Tab key="logs" title={<div className="flex items-center gap-2"><Terminal className="h-4 w-4" /><span>原始日志</span></div>}>
	                      <div className="rounded-xl border border-gray-200 p-4 dark:border-gray-700">
	                        <Tabs variant="underlined" aria-label="Deployment logs">
	                          <Tab key="stdout" title="标准输出">
	                            <Textarea className="mt-3" label="Stdout" minRows={16} value={stdoutArtifact?.value || ''} readOnly />
	                          </Tab>
	                          <Tab key="stderr" title="错误输出">
	                            <Textarea className="mt-3" label="Stderr" minRows={16} value={stderrArtifact?.value || ''} readOnly />
	                          </Tab>
	                        </Tabs>
	                      </div>
	                    </Tab>
	                  </Tabs>
	                </>
	              ) : (
                <div className="rounded-md border border-dashed border-gray-300 p-8 text-center text-sm text-gray-500 dark:border-gray-700">
                  暂无部署记录
                </div>
              )}
            </CardBody>
          </Card>
        </Tab>
      </Tabs>
    </div>
  );
}
