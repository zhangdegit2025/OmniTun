import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiRequest } from '@/lib/api'
import type { Tunnel } from '@/lib/types'
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from '@/components/ui/Card'
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '@/components/ui/Table'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Dialog } from '@/components/ui/Dialog'
import { Badge } from '@/components/ui/Badge'
import { Skeleton } from '@/components/ui/Skeleton'
import { useToast } from '@/components/ui/useToast'
import { AlertCircle, Plus, Trash2, Globe, RefreshCw, Info, Copy, Check } from 'lucide-react'

interface Domain {
  id: string
  tunnel_id: string
  tunnel_name: string
  domain: string
  status: string
  verification_status: 'pending' | 'verified' | 'failed'
  created_at: string
  dns_instructions?: string
}

type VerificationStatus = 'pending' | 'verified' | 'failed'

const verificationVariant: Record<VerificationStatus, 'warning' | 'success' | 'destructive'> = {
  pending: 'warning',
  verified: 'success',
  failed: 'destructive',
}

export default function Domains() {
  const { t } = useTranslation()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const [showAdd, setShowAdd] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<Domain | null>(null)
  const [showInstructions, setShowInstructions] = useState<Domain | null>(null)
  const [copiedRecord, setCopiedRecord] = useState<string | null>(null)
  const [newDomain, setNewDomain] = useState({ domain: '', tunnel_id: '' })

  const domainsQuery = useQuery<Domain[]>({
    queryKey: ['domains'],
    queryFn: () => apiRequest<Domain[]>('/v1/domains'),
  })

  const tunnelsQuery = useQuery<Tunnel[]>({
    queryKey: ['tunnels'],
    queryFn: () => apiRequest<Tunnel[] | { tunnels: Tunnel[] }>('/v1/tunnels').then(data => {
      if (Array.isArray(data)) return data
      if (data && Array.isArray((data as { tunnels: Tunnel[] }).tunnels)) return (data as { tunnels: Tunnel[] }).tunnels
      return []
    }),
    enabled: showAdd,
  })

  const addDomain = useMutation<Domain, Error, { domain: string; tunnel_id: string }>({
    mutationFn: (input) =>
      apiRequest<Domain>('/v1/domains', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['domains'] })
      setShowAdd(false)
      setNewDomain({ domain: '', tunnel_id: '' })
      setShowInstructions(data)
      toast({ title: t('domains.created'), variant: 'success' })
    },
  })

  const removeDomain = useMutation<void, Error, string>({
    mutationFn: (id) =>
      apiRequest<void>(`/v1/domains/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['domains'] })
      setDeleteTarget(null)
      toast({ title: t('domains.removed'), variant: 'success' })
    },
  })

  const verifyDomain = useMutation<{ verified: boolean; status: string }, Error, string>({
    mutationFn: (id) =>
      apiRequest<{ verified: boolean; status: string }>(`/v1/domains/${id}/verify`, { method: 'POST' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['domains'] })
      toast({ title: t('domains.verify_triggered'), variant: 'success' })
    },
  })

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!newDomain.domain.trim() || !newDomain.tunnel_id) return
    await addDomain.mutateAsync(newDomain)
  }

  const copyToClipboard = async (text: string) => {
    await navigator.clipboard.writeText(text)
    setCopiedRecord(text)
    toast({ title: t('common.copied') })
    setTimeout(() => setCopiedRecord(null), 2000)
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">{t('domains.title')}</h1>
          <p className="text-sm text-muted-foreground">{t('domains.subtitle')}</p>
        </div>
        <Button onClick={() => setShowAdd(true)}>
          <Plus className="mr-1 h-4 w-4" />
          {t('domains.add')}
        </Button>
      </div>

      {/* DNS Instructions Panel */}
      <Card className="border-blue-200 bg-blue-50 dark:border-blue-800 dark:bg-blue-950">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm">
            <Info className="h-4 w-4 text-blue-600" />
            {t('domains.dns_title')}
          </CardTitle>
          <CardDescription>{t('domains.dns_description')}</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-2 text-sm text-blue-800 dark:text-blue-200">
            <p>{t('domains.dns_step1')}</p>
            <p>{t('domains.dns_step2')}</p>
            <p>{t('domains.dns_step3')}</p>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardContent className="p-0">
          {domainsQuery.isLoading ? (
            <div className="space-y-3 p-6">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          ) : domainsQuery.isError ? (
            <div className="flex flex-col items-center gap-2 py-10 text-center">
              <AlertCircle className="h-8 w-8 text-destructive" />
              <p className="text-sm text-destructive">{t('domains.failed_load')}</p>
              <Button variant="outline" size="sm" onClick={() => domainsQuery.refetch()}>
                {t('common.retry')}
              </Button>
            </div>
          ) : (domainsQuery.data?.length ?? 0) === 0 ? (
            <div className="flex flex-col items-center gap-3 py-10 text-center">
              <Globe className="h-10 w-10 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">{t('domains.empty')}</p>
              <Button onClick={() => setShowAdd(true)}>
                <Plus className="mr-1 h-4 w-4" />
                {t('domains.add_first')}
              </Button>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('domains.table.domain')}</TableHead>
                  <TableHead>{t('domains.table.tunnel')}</TableHead>
                  <TableHead>{t('domains.table.verification')}</TableHead>
                  <TableHead>{t('domains.table.created')}</TableHead>
                  <TableHead className="text-right">{t('domains.table.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {domainsQuery.data?.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell className="font-medium">
                      <button
                        type="button"
                        onClick={() => copyToClipboard(item.domain)}
                        className="flex items-center gap-1 text-sm hover:text-foreground"
                      >
                        {item.domain}
                        {copiedRecord === item.domain ? (
                          <Check className="h-3 w-3 text-emerald-500" />
                        ) : (
                          <Copy className="h-3 w-3" />
                        )}
                      </button>
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {item.tunnel_name || item.tunnel_id?.slice(0, 8)}
                    </TableCell>
                    <TableCell>
                      <Badge variant={verificationVariant[item.verification_status]}>
                        {t(`domains.verification.${item.verification_status}`)}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground text-sm">
                      {new Date(item.created_at).toLocaleDateString()}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex items-center justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => verifyDomain.mutate(item.id)}
                          disabled={verifyDomain.isPending}
                        >
                          <RefreshCw className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => setDeleteTarget(item)}
                        >
                          <Trash2 className="h-4 w-4 text-destructive" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
        {domainsQuery.data && domainsQuery.data.length > 10 && (
          <CardFooter className="flex items-center justify-between border-t px-6 py-3">
            <p className="text-sm text-muted-foreground">{domainsQuery.data.length} {t('domains.total')}</p>
          </CardFooter>
        )}
      </Card>

      {/* Add Domain Dialog */}
      <Dialog
        open={showAdd}
        onClose={() => {
          setShowAdd(false)
          setNewDomain({ domain: '', tunnel_id: '' })
        }}
        title={t('domains.add_dialog.title')}
        description={t('domains.add_dialog.description')}
      >
        <form onSubmit={handleAdd} className="flex flex-col gap-4">
          <Input
            label={t('domains.add_dialog.domain')}
            placeholder="www.example.com"
            value={newDomain.domain}
            onChange={(e) => setNewDomain((p) => ({ ...p, domain: e.target.value }))}
          />
          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium">{t('domains.add_dialog.tunnel')}</label>
            <select
              value={newDomain.tunnel_id}
              onChange={(e) => setNewDomain((p) => ({ ...p, tunnel_id: e.target.value }))}
              className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
            >
              <option value="">{t('domains.add_dialog.select_tunnel')}</option>
              {tunnelsQuery.data?.map((tunnel) => (
                <option key={tunnel.id} value={tunnel.id}>
                  {tunnel.name} ({tunnel.protocol?.toUpperCase()})
                </option>
              ))}
            </select>
          </div>
          {addDomain.error && (
            <p className="text-sm text-destructive">
              {(addDomain.error as Error)?.message ?? t('domains.add_dialog.failed_add')}
            </p>
          )}
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="outline" type="button" onClick={() => setShowAdd(false)}>
              {t('domains.add_dialog.cancel')}
            </Button>
            <Button type="submit" disabled={addDomain.isPending}>
              {addDomain.isPending ? t('domains.add_dialog.adding') : t('domains.add_dialog.submit')}
            </Button>
          </div>
        </form>
      </Dialog>

      {/* DNS Instructions Dialog (shown after adding a domain) */}
      <Dialog
        open={!!showInstructions}
        onClose={() => setShowInstructions(null)}
        title={t('domains.instructions_dialog.title')}
        description={t('domains.instructions_dialog.description')}
      >
        <div className="flex flex-col gap-4">
          <div className="rounded-lg border border-blue-200 bg-blue-50 p-4 dark:border-blue-800 dark:bg-blue-950">
            <p className="text-sm font-medium text-blue-800 dark:text-blue-100">
              {t('domains.instructions_dialog.record_type')}: CNAME
            </p>
            <div className="mt-2 space-y-2">
              <div>
                <p className="text-xs text-blue-600 dark:text-blue-400">{t('domains.instructions_dialog.name')}</p>
                <code className="mt-0.5 block rounded bg-white px-3 py-1.5 text-sm dark:bg-background">
                  {showInstructions?.domain ?? ''}
                </code>
              </div>
              <div>
                <p className="text-xs text-blue-600 dark:text-blue-400">{t('domains.instructions_dialog.value')}</p>
                <div className="flex items-center gap-2">
                  <code className="flex-1 rounded bg-white px-3 py-1.5 text-sm dark:bg-background">
                    {showInstructions?.tunnel_id?.slice(0, 8) ?? ''}.omnitun-edge.com
                  </code>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() =>
                      copyToClipboard(`${showInstructions?.tunnel_id?.slice(0, 8) ?? ''}.omnitun-edge.com`)
                    }
                  >
                    {copiedRecord === `${showInstructions?.tunnel_id?.slice(0, 8) ?? ''}.omnitun-edge.com` ? (
                      <Check className="h-3 w-3 text-emerald-500" />
                    ) : (
                      <Copy className="h-3 w-3" />
                    )}
                  </Button>
                </div>
              </div>
            </div>
          </div>
          <p className="text-sm text-muted-foreground">{t('domains.instructions_dialog.note')}</p>
          <Button onClick={() => setShowInstructions(null)}>{t('domains.instructions_dialog.done')}</Button>
        </div>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        title={t('domains.delete_dialog.title')}
        description={t('domains.delete_dialog.message')}
      >
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="outline" onClick={() => setDeleteTarget(null)}>
            {t('domains.delete_dialog.cancel')}
          </Button>
          <Button
            variant="destructive"
            onClick={() => deleteTarget && removeDomain.mutate(deleteTarget.id)}
            disabled={removeDomain.isPending}
          >
            {removeDomain.isPending ? t('domains.delete_dialog.removing') : t('domains.delete_dialog.confirm')}
          </Button>
        </div>
      </Dialog>
    </div>
  )
}
