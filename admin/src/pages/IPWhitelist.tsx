import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { apiRequest } from '@/lib/api'
import type { IPWhitelistConfig } from '@/lib/types'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Skeleton } from '@/components/ui/Skeleton'
import { AlertCircle, Plus, Trash2, Shield, ShieldOff } from 'lucide-react'

const cidrRegex = /^(\d{1,3}\.){3}\d{1,3}(\/\d{1,2})?$/

function validateCIDR(value: string): boolean {
  if (!cidrRegex.test(value)) return false
  const parts = value.split('/')[0].split('.')
  return parts.every((p) => {
    const n = parseInt(p, 10)
    return n >= 0 && n <= 255
  })
}

export default function IPWhitelist() {
  const queryClient = useQueryClient()
  const { t } = useTranslation()
  const [orgId, setOrgId] = useState('')
  const [newCIDR, setNewCIDR] = useState('')
  const [newLabel, setNewLabel] = useState('')
  const [addError, setAddError] = useState('')

  const enabledOrgId = orgId.trim() || '_'

  const whitelistQuery = useQuery<IPWhitelistConfig>({
    queryKey: ['admin', 'ip-whitelist', enabledOrgId],
    queryFn: () =>
      apiRequest(`/api/admin/v1/organizations/${enabledOrgId}/ip-whitelist`),
    enabled: orgId.trim().length > 0,
  })

  const addMutation = useMutation({
    mutationFn: (data: { cidr: string; label: string }) =>
      apiRequest(`/api/admin/v1/organizations/${orgId}/ip-whitelist`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      setNewCIDR('')
      setNewLabel('')
      setAddError('')
      queryClient.invalidateQueries({ queryKey: ['admin', 'ip-whitelist'] })
    },
    onError: (err: Error) => {
      setAddError(err.message)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (entryId: string) =>
      apiRequest(`/api/admin/v1/organizations/${orgId}/ip-whitelist/${entryId}`, {
        method: 'DELETE',
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'ip-whitelist'] })
    },
  })

  const handleAdd = () => {
    if (!validateCIDR(newCIDR)) {
      setAddError(t('ipWhitelist.invalidCidr'))
      return
    }
    addMutation.mutate({
      cidr: newCIDR,
      label: newLabel || newCIDR,
    })
  }

  const entries = whitelistQuery.data?.entries ?? []
  const enabled = whitelistQuery.data?.enabled ?? false

  return (
    <div className="space-y-6 p-6">
      <div>
        <h1 className="text-2xl font-bold">{t('ipWhitelist.title')}</h1>
        <p className="text-sm text-muted-foreground">
          {t('ipWhitelist.subtitle')}
        </p>
      </div>

      <Card>
        <CardContent className="pt-6">
          <Input
            label={t('ipWhitelist.orgId')}
            placeholder={t('ipWhitelist.orgIdPlaceholder')}
            value={orgId}
            onChange={(e) => setOrgId(e.target.value)}
          />
        </CardContent>
      </Card>

      {orgId.trim().length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center gap-3 py-12 text-center">
            <Shield className="h-12 w-12 text-muted-foreground" />
            <p className="text-sm text-muted-foreground">
              {t('ipWhitelist.enterOrgPrompt')}
            </p>
          </CardContent>
        </Card>
      ) : (
        <>
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                {enabled ? (
                  <Shield className="h-5 w-5 text-green-500" />
                ) : (
                  <ShieldOff className="h-5 w-5 text-muted-foreground" />
                )}
                {t('ipWhitelist.whitelistStatus')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="flex items-center gap-4">
                <Badge variant={enabled ? 'success' : 'secondary'}>
                  {enabled ? t('ipWhitelist.enforcementEnabled') : t('ipWhitelist.enforcementDisabled')}
                </Badge>
                <span className="text-sm text-muted-foreground">
                  {enabled
                    ? t('ipWhitelist.enforcementEnabledDesc')
                    : t('ipWhitelist.enforcementDisabledDesc')}
                </span>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>{t('ipWhitelist.addIpCidr')}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="flex flex-wrap items-end gap-3">
                <div className="min-w-[200px] flex-1">
                  <Input
                    label={t('ipWhitelist.ipOrCidr')}
                    placeholder={t('ipWhitelist.ipCidrPlaceholder')}
                    value={newCIDR}
                    onChange={(e) => {
                      setNewCIDR(e.target.value)
                      setAddError('')
                    }}
                  />
                </div>
                <div className="min-w-[150px] flex-1">
                  <Input
                    label={t('ipWhitelist.labelOptional')}
                    placeholder={t('ipWhitelist.labelPlaceholder')}
                    value={newLabel}
                    onChange={(e) => setNewLabel(e.target.value)}
                  />
                </div>
                <Button
                  onClick={handleAdd}
                  disabled={addMutation.isPending || !newCIDR.trim()}
                >
                  <Plus className="mr-1 h-4 w-4" />
                  {addMutation.isPending ? t('ipWhitelist.adding') : t('ipWhitelist.add')}
                </Button>
              </div>
              {addError && (
                <p className="mt-2 text-sm text-destructive">{addError}</p>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>
                {t('ipWhitelist.whitelistEntries')}
                {entries.length > 0 && (
                  <span className="ml-2 text-sm font-normal text-muted-foreground">
                    ({t('ipWhitelist.entries_other', { count: entries.length })})
                  </span>
                )}
              </CardTitle>
            </CardHeader>
            <CardContent>
              {whitelistQuery.isLoading ? (
                <div className="space-y-2">
                  {Array.from({ length: 3 }).map((_, i) => (
                    <Skeleton key={i} className="h-10 w-full" />
                  ))}
                </div>
              ) : whitelistQuery.isError ? (
                <div className="flex flex-col items-center gap-2 rounded-lg border border-destructive/50 p-6 text-center">
                  <AlertCircle className="h-8 w-8 text-destructive" />
                  <p className="text-sm text-destructive">{t('ipWhitelist.failedToLoad')}</p>
                  <Button variant="outline" size="sm" onClick={() => whitelistQuery.refetch()}>
                    {t('common.retry')}
                  </Button>
                </div>
              ) : entries.length === 0 ? (
                <div className="py-8 text-center text-sm text-muted-foreground">
                  {t('ipWhitelist.noEntries')}
                </div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('ipWhitelist.ipCidr')}</TableHead>
                      <TableHead>{t('ipWhitelist.label')}</TableHead>
                      <TableHead>{t('ipWhitelist.added')}</TableHead>
                      <TableHead className="w-20" />
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {entries.map((entry) => (
                      <TableRow key={entry.id}>
                        <TableCell>
                          <code className="rounded bg-muted px-1.5 py-0.5 text-xs font-mono">
                            {entry.cidr}
                          </code>
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                          {entry.label}
                        </TableCell>
                        <TableCell className="whitespace-nowrap text-sm text-muted-foreground">
                          {new Date(entry.created_at).toLocaleDateString()}
                        </TableCell>
                        <TableCell>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => deleteMutation.mutate(entry.id)}
                            disabled={deleteMutation.isPending}
                          >
                            <Trash2 className="h-4 w-4 text-destructive" />
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>

          {enabled && (
            <div className="rounded-lg border border-amber-200 bg-amber-50 p-4 dark:border-amber-800 dark:bg-amber-950">
              <p className="text-sm text-amber-800 dark:text-amber-200">
                <strong>{t('ipWhitelist.note')}</strong>
              </p>
            </div>
          )}
        </>
      )}
    </div>
  )
}
