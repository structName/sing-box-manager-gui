import { useState } from 'react';
import { Button, Card, CardBody, CardHeader, Input } from '@nextui-org/react';
import { ShieldCheck } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { toast } from '../components/Toast';
import { useAuthStore } from '../store/authStore';

const MIN_PASSWORD_LENGTH = 8;

function getApiErrorMessage(error: unknown, fallback: string) {
  const response = (error as { response?: { data?: { error?: string } } })?.response;
  return response?.data?.error || fallback;
}

export default function Setup() {
  const navigate = useNavigate();
  const bootstrap = useAuthStore((state) => state.bootstrap);
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async () => {
    if (password.trim().length < MIN_PASSWORD_LENGTH) {
      toast.error(`密码长度至少为 ${MIN_PASSWORD_LENGTH} 位`);
      return;
    }
    if (password !== confirmPassword) {
      toast.error('两次输入的密码不一致');
      return;
    }

    try {
      setSubmitting(true);
      await bootstrap(password, confirmPassword);
      navigate('/', { replace: true });
    } catch (error) {
      toast.error(getApiErrorMessage(error, '初始化管理员密码失败'));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-slate-100 via-white to-emerald-50 px-4 dark:from-slate-950 dark:via-slate-900 dark:to-slate-950">
      <Card className="w-full max-w-lg border border-slate-200/70 bg-white/95 shadow-xl dark:border-slate-800 dark:bg-slate-900/95">
        <CardHeader className="flex flex-col items-start gap-3 px-6 pt-6">
          <div className="flex items-center gap-3">
            <div className="rounded-2xl bg-emerald-50 p-3 text-emerald-600 dark:bg-emerald-950/30 dark:text-emerald-300">
              <ShieldCheck className="h-6 w-6" />
            </div>
            <div>
              <h1 className="text-xl font-semibold text-slate-900 dark:text-white">初始化管理员密码</h1>
              <p className="text-sm text-slate-500 dark:text-slate-400">首次访问需要先设置管理面板密码，完成后将直接进入工作区。</p>
            </div>
          </div>
        </CardHeader>
        <CardBody className="space-y-5 px-6 pb-6">
          <Input
            type="password"
            label="管理员密码"
            placeholder={`至少 ${MIN_PASSWORD_LENGTH} 位`}
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
          <Input
            type="password"
            label="确认密码"
            placeholder="再次输入管理员密码"
            value={confirmPassword}
            onChange={(event) => setConfirmPassword(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter' && !submitting) {
                void handleSubmit();
              }
            }}
          />
          <Button color="primary" isLoading={submitting} onPress={() => void handleSubmit()}>
            完成初始化
          </Button>
        </CardBody>
      </Card>
    </div>
  );
}
