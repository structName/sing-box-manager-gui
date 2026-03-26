import { useEffect, useState } from 'react';
import { toast } from '../../components/Toast';
import { useStore } from '../../store';
import type { Settings as SettingsType } from '../../store';

function getApiErrorMessage(error: unknown, fallback: string) {
  const resp = (error as { response?: { data?: { error?: string } } })?.response;
  return resp?.data?.error || fallback;
}

export function useSettingsForm() {
  const { settings, fetchSettings, updateSettings } = useStore();
  const [formData, setFormData] = useState<SettingsType | null>(null);

  useEffect(() => {
    fetchSettings();
  }, [fetchSettings]);

  useEffect(() => {
    if (settings) {
      setFormData(settings);
    }
  }, [settings]);

  const handleSave = async () => {
    if (!formData) {
      return;
    }

    try {
      await updateSettings(formData);
      toast.success('设置已保存');
    } catch (error: unknown) {
      toast.error(getApiErrorMessage(error, '保存设置失败'));
    }
  };

  return { formData, setFormData, handleSave };
}
