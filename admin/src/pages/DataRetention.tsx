import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { apiRequest } from '@/lib/api'
import type { RetentionData } from '@/lib/types'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Skeleton } from '@/components/ui/Skeleton'
import { AlertCircle, Save, Trash2, Clock } from 'lucide-react'

const retentionOptions = [
  { value: '30d', label: '30 Days' },
  { value: '90d', label: '90 Days' },
  { value: '1y', label: '1 Year' },
  { value: '3y', label: '3 Years' },
]

export default function DataRetention() {
  const queryClient = useQueryClient()
  const { t } = useTranslation()
  const [settings, setSettings] = useState<Record<string, string>>({})

  const retentionQuery = useQuery<RetentionData>({
    queryKey: ['admin', 'retention'],
    queryFn: () => apiRequest('/api/admin/v1/retention'),
  })

  const updateMutation = useMutation({
    mutationFn: (data: Record<string, string>) =>
      apiRequest('/api/admin/v1/retention', {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'retention'] })
    },
  })

  const cleanupMutation = useMutation({
    mutationFn: () =>
      apiRequest('/api/admin/v1/retention/cleanup', { method: 'POST' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'retention'] })
    },
  })

  const data = retentionQuery.data
  const usage = data?.usage
  const planLimits = data?.plan_limits ?? {}

  if (!retentionQuery.isLoading && data && Object.keys(settings).length === 0) {
    setSettings({ ...data.settings })
  }

  const handleSelectChange = (key: string, value: string) => {
    setSettings((prev) => ({ ...prev, [key]: value }))
  }

  const handleSave = (dataType: string) => {
    updateMutation.mutate({ [dataType]: settings[dataType] })
  }

  const formatBytes = (bytes: number) => {
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} KB`
    if (bytes < 1073741824) return `${(bytes / 1048576).toFixed(1)} MB`
    return `${(bytes / 1073741824).toFixed(1)} GB`
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">{t('dataRetention.title')}</h1>
          <p className="text-sm text-muted-foreground">
            {t('dataRetention.subtitle')}
          </p>
        </div>
        <Button
          variant="outline"
          onClick={() => cleanupMutation.mutate()}
          disabled={cleanupMutation.isPending}
        >
          <Trash2 className="mr-2 h-4 w-4" />
          {cleanupMutation.isPending ? t('dataRetention.running') : t('dataRetention.triggerCleanup')}
        </Button>
      </div>

      {retentionQuery.isLoading ? (
        <Card>
          <CardContent className="space-y-4 pt-6">
            <Skeleton className="h-16 w-full" />
            <Skeleton className="h-16 w-full" />
            <Skeleton className="h-16 w-full" />
          </CardContent>
        </Card>
      ) : retentionQuery.isError ? (
        <Card>
          <CardContent className="flex flex-col items-center gap-2 py-12 text-center">
            <AlertCircle className="h-8 w-8 text-destructive" />
            <p className="text-sm text-destructive">{t('dataRetention.failedToLoad')}</p>
            <Button variant="outline" size="sm" onClick={() => retentionQuery.refetch()}>
              {t('common.retry')}
            </Button>
          </CardContent>
        </Card>
      ) : (
        <>
          <Card>
            <CardContent className="pt-6">
              <div className="mb-4 flex items-center gap-2 text-sm text-muted-foreground">
                <Clock className="h-4 w-4" />
                {t('dataRetention.planLimits')}
              </div>
              <div className="grid gap-3 text-xs">
                {Object.entries(planLimits).map(([plan, limit]) => (
                  <div key={plan} className="flex items-center gap-2">
                    <Badge variant="secondary" className="capitalize">
                      {plan}
                    </Badge>
                    <span className="text-muted-foreground">{t('dataRetention.max', { limit })}</span>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>

          {[
            {
              key: 'audit_logs',
              label: t('dataRetention.auditLogs'),
              description: t('dataRetention.auditLogsDesc'),
            },
            {
              key: 'traffic_events',
              label: t('dataRetention.trafficEvents'),
              description: t('dataRetention.trafficEventsDesc'),
            },
            {
              key: 'user_sessions',
              label: t('dataRetention.userSessions'),
              description: t('dataRetention.userSessionsDesc'),
            },
          ].map(({ key, label, description }) => {
            const usageItem = usage?.[key as keyof typeof usage]
            return (
              <Card key={key}>
                <CardHeader>
                  <CardTitle>{label}</CardTitle>
                </CardHeader>
                <CardContent>
                  <p className="mb-4 text-sm text-muted-foreground">{description}</p>

                  {usageItem && (
                    <div className="mb-4 grid gap-2 rounded-md bg-muted/50 p-3 text-sm">
                      <div className="flex justify-between">
                        <span className="text-muted-foreground">{t('dataRetention.currentRecords')}:</span>
                        <span className="font-mono font-medium">
                          {usageItem.count.toLocaleString()}
                        </span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-muted-foreground">{t('dataRetention.storageUsed')}:</span>
                        <span className="font-mono font-medium">
                          {formatBytes(usageItem.size_bytes)}
                        </span>
                      </div>
                    </div>
                  )}

                  <div className="flex items-end gap-3">
                    <div className="flex-1">
                      <label className="mb-1 block text-xs font-medium text-muted-foreground">
                        {t('dataRetention.retentionPeriod')}
                      </label>
                      <select
                        className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                        value={settings[key] ?? ''}
                        onChange={(e) => handleSelectChange(key, e.target.value)}
                      >
                        {retentionOptions.map((opt) => (
                          <option key={opt.value} value={opt.value}>
                            {opt.label}
                          </option>
                        ))}
                      </select>
                    </div>
                    <Button
                      size="sm"
                      onClick={() => handleSave(key)}
                      disabled={updateMutation.isPending || settings[key] === (data as any)?.settings?.[key]}
                    >
                      <Save className="mr-1 h-4 w-4" />
                      {t('common.save')}
                    </Button>
                  </div>
                </CardContent>
              </Card>
            )
          })}

          {data?.last_cleanup && (
            <Card>
              <CardContent className="flex items-center gap-2 py-4 text-sm text-muted-foreground">
                <Clock className="h-4 w-4" />
                {t('dataRetention.lastCleanup', { date: new Date(data.last_cleanup).toLocaleString() })}
              </CardContent>
            </Card>
          )}

          {cleanupMutation.data && (
            <Card className="border-green-200 bg-green-50 dark:border-green-800 dark:bg-green-950">
              <CardContent className="py-4">
                <p className="text-sm font-medium text-green-800 dark:text-green-200">
                  {t('dataRetention.cleanupSuccess')}
                </p>
                <pre className="mt-2 text-xs text-green-700 dark:text-green-300">
                  {JSON.stringify(cleanupMutation.data, null, 2)}
                </pre>
              </CardContent>
            </Card>
          )}
        </>
      )}
    </div>
  )
}
