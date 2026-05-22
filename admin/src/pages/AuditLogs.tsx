import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { apiRequest } from '@/lib/api'
import type { AuditLog } from '@/lib/types'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Skeleton } from '@/components/ui/Skeleton'
import { AlertCircle, Download, Search, ChevronDown, ChevronRight } from 'lucide-react'

const actionBadgeVariant: Record<string, 'default' | 'success' | 'warning' | 'destructive' | 'secondary'> = {
  'user.register': 'success',
  'user.login': 'default',
  'tunnel.create': 'success',
  'tunnel.delete': 'destructive',
  'apikey.create': 'success',
  'apikey.revoke': 'destructive',
  'mesh.delete': 'destructive',
  'mesh.invite_created': 'warning',
  'mesh.node_joined': 'success',
  'mesh.node_removed': 'destructive',
}

export default function AuditLogs() {
  const { t } = useTranslation()
  const [filters, setFilters] = useState({
    org_id: '',
    action: '',
    resource_type: '',
    user_id: '',
    from: '',
    to: '',
  })
  const [page, setPage] = useState(0)
  const [expandedRow, setExpandedRow] = useState<string | null>(null)
  const limit = 50

  const buildQueryString = () => {
    const params = new URLSearchParams()
    if (filters.org_id) params.set('org_id', filters.org_id)
    if (filters.action) params.set('action', filters.action)
    if (filters.resource_type) params.set('resource_type', filters.resource_type)
    if (filters.user_id) params.set('user_id', filters.user_id)
    if (filters.from) params.set('from', filters.from)
    if (filters.to) params.set('to', filters.to)
    params.set('limit', String(limit))
    params.set('offset', String(page * limit))
    return params.toString()
  }

  const logsQuery = useQuery<{ logs: AuditLog[]; total: number }>({
    queryKey: ['admin', 'audit-logs', filters, page],
    queryFn: () => apiRequest(`/api/admin/v1/audit-logs?${buildQueryString()}`),
  })

  const exportUrl = `/api/admin/v1/audit-logs/export?${buildQueryString()}`

  const data = logsQuery.data
  const total = data?.total ?? 0
  const totalPages = Math.ceil(total / limit)
  const showSkeleton = logsQuery.isLoading

  const handleFilterChange = (key: string, value: string) => {
    setFilters((prev) => ({ ...prev, [key]: value }))
    setPage(0)
  }

  const getSeverityBadge = (action: string) => {
    const variant = actionBadgeVariant[action] ?? 'secondary'
    return <Badge variant={variant}>{action}</Badge>
  }

  const toggleRow = (id: string) => {
    setExpandedRow((prev) => (prev === id ? null : id))
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">{t('auditLogs.title')}</h1>
          <p className="text-sm text-muted-foreground">{t('auditLogs.subtitle')}</p>
        </div>
        <Button variant="outline" size="sm" onClick={() => window.open(exportUrl, '_blank')}>
          <Download className="mr-2 h-4 w-4" />
          {t('auditLogs.exportJson')}
        </Button>
      </div>

      <Card>
        <CardContent className="pt-6">
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <Input
              label={t('auditLogs.orgId')}
              placeholder={t('auditLogs.filterByOrg')}
              value={filters.org_id}
              onChange={(e) => handleFilterChange('org_id', e.target.value)}
            />
            <Input
              label={t('auditLogs.action')}
              placeholder={t('auditLogs.filterByAction')}
              value={filters.action}
              onChange={(e) => handleFilterChange('action', e.target.value)}
            />
            <Input
              label={t('auditLogs.resourceType')}
              placeholder={t('auditLogs.filterByResource')}
              value={filters.resource_type}
              onChange={(e) => handleFilterChange('resource_type', e.target.value)}
            />
            <Input
              label={t('auditLogs.userId')}
              placeholder={t('auditLogs.filterByUser')}
              value={filters.user_id}
              onChange={(e) => handleFilterChange('user_id', e.target.value)}
            />
            <Input
              label={t('auditLogs.fromDate')}
              type="datetime-local"
              value={filters.from}
              onChange={(e) => handleFilterChange('from', e.target.value)}
            />
            <Input
              label={t('auditLogs.toDate')}
              type="datetime-local"
              value={filters.to}
              onChange={(e) => handleFilterChange('to', e.target.value)}
            />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>
            {t('auditLogs.results')}{' '}
            {total > 0 && (
              <span className="text-sm font-normal text-muted-foreground">
                ({t('auditLogs.total', { total })})
              </span>
            )}
          </CardTitle>
        </CardHeader>
        <CardContent>
          {logsQuery.isError ? (
            <div className="flex flex-col items-center gap-2 rounded-lg border border-destructive/50 p-6 text-center">
              <AlertCircle className="h-8 w-8 text-destructive" />
              <p className="text-sm text-destructive">{t('auditLogs.failedToLoad')}</p>
              <Button variant="outline" size="sm" onClick={() => logsQuery.refetch()}>
                {t('common.retry')}
              </Button>
            </div>
          ) : (
            <>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-8" />
                    <TableHead>{t('auditLogs.time')}</TableHead>
                    <TableHead>{t('auditLogs.action')}</TableHead>
                    <TableHead>{t('auditLogs.resource')}</TableHead>
                    <TableHead>{t('auditLogs.orgId')}</TableHead>
                    <TableHead>{t('auditLogs.userId')}</TableHead>
                    <TableHead>{t('auditLogs.clientIp')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {showSkeleton
                    ? Array.from({ length: 5 }).map((_, i) => (
                        <TableRow key={i}>
                          <TableCell><Skeleton className="h-4 w-4" /></TableCell>
                          <TableCell><Skeleton className="h-4 w-32" /></TableCell>
                          <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                          <TableCell><Skeleton className="h-4 w-20" /></TableCell>
                          <TableCell><Skeleton className="h-4 w-20" /></TableCell>
                          <TableCell><Skeleton className="h-4 w-20" /></TableCell>
                          <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                        </TableRow>
                      ))
                    : (data?.logs ?? []).map((log) => (
                        <>
                          <TableRow key={log.id} className="cursor-pointer hover:bg-muted/50" onClick={() => toggleRow(log.id)}>
                            <TableCell>
                              {expandedRow === log.id ? (
                                <ChevronDown className="h-4 w-4" />
                              ) : (
                                <ChevronRight className="h-4 w-4" />
                              )}
                            </TableCell>
                            <TableCell className="whitespace-nowrap text-muted-foreground">
                              {new Date(log.created_at).toLocaleString()}
                            </TableCell>
                            <TableCell>{getSeverityBadge(log.action)}</TableCell>
                            <TableCell>
                              <span className="text-xs text-muted-foreground">{log.resource_type}</span>
                              <br />
                              <span className="font-mono text-xs">{log.resource_id?.slice(0, 12)}</span>
                            </TableCell>
                            <TableCell className="font-mono text-xs text-muted-foreground">
                              {log.org_id?.slice(0, 8)}
                            </TableCell>
                            <TableCell className="font-mono text-xs text-muted-foreground">
                              {log.user_id?.slice(0, 8)}
                            </TableCell>
                            <TableCell className="font-mono text-xs text-muted-foreground">
                              {log.client_ip}
                            </TableCell>
                          </TableRow>
                          {expandedRow === log.id && (
                            <TableRow key={`${log.id}-details`}>
                              <TableCell />
                              <TableCell colSpan={6}>
                                <div className="rounded-md bg-muted/50 p-3">
                                  <div className="grid gap-2 text-sm">
                                    <div>
                                      <span className="font-medium text-muted-foreground">{t('auditLogs.fullResourceId')}: </span>
                                      <span className="font-mono">{log.resource_id}</span>
                                    </div>
                                    <div>
                                      <span className="font-medium text-muted-foreground">{t('auditLogs.fullOrgId')}: </span>
                                      <span className="font-mono">{log.org_id}</span>
                                    </div>
                                    <div>
                                      <span className="font-medium text-muted-foreground">{t('auditLogs.fullUserId')}: </span>
                                      <span className="font-mono">{log.user_id}</span>
                                    </div>
                                    {log.details && (
                                      <div>
                                        <span className="font-medium text-muted-foreground">{t('auditLogs.details')}: </span>
                                        <span className="font-mono text-xs">{log.details}</span>
                                      </div>
                                    )}
                                  </div>
                                </div>
                              </TableCell>
                            </TableRow>
                          )}
                        </>
                      ))}
                </TableBody>
              </Table>

              {data?.logs && data.logs.length === 0 && (
                <div className="py-8 text-center text-sm text-muted-foreground">
                  <Search className="mx-auto mb-2 h-6 w-6" />
                  {t('auditLogs.noResults')}
                </div>
              )}

              {totalPages > 1 && (
                <div className="mt-4 flex items-center justify-between">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={page === 0}
                    onClick={() => setPage((p) => p - 1)}
                  >
                    {t('auditLogs.previous')}
                  </Button>
                  <span className="text-sm text-muted-foreground">
                    {t('auditLogs.page', { current: page + 1, total: totalPages })}
                  </span>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={page >= totalPages - 1}
                    onClick={() => setPage((p) => p + 1)}
                  >
                    {t('auditLogs.next')}
                  </Button>
                </div>
              )}
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
