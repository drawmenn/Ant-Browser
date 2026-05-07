export interface UsernameScanGeneratorOptions {
  sourceText: string
  targetLength: number
  allowDigits: boolean
  maxResults: number
}

export interface UsernameScanStartRequest {
  seeds: string[]
  candidates: string[]
  provider: 'mock' | 'browser' | string
  browser: UsernameScanBrowserOptions
  intervalMs: number
  limit: number
}

export interface UsernameScanBrowserOptions {
  profileId: string
  url: string
  inputSelector: string
  submitSelector: string
  resultSelector: string
  availableText: string
  takenText: string
  holdText: string
  waitAfterSubmitMs: number
  timeoutMs: number
}

export interface UsernameScanStats {
  checked: number
  hit: number
  taken: number
  hold: number
  error: number
  rate: number
}

export interface UsernameScanHistory {
  checked: number[]
  hit: number[]
  rate: number[]
}

export interface UsernameScanResult {
  name: string
  status: 'available' | 'taken' | 'hold' | 'error' | string
  message: string
  checkedAt: string
}

export interface UsernameScanLogEntry {
  level: 'info' | 'success' | 'warning' | 'error' | string
  message: string
  params: Record<string, unknown>
  timestamp: string
}

export interface UsernameScanSnapshot {
  running: boolean
  paused: boolean
  provider: string
  queueSize: number
  nextIndex: number
  activeName: string
  stats: UsernameScanStats
  history: UsernameScanHistory
  results: UsernameScanResult[]
  logs: UsernameScanLogEntry[]
}

export const emptyUsernameScanSnapshot: UsernameScanSnapshot = {
  running: false,
  paused: false,
  provider: 'mock',
  queueSize: 0,
  nextIndex: 0,
  activeName: '',
  stats: {
    checked: 0,
    hit: 0,
    taken: 0,
    hold: 0,
    error: 0,
    rate: 0,
  },
  history: {
    checked: [],
    hit: [],
    rate: [],
  },
  results: [],
  logs: [],
}
