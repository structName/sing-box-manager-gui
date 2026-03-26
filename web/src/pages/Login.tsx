import { useMemo, useState } from 'react';
import { Button, Card, CardBody, CardHeader, Input } from '@nextui-org/react';
import { Activity } from 'lucide-react';
import { useLocation, useNavigate } from 'react-router-dom';
import { toast } from '../components/Toast';
import { useAuthStore } from '../store/authStore';

function getApiErrorMessage(error: unknown, fallback: string) {
  const response = (error as { response?: { data?: { error?: string } } })?.response;
  return response?.data?.error || fallback;
}

export default function Login() {
  const navigate = useNavigate();
  const location = useLocation();
  const login = useAuthStore((state) => state.login);
  const [password, setPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const redirectPath = useMemo(() => {
    const nextPath = (location.state as { from?: { pathname?: string } } | null)?.from?.pathname;
    return nextPath || '/';
  }, [location.state]);

  const handleSubmit = async () => {
    if (!password.trim()) {
      toast.error('请输入管理员密码');
      return;
    }

    try {
      setSubmitting(true);
      await login(password);
      navigate(redirectPath, { replace: true });
    } catch (error) {
      toast.error(getApiErrorMessage(error, '登录失败'));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-slate-100 via-white to-cyan-50 px-4 dark:from-slate-950 dark:via-slate-900 dark:to-slate-950">
      <Card className="w-full max-w-md border border-slate-200/70 bg-white/95 shadow-xl dark:border-slate-800 dark:bg-slate-900/95">
        <CardHeader className="flex flex-col items-start gap-3 px-6 pt-6">
          <div className="flex items-center gap-3">
            <div className="rounded-2xl bg-cyan-50 p-3 text-cyan-600 dark:bg-cyan-950/30 dark:text-cyan-300">
              <Activity className="h-6 w-6" />
            </div>
            <div>
              <h1 className="text-xl font-semibold text-slate-900 dark:text-white">管理员登录</h1>
              <p className="text-sm text-slate-500 dark:text-slate-400">输入管理面板密码以继续访问工作区。</p>
            </div>
          </div>
        </CardHeader>
        <CardBody className="space-y-5 px-6 pb-6">
          <Input
            type="password"
            label="管理员密码"
            placeholder="请输入已设置的密码"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter' && !submitting) {
                void handleSubmit();
              }
            }}
          />
          <Button color="primary" isLoading={submitting} onPress={() => void handleSubmit()}>
            登录
          </Button>
        </CardBody>
      </Card>
    </div>
  );
}
