const LOOPBACK_HOST = '127.0.0.1';
const MASKED_SECRET = '••••••';

export function resolveZashboardHost(host?: string) {
  const rawHost = (host || (typeof window !== 'undefined' ? window.location.hostname : '')).trim();
  if (!rawHost || rawHost === 'localhost' || rawHost === '::1') {
    return LOOPBACK_HOST;
  }

  return rawHost;
}

export function buildZashboardSetupUrl(port: number, secret?: string, host?: string) {
  const zashboardHost = resolveZashboardHost(host);
  const params = new URLSearchParams({
    hostname: zashboardHost,
    port: String(port),
  });

  if (secret) {
    params.set('secret', secret);
  }

  return `http://${zashboardHost}:${port}/ui/#/setup?${params.toString()}`;
}

export function buildZashboardPanelUrl(port: number, secret?: string, host?: string) {
  return buildZashboardSetupUrl(port, secret, host);
}

export function buildZashboardSetupPreviewUrl(port: number, secret?: string, host?: string) {
  const zashboardHost = resolveZashboardHost(host);
  const params = new URLSearchParams({
    hostname: zashboardHost,
    port: String(port),
  });

  if (secret) {
    params.set('secret', MASKED_SECRET);
  }

  return `http://${zashboardHost}:${port}/ui/#/setup?${params.toString()}`;
}
