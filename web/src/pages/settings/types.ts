export interface KernelInfo {
  installed: boolean;
  version: string;
  path: string;
  os: string;
  arch: string;
}

export interface DownloadProgress {
  status: 'idle' | 'preparing' | 'downloading' | 'extracting' | 'installing' | 'completed' | 'error';
  progress: number;
  message: string;
  downloaded?: number;
  total?: number;
}

export interface GithubRelease {
  tag_name: string;
  name: string;
}

export interface DaemonStatus {
  installed: boolean;
  running: boolean;
  supported: boolean;
}

export interface HostFormState {
  domain: string;
  enabled: boolean;
}
