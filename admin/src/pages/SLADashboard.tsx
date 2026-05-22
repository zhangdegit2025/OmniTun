import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { apiRequest } from '@/lib/api'
import type { SLAUptimeData, SLAIncident, SLACreditData } from '@/lib/types'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Skeleton } from '@/components/ui/Skeleton'
import { AlertCircle } from 'lucide-react'

function getUptimeColor(value: number): string {
  if (value >= 99.9) return 'bg-green-500'
  if (value >= 99.0) return 'bg-yellow-500'
  return 'bg-red-500'
}

function getUptimeTextColor(value: number): string {
  if (value >= 99.9) return 'text-green-600 dark:text-green-400'
  if (value >= 99.0) return 'text-yellow-600 dark:text-yellow-400'
  return 'text-red-600 dark:text-red-400'
}

function UptimeBar({
  label,
  value,
  previous,
}: {
  label: string
  value: number
  previous?: number
}) {
  const color = getUptimeColor(value)
  const textColor = getUptimeTextColor(value)
  const change = previous != null ? (value - previous).toFixed(2) : null

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium">{label}</span>
        <div className="flex items-center gap-2">
          <span className={`text-lg font-bold ${textColor}`}>{value}%</span>
          {change != null && (
            <Badge variant={parseFloat(change) >= 0 ? 'success' : 'destructive'} className="text-xs">
              {parseFloat(change) >= 0 ? '+' : ''}
              {change}%
            </Badge>
          )}
        </div>
      </div>
      <div className="h-3 w-full overflow-hidden rounded-full bg-muted">
        <div
          className={`h-full rounded-full transition-all duration-500 ${color}`}
          style={{ width: `${Math.min(value, 100)}%` }}
        />
      </div>
    </div>
  )
}

export default function SLADashboard() {
  const [tab, setTab] = useState<'uptime' | 'incidents' | 'credits'>('uptime')
  const { t } = useTranslation()

  const uptimeQuery = useQuery<SLAUptimeData>({
    queryKey: ['admin', 'sla', 'uptime'],
    queryFn: () => apiRequest('/api/admin/v1/sla/uptime'),
    enabled: tab === 'uptime',
  })

  const incidentsQuery = useQuery<{ incidents: SLAIncident[] }>({
    queryKey: ['admin', 'sla', 'incidents'],
    queryFn: () => apiRequest('/api/admin/v1/sla/incidents'),
    enabled: tab === 'incidents',
  })

  const creditsQuery = useQuery<SLACreditData>({
    queryKey: ['admin', 'sla', 'credits'],
    queryFn: () => apiRequest('/api/admin/v1/sla/credits'),
    enabled: tab === 'credits',
  })

  const tabs = [
    { id: 'uptime' as const, label: t('sla.uptime') },
    { id: 'incidents' as const, label: t('sla.incidents') },
    { id: 'credits' as const, label: t('sla.slaCredits') },
  ]

  return (
    <div className="space-y-6 p-6">
      <div>
        <h1 className="text-2xl font-bold">{t('sla.title')}</h1>
        <p className="text-sm text-muted-foreground">
          {t('sla.subtitle')}
        </p>
      </div>

      <div className="flex gap-2">
        {tabs.map((t) => (
          <Button
            key={t.id}
            variant={tab === t.id ? 'default' : 'outline'}
            size="sm"
            onClick={() => setTab(t.id)}
          >
            {t.label}
          </Button>
        ))}
      </div>

      {tab === 'uptime' && (
        <>
          {uptimeQuery.isLoading ? (
            <Card>
              <CardContent className="space-y-4 pt-6">
                <Skeleton className="h-16 w-full" />
                <Skeleton className="h-16 w-full" />
                <Skeleton className="h-16 w-full" />
              </CardContent>
            </Card>
          ) : uptimeQuery.isError ? (
            <Card>
              <CardContent className="flex flex-col items-center gap-2 py-12 text-center">
                <AlertCircle className="h-8 w-8 text-destructive" />
                <p className="text-sm text-destructive">{t('sla.failedToLoadUptime')}</p>
                <Button variant="outline" size="sm" onClick={() => uptimeQuery.refetch()}>
                  {t('common.retry')}
                </Button>
              </CardContent>
            </Card>
          ) : (
            <>
              <Card>
                <CardHeader>
                  <CardTitle>{t('sla.currentMonth')}</CardTitle>
                </CardHeader>
                <CardContent className="space-y-6">
                  <UptimeBar
                    label={t('sla.apiUptime')}
                    value={uptimeQuery.data?.current_month.api_uptime ?? 0}
                    previous={uptimeQuery.data?.previous_month.api_uptime}
                  />
                  <UptimeBar
                    label={t('sla.tunnelControlPlane')}
                    value={uptimeQuery.data?.current_month.tunnel_control_plane ?? 0}
                    previous={uptimeQuery.data?.previous_month.tunnel_control_plane}
                  />
                  <UptimeBar
                    label={t('sla.relayDataPlane')}
                    value={uptimeQuery.data?.current_month.relay_data_plane ?? 0}
                    previous={uptimeQuery.data?.previous_month.relay_data_plane}
                  />
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>{t('sla.monthlyTrend')}</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="overflow-x-auto">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>{t('sla.month')}</TableHead>
                          <TableHead>{t('sla.apiUptime')}</TableHead>
                          <TableHead>{t('sla.controlPlane')}</TableHead>
                          <TableHead>{t('sla.dataPlane')}</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {uptimeQuery.data?.monthly_trend.map((point) => (
                          <TableRow key={point.month}>
                            <TableCell className="font-medium">{point.month}</TableCell>
                            <TableCell>
                              <span className={getUptimeTextColor(point.api_uptime)}>
                                {point.api_uptime}%
                              </span>
                            </TableCell>
                            <TableCell>
                              <span className={getUptimeTextColor(point.control_plane)}>
                                {point.control_plane}%
                              </span>
                            </TableCell>
                            <TableCell>
                              <span className={getUptimeTextColor(point.data_plane)}>
                                {point.data_plane}%
                              </span>
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                </CardContent>
              </Card>
            </>
          )}
        </>
      )}

      {tab === 'incidents' && (
        <>
          {incidentsQuery.isLoading ? (
            <Card>
              <CardContent className="space-y-2 pt-6">
                {Array.from({ length: 5 }).map((_, i) => (
                  <Skeleton key={i} className="h-10 w-full" />
                ))}
              </CardContent>
            </Card>
          ) : incidentsQuery.isError ? (
            <Card>
              <CardContent className="flex flex-col items-center gap-2 py-12 text-center">
                <AlertCircle className="h-8 w-8 text-destructive" />
                <p className="text-sm text-destructive">{t('sla.failedToLoadIncidents')}</p>
                <Button variant="outline" size="sm" onClick={() => incidentsQuery.refetch()}>
                  {t('common.retry')}
                </Button>
              </CardContent>
            </Card>
          ) : (
            <Card>
              <CardHeader>
                <CardTitle>{t('sla.incidentLog')}</CardTitle>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('sla.date')}</TableHead>
                      <TableHead>{t('sla.duration')}</TableHead>
                      <TableHead>{t('sla.impact')}</TableHead>
                      <TableHead>{t('sla.rootCause')}</TableHead>
                      <TableHead>{t('sla.services')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {incidentsQuery.data?.incidents.map((inc) => (
                      <TableRow key={inc.id}>
                        <TableCell className="whitespace-nowrap">
                          {new Date(inc.date).toLocaleDateString()}
                        </TableCell>
                        <TableCell>
                          <Badge
                            variant={
                              inc.duration_minutes < 15
                                ? 'success'
                                : inc.duration_minutes < 60
                                  ? 'warning'
                                  : 'destructive'
                            }
                          >
                            {inc.duration}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-sm">{inc.impact}</TableCell>
                        <TableCell className="max-w-[250px] truncate text-sm text-muted-foreground">
                          {inc.root_cause}
                        </TableCell>
                        <TableCell>
                          <div className="flex flex-wrap gap-1">
                            {inc.affected_services.map((svc) => (
                              <Badge key={svc} variant="secondary" className="text-xs">
                                {svc}
                              </Badge>
                            ))}
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          )}
        </>
      )}

      {tab === 'credits' && (
        <>
          {creditsQuery.isLoading ? (
            <Card>
              <CardContent className="space-y-2 pt-6">
                <Skeleton className="h-16 w-full" />
                <Skeleton className="h-32 w-full" />
              </CardContent>
            </Card>
          ) : creditsQuery.isError ? (
            <Card>
              <CardContent className="flex flex-col items-center gap-2 py-12 text-center">
                <AlertCircle className="h-8 w-8 text-destructive" />
                <p className="text-sm text-destructive">{t('sla.failedToLoadCredits')}</p>
                <Button variant="outline" size="sm" onClick={() => creditsQuery.refetch()}>
                  {t('common.retry')}
                </Button>
              </CardContent>
            </Card>
          ) : (
            <>
              <Card>
                <CardHeader>
                  <CardTitle>{t('sla.slaStatusCurrent')}</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="grid gap-4 sm:grid-cols-2">
                    <div className="rounded-md bg-muted/50 p-4">
                      <p className="text-sm text-muted-foreground">{t('sla.actualUptime')}</p>
                      <p className={`text-2xl font-bold ${getUptimeTextColor(creditsQuery.data?.current_month.actual_uptime ?? 0)}`}>
                        {creditsQuery.data?.current_month.actual_uptime}%
                      </p>
                    </div>
                    <div className="rounded-md bg-muted/50 p-4">
                      <p className="text-sm text-muted-foreground">{t('sla.slaThreshold')}</p>
                      <p className="text-2xl font-bold">{creditsQuery.data?.current_month.breach_threshold}%</p>
                    </div>
                    <div className="rounded-md bg-muted/50 p-4">
                      <p className="text-sm text-muted-foreground">{t('sla.slaBreached')}</p>
                      <p className="text-2xl font-bold">
                        {creditsQuery.data?.current_month.sla_breached ? (
                          <span className="text-destructive">{t('common.yes')}</span>
                        ) : (
                          <span className="text-green-500">{t('common.no')}</span>
                        )}
                      </p>
                    </div>
                    <div className="rounded-md bg-muted/50 p-4">
                      <p className="text-sm text-muted-foreground">{t('sla.creditsOwed')}</p>
                      <p className="text-2xl font-bold">
                        ${creditsQuery.data?.current_month.credits_owed?.toLocaleString() ?? '0'}
                      </p>
                    </div>
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>{t('sla.creditHistory')}</CardTitle>
                </CardHeader>
                <CardContent>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t('sla.month')}</TableHead>
                        <TableHead>{t('sla.uptime')}</TableHead>
                        <TableHead>{t('sla.slaMet')}</TableHead>
                        <TableHead>{t('sla.creditsOwed')}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {creditsQuery.data?.history.map((h) => (
                        <TableRow key={h.month}>
                          <TableCell className="font-medium">{h.month}</TableCell>
                          <TableCell>
                            <span className={getUptimeTextColor(h.uptime)}>{h.uptime}%</span>
                          </TableCell>
                          <TableCell>
                            <Badge variant={h.sla_met ? 'success' : 'destructive'}>
                              {h.sla_met ? t('sla.met') : t('sla.breached')}
                            </Badge>
                          </TableCell>
                          <TableCell>
                            {h.credits > 0 ? (
                              <span className="font-medium text-destructive">
                                ${h.credits.toLocaleString()}
                              </span>
                            ) : (
                              <span className="text-muted-foreground">$0</span>
                            )}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                  <div className="mt-4 text-right text-sm font-medium">
                    {t('sla.totalCreditsYtd')}:{' '}
                    <span className="text-destructive">
                      ${creditsQuery.data?.total_credits_ytd?.toLocaleString() ?? '0'}
                    </span>
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>{t('sla.creditFormula')}</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="grid gap-2">
                    {Object.entries(creditsQuery.data?.credit_formula ?? {}).map(([range, credit]) => (
                      <div key={range} className="flex items-center justify-between rounded-md bg-muted/50 px-3 py-2 text-sm">
                        <span className="font-mono">{range}%</span>
                        <span>{credit}</span>
                      </div>
                    ))}
                    {Object.entries(creditsQuery.data?.sla_thresholds ?? {}).map(([svc, threshold]) => (
                      <div key={svc} className="flex items-center justify-between rounded-md bg-muted/50 px-3 py-2 text-sm">
                        <span className="capitalize">{svc.replace('_', ' ')} threshold</span>
                        <span className="font-mono">{threshold}%</span>
                      </div>
                    ))}
                  </div>
                </CardContent>
              </Card>
            </>
          )}
        </>
      )}
    </div>
  )
}
