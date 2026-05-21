import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiRequest } from '@/lib/api'
import type { Webhook, WebhookDelivery } from '@/lib/types'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/Card'
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
import {
  AlertCircle,
  Plus,
  Trash2,
  Edit,
  Copy,
  Check,
  RefreshCw,
  Send,
  ChevronDown,
  ChevronRight,
  Webhook as WebhookIcon,
} from 'lucide-react'

const WEBHOOK_EVENTS = [
  'tunnel.started',
  'tunnel.stopped',
  'tunnel.error',
  'cert.expiring',
  'quota.warning',
  'org.member_joined',
] as const

function generateSecret(): string {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'
  let result = 'whsec_'
  for (let i = 0; i < 32; i++) {
    result += chars.charAt(Math.floor(Math.random() * chars.length))
  }
  return result
}

export default function Webhooks() {
  const { t } = useTranslation()
  const { toast } = useToast()
  const queryClient = useQueryClient()

  const [showCreate, setShowCreate] = useState(false)
  const [editTarget, setEditTarget] = useState<Webhook | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<Webhook | null>(null)
  const [deliveriesTarget, setDeliveriesTarget] = useState<Webhook | null>(null)
  const [expandedDelivery, setExpandedDelivery] = useState<string | null>(null)
  const [copiedSecret, setCopiedSecret] = useState(false)

  const [form, setForm] = useState({
    name: '',
    url: '',
    events: [] as string[],
    secret: generateSecret(),
  })

  const webhooksQuery = useQuery<Webhook[]>({
    queryKey: ['webhooks'],
    queryFn: () => apiRequest<Webhook[]>('/v1/webhooks'),
  })

  const deliveriesQuery = useQuery<WebhookDelivery[]>({
    queryKey: ['webhooks', deliveriesTarget?.id, 'deliveries'],
    queryFn: () => apiRequest<WebhookDelivery[]>(`/v1/webhooks/${deliveriesTarget!.id}/deliveries`),
    enabled: !!deliveriesTarget,
  })

  const createMutation = useMutation<Webhook, Error, typeof form>({
    mutationFn: (data) =>
      apiRequest<Webhook>('/v1/webhooks', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['webhooks'] })
      setShowCreate(false)
      resetForm()
      toast({ title: t('webhooks.created'), variant: 'success' })
    },
  })

  const updateMutation = useMutation<Webhook, Error, { id: string } & typeof form>({
    mutationFn: ({ id, ...data }) =>
      apiRequest<Webhook>(`/v1/webhooks/${id}`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['webhooks'] })
      setEditTarget(null)
      toast({ title: t('webhooks.updated'), variant: 'success' })
    },
  })

  const deleteMutation = useMutation<void, Error, string>({
    mutationFn: (id) => apiRequest<void>(`/v1/webhooks/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['webhooks'] })
      setDeleteTarget(null)
      toast({ title: t('webhooks.deleted'), variant: 'success' })
    },
  })

  const testMutation = useMutation<{ status_code: number; body: string }, Error, string>({
    mutationFn: (id) =>
      apiRequest<{ status_code: number; body: string }>(`/v1/webhooks/${id}/test`, { method: 'POST' }),
    onSuccess: (data) => {
      toast({
        title: t('webhooks.create_dialog.test_sent', { status: data.status_code }),
        variant: data.status_code >= 200 && data.status_code < 300 ? 'success' : 'warning',
      })
    },
  })

  function resetForm() {
    setForm({ name: '', url: '', events: [], secret: generateSecret() })
  }

  function openEdit(webhook: Webhook) {
    setForm({
      name: webhook.name,
      url: webhook.url,
      events: webhook.events,
      secret: webhook.secret,
    })
    setEditTarget(webhook)
  }

  function openCreate() {
    resetForm()
    setShowCreate(true)
  }

  function toggleEvent(event: string) {
    setForm((prev) => ({
      ...prev,
      events: prev.events.includes(event)
        ? prev.events.filter((e) => e !== event)
        : [...prev.events, event],
    }))
  }

  function regenerateSecret() {
    setForm((prev) => ({ ...prev, secret: generateSecret() }))
  }

  async function copySecret() {
    await navigator.clipboard.writeText(form.secret)
    setCopiedSecret(true)
    toast({ title: t('common.copied'), variant: 'success' })
    setTimeout(() => setCopiedSecret(false), 2000)
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (editTarget) {
      await updateMutation.mutateAsync({ id: editTarget.id, ...form })
    } else {
      await createMutation.mutateAsync(form)
    }
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">{t('webhooks.title')}</h1>
          <p className="text-sm text-muted-foreground">{t('webhooks.subtitle')}</p>
        </div>
        <Button onClick={openCreate}>
          <Plus className="mr-1 h-4 w-4" />
          {t('webhooks.create')}
        </Button>
      </div>

      <Card>
        <CardContent className="p-0">
          {webhooksQuery.isLoading ? (
            <div className="space-y-3 p-6">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          ) : webhooksQuery.isError ? (
            <div className="flex flex-col items-center gap-2 py-10 text-center">
              <AlertCircle className="h-8 w-8 text-destructive" />
              <p className="text-sm text-destructive">{t('webhooks.failed_load')}</p>
              <Button variant="outline" size="sm" onClick={() => webhooksQuery.refetch()}>
                {t('common.retry')}
              </Button>
            </div>
          ) : !webhooksQuery.data?.length ? (
            <div className="flex flex-col items-center gap-3 py-10 text-center">
              <WebhookIcon className="h-10 w-10 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">{t('webhooks.empty')}</p>
              <Button onClick={openCreate}>
                <Plus className="mr-1 h-4 w-4" />
                {t('webhooks.create')}
              </Button>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('webhooks.table.name')}</TableHead>
                  <TableHead>{t('webhooks.table.url')}</TableHead>
                  <TableHead>{t('webhooks.table.events')}</TableHead>
                  <TableHead>{t('webhooks.table.status')}</TableHead>
                  <TableHead>{t('webhooks.table.last_delivery')}</TableHead>
                  <TableHead className="text-right">{t('webhooks.table.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {webhooksQuery.data.map((webhook) => (
                  <TableRow key={webhook.id}>
                    <TableCell className="font-medium">{webhook.name}</TableCell>
                    <TableCell className="max-w-[200px] truncate text-xs font-mono text-muted-foreground">
                      {webhook.url}
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {webhook.events.map((ev) => (
                          <Badge key={ev} variant="secondary" className="text-xs">
                            {(t as any)(`webhooks.events.${ev}`) || ev}
                          </Badge>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={webhook.status === 'active' ? 'success' : webhook.status === 'failed' ? 'destructive' : 'secondary'}>
                        {t(`webhooks.status.${webhook.status}`)}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground text-xs">
                      {webhook.last_delivery_at
                        ? new Date(webhook.last_delivery_at).toLocaleString()
                        : '—'}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex items-center justify-end gap-1">
                        <Button variant="ghost" size="sm" onClick={() => testMutation.mutate(webhook.id)} disabled={testMutation.isPending}>
                          <Send className="h-4 w-4" />
                        </Button>
                        <Button variant="ghost" size="sm" onClick={() => setDeliveriesTarget(webhook)}>
                          <ChevronDown className="h-4 w-4" />
                        </Button>
                        <Button variant="ghost" size="sm" onClick={() => openEdit(webhook)}>
                          <Edit className="h-4 w-4" />
                        </Button>
                        <Button variant="ghost" size="sm" onClick={() => setDeleteTarget(webhook)}>
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
      </Card>

      {/* Delivery Log Panel */}
      {deliveriesTarget && (
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0">
            <div>
              <CardTitle className="text-base">
                {t('webhooks.deliveries.title')}: {deliveriesTarget.name}
              </CardTitle>
              <CardDescription>{t('webhooks.deliveries.recent')}</CardDescription>
            </div>
            <Button variant="ghost" size="sm" onClick={() => { setDeliveriesTarget(null); setExpandedDelivery(null) }}>
              <ChevronRight className="h-4 w-4" />
            </Button>
          </CardHeader>
          <CardContent>
            {deliveriesQuery.isLoading ? (
              <Skeleton className="h-20 w-full" />
            ) : !deliveriesQuery.data?.length ? (
              <p className="py-4 text-center text-sm text-muted-foreground">{t('webhooks.deliveries.empty')}</p>
            ) : (
              <div className="space-y-2">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('webhooks.deliveries.table.time')}</TableHead>
                      <TableHead>{t('webhooks.deliveries.table.event')}</TableHead>
                      <TableHead>{t('webhooks.deliveries.table.status')}</TableHead>
                      <TableHead>{t('webhooks.deliveries.table.duration')}</TableHead>
                      <TableHead>{t('webhooks.deliveries.table.retries')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {deliveriesQuery.data.map((del) => (
                      <>
                        <TableRow
                          key={del.id}
                          className="cursor-pointer hover:bg-accent"
                          onClick={() => setExpandedDelivery(expandedDelivery === del.id ? null : del.id)}
                        >
                          <TableCell className="text-xs text-muted-foreground">
                            {new Date(del.created_at).toLocaleString()}
                          </TableCell>
                          <TableCell>
                            <Badge variant="secondary" className="text-xs">{del.event}</Badge>
                          </TableCell>
                          <TableCell>
                            <Badge variant={del.status === 'success' ? 'success' : del.status === 'failed' ? 'destructive' : 'secondary'}>
                              {del.status} {del.status_code ? `(${del.status_code})` : ''}
                            </Badge>
                          </TableCell>
                          <TableCell className="text-xs text-muted-foreground">{del.duration_ms}ms</TableCell>
                          <TableCell className="text-xs text-muted-foreground">{del.retry_count}</TableCell>
                        </TableRow>
                        {expandedDelivery === del.id && (
                          <TableRow key={`${del.id}-detail`}>
                            <TableCell colSpan={5} className="bg-muted/30">
                              <div className="grid gap-4 md:grid-cols-2 p-2">
                                <div>
                                  <p className="text-xs font-medium mb-1">{t('webhooks.deliveries.detail.request')}</p>
                                  <pre className="rounded bg-background p-2 text-xs font-mono overflow-auto max-h-[150px]">
                                    {Object.entries(del.request_headers ?? {}).map(([k, v]) => `${k}: ${v}\n`)}
                                    {del.request_body ? `\n${del.request_body}` : ''}
                                  </pre>
                                </div>
                                <div>
                                  <p className="text-xs font-medium mb-1">{t('webhooks.deliveries.detail.response')}</p>
                                  <pre className="rounded bg-background p-2 text-xs font-mono overflow-auto max-h-[150px]">
                                    {Object.entries(del.response_headers ?? {}).map(([k, v]) => `${k}: ${v}\n`)}
                                    {del.response_body ? `\n${del.response_body}` : ''}
                                  </pre>
                                </div>
                              </div>
                            </TableCell>
                          </TableRow>
                        )}
                      </>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* Create / Edit Dialog */}
      <Dialog
        open={showCreate || !!editTarget}
        onClose={() => { setShowCreate(false); setEditTarget(null) }}
        title={editTarget ? t('webhooks.edit_dialog.title') : t('webhooks.create_dialog.title')}
        description={editTarget ? t('webhooks.edit_dialog.description') : t('webhooks.create_dialog.description')}
      >
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <Input
            label={t('webhooks.create_dialog.name')}
            placeholder={t('webhooks.create_dialog.name_placeholder')}
            value={form.name}
            onChange={(e) => setForm((p) => ({ ...p, name: e.target.value }))}
          />
          <Input
            label={t('webhooks.create_dialog.url')}
            placeholder={t('webhooks.create_dialog.url_placeholder')}
            value={form.url}
            onChange={(e) => setForm((p) => ({ ...p, url: e.target.value }))}
          />

          <div>
            <label className="text-sm font-medium">{t('webhooks.create_dialog.events')}</label>
            <div className="mt-2 grid grid-cols-2 gap-2">
              {WEBHOOK_EVENTS.map((ev) => (
                <label key={ev} className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={form.events.includes(ev)}
                    onChange={() => toggleEvent(ev)}
                    className="h-4 w-4 rounded border-input"
                  />
                  {t(`webhooks.events.${ev}`)}
                </label>
              ))}
            </div>
          </div>

          <div>
            <label className="text-sm font-medium">{t('webhooks.create_dialog.secret')}</label>
            <div className="mt-1.5 flex items-center gap-2">
              <Input
                value={form.secret}
                onChange={(e) => setForm((p) => ({ ...p, secret: e.target.value }))}
                className="flex-1 font-mono text-xs"
              />
              <Button variant="outline" size="sm" type="button" onClick={regenerateSecret}>
                <RefreshCw className="h-4 w-4" />
              </Button>
              <Button variant="outline" size="sm" type="button" onClick={copySecret}>
                {copiedSecret ? <Check className="h-4 w-4 text-emerald-500" /> : <Copy className="h-4 w-4" />}
              </Button>
            </div>
          </div>

          {(createMutation.error || updateMutation.error) && (
            <p className="text-sm text-destructive">
              {((createMutation.error || updateMutation.error) as Error)?.message ?? t('webhooks.create_dialog.failed_create')}
            </p>
          )}

          <div className="flex justify-end gap-2 pt-2">
            <Button variant="outline" type="button" onClick={() => { setShowCreate(false); setEditTarget(null) }}>
              {t('webhooks.create_dialog.cancel')}
            </Button>
            <Button type="submit" disabled={createMutation.isPending || updateMutation.isPending}>
              {createMutation.isPending || updateMutation.isPending
                ? t('webhooks.create_dialog.creating')
                : editTarget
                  ? t('webhooks.edit_dialog.save')
                  : t('webhooks.create_dialog.submit')}
            </Button>
          </div>
        </form>
      </Dialog>

      {/* Delete Confirmation */}
      <Dialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        title={t('webhooks.delete_dialog.title')}
        description={t('webhooks.delete_dialog.message')}
      >
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="outline" onClick={() => setDeleteTarget(null)}>
            {t('webhooks.delete_dialog.cancel')}
          </Button>
          <Button
            variant="destructive"
            onClick={() => deleteTarget && deleteMutation.mutate(deleteTarget.id)}
            disabled={deleteMutation.isPending}
          >
            {deleteMutation.isPending ? t('webhooks.delete_dialog.deleting') : t('webhooks.delete_dialog.confirm')}
          </Button>
        </div>
      </Dialog>
    </div>
  )
}
