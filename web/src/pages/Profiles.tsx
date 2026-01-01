import { useEffect, useState, useRef } from 'react';
import { Card, CardBody, Button, Chip, Modal, ModalContent, ModalHeader, ModalBody, ModalFooter, Input, Textarea, useDisclosure } from '@nextui-org/react';
import { Plus, Layers, Download, Upload, Trash2, Pencil, Check, RefreshCw } from 'lucide-react';
import { profileApi } from '../api';
import { toast } from '../components/Toast';

// Profile 类型
interface Profile {
  id: string;
  name: string;
  description: string;
  created_at: string;
  updated_at: string;
  is_active: boolean;
}

export default function Profiles() {
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [loading, setLoading] = useState(true);
  const [activating, setActivating] = useState<string | null>(null);

  // 创建/编辑 Modal
  const { isOpen, onOpen, onClose } = useDisclosure();
  const [editingProfile, setEditingProfile] = useState<Profile | null>(null);
  const [formData, setFormData] = useState({ name: '', description: '' });
  const [submitting, setSubmitting] = useState(false);

  // 导入文件输入
  const importInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    fetchProfiles();
  }, []);

  const fetchProfiles = async () => {
    try {
      const res = await profileApi.getAll();
      setProfiles(res.data.data || []);
    } catch (error: any) {
      toast.error('获取配置方案列表失败');
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = () => {
    setEditingProfile(null);
    setFormData({ name: '', description: '' });
    onOpen();
  };

  const handleEdit = (profile: Profile) => {
    setEditingProfile(profile);
    setFormData({ name: profile.name, description: profile.description });
    onOpen();
  };

  const handleSubmit = async () => {
    if (!formData.name.trim()) {
      toast.error('请输入方案名称');
      return;
    }

    setSubmitting(true);
    try {
      if (editingProfile) {
        await profileApi.update(editingProfile.id, formData);
        toast.success('方案已更新');
      } else {
        await profileApi.create(formData.name, formData.description);
        toast.success('方案已创建');
      }
      onClose();
      fetchProfiles();
    } catch (error: any) {
      toast.error(error.response?.data?.error || '操作失败');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async (profile: Profile) => {
    if (profile.is_active) {
      toast.error('不能删除当前激活的方案');
      return;
    }

    if (!confirm(`确定要删除方案 "${profile.name}" 吗？`)) {
      return;
    }

    try {
      await profileApi.delete(profile.id);
      toast.success('方案已删除');
      fetchProfiles();
    } catch (error: any) {
      toast.error(error.response?.data?.error || '删除失败');
    }
  };

  const handleActivate = async (profile: Profile) => {
    if (profile.is_active) {
      return;
    }

    setActivating(profile.id);
    try {
      await profileApi.activate(profile.id);
      toast.success(`已切换到方案: ${profile.name}`);
      fetchProfiles();
    } catch (error: any) {
      toast.error(error.response?.data?.error || '切换失败');
    } finally {
      setActivating(null);
    }
  };

  const handleSnapshot = async (profile: Profile) => {
    try {
      await profileApi.snapshot(profile.id);
      toast.success('快照已更新');
      fetchProfiles();
    } catch (error: any) {
      toast.error(error.response?.data?.error || '更新快照失败');
    }
  };

  const handleExport = (profile: Profile) => {
    window.open(profileApi.exportUrl(profile.id), '_blank');
    toast.success('正在下载方案...');
  };

  const handleImportClick = () => {
    importInputRef.current?.click();
  };

  const handleImportFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    if (!file.name.endsWith('.json')) {
      toast.error('请选择 JSON 格式的文件');
      e.target.value = '';
      return;
    }

    try {
      await profileApi.import(file);
      toast.success('方案导入成功');
      fetchProfiles();
    } catch (error: any) {
      toast.error(error.response?.data?.error || '导入失败');
    } finally {
      e.target.value = '';
    }
  };

  const formatDate = (dateStr: string) => {
    return new Date(dateStr).toLocaleString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    });
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
          <h1 className="text-2xl font-bold text-gray-800 dark:text-white">配置方案</h1>
          <p className="text-sm text-gray-500 mt-1">
            创建和管理不同的配置方案，快速切换订阅、规则和设置
          </p>
        </div>
        <div className="flex gap-2">
          <Button
            variant="flat"
            startContent={<Upload className="w-4 h-4" />}
            onPress={handleImportClick}
          >
            导入
          </Button>
          <Button
            color="primary"
            startContent={<Plus className="w-4 h-4" />}
            onPress={handleCreate}
          >
            新建方案
          </Button>
        </div>
      </div>

      {/* 隐藏的文件输入 */}
      <input
        ref={importInputRef}
        type="file"
        accept=".json"
        className="hidden"
        onChange={handleImportFile}
      />

      {/* Profile 列表 */}
      {profiles.length === 0 ? (
        <Card>
          <CardBody className="text-center py-12">
            <Layers className="w-12 h-12 mx-auto text-gray-400 mb-4" />
            <p className="text-gray-500 mb-4">暂无配置方案</p>
            <Button color="primary" onPress={handleCreate}>
              创建第一个方案
            </Button>
          </CardBody>
        </Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {profiles.map((profile) => (
            <Card
              key={profile.id}
              className={`transition-all ${
                profile.is_active
                  ? 'border-2 border-primary shadow-lg'
                  : 'hover:shadow-md'
              }`}
            >
              <CardBody className="p-4">
                <div className="flex justify-between items-start mb-2">
                  <div className="flex-1 min-w-0">
                    <h3 className="font-semibold text-lg truncate">{profile.name}</h3>
                    {profile.description && (
                      <p className="text-sm text-gray-500 mt-1 line-clamp-2">
                        {profile.description}
                      </p>
                    )}
                  </div>
                  {profile.is_active && (
                    <Chip color="primary" size="sm" className="ml-2 shrink-0">
                      当前
                    </Chip>
                  )}
                </div>

                <p className="text-xs text-gray-400 mb-4">
                  更新于 {formatDate(profile.updated_at)}
                </p>

                <div className="flex flex-wrap gap-2">
                  {!profile.is_active && (
                    <Button
                      size="sm"
                      color="primary"
                      startContent={<Check className="w-3 h-3" />}
                      onPress={() => handleActivate(profile)}
                      isLoading={activating === profile.id}
                    >
                      切换
                    </Button>
                  )}
                  <Button
                    size="sm"
                    variant="flat"
                    startContent={<RefreshCw className="w-3 h-3" />}
                    onPress={() => handleSnapshot(profile)}
                    title="更新为当前配置"
                  >
                    更新快照
                  </Button>
                  <Button
                    size="sm"
                    variant="flat"
                    startContent={<Download className="w-3 h-3" />}
                    onPress={() => handleExport(profile)}
                  >
                    导出
                  </Button>
                  <Button
                    size="sm"
                    variant="flat"
                    isIconOnly
                    onPress={() => handleEdit(profile)}
                  >
                    <Pencil className="w-3 h-3" />
                  </Button>
                  <Button
                    size="sm"
                    variant="flat"
                    color="danger"
                    isIconOnly
                    onPress={() => handleDelete(profile)}
                    isDisabled={profile.is_active}
                  >
                    <Trash2 className="w-3 h-3" />
                  </Button>
                </div>
              </CardBody>
            </Card>
          ))}
        </div>
      )}

      {/* 创建/编辑 Modal */}
      <Modal isOpen={isOpen} onClose={onClose}>
        <ModalContent>
          <ModalHeader>
            {editingProfile ? '编辑方案' : '新建方案'}
          </ModalHeader>
          <ModalBody className="gap-4">
            <Input
              label="方案名称"
              placeholder="例如：工作、旅行、默认"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
            />
            <Textarea
              label="描述（可选）"
              placeholder="方案的用途说明"
              value={formData.description}
              onChange={(e) => setFormData({ ...formData, description: e.target.value })}
              minRows={2}
            />
            {!editingProfile && (
              <p className="text-sm text-gray-500">
                新建方案将基于当前配置创建快照
              </p>
            )}
          </ModalBody>
          <ModalFooter>
            <Button variant="flat" onPress={onClose}>
              取消
            </Button>
            <Button
              color="primary"
              onPress={handleSubmit}
              isLoading={submitting}
              isDisabled={!formData.name.trim()}
            >
              {editingProfile ? '保存' : '创建'}
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </div>
  );
}
