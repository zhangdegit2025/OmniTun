import { lazy, Suspense } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { apiRequest } from '@/lib/api'
import type { TunnelStats, RecentEvent } from '@/lib/types'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card'
import { Skeleton } from '@/components/ui/Skeleton'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import {
  AlertCircle,
  Activity,
  ArrowDown,
  Users,
  Zap,
  Plus,
} from 'lucide-react'

const TrafficAreaChart = lazy(() => import('@/components/TrafficAreaChart'))

const eventStatusVariant: Record<string, 'default' | 'success' | 'warning' | 'destructive'> = {
  created: 'default',
  started: 'success',
  stopped: 'warning',
  error: 'destructive',
}

function generateMockTrafficData() {
  const data = []
  const now = new Date()
  for (let i = 23; i >= 0; i--) {
    const t = new Date(now.getTime() - i * 60 * 60 * 1000)
    data.push({
      time: t.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
      in: Math.floor(Math.random() * 50000 + 10000),
      out: Math.floor(Math.random() * 30000 + 5000),
    })
  }
  return data
}

export default function Dashboard() {
  const { t } = useTranslation()

  const statsQuery = useQuery<TunnelStats>({
    queryKey: ['dashboard', 'stats'],
    queryFn: () => apiRequest<TunnelStats>('/v1/dashboard/stats'),
  })

  const eventsQuery = useQuery<RecentEvent[]>({
    queryKey: ['dashboard', 'events'],
    queryFn: () => apiRequest<RecentEvent[]>('/v1/dashboard/events'),
  })

  const mockTraffic = generateMockTrafficData()

  const stats = statsQuery.data
  const isEmpty = stats && stats.total_tunnels === 0
  const showSkeleton = statsQuery.isLoading

  const eventTypeLabel = (type: string) => {
    const key = `dashboard.event.${type}` as keyof typeof t
    return t(key, type)
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">{t('dashboard.title')}</h1>
          <p className="text-sm text-muted-foreground">{t('dashboard.subtitle')}</p>
        </div>
      </div>

      {/* Stats Cards */}
      {statsQuery.isError ? (
        <div className="flex flex-col items-center gap-2 rounded-lg border p-6 text-center">
          <AlertCircle className="h-8 w-8 text-destructive" />
          <p className="text-sm text-destructive">{t('dashboard.failed_stats')}</p>
          <Button variant="outline" size="sm" onClick={() => statsQuery.refetch()}>
            {t('common.retry')}
          </Button>
        </div>
      ) : (
        <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {showSkeleton ? (
            <>
              <Skeleton className="h-28" />
              <Skeleton className="h-28" />
              <Skeleton className="h-28" />
              <Skeleton className="h-28" />
            </>
          ) : (
            <>
              <Card>
                <CardHeader className="flex flex-row items-center justify-between pb-2">
                  <CardTitle className="text-sm font-medium text-muted-foreground">{t('dashboard.active_tunnels')}</CardTitle>
                  <Activity className="h-4 w-4 text-emerald-500" />
                </CardHeader>
                <CardContent>
                  <div className="flex items-baseline gap-2">
                    <p className="text-2xl font-bold">{stats?.active_tunnels ?? 0}</p>
                    <Badge variant="success">{t('common.active')}</Badge>
                  </div>
                </CardContent>
              </Card>
              <Card>
                <CardHeader className="flex flex-row items-center justify-between pb-2">
                  <CardTitle className="text-sm font-medium text-muted-foreground">{t('dashboard.total_traffic')}</CardTitle>
                  <ArrowDown className="h-4 w-4 text-primary" />
                </CardHeader>
                <CardContent>
                  <p className="text-2xl font-bold">
                    {stats ? formatBytes(stats.total_traffic_in + stats.total_traffic_out) : '0 B'}
                  </p>
                </CardContent>
              </Card>
              <Card>
                <CardHeader className="flex flex-row items-center justify-between pb-2">
                  <CardTitle className="text-sm font-medium text-muted-foreground">{t('dashboard.active_connections')}</CardTitle>
                  <Users className="h-4 w-4 text-primary" />
                </CardHeader>
                <CardContent>
                  <p className="text-2xl font-bold">{stats?.active_connections ?? 0}</p>
                </CardContent>
              </Card>
              <Card>
                <CardHeader className="flex flex-row items-center justify-between pb-2">
                  <CardTitle className="text-sm font-medium text-muted-foreground">{t('dashboard.today_requests')}</CardTitle>
                  <Zap className="h-4 w-4 text-amber-500" />
                </CardHeader>
                <CardContent>
                  <p className="text-2xl font-bold">{stats?.today_requests?.toLocaleString() ?? '0'}</p>
                </CardContent>
              </Card>
            </>
          )}
        </section>
      )}

      {/* Traffic Chart */}
      <Card>
        <CardHeader>
          <CardTitle>{t('dashboard.traffic_timeline')}</CardTitle>
        </CardHeader>
        <CardContent>
          {showSkeleton ? (
            <Skeleton className="h-64 w-full" />
          ) : (
            <Suspense fallback={<Skeleton className="h-64 w-full" />}>
              <TrafficAreaChart data={mockTraffic} />
            </Suspense>
          )}
        </CardContent>
      </Card>

      {/* Recent Events */}
      <Card>
        <CardHeader>
          <CardTitle>{t('dashboard.recent_events')}</CardTitle>
        </CardHeader>
        <CardContent>
          {eventsQuery.isLoading ? (
            <div className="space-y-3">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          ) : eventsQuery.isError ? (
            <div className="flex flex-col items-center gap-2 py-6 text-center">
              <AlertCircle className="h-8 w-8 text-destructive" />
              <p className="text-sm text-destructive">{t('dashboard.failed_events')}</p>
              <Button variant="outline" size="sm" onClick={() => eventsQuery.refetch()}>
                {t('common.retry')}
              </Button>
            </div>
          ) : eventsQuery.data?.length === 0 || isEmpty ? (
            <div className="flex flex-col items-center gap-3 py-10 text-center">
              <Zap className="h-10 w-10 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">
                {t('dashboard.empty')}
              </p>
              <Link to="/tunnels">
                <Button>
                  <Plus className="mr-1 h-4 w-4" />
                  {t('dashboard.create_first')}
                </Button>
              </Link>
            </div>
          ) : eventsQuery.data ? (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('dashboard.table.time')}</TableHead>
                  <TableHead>{t('dashboard.table.event')}</TableHead>
                  <TableHead>{t('dashboard.table.tunnel')}</TableHead>
                  <TableHead>{t('dashboard.table.status')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {eventsQuery.data.map((event) => (
                  <TableRow key={event.id}>
                    <TableCell className="text-muted-foreground">
                      {new Date(event.created_at).toLocaleString()}
                    </TableCell>
                    <TableCell>
                      <Badge variant={eventStatusVariant[event.status] ?? 'secondary'}>
                        {eventTypeLabel(event.type)}
                      </Badge>
                    </TableCell>
                    <TableCell>{event.tunnel_name ?? event.tunnel_id ?? '—'}</TableCell>
                    <TableCell>
                      <Badge variant={eventStatusVariant[event.status] ?? 'secondary'}>
                        {eventTypeLabel(event.status)}
                      </Badge>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          ) : null}
        </CardContent>
      </Card>
    </div>
  )
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
}
