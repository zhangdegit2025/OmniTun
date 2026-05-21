import { useState, useEffect, lazy, Suspense } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useTunnel } from '@/hooks/useTunnels'
import { useTunnels } from '@/hooks/useTunnels'
import { useWebSocket } from '@/hooks/useWebSocket'
import { apiRequest } from '@/lib/api'
import { useQuery } from '@tanstack/react-query'
import type { ConnectionLogEntry, TrafficPoint } from '@/lib/types'
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
import { Skeleton } from '@/components/ui/Skeleton'
import {
  AlertCircle,
  ArrowLeft,
  ExternalLink,
  Activity,
  Play,
  Square,
  RotateCw,
  Edit,
  Eye,
} from 'lucide-react'
import { format, formatDistanceToNow } from 'date-fns'
import { useToast } from '@/components/ui/useToast'

const TrafficLineChart = lazy(() => import('@/components/TrafficLineChart'))

const statusVariant = {
  active: 'success' as const,
  stopped: 'secondary' as const,
  error: 'destructive' as const,
}

function generateMockTraffic(): TrafficPoint[] {
  const data: TrafficPoint[] = []
  const now = Date.now()
  for (let i = 29; i >= 0; i--) {
    data.push({
      timestamp: new Date(now - i * 5000).toISOString(),
      bytes_in: Math.floor(Math.random() * 5000 + 2000),
      bytes_out: Math.floor(Math.random() * 3000 + 1000),
    })
  }
  return data
}

export default function TunnelDetail() {
  const { id } = useParams<{ id: string }>()
  const { t } = useTranslation()
  const { toast } = useToast()
  const { data: tunnel, isLoading, error, refetch } = useTunnel(id)
  const { startTunnel, stopTunnel, restartTunnel, updateTunnel } = useTunnels()
  const { connectionState } = useWebSocket('ws://localhost:8080/ws')
  const [showEdit, setShowEdit] = useState(false)
  const [editName, setEditName] = useState('')
  const [editAuthMode, setEditAuthMode] = useState<'none' | 'basic' | 'oauth'>('none')
  const [editMaxConnections, setEditMaxConnections] = useState(10)
  const [mockTraffic, setMockTraffic] = useState<TrafficPoint[]>(generateMockTraffic())

  useEffect(() => {
    const timer = setInterval(() => {
      setMockTraffic((prev) => {
        const next = [...prev.slice(1)]
        next.push({
          timestamp: new Date().toISOString(),
          bytes_in: Math.floor(Math.random() * 8000 + 2000),
          bytes_out: Math.floor(Math.random() * 5000 + 1000),
        })
        return next
      })
    }, 5000)
    return () => clearInterval(timer)
  }, [])

  const logsQuery = useQuery<ConnectionLogEntry[]>({
    queryKey: ['tunnel', id, 'logs'],
    queryFn: () => apiRequest<ConnectionLogEntry[]>(`/v1/tunnels/${id}/logs`),
    enabled: !!id,
    refetchInterval: 5000,
  })

  const openEdit = () => {
    if (!tunnel) return
    setEditName(tunnel.name)
    setShowEdit(true)
  }

  const handleEdit = async () => {
    if (!id) return
    try {
      await updateTunnel.mutateAsync({
        id,
        name: editName,
        auth_mode: editAuthMode,
        max_connections: editMaxConnections,
      })
      setShowEdit(false)
      refetch()
      toast({ title: t('tunnel_detail.updated'), variant: 'success' })
    } catch {
      // handled by mutation state
    }
  }

  return (
    <div className="space-y-6 p-6">
      {/* Breadcrumb */}
      <nav className="flex items-center gap-2 text-sm text-muted-foreground">
        <Link to="/tunnels" className="hover:text-foreground">
          {t('tunnel_detail.breadcrumb')}
        </Link>
        <span>/</span>
        <span className="text-foreground">{tunnel?.name ?? '...'}</span>
      </nav>

      <Link
        to="/tunnels"
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft className="h-4 w-4" />
        {t('tunnel_detail.back')}
      </Link>

      {isLoading ? (
        <Card>
          <CardHeader>
            <Skeleton className="h-6 w-48" />
            <Skeleton className="h-4 w-32" />
          </CardHeader>
          <CardContent className="space-y-4">
            <Skeleton className="h-20 w-full" />
            <Skeleton className="h-40 w-full" />
          </CardContent>
        </Card>
      ) : error ? (
        <Card>
          <CardContent className="flex flex-col items-center gap-2 py-10">
            <AlertCircle className="h-8 w-8 text-destructive" />
            <p className="text-sm text-destructive">{t('tunnel_detail.failed_load')}</p>
            <Button variant="outline" size="sm" onClick={() => refetch()}>
              {t('common.retry')}
            </Button>
          </CardContent>
        </Card>
      ) : !tunnel ? (
        <Card>
            <CardContent className="py-10 text-center text-sm text-muted-foreground">
            {t('tunnel_detail.not_found')}
          </CardContent>
        </Card>
      ) : (
        <>
          {/* Tunnel Info Card */}
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle className="text-2xl">{tunnel.name}</CardTitle>
                  <CardDescription className="mt-1">
                    Created {format(new Date(tunnel.created_at), 'PPP')} &middot;{' '}
                    {formatDistanceToNow(new Date(tunnel.created_at), { addSuffix: true })}
                  </CardDescription>
                </div>
                <div className="flex items-center gap-2">
                  <Badge variant={statusVariant[tunnel.status]} className="px-3 py-1 text-sm">
                    {t(`tunnels.status.${tunnel.status}`)}
                  </Badge>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <dl className="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
                <div>
                  <dt className="text-sm text-muted-foreground">{t('tunnel_detail.protocol')}</dt>
                  <dd className="mt-1 font-mono text-sm uppercase">{t(`tunnels.protocols.${tunnel.protocol}`)}</dd>
                </div>
                <div>
                  <dt className="text-sm text-muted-foreground">{t('tunnel_detail.local_address')}</dt>
                  <dd className="mt-1 font-mono text-sm">localhost:{tunnel.local_port}</dd>
                </div>
                <div>
                  <dt className="text-sm text-muted-foreground">{t('tunnel_detail.remote_port')}</dt>
                  <dd className="mt-1 font-mono text-sm">:{tunnel.remote_port}</dd>
                </div>
                <div>
                  <dt className="text-sm text-muted-foreground">{t('tunnel_detail.domain')}</dt>
                  <dd className="mt-1 flex items-center gap-1 text-sm">
                    {tunnel.domain ? (
                      <>
                        {tunnel.domain}
                        <ExternalLink className="h-3 w-3 text-muted-foreground" />
                      </>
                    ) : (
                      <span className="text-muted-foreground">—</span>
                    )}
                  </dd>
                </div>
                <div>
                  <dt className="text-sm text-muted-foreground">{t('tunnel_detail.traffic')}</dt>
                  <dd className="mt-1 text-sm">
                    {formatBytes(tunnel.traffic_in + tunnel.traffic_out)}
                  </dd>
                </div>
              </dl>

              {/* Action Buttons */}
              <div className="mt-6 flex items-center gap-2 border-t pt-4">
                {tunnel.status !== 'active' ? (
                  <Button size="sm" onClick={() => startTunnel.mutate(tunnel.id)} disabled={startTunnel.isPending}>
                    <Play className="mr-1 h-4 w-4" />
                    {startTunnel.isPending ? t('tunnel_detail.actions.starting') : t('tunnel_detail.actions.start')}
                  </Button>
                ) : (
                  <Button variant="outline" size="sm" onClick={() => stopTunnel.mutate(tunnel.id)} disabled={stopTunnel.isPending}>
                    <Square className="mr-1 h-4 w-4" />
                    {stopTunnel.isPending ? t('tunnel_detail.actions.stopping') : t('tunnel_detail.actions.stop')}
                  </Button>
                )}
                <Button variant="outline" size="sm" onClick={() => restartTunnel.mutate(tunnel.id)} disabled={restartTunnel.isPending}>
                  <RotateCw className={`mr-1 h-4 w-4 ${restartTunnel.isPending ? 'animate-spin' : ''}`} />
                  {restartTunnel.isPending ? t('tunnel_detail.actions.restarting') : t('tunnel_detail.actions.restart')}
                </Button>
                <div className="flex-1" />
                <Link to={`/tunnels/${tunnel.id}/inspect`}>
                  <Button variant="outline" size="sm">
                    <Eye className="mr-1 h-4 w-4" />
                    Inspect Traffic
                  </Button>
                </Link>
                <Button variant="outline" size="sm" onClick={openEdit}>
                  <Edit className="mr-1 h-4 w-4" />
                  {t('tunnel_detail.edit')}
                </Button>
              </div>
            </CardContent>
          </Card>

          {/* Real-time Traffic Chart */}
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle className="flex items-center gap-2">
                  <Activity className="h-5 w-5" />
                  {t('tunnel_detail.traffic_graph')}
                </CardTitle>
                <span className="text-xs text-muted-foreground">
                  WS: {connectionState}
                </span>
              </div>
            </CardHeader>
            <CardContent>
              <Suspense fallback={<Skeleton className="h-[250px] w-full" />}>
                <TrafficLineChart data={mockTraffic} />
              </Suspense>
            </CardContent>
          </Card>

          {/* Connection Log */}
          <Card>
            <CardHeader>
              <CardTitle>{t('tunnel_detail.connection_log')}</CardTitle>
            </CardHeader>
            <CardContent>
              {logsQuery.isLoading ? (
                <div className="space-y-2">
                  <Skeleton className="h-8 w-full" />
                  <Skeleton className="h-8 w-full" />
                  <Skeleton className="h-8 w-full" />
                </div>
              ) : logsQuery.data?.length === 0 ? (
                <div className="py-6 text-center text-sm text-muted-foreground">
                  {t('tunnel_detail.no_logs')}
                </div>
              ) : logsQuery.data ? (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('tunnel_detail.table.time')}</TableHead>
                      <TableHead>{t('tunnel_detail.table.client_ip')}</TableHead>
                      <TableHead>{t('tunnel_detail.table.method')}</TableHead>
                      <TableHead>{t('tunnel_detail.table.path')}</TableHead>
                      <TableHead>{t('tunnel_detail.table.status')}</TableHead>
                      <TableHead className="text-right">{t('tunnel_detail.table.bytes')}</TableHead>
                      <TableHead className="text-right">{t('tunnel_detail.table.duration')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {logsQuery.data.map((entry) => (
                      <TableRow key={entry.id}>
                        <TableCell className="text-muted-foreground text-xs">
                          {new Date(entry.timestamp).toLocaleString()}
                        </TableCell>
                        <TableCell className="font-mono text-xs">{entry.client_ip}</TableCell>
                        <TableCell>
                          <Badge variant="secondary" className="text-xs">{entry.method}</Badge>
                        </TableCell>
                        <TableCell className="max-w-[200px] truncate text-xs">{entry.path}</TableCell>
                        <TableCell>
                          <Badge variant={entry.status_code < 400 ? 'success' : 'destructive'} className="text-xs">
                            {entry.status_code}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-right text-xs">{formatBytes(entry.bytes_sent)}</TableCell>
                        <TableCell className="text-right text-xs">{entry.duration_ms}ms</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              ) : (
                <div className="flex flex-col items-center gap-2 py-6 text-center">
                  <AlertCircle className="h-8 w-8 text-destructive" />
                  <p className="text-sm text-destructive">{t('tunnel_detail.failed_logs')}</p>
                  <Button variant="outline" size="sm" onClick={() => logsQuery.refetch()}>
                    {t('common.retry')}
                  </Button>
                </div>
              )}
            </CardContent>
          </Card>
        </>
      )}

      {/* Edit Dialog */}
      <Dialog
        open={showEdit}
        onClose={() => setShowEdit(false)}
        title={t('tunnel_detail.edit_dialog.title')}
        description={t('tunnel_detail.edit_dialog.description')}
      >
        <form
          onSubmit={(e) => {
            e.preventDefault()
            handleEdit()
          }}
          className="flex flex-col gap-4"
        >
          <Input
            label={t('tunnel_detail.edit_dialog.name')}
            value={editName}
            onChange={(e) => setEditName(e.target.value)}
          />
          <div>
            <label className="text-sm font-medium">{t('tunnel_detail.edit_dialog.auth_mode')}</label>
            <select
              value={editAuthMode}
              onChange={(e) => setEditAuthMode(e.target.value as 'none' | 'basic' | 'oauth')}
              className="mt-1.5 flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
            >
              <option value="none">{t('tunnel_detail.edit_dialog.auth_none')}</option>
              <option value="basic">{t('tunnel_detail.edit_dialog.auth_basic')}</option>
              <option value="oauth">{t('tunnel_detail.edit_dialog.auth_oauth')}</option>
            </select>
          </div>
          <Input
            label={t('tunnel_detail.edit_dialog.max_connections')}
            type="number"
            value={editMaxConnections}
            onChange={(e) => setEditMaxConnections(Number(e.target.value))}
          />
          {updateTunnel.error && (
            <p className="text-sm text-destructive">
              {(updateTunnel.error as Error)?.message ?? t('tunnel_detail.failed_load')}
            </p>
          )}
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="outline" type="button" onClick={() => setShowEdit(false)}>
              {t('tunnel_detail.edit_dialog.cancel')}
            </Button>
            <Button type="submit" disabled={updateTunnel.isPending}>
              {updateTunnel.isPending ? t('tunnel_detail.edit_dialog.saving') : t('tunnel_detail.edit_dialog.save')}
            </Button>
          </div>
        </form>
      </Dialog>
    </div>
  )
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
}
