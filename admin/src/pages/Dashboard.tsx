import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { apiRequest } from '@/lib/api'
import type { DashboardStats, RecentSignup } from '@/lib/types'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card'
import { Skeleton } from '@/components/ui/Skeleton'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import {
  AlertCircle,
  Building2,
  Activity,
  ArrowDownUp,
  Server,
} from 'lucide-react'

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
}

const mockRecentSignups: RecentSignup[] = [
  { id: '1', org_name: 'Acme Corp', email: 'admin@acme.com', plan: 'pro', created_at: '2026-05-21T10:30:00Z' },
  { id: '2', org_name: 'Globex Inc', email: 'hello@globex.io', plan: 'enterprise', created_at: '2026-05-21T09:15:00Z' },
  { id: '3', org_name: 'Initech', email: 'ops@initech.dev', plan: 'free', created_at: '2026-05-20T16:45:00Z' },
  { id: '4', org_name: 'Umbrella', email: 'support@umbrella.com', plan: 'pro', created_at: '2026-05-20T14:20:00Z' },
  { id: '5', org_name: 'Stark Industries', email: 'tony@stark.net', plan: 'enterprise', created_at: '2026-05-20T08:00:00Z' },
]

const planBadgeVariant: Record<string, 'default' | 'success' | 'warning' | 'secondary'> = {
  free: 'secondary',
  pro: 'default',
  enterprise: 'success',
}

export default function Dashboard() {
  const { t } = useTranslation()
  const statsQuery = useQuery<DashboardStats>({
    queryKey: ['admin', 'dashboard', 'stats'],
    queryFn: () => apiRequest<DashboardStats>('/api/admin/v1/dashboard/metrics'),
  })

  const stats = statsQuery.data
  const showSkeleton = statsQuery.isLoading

  return (
    <div className="space-y-6 p-6">
      <div>
        <h1 className="text-2xl font-bold">{t('dashboard.title')}</h1>
        <p className="text-sm text-muted-foreground">{t('dashboard.subtitle')}</p>
      </div>

      {statsQuery.isError ? (
        <div className="flex flex-col items-center gap-2 rounded-lg border border-destructive/50 p-6 text-center">
          <AlertCircle className="h-8 w-8 text-destructive" />
          <p className="text-sm text-destructive">{t('dashboard.failedToLoad')}</p>
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
                  <CardTitle className="text-sm font-medium text-muted-foreground">{t('dashboard.totalOrgs')}</CardTitle>
                  <Building2 className="h-4 w-4 text-primary" />
                </CardHeader>
                <CardContent>
                  <div className="flex items-baseline gap-2">
                    <p className="text-2xl font-bold">{stats?.total_orgs ?? 0}</p>
                    <Badge variant="success">{t('dashboard.active')}</Badge>
                  </div>
                </CardContent>
              </Card>
              <Card>
                <CardHeader className="flex flex-row items-center justify-between pb-2">
                  <CardTitle className="text-sm font-medium text-muted-foreground">{t('dashboard.activeTunnels')}</CardTitle>
                  <Activity className="h-4 w-4 text-emerald-500" />
                </CardHeader>
                <CardContent>
                  <p className="text-2xl font-bold">{stats?.active_tunnels ?? 0}</p>
                </CardContent>
              </Card>
              <Card>
                <CardHeader className="flex flex-row items-center justify-between pb-2">
                  <CardTitle className="text-sm font-medium text-muted-foreground">{t('dashboard.todayTraffic')}</CardTitle>
                  <ArrowDownUp className="h-4 w-4 text-primary" />
                </CardHeader>
                <CardContent>
                  <p className="text-2xl font-bold">
                    {stats ? formatBytes(stats.today_traffic_bytes) : '0 B'}
                  </p>
                </CardContent>
              </Card>
              <Card>
                <CardHeader className="flex flex-row items-center justify-between pb-2">
                  <CardTitle className="text-sm font-medium text-muted-foreground">{t('dashboard.activeRelays')}</CardTitle>
                  <Server className="h-4 w-4 text-amber-500" />
                </CardHeader>
                <CardContent>
                  <p className="text-2xl font-bold">{stats?.active_relays ?? 0}</p>
                </CardContent>
              </Card>
            </>
          )}
        </section>
      )}

      <section className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>{t('dashboard.recentSignups')}</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('dashboard.organization')}</TableHead>
                  <TableHead>{t('common.plan')}</TableHead>
                  <TableHead>{t('dashboard.date')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {mockRecentSignups.map((signup) => (
                  <TableRow key={signup.id}>
                    <TableCell>
                      <div>
                        <p className="font-medium">{signup.org_name}</p>
                        <p className="text-xs text-muted-foreground">{signup.email}</p>
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={planBadgeVariant[signup.plan] ?? 'secondary'}>
                        {signup.plan}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {new Date(signup.created_at).toLocaleDateString()}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t('dashboard.systemHealth')}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              <div className="flex items-center justify-between rounded-md border border-border p-3">
                <div className="flex items-center gap-2">
                  <div className="h-2.5 w-2.5 rounded-full bg-emerald-500" />
                  <span className="text-sm font-medium">{t('dashboard.apiServer')}</span>
                </div>
                <span className="text-xs text-muted-foreground">{t('dashboard.uptime')}</span>
              </div>
              <div className="flex items-center justify-between rounded-md border border-border p-3">
                <div className="flex items-center gap-2">
                  <div className="h-2.5 w-2.5 rounded-full bg-emerald-500" />
                  <span className="text-sm font-medium">{t('dashboard.database')}</span>
                </div>
                <span className="text-xs text-muted-foreground">{t('dashboard.uptime')}</span>
              </div>
              <div className="flex items-center justify-between rounded-md border border-border p-3">
                <div className="flex items-center gap-2">
                  <div className="h-2.5 w-2.5 rounded-full bg-amber-500" />
                  <span className="text-sm font-medium">{t('dashboard.relayMesh')}</span>
                </div>
                <span className="text-xs text-muted-foreground">{t('dashboard.uptimeRelayMesh')}</span>
              </div>
              <div className="flex items-center justify-between rounded-md border border-border p-3">
                <div className="flex items-center gap-2">
                  <div className="h-2.5 w-2.5 rounded-full bg-emerald-500" />
                  <span className="text-sm font-medium">{t('dashboard.webSocketGateway')}</span>
                </div>
                <span className="text-xs text-muted-foreground">{t('dashboard.uptime')}</span>
              </div>
            </div>
          </CardContent>
        </Card>
      </section>
    </div>
  )
}
