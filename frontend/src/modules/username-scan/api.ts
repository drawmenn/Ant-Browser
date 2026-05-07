import type {
  UsernameScanGeneratorOptions,
  UsernameScanLogEntry,
  UsernameScanSnapshot,
  UsernameScanStartRequest,
} from './types'
import { emptyUsernameScanSnapshot } from './types'

const getBindings = async () => {
  try {
    return await import('../../wailsjs/go/main/App')
  } catch {
    return null
  }
}

const getGoApp = () => (window as any).go?.main?.App

export async function generateUsernameCandidates(options: UsernameScanGeneratorOptions): Promise<string[]> {
  const bindings: any = await getBindings()
  if (bindings?.UsernameScanGenerate) {
    return (await bindings.UsernameScanGenerate(options)) || []
  }
  const goApp = getGoApp()
  if (goApp?.UsernameScanGenerate) {
    return (await goApp.UsernameScanGenerate(options)) || []
  }
  return mockGenerate(options)
}

export async function startUsernameScan(request: UsernameScanStartRequest): Promise<UsernameScanSnapshot> {
  const bindings: any = await getBindings()
  if (bindings?.UsernameScanStart) {
    return normalizeSnapshot(await bindings.UsernameScanStart(request))
  }
  const goApp = getGoApp()
  if (goApp?.UsernameScanStart) {
    return normalizeSnapshot(await goApp.UsernameScanStart(request))
  }
  return emptyUsernameScanSnapshot
}

export async function pauseUsernameScan(): Promise<UsernameScanSnapshot> {
  const bindings: any = await getBindings()
  if (bindings?.UsernameScanPause) {
    return normalizeSnapshot(await bindings.UsernameScanPause())
  }
  const goApp = getGoApp()
  if (goApp?.UsernameScanPause) {
    return normalizeSnapshot(await goApp.UsernameScanPause())
  }
  return emptyUsernameScanSnapshot
}

export async function stopUsernameScan(): Promise<UsernameScanSnapshot> {
  const bindings: any = await getBindings()
  if (bindings?.UsernameScanStop) {
    return normalizeSnapshot(await bindings.UsernameScanStop())
  }
  const goApp = getGoApp()
  if (goApp?.UsernameScanStop) {
    return normalizeSnapshot(await goApp.UsernameScanStop())
  }
  return emptyUsernameScanSnapshot
}

export async function fetchUsernameScanSnapshot(): Promise<UsernameScanSnapshot> {
  const bindings: any = await getBindings()
  if (bindings?.UsernameScanSnapshot) {
    return normalizeSnapshot(await bindings.UsernameScanSnapshot())
  }
  const goApp = getGoApp()
  if (goApp?.UsernameScanSnapshot) {
    return normalizeSnapshot(await goApp.UsernameScanSnapshot())
  }
  return emptyUsernameScanSnapshot
}

export function normalizeSnapshot(payload: any): UsernameScanSnapshot {
  if (!payload) return emptyUsernameScanSnapshot
  return {
    ...emptyUsernameScanSnapshot,
    ...payload,
    stats: {
      ...emptyUsernameScanSnapshot.stats,
      ...(payload.stats || {}),
    },
    history: {
      checked: payload.history?.checked || [],
      hit: payload.history?.hit || [],
      rate: payload.history?.rate || [],
    },
    results: payload.results || [],
    logs: payload.logs || [],
  }
}

export function normalizeLogEntry(payload: any): UsernameScanLogEntry {
  return {
    level: String(payload?.level || 'info'),
    message: String(payload?.message || ''),
    params: payload?.params || {},
    timestamp: String(payload?.timestamp || new Date().toISOString()),
  }
}

function mockGenerate(options: UsernameScanGeneratorOptions): string[] {
  const targetLength = Math.max(6, Math.min(30, Number(options.targetLength) || 6))
  const maxResults = Math.max(10, Math.min(80, Number(options.maxResults) || 20))
  const terms = options.sourceText
    .toLowerCase()
    .split(/[^a-z0-9]+/)
    .filter(Boolean)
  const base = terms.join('').replace(/[^a-z0-9]/g, '') || 'sample'
  const out: string[] = []
  for (let i = 0; i < maxResults; i += 1) {
    const suffix = options.allowDigits ? String(i + 1).padStart(2, '0') : String.fromCharCode(97 + (i % 26))
    const raw = `${base}${suffix}${'x'.repeat(targetLength)}`
    const value = raw.slice(0, targetLength)
    if (!out.includes(value)) out.push(value)
  }
  return out
}
