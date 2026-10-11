const LOOPBACK_HOST = '127.0.0.1';
const MASKED_SECRET = '••••••';

export function resolveZashboardHost(host?: string) {
  const rawHost = (host || (typeof window !== 'undefined' ? window.location.hostname : '')).trim();
  if (!rawHost || rawHost === 'localhost' || rawHost === '::1') {
    return LOOPBACK_HOST;
  }

  return rawHost;
}

// buildZashboardPanelUrl opens the embedded panel without a setup query.
// The Clash API secret must not be placed in the navigation URL: the hash is
// still stored in history and session restore, and this origin cannot clear it.
export function buildZashboardPanelUrl(port: number, host?: string) {
  const zashboardHost = resolveZashboardHost(host);
  return `http://${zashboardHost}:${port}/ui/`;
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
