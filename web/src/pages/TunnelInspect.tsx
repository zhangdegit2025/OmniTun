import { useState, useEffect, useRef, useCallback } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useTunnel } from '@/hooks/useTunnels'
import { useWebSocket } from '@/hooks/useWebSocket'
import { getAccessToken } from '@/lib/auth'
import type { InspectEntry } from '@/lib/types'
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from '@/components/ui/Card'
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '@/components/ui/Table'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Dialog } from '@/components/ui/Dialog'
import { useToast } from '@/components/ui/useToast'
import {
  ArrowLeft,
  Pause,
  Play,
  Trash2,
  Search,
  Copy,
  RotateCw,
  X,
} from 'lucide-react'

const MAX_ENTRIES = 200
const WS_URL = 'ws://localhost:8080/ws'

function generateMockEntry(): InspectEntry {
  const methods = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH']
  const paths = ['/api/users', '/api/tunnels', '/api/health', '/api/data', '/api/config', '/', '/static/app.js', '/favicon.ico']
  const statusCodes = [200, 200, 200, 201, 204, 301, 302, 304, 400, 401, 403, 404, 500, 502, 503]
  const method = methods[Math.floor(Math.random() * methods.length)]
  const status = statusCodes[Math.floor(Math.random() * statusCodes.length)]
  return {
    id: Math.random().toString(36).slice(2, 10),
    timestamp: new Date().toISOString(),
    method,
    path: paths[Math.floor(Math.random() * paths.length)],
    status_code: status,
    duration_ms: Math.floor(Math.random() * 500),
    client_ip: `192.168.${Math.floor(Math.random() * 255)}.${Math.floor(Math.random() * 255)}`,
    request_headers: {
      'Host': 'example.omnitun.io',
      'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64)',
      'Accept': 'application/json',
      'Content-Type': 'application/json',
      'Authorization': 'Bearer eyJ...',
    },
    request_body: method !== 'GET' ? JSON.stringify({ action: 'test', value: Math.floor(Math.random() * 100) }, null, 2) : undefined,
    response_headers: {
      'Content-Type': 'application/json',
      'X-Request-ID': Math.random().toString(36).slice(2, 10),
    },
    response_body: JSON.stringify({ ok: true, id: Math.floor(Math.random() * 1000) }, null, 2),
  }
}

function statusColor(code: number): string {
  if (code >= 500) return 'text-red-600 dark:text-red-400'
  if (code >= 400) return 'text-yellow-600 dark:text-yellow-400'
  if (code >= 300) return 'text-blue-600 dark:text-blue-400'
  return 'text-emerald-600 dark:text-emerald-400'
}

function formatTimestamp(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' }) + '.' + String(d.getMilliseconds()).padStart(3, '0')
}

export default function TunnelInspect() {
  const { id } = useParams<{ id: string }>()
  const { t } = useTranslation()
  const { toast } = useToast()
  const { data: tunnel } = useTunnel(id)
  const { connectionState } = useWebSocket(WS_URL, {})

  const [entries, setEntries] = useState<InspectEntry[]>([])
  const [paused, setPaused] = useState(false)
  const [filter, setFilter] = useState('')
  const [selectedEntry, setSelectedEntry] = useState<InspectEntry | null>(null)
  const [showReplay, setShowReplay] = useState(false)
  const [replayBody, setReplayBody] = useState('')
  const [replayHeaders, setReplayHeaders] = useState('')
  const [replaying, setReplaying] = useState(false)
  const [highlightedId, setHighlightedId] = useState<string | null>(null)

  const tableRef = useRef<HTMLDivElement>(null)
  const mockTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const pausedRef = useRef(paused)
  pausedRef.current = paused

  const addEntry = useCallback((entry: InspectEntry) => {
    setEntries((prev) => {
      const next = [entry, ...prev]
      return next.slice(0, MAX_ENTRIES)
    })
    setHighlightedId(entry.id)
    setTimeout(() => setHighlightedId(null), 2000)
  }, [])

  useEffect(() => {
    const token = getAccessToken()
    let ws: WebSocket | null = null
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null

    function connect() {
      try {
        ws = new WebSocket(`${WS_URL}?token=${encodeURIComponent(token ?? '')}`)
        ws.onmessage = (event) => {
          if (pausedRef.current) return
          try {
            const msg = JSON.parse(event.data)
            if (msg.type === 'traffic' && msg.data) {
              addEntry(msg.data as InspectEntry)
            }
          } catch {
            // ignore non-JSON
          }
        }
        ws.onclose = () => {
          reconnectTimer = setTimeout(connect, 3000)
        }
      } catch {
        reconnectTimer = setTimeout(connect, 3000)
      }
    }

    connect()

    return () => {
      ws?.close()
      if (reconnectTimer) clearTimeout(reconnectTimer)
    }
  }, [addEntry])

  useEffect(() => {
    mockTimerRef.current = setInterval(() => {
      if (!pausedRef.current) {
        addEntry(generateMockEntry())
      }
    }, 800)
    return () => {
      if (mockTimerRef.current) clearInterval(mockTimerRef.current)
    }
  }, [addEntry])

  const filtered = filter
    ? entries.filter((e) => {
        const q = filter.toLowerCase()
        return (
          e.method.toLowerCase().includes(q) ||
          e.path.toLowerCase().includes(q) ||
          String(e.status_code).includes(q) ||
          e.client_ip.includes(q)
        )
      })
    : entries

  const handleClear = () => setEntries([])

  const handleCopyRequest = (entry: InspectEntry) => {
    const text = `${entry.method} ${entry.path}\n${Object.entries(entry.request_headers ?? {}).map(([k, v]) => `${k}: ${v}`).join('\n')}\n\n${entry.request_body ?? ''}`
    navigator.clipboard.writeText(text)
    toast({ title: t('common.copied'), variant: 'success' })
  }

  const handleCopyResponse = (entry: InspectEntry) => {
    const text = `Status: ${entry.status_code}\n${Object.entries(entry.response_headers ?? {}).map(([k, v]) => `${k}: ${v}`).join('\n')}\n\n${entry.response_body ?? ''}`
    navigator.clipboard.writeText(text)
    toast({ title: t('common.copied'), variant: 'success' })
  }

  const handleOpenReplay = (entry: InspectEntry) => {
    setReplayBody(entry.request_body ?? '')
    setReplayHeaders(JSON.stringify(entry.request_headers ?? {}, null, 2))
    setShowReplay(true)
  }

  const handleReplay = async () => {
    if (!selectedEntry) return
    setReplaying(true)
    await new Promise((resolve) => setTimeout(resolve, 800))
    setReplaying(false)
    setShowReplay(false)
    toast({ title: 'Request replayed (mock)', variant: 'success' })
  }

  return (
    <div className="space-y-4 p-6">
      <nav className="flex items-center gap-2 text-sm text-muted-foreground">
        <Link to="/tunnels" className="hover:text-foreground">
          {t('tunnel_detail.breadcrumb')}
        </Link>
        <span>/</span>
        <Link to={`/tunnels/${id}`} className="hover:text-foreground">
          {tunnel?.name ?? '...'}
        </Link>
        <span>/</span>
        <span className="text-foreground">{t('inspect.title')}</span>
      </nav>

      <Link
        to={`/tunnels/${id}`}
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft className="h-4 w-4" />
        {t('inspect.back')}
      </Link>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
          <div>
            <CardTitle>{t('inspect.title')}</CardTitle>
            <CardDescription>
              {t('inspect.subtitle')}: {tunnel?.name ?? id}
            </CardDescription>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-xs text-muted-foreground">
              WS: {connectionState} | {entries.length} entries
            </span>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPaused(!paused)}
            >
              {paused ? <Play className="mr-1 h-4 w-4" /> : <Pause className="mr-1 h-4 w-4" />}
              {paused ? t('inspect.resume') : t('inspect.pause')}
            </Button>
            <Button variant="outline" size="sm" onClick={handleClear}>
              <Trash2 className="mr-1 h-4 w-4" />
              {t('inspect.clear')}
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          <div className="mb-3">
            <div className="relative">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                placeholder={t('inspect.filter_placeholder')}
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
                className="pl-9"
              />
            </div>
          </div>

          <div ref={tableRef} className="max-h-[60vh] overflow-auto rounded-md border">
            <Table>
              <TableHeader className="sticky top-0 bg-card z-10">
                <TableRow>
                  <TableHead className="w-[100px]">{t('inspect.table.time')}</TableHead>
                  <TableHead className="w-[70px]">{t('inspect.table.method')}</TableHead>
                  <TableHead>{t('inspect.table.path')}</TableHead>
                  <TableHead className="w-[70px]">{t('inspect.table.status')}</TableHead>
                  <TableHead className="w-[80px] text-right">{t('inspect.table.duration')}</TableHead>
                  <TableHead className="w-[130px]">{t('inspect.table.client_ip')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filtered.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={6} className="h-24 text-center text-muted-foreground">
                      {entries.length === 0 ? t('inspect.empty') : t('inspect.no_match')}
                    </TableCell>
                  </TableRow>
                ) : (
                  filtered.map((entry) => (
                    <TableRow
                      key={entry.id}
                      onClick={() => setSelectedEntry(entry)}
                      className={`cursor-pointer transition-colors hover:bg-accent ${
                        highlightedId === entry.id ? 'bg-yellow-100 dark:bg-yellow-900/30' : ''
                      }`}
                    >
                      <TableCell className="text-xs text-muted-foreground font-mono">
                        {formatTimestamp(entry.timestamp)}
                      </TableCell>
                      <TableCell>
                        <Badge variant="secondary" className="text-xs font-mono">
                          {entry.method}
                        </Badge>
                      </TableCell>
                      <TableCell className="max-w-[300px] truncate text-xs font-mono">
                        {entry.path}
                      </TableCell>
                      <TableCell>
                        <span className={`text-xs font-mono font-bold ${statusColor(entry.status_code)}`}>
                          {entry.status_code}
                        </span>
                      </TableCell>
                      <TableCell className="text-right text-xs text-muted-foreground">
                        {entry.duration_ms}ms
                      </TableCell>
                      <TableCell className="text-xs font-mono text-muted-foreground">
                        {entry.client_ip}
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      {/* Detail Panel */}
      {selectedEntry && (
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-base">{t('inspect.detail.title')}</CardTitle>
            <Button variant="ghost" size="sm" onClick={() => setSelectedEntry(null)}>
              <X className="h-4 w-4" />
            </Button>
          </CardHeader>
          <CardContent>
            <div className="grid gap-6 lg:grid-cols-2">
              {/* Request */}
              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <h4 className="text-sm font-semibold">{t('inspect.detail.request')}</h4>
                  <div className="flex gap-2">
                    <Button variant="outline" size="sm" onClick={() => handleCopyRequest(selectedEntry)}>
                      <Copy className="mr-1 h-3 w-3" />
                      {t('inspect.detail.copy_request')}
                    </Button>
                    <Button variant="outline" size="sm" onClick={() => handleOpenReplay(selectedEntry)}>
                      <RotateCw className="mr-1 h-3 w-3" />
                      {t('inspect.detail.replay')}
                    </Button>
                  </div>
                </div>
                <div className="flex gap-2 text-sm">
                  <Badge variant="secondary" className="font-mono text-xs">{selectedEntry.method}</Badge>
                  <code className="text-xs">{selectedEntry.path}</code>
                </div>
                <div>
                  <p className="text-xs font-medium text-muted-foreground mb-1">{t('inspect.detail.headers')}</p>
                  <pre className="rounded-md bg-muted p-3 text-xs font-mono overflow-auto max-h-[200px]">
                    {Object.entries(selectedEntry.request_headers ?? {}).map(([k, v]) => `${k}: ${v}\n`).join('')}
                  </pre>
                </div>
                <div>
                  <p className="text-xs font-medium text-muted-foreground mb-1">{t('inspect.detail.body')}</p>
                  <pre className="rounded-md bg-muted p-3 text-xs font-mono overflow-auto max-h-[200px] whitespace-pre-wrap">
                    {selectedEntry.request_body || <span className="text-muted-foreground">{t('inspect.detail.no_body')}</span>}
                  </pre>
                </div>
              </div>

              {/* Response */}
              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <h4 className="text-sm font-semibold">{t('inspect.detail.response')}</h4>
                  <Button variant="outline" size="sm" onClick={() => handleCopyResponse(selectedEntry)}>
                    <Copy className="mr-1 h-3 w-3" />
                    {t('inspect.detail.copy_response')}
                  </Button>
                </div>
                <div className="flex gap-4 text-sm">
                  <div>
                    <span className="text-muted-foreground">{t('inspect.detail.status')}:</span>{' '}
                    <span className={`font-mono font-bold ${statusColor(selectedEntry.status_code)}`}>
                      {selectedEntry.status_code}
                    </span>
                  </div>
                  <div>
                    <span className="text-muted-foreground">{t('inspect.detail.duration')}:</span>{' '}
                    <span className="font-mono">{selectedEntry.duration_ms}ms</span>
                  </div>
                </div>
                <div>
                  <p className="text-xs font-medium text-muted-foreground mb-1">{t('inspect.detail.headers')}</p>
                  <pre className="rounded-md bg-muted p-3 text-xs font-mono overflow-auto max-h-[200px]">
                    {Object.entries(selectedEntry.response_headers ?? {}).map(([k, v]) => `${k}: ${v}\n`).join('')}
                  </pre>
                </div>
                <div>
                  <p className="text-xs font-medium text-muted-foreground mb-1">{t('inspect.detail.body')}</p>
                  <pre className="rounded-md bg-muted p-3 text-xs font-mono overflow-auto max-h-[200px] whitespace-pre-wrap">
                    {selectedEntry.response_body || <span className="text-muted-foreground">{t('inspect.detail.no_body')}</span>}
                  </pre>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Replay Dialog */}
      <Dialog
        open={showReplay}
        onClose={() => setShowReplay(false)}
        title={t('inspect.replay_dialog.title')}
        description={t('inspect.replay_dialog.description')}
      >
        <form
          onSubmit={(e) => { e.preventDefault(); handleReplay() }}
          className="flex flex-col gap-4"
        >
          <div>
            <label className="text-sm font-medium">{t('inspect.detail.headers')}</label>
            <textarea
              value={replayHeaders}
              onChange={(e) => setReplayHeaders(e.target.value)}
              rows={6}
              className="mt-1.5 w-full rounded-md border border-input bg-transparent px-3 py-2 text-xs font-mono shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
            />
          </div>
          <div>
            <label className="text-sm font-medium">{t('inspect.detail.body')}</label>
            <textarea
              value={replayBody}
              onChange={(e) => setReplayBody(e.target.value)}
              rows={8}
              className="mt-1.5 w-full rounded-md border border-input bg-transparent px-3 py-2 text-xs font-mono shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
            />
          </div>
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="outline" type="button" onClick={() => setShowReplay(false)}>
              {t('inspect.replay_dialog.cancel')}
            </Button>
            <Button type="submit" disabled={replaying}>
              {replaying ? t('inspect.replay_dialog.sending') : t('inspect.replay_dialog.send')}
            </Button>
          </div>
        </form>
      </Dialog>
    </div>
  )
}
