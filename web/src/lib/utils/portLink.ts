// Utilities for resolving web URLs for container ports and custom application labels.

export interface PortLinkSettings {
  mode: 'auto' | 'custom' | 'localhost';
  customHost: string;
  protocol: 'auto' | 'http' | 'https';
}

export const DEFAULT_PORT_LINK_SETTINGS: PortLinkSettings = {
  mode: 'auto',
  customHost: '',
  protocol: 'auto',
};

export function getPortLinkSettings(): PortLinkSettings {
  if (typeof window === 'undefined') {
    return DEFAULT_PORT_LINK_SETTINGS;
  }
  const mode = (localStorage.getItem('pdswarm_port_host_mode') || 'auto') as 'auto' | 'custom' | 'localhost';
  const customHost = localStorage.getItem('pdswarm_port_custom_host') || '';
  const protocol = (localStorage.getItem('pdswarm_port_protocol') || 'auto') as 'auto' | 'http' | 'https';
  return { mode, customHost, protocol };
}

export function savePortLinkSettings(settings: Partial<PortLinkSettings>): void {
  if (typeof window === 'undefined') return;
  if (settings.mode !== undefined) {
    localStorage.setItem('pdswarm_port_host_mode', settings.mode);
  }
  if (settings.customHost !== undefined) {
    localStorage.setItem('pdswarm_port_custom_host', settings.customHost.trim());
  }
  if (settings.protocol !== undefined) {
    localStorage.setItem('pdswarm_port_protocol', settings.protocol);
  }
}

/**
 * Returns the resolved hostname or IP to use for opening public ports.
 * Priority:
 * 1. Container label `pdswarm.host` or `pdswarm.ip`
 * 2. Settings: customHost (if mode === 'custom')
 * 3. Settings: localhost (if mode === 'localhost')
 * 4. Settings: auto -> window.location.hostname (falling back to 'localhost')
 */
export function resolveContainerHost(labels?: Record<string, string>): string {
  // 1. Container-specific override via label (highest priority)
  if (labels) {
    const labelHost = labels['pdswarm.host'] || labels['pdswarm.ip'];
    if (labelHost && labelHost.trim()) {
      return labelHost.trim();
    }
  }

  // 2. Global IP / Host from settings
  const settings = getPortLinkSettings();
  if (settings.customHost && settings.customHost.trim()) {
    return settings.customHost.trim();
  }

  // 3. Fallback: current browser hostname (or localhost)
  if (typeof window !== 'undefined' && window.location.hostname) {
    return window.location.hostname;
  }
  return 'localhost';
}

/**
 * Returns the resolved protocol (e.g. "http" or "https") without trailing colon.
 */
export function resolveContainerProtocol(labels?: Record<string, string>): string {
  if (labels) {
    const labelProto = labels['pdswarm.protocol'] || labels['pdswarm.proto'];
    if (labelProto && (labelProto.toLowerCase() === 'https' || labelProto.toLowerCase() === 'http')) {
      return labelProto.toLowerCase();
    }
  }

  const settings = getPortLinkSettings();
  if (settings.protocol === 'http') return 'http';
  if (settings.protocol === 'https') return 'https';

  if (typeof window !== 'undefined' && window.location.protocol) {
    return window.location.protocol.replace(':', '');
  }
  return 'http';
}

/**
 * Resolves a custom app URL if defined via container labels:
 * - pdswarm.url: e.g. "https://photos.mydomain.com", "http://192.168.1.100:2283", "photos.local"
 * - pdswarm.link / pdswarm.open.url
 * - traefik.http.routers.<name>.rule: extracts Host(`subdomain.domain.com`)
 */
export function getContainerCustomUrl(labels?: Record<string, string>): string | null {
  if (!labels) return null;

  for (const key of ['pdswarm.url', 'pdswarm.link', 'pdswarm.open.url']) {
    if (labels[key]) {
      let val = labels[key].trim();
      if (!val) continue;
      if (!val.startsWith('http://') && !val.startsWith('https://')) {
        val = `http://${val}`;
      }
      return val;
    }
  }

  // Check Traefik router rules
  for (const [key, val] of Object.entries(labels)) {
    if (key.startsWith('traefik.http.routers.') && key.endsWith('.rule')) {
      const match = val.match(/Host\(`([^`]+)`\)/i) || val.match(/Host\("([^"]+)"\)/i) || val.match(/Host\(([^)]+)\)/i);
      if (match && match[1]) {
        return `https://${match[1].trim()}`;
      }
    }
  }

  return null;
}

/**
 * Returns the complete URL to open a specific exposed public port.
 */
export function getPortOpenUrl(publicPort: number | string, labels?: Record<string, string>): string {
  const host = resolveContainerHost(labels);
  const proto = resolveContainerProtocol(labels);
  return `${proto}://${host}:${publicPort}`;
}

/**
 * Formats a URL for display in a compact badge (e.g. "photos.local" or "my-app:8080").
 */
export function formatUrlBadge(url: string): string {
  try {
    const parsed = new URL(url);
    const hostWithPort = parsed.port ? `${parsed.hostname}:${parsed.port}` : parsed.hostname;
    const path = parsed.pathname && parsed.pathname !== '/' ? parsed.pathname : '';
    return `${hostWithPort}${path}`;
  } catch {
    return url.replace(/^https?:\/\//, '');
  }
}
