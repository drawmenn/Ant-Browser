import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  AlertTriangle,
  CheckCircle2,
  Clock,
  Pause,
  Play,
  RefreshCw,
  Search,
  Square,
  Wand2,
  XCircle,
} from 'lucide-react'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { Badge, Button, Card, FormItem, Input, Select, Switch, Textarea, toast } from '../../shared/components'
import { fetchBrowserProfiles } from '../browser/api'
import type { BrowserProfile } from '../browser/types'
import {
  fetchUsernameScanSnapshot,
  generateUsernameCandidates,
  normalizeLogEntry,
  normalizeSnapshot,
  pauseUsernameScan,
  startUsernameScan,
  stopUsernameScan,
} from './api'
import type { UsernameScanLogEntry, UsernameScanResult, UsernameScanSnapshot } from './types'
import { emptyUsernameScanSnapshot } from './types'

const SNAPSHOT_EVENT = 'username-scan:snapshot'
const LOG_EVENT = 'username-scan:log'

const PROVIDER_OPTIONS = [
  { value: 'mock', label: 'Mock' },
  { value: 'browser', label: 'Browser' },
]

function splitLines(value: string): string[] {
  return value
    .split(/\r?\n|,/)
    .map(item => item.trim())
    .filter(Boolean)
}

function formatTime(value: string): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleTimeString()
}

function statusBadge(result: UsernameScanResult) {
  if (result.status === 'available') {
    return <Badge variant="success">available</Badge>
  }
  if (result.status === 'error') {
    return <Badge variant="error">error</Badge>
  }
  if (result.status === 'hold') {
    return <Badge variant="warning">hold</Badge>
  }
  return <Badge>taken</Badge>
}

function StatTile({
  label,
  value,
  icon,
}: {
  label: string
  value: string | number
  icon: React.ReactNode
}) {
  return (
    <div className="rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-card)] px-4 py-3">
      <div className="flex items-center justify-between gap-3">
        <span className="text-xs text-[var(--color-text-muted)]">{label}</span>
        <div className="text-[var(--color-accent)]">{icon}</div>
      </div>
      <div className="mt-2 text-xl font-semibold text-[var(--color-text-primary)]">{value}</div>
    </div>
  )
}

export function UsernameScanPage() {
  const [sourceText, setSourceText] = useState('james alex')
  const [targetLength, setTargetLength] = useState(6)
  const [allowDigits, setAllowDigits] = useState(false)
  const [maxResults, setMaxResults] = useState(80)
  const [candidateText, setCandidateText] = useState('')
  const [intervalMs, setIntervalMs] = useState(500)
  const [limit, setLimit] = useState(80)
  const [generating, setGenerating] = useState(false)
  const [snapshot, setSnapshot] = useState<UsernameScanSnapshot>(emptyUsernameScanSnapshot)
  const [provider, setProvider] = useState<'mock' | 'browser'>('mock')
  const [profiles, setProfiles] = useState<BrowserProfile[]>([])
  const [loadingProfiles, setLoadingProfiles] = useState(false)
  const [profileId, setProfileId] = useState('')
  const [targetUrl, setTargetUrl] = useState('')
  const [inputSelector, setInputSelector] = useState('input[name="username"]')
  const [submitSelector, setSubmitSelector] = useState('')
  const [resultSelector, setResultSelector] = useState('body')
  const [availableText, setAvailableText] = useState('available')
  const [takenText, setTakenText] = useState('taken')
  const [holdText, setHoldText] = useState('')
  const [waitAfterSubmitMs, setWaitAfterSubmitMs] = useState(1200)
  const [timeoutMs, setTimeoutMs] = useState(8000)

  const loadProfiles = useCallback(async () => {
    setLoadingProfiles(true)
    try {
      const list = await fetchBrowserProfiles()
      setProfiles(list)
      const running = list.find(item => item.running && item.debugReady)
      setProfileId(current => current || running?.profileId || '')
    } catch (error: any) {
      toast.error(error?.message || 'Profile list failed to load')
    } finally {
      setLoadingProfiles(false)
    }
  }, [])

  useEffect(() => {
    let disposed = false

    void fetchUsernameScanSnapshot()
      .then(data => {
        if (!disposed) setSnapshot(data)
      })
      .catch(() => {})

    const offSnapshot = EventsOn(SNAPSHOT_EVENT, (payload: any) => {
      setSnapshot(normalizeSnapshot(payload))
    })
    const offLog = EventsOn(LOG_EVENT, (payload: any) => {
      const entry = normalizeLogEntry(payload)
      if (entry.level === 'error') {
        toast.error(entry.message)
      }
    })

    return () => {
      disposed = true
      offSnapshot?.()
      offLog?.()
    }
  }, [])

  useEffect(() => {
    void loadProfiles()
  }, [loadProfiles])

  const candidates = useMemo(() => splitLines(candidateText), [candidateText])
  const profileOptions = useMemo(() => [
    { value: '', label: 'Select profile' },
    ...profiles.map(profile => ({
      value: profile.profileId,
      label: `${profile.profileName}${profile.running && profile.debugReady ? ' · ready' : profile.running ? ' · starting' : ' · stopped'}`,
    })),
  ], [profiles])
  const selectedProfile = useMemo(
    () => profiles.find(profile => profile.profileId === profileId) || null,
    [profiles, profileId]
  )
  const browserConfigInvalid = provider === 'browser' && (
    !profileId ||
    !selectedProfile?.running ||
    !selectedProfile?.debugReady ||
    !inputSelector.trim() ||
    (!availableText.trim() && !takenText.trim() && !holdText.trim())
  )
  const progress = snapshot.queueSize > 0 ? Math.round((snapshot.nextIndex / snapshot.queueSize) * 100) : 0
  const scanDisabled = snapshot.running && !snapshot.paused

  const handleGenerate = async () => {
    setGenerating(true)
    try {
      const generated = await generateUsernameCandidates({
        sourceText,
        targetLength,
        allowDigits,
        maxResults,
      })
      setCandidateText(generated.join('\n'))
      setLimit(generated.length || maxResults)
      toast.success(`Generated ${generated.length} candidates`)
    } catch (error: any) {
      toast.error(error?.message || 'Candidate generation failed')
    } finally {
      setGenerating(false)
    }
  }

  const handleStart = async () => {
    if (provider === 'browser' && browserConfigInvalid) {
      toast.error('Browser provider config is incomplete')
      return
    }

    try {
      const nextSnapshot = await startUsernameScan({
        seeds: splitLines(sourceText),
        candidates,
        provider,
        browser: {
          profileId,
          url: targetUrl,
          inputSelector,
          submitSelector,
          resultSelector,
          availableText,
          takenText,
          holdText,
          waitAfterSubmitMs,
          timeoutMs,
        },
        intervalMs,
        limit,
      })
      setSnapshot(nextSnapshot)
    } catch (error: any) {
      toast.error(error?.message || 'Scan failed to start')
    }
  }

  const handlePause = async () => {
    setSnapshot(await pauseUsernameScan())
  }

  const handleStop = async () => {
    setSnapshot(await stopUsernameScan())
  }

  return (
    <div className="space-y-5 animate-fade-in">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">Username Scan</h1>
          <p className="mt-1 text-sm text-[var(--color-text-muted)]">Native task runner with mock and browser providers.</p>
        </div>
        <div className="flex items-center gap-2">
          <Badge variant={snapshot.paused ? 'warning' : snapshot.running ? 'success' : 'default'} dot>
            {snapshot.paused ? 'paused' : snapshot.running ? 'running' : 'idle'}
          </Badge>
          <Badge variant="info">provider: {snapshot.provider || 'mock'}</Badge>
        </div>
      </div>

      <div className="grid grid-cols-2 lg:grid-cols-5 gap-3">
        <StatTile label="Checked" value={snapshot.stats.checked} icon={<Search className="w-4 h-4" />} />
        <StatTile label="Available" value={snapshot.stats.hit} icon={<CheckCircle2 className="w-4 h-4" />} />
        <StatTile label="Taken" value={snapshot.stats.taken} icon={<XCircle className="w-4 h-4" />} />
        <StatTile label="Errors" value={snapshot.stats.error} icon={<AlertTriangle className="w-4 h-4" />} />
        <StatTile label="Hit Rate" value={`${snapshot.stats.rate.toFixed(1)}%`} icon={<Clock className="w-4 h-4" />} />
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-[420px_1fr] gap-5">
        <div className="space-y-5">
          <Card
            title="Candidate Builder"
            actions={
              <Button size="sm" onClick={handleGenerate} loading={generating}>
                <Wand2 className="w-3.5 h-3.5" /> Generate
              </Button>
            }
          >
            <div className="space-y-4">
              <FormItem label="Source terms">
                <Textarea
                  rows={4}
                  value={sourceText}
                  onChange={event => setSourceText(event.target.value)}
                  placeholder="james, alex, brand terms"
                />
              </FormItem>
              <div className="grid grid-cols-2 gap-3">
                <FormItem label="Length">
                  <Input
                    type="number"
                    min={6}
                    max={30}
                    value={targetLength}
                    onChange={event => setTargetLength(Number(event.target.value) || 6)}
                  />
                </FormItem>
                <FormItem label="Max">
                  <Input
                    type="number"
                    min={10}
                    max={500}
                    value={maxResults}
                    onChange={event => setMaxResults(Number(event.target.value) || 80)}
                  />
                </FormItem>
              </div>
              <div className="flex items-center justify-between rounded-lg border border-[var(--color-border-muted)] px-3 py-2">
                <span className="text-sm text-[var(--color-text-secondary)]">Allow digits</span>
                <Switch checked={allowDigits} onChange={setAllowDigits} />
              </div>
            </div>
          </Card>

          <Card
            title="Provider"
            actions={
              <Button size="sm" variant="secondary" onClick={() => void loadProfiles()} loading={loadingProfiles}>
                <RefreshCw className="w-3.5 h-3.5" /> Refresh
              </Button>
            }
          >
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-3">
                <FormItem label="Mode">
                  <Select
                    value={provider}
                    disabled={scanDisabled}
                    onChange={event => setProvider(event.target.value as 'mock' | 'browser')}
                    options={PROVIDER_OPTIONS}
                  />
                </FormItem>
                <FormItem label="Profile">
                  <Select
                    value={profileId}
                    disabled={scanDisabled || provider !== 'browser'}
                    onChange={event => setProfileId(event.target.value)}
                    options={profileOptions}
                  />
                </FormItem>
              </div>

              {provider === 'browser' && (
                <div className="space-y-4">
                  <FormItem label="URL">
                    <Input
                      value={targetUrl}
                      disabled={scanDisabled}
                      onChange={event => setTargetUrl(event.target.value)}
                      placeholder="https://example.com/signup"
                    />
                  </FormItem>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                    <FormItem label="Input selector">
                      <Input
                        value={inputSelector}
                        disabled={scanDisabled}
                        onChange={event => setInputSelector(event.target.value)}
                        placeholder='input[name="username"]'
                      />
                    </FormItem>
                    <FormItem label="Submit selector">
                      <Input
                        value={submitSelector}
                        disabled={scanDisabled}
                        onChange={event => setSubmitSelector(event.target.value)}
                        placeholder="button[type=submit]"
                      />
                    </FormItem>
                  </div>
                  <FormItem label="Result selector">
                    <Input
                      value={resultSelector}
                      disabled={scanDisabled}
                      onChange={event => setResultSelector(event.target.value)}
                      placeholder="body"
                    />
                  </FormItem>
                  <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                    <FormItem label="Available text">
                      <Textarea
                        rows={3}
                        value={availableText}
                        disabled={scanDisabled}
                        onChange={event => setAvailableText(event.target.value)}
                        placeholder="available"
                      />
                    </FormItem>
                    <FormItem label="Taken text">
                      <Textarea
                        rows={3}
                        value={takenText}
                        disabled={scanDisabled}
                        onChange={event => setTakenText(event.target.value)}
                        placeholder="taken"
                      />
                    </FormItem>
                    <FormItem label="Hold text">
                      <Textarea
                        rows={3}
                        value={holdText}
                        disabled={scanDisabled}
                        onChange={event => setHoldText(event.target.value)}
                        placeholder="try again"
                      />
                    </FormItem>
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <FormItem label="Wait ms">
                      <Input
                        type="number"
                        min={100}
                        max={30000}
                        value={waitAfterSubmitMs}
                        disabled={scanDisabled}
                        onChange={event => setWaitAfterSubmitMs(Number(event.target.value) || 1200)}
                      />
                    </FormItem>
                    <FormItem label="Timeout ms">
                      <Input
                        type="number"
                        min={1000}
                        max={60000}
                        value={timeoutMs}
                        disabled={scanDisabled}
                        onChange={event => setTimeoutMs(Number(event.target.value) || 8000)}
                      />
                    </FormItem>
                  </div>
                </div>
              )}
            </div>
          </Card>

          <Card title="Scan Controls">
            <div className="space-y-4">
              <FormItem label="Candidates">
                <Textarea
                  rows={9}
                  value={candidateText}
                  disabled={scanDisabled}
                  onChange={event => setCandidateText(event.target.value)}
                  placeholder="one candidate per line"
                />
              </FormItem>
              <div className="grid grid-cols-2 gap-3">
                <FormItem label="Interval ms">
                  <Input
                    type="number"
                    min={100}
                    max={10000}
                    value={intervalMs}
                    disabled={scanDisabled}
                    onChange={event => setIntervalMs(Number(event.target.value) || 500)}
                  />
                </FormItem>
                <FormItem label="Limit">
                  <Input
                    type="number"
                    min={1}
                    max={500}
                    value={limit}
                    disabled={scanDisabled}
                    onChange={event => setLimit(Number(event.target.value) || 80)}
                  />
                </FormItem>
              </div>
              <div className="flex flex-wrap gap-2">
                <Button onClick={handleStart} disabled={scanDisabled || candidates.length === 0 || browserConfigInvalid}>
                  <Play className="w-4 h-4" /> Start
                </Button>
                <Button variant="secondary" onClick={handlePause} disabled={!snapshot.running}>
                  <Pause className="w-4 h-4" /> Pause
                </Button>
                <Button variant="danger" onClick={handleStop} disabled={!snapshot.running && !snapshot.paused}>
                  <Square className="w-4 h-4" /> Stop
                </Button>
              </div>
              <div>
                <div className="mb-1 flex items-center justify-between text-xs text-[var(--color-text-muted)]">
                  <span>{snapshot.activeName || 'idle'}</span>
                  <span>{progress}%</span>
                </div>
                <div className="h-2 overflow-hidden rounded-full bg-[var(--color-bg-muted)]">
                  <div
                    className="h-full rounded-full bg-[var(--color-accent)] transition-all"
                    style={{ width: `${progress}%` }}
                  />
                </div>
              </div>
            </div>
          </Card>
        </div>

        <div className="space-y-5">
          <Card title="Results" subtitle={`${snapshot.results.length} stored`}>
            <div className="overflow-hidden rounded-lg border border-[var(--color-border-muted)]">
              <div className="grid grid-cols-[140px_120px_1fr_90px] bg-[var(--color-bg-secondary)] px-3 py-2 text-xs font-medium text-[var(--color-text-muted)]">
                <span>Name</span>
                <span>Status</span>
                <span>Message</span>
                <span>Time</span>
              </div>
              <div className="max-h-[360px] overflow-y-auto">
                {snapshot.results.length === 0 ? (
                  <div className="px-3 py-8 text-center text-sm text-[var(--color-text-muted)]">No results yet</div>
                ) : (
                  snapshot.results.map(result => (
                    <div
                      key={`${result.name}-${result.checkedAt}`}
                      className="grid grid-cols-[140px_120px_1fr_90px] items-center gap-2 border-t border-[var(--color-border-muted)] px-3 py-2 text-sm"
                    >
                      <span className="font-mono text-[var(--color-text-primary)]">{result.name}</span>
                      <span>{statusBadge(result)}</span>
                      <span className="truncate text-[var(--color-text-secondary)]" title={result.message}>
                        {result.message}
                      </span>
                      <span className="text-xs text-[var(--color-text-muted)]">{formatTime(result.checkedAt)}</span>
                    </div>
                  ))
                )}
              </div>
            </div>
          </Card>

          <Card title="Task Log" subtitle={`${snapshot.logs.length} entries`}>
            <div className="max-h-[260px] space-y-2 overflow-y-auto rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-secondary)] p-3">
              {snapshot.logs.length === 0 ? (
                <div className="py-8 text-center text-sm text-[var(--color-text-muted)]">No log entries yet</div>
              ) : (
                snapshot.logs.map((entry: UsernameScanLogEntry) => (
                  <div key={`${entry.timestamp}-${entry.message}`} className="flex items-start gap-2 text-sm">
                    <span className="w-16 shrink-0 text-xs text-[var(--color-text-muted)]">{formatTime(entry.timestamp)}</span>
                    <Badge
                      variant={entry.level === 'error' ? 'error' : entry.level === 'success' ? 'success' : entry.level === 'warning' ? 'warning' : 'default'}
                      size="sm"
                    >
                      {entry.level}
                    </Badge>
                    <span className="min-w-0 flex-1 text-[var(--color-text-secondary)]">{entry.message}</span>
                  </div>
                ))
              )}
            </div>
          </Card>
        </div>
      </div>
    </div>
  )
}
