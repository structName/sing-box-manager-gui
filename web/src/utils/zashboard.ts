const LOOPBACK_HOST = '127.0.0.1';
const MASKED_SECRET = '••••••';

export function buildZashboardSetupUrl(port: number, secret?: string) {
  const params = new URLSearchParams({
    hostname: LOOPBACK_HOST,
    port: String(port),
  });

  if (secret) {
    params.set('secret', secret);
  }

  return `http://${LOOPBACK_HOST}:${port}/ui/#/setup?${params.toString()}`;
}

export function buildZashboardPanelUrl(port: number, secret?: string) {
  return buildZashboardSetupUrl(port, secret);
}

export function buildZashboardSetupPreviewUrl(port: number, secret?: string) {
  const params = new URLSearchParams({
    hostname: LOOPBACK_HOST,
    port: String(port),
  });

  if (secret) {
    params.set('secret', MASKED_SECRET);
  }

  return `http://${LOOPBACK_HOST}:${port}/ui/#/setup?${params.toString()}`;
}
