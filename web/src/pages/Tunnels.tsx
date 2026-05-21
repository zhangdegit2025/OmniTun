import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useTunnels } from '@/hooks/useTunnels'
import type { Tunnel, TunnelTemplate, BatchResult } from '@/lib/types'
import { apiRequest } from '@/lib/api'
import { Card, CardContent } from '@/components/ui/Card'
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
  Copy,
  Check,
  Play,
  Square,
  ExternalLink,
  Server,
  Globe,
  Radio,
  Database,
  Terminal,
  Layers,
  X,
} from 'lucide-react'

const statusVariant = {
  active: 'success' as const,
  stopped: 'secondary' as const,
  error: 'destructive' as const,
}

const TAG_COLORS = [
  'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
  'bg-emerald-100 text-emerald-800 dark:bg-emerald-900 dark:text-emerald-200',
  'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200',
  'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200',
  'bg-pink-100 text-pink-800 dark:bg-pink-900 dark:text-pink-200',
  'bg-cyan-100 text-cyan-800 dark:bg-cyan-900 dark:text-cyan-200',
]

const TUNNEL_TEMPLATES: TunnelTemplate[] = [
  {
    id: 'http-api',
    name: 'HTTP API',
    icon: 'Globe',
    protocol: 'http',
    local_port: 8080,
    tls_mode: 'edge',
    compression: false,
    description: 'REST API with TLS and compression',
  },
  {
    id: 'grpc',
    name: 'gRPC',
    icon: 'Radio',
    protocol: 'tcp',
    local_port: 50051,
    tls_mode: 'edge',
    compression: false,
    description: 'gRPC service with HTTP/2',
  },
  {
    id: 'websocket',
    name: 'WebSocket',
    icon: 'Layers',
    protocol: 'tcp',
    local_port: 3001,
    tls_mode: 'edge',
    compression: true,
    description: 'Persistent WebSocket connections',
  },
  {
    id: 'tcp-database',
    name: 'TCP Database',
    icon: 'Database',
    protocol: 'tcp',
    local_port: 5432,
    tls_mode: 'none',
    compression: false,
    description: 'Database tunnel for PostgreSQL/MySQL',
  },
  {
    id: 'ssh',
    name: 'SSH',
    icon: 'Terminal',
    protocol: 'tcp',
    local_port: 22,
    tls_mode: 'none',
    compression: false,
    description: 'Secure shell tunnel',
  },
]

const PAGE_SIZE = 10

export default function Tunnels() {
  const { t } = useTranslation()
  const { toast } = useToast()
  const {
    tunnels,
    isLoading,
    error,
    refetch,
    createTunnel,
    deleteTunnel,
    startTunnel,
    stopTunnel,
  } = useTunnels()

  const [showCreate, setShowCreate] = useState(false)
  const [createTab, setCreateTab] = useState<'scratch' | 'templates'>('scratch')
  const [deleteTarget, setDeleteTarget] = useState<Tunnel | null>(null)
  const [batchDeleteTargets, setBatchDeleteTargets] = useState<string[] | null>(null)
  const [copiedDomain, setCopiedDomain] = useState<string | null>(null)
  const [page, setPage] = useState(0)
  const [newTunnel, setNewTunnel] = useState({
    name: '',
    protocol: 'tcp' as Tunnel['protocol'],
    local_port: 0,
    remote_port: 0,
    domain: '',
    tags: [] as string[],
  })
  const [tagInput, setTagInput] = useState('')
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [batchProcessing, setBatchProcessing] = useState(false)
  const [tagFilter, setTagFilter] = useState<string | null>(null)

  const filteredTunnels = tagFilter
    ? tunnels.filter((t) => t.tags?.includes(tagFilter))
    : tunnels

  const paginatedTunnels = filteredTunnels.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE)
  const hasMore = (page + 1) * PAGE_SIZE < filteredTunnels.length

  const allSelected = paginatedTunnels.length > 0 && paginatedTunnels.every((t) => selectedIds.has(t.id))

  const toggleSelect = (id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const toggleSelectAll = () => {
    if (allSelected) {
      setSelectedIds(new Set())
    } else {
      setSelectedIds(new Set(paginatedTunnels.map((t) => t.id)))
    }
  }

  const clearSelection = () => setSelectedIds(new Set())

  const handleCreate = async () => {
    try {
      await createTunnel.mutateAsync({
        name: newTunnel.name,
        protocol: newTunnel.protocol,
        local_port: newTunnel.local_port,
        remote_port: newTunnel.remote_port,
        domain: newTunnel.domain || undefined,
      })
      if (newTunnel.tags.length > 0) {
        apiRequest(`/v1/tunnels/${tunnels[0]?.id}/tags`, {
          method: 'PUT',
          body: JSON.stringify({ tags: newTunnel.tags }),
        }).catch(() => {})
      }
      setShowCreate(false)
      setNewTunnel({ name: '', protocol: 'tcp', local_port: 0, remote_port: 0, domain: '', tags: [] })
      setTagInput('')
      setCreateTab('scratch')
      toast({ title: t('tunnels.created'), variant: 'success' })
    } catch {
      // Error handled by mutation state
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    try {
      await deleteTunnel.mutateAsync(deleteTarget.id)
      setDeleteTarget(null)
      toast({ title: t('tunnels.deleted'), variant: 'success' })
    } catch {
      // Error handled by mutation state
    }
  }

  const handleClone = (tunnel: Tunnel) => {
    setNewTunnel({
      name: `${tunnel.name}-copy`,
      protocol: tunnel.protocol,
      local_port: tunnel.local_port,
      remote_port: tunnel.remote_port,
      domain: '',
      tags: tunnel.tags || [],
    })
    setCreateTab('scratch')
    setShowCreate(true)
  }

  const handleBatchStart = async () => {
    setBatchProcessing(true)
    try {
      const ids = Array.from(selectedIds)
      await apiRequest<BatchResult>('/v1/tunnels/batch/start', {
        method: 'POST',
        body: JSON.stringify({ ids }),
      })
      clearSelection()
      refetch()
      toast({ title: `${t('batch.start_all')} (${ids.length})`, variant: 'success' })
    } catch {
      toast({ title: t('tunnels.failed_load'), variant: 'error' })
    } finally {
      setBatchProcessing(false)
    }
  }

  const handleBatchStop = async () => {
    setBatchProcessing(true)
    try {
      const ids = Array.from(selectedIds)
      await apiRequest<BatchResult>('/v1/tunnels/batch/stop', {
        method: 'POST',
        body: JSON.stringify({ ids }),
      })
      clearSelection()
      refetch()
      toast({ title: `${t('batch.stop_all')} (${ids.length})`, variant: 'success' })
    } catch {
      toast({ title: t('tunnels.failed_load'), variant: 'error' })
    } finally {
      setBatchProcessing(false)
    }
  }

  const handleBatchDelete = async () => {
    if (!batchDeleteTargets) return
    setBatchProcessing(true)
    try {
      await apiRequest<BatchResult>('/v1/tunnels/batch/delete', {
        method: 'POST',
        body: JSON.stringify({ ids: batchDeleteTargets }),
      })
      setBatchDeleteTargets(null)
      clearSelection()
      refetch()
      toast({ title: t('tunnels.deleted'), variant: 'success' })
    } catch {
      toast({ title: t('tunnels.failed_load'), variant: 'error' })
    } finally {
      setBatchProcessing(false)
    }
  }

  const copyDomain = async (domain: string) => {
    await navigator.clipboard.writeText(domain)
    setCopiedDomain(domain)
    toast({ title: t('common.copied'), variant: 'success' })
    setTimeout(() => setCopiedDomain(null), 2000)
  }

  const applyTemplate = (tmpl: TunnelTemplate) => {
    setNewTunnel((p) => ({
      ...p,
      name: tmpl.name,
      protocol: tmpl.protocol,
      local_port: tmpl.local_port,
    }))
    setCreateTab('scratch')
    toast({ title: t('tunnels.template_applied', { name: tmpl.name }), variant: 'success' })
  }

  const handleAddTag = () => {
    const tag = tagInput.trim()
    if (!tag || newTunnel.tags.includes(tag)) return
    setNewTunnel((p) => ({ ...p, tags: [...p.tags, tag] }))
    setTagInput('')
  }

  const handleRemoveTag = (tag: string) => {
    setNewTunnel((p) => ({ ...p, tags: p.tags.filter((t) => t !== tag) }))
  }

  const handleTagFilter = (tag: string | null) => {
    setTagFilter(tagFilter === tag ? null : tag)
    setPage(0)
  }

  const templateIcons: Record<string, React.FC<{ className?: string }>> = {
    Globe, Radio, Database, Terminal, Layers,
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">{t('tunnels.title')}</h1>
          <p className="text-sm text-muted-foreground">{t('tunnels.subtitle')}</p>
        </div>
        <Button onClick={() => setShowCreate(true)}>
          <Plus className="mr-1 h-4 w-4" />
          {t('tunnels.create')}
        </Button>
      </div>

      {/* Batch Action Bar */}
      {selectedIds.size > 0 && (
        <div className="flex items-center gap-3 rounded-lg border bg-muted/50 px-4 py-3">
          <span className="text-sm font-medium">{t('batch.selected', { count: selectedIds.size })}</span>
          <div className="flex gap-2">
            <Button variant="outline" size="sm" onClick={handleBatchStart} disabled={batchProcessing}>
              <Play className="mr-1 h-3.5 w-3.5 text-emerald-500" />
              {t('batch.start_all')}
            </Button>
            <Button variant="outline" size="sm" onClick={handleBatchStop} disabled={batchProcessing}>
              <Square className="mr-1 h-3.5 w-3.5 text-amber-500" />
              {t('batch.stop_all')}
            </Button>
            <Button variant="destructive" size="sm" onClick={() => setBatchDeleteTargets(Array.from(selectedIds))} disabled={batchProcessing}>
              <Trash2 className="mr-1 h-3.5 w-3.5" />
              {t('batch.delete_all')}
            </Button>
          </div>
          <Button variant="ghost" size="sm" onClick={clearSelection}>
            <X className="h-4 w-4" />
          </Button>
        </div>
      )}

      {/* Tag Filter */}
      {tagFilter && (
        <div className="flex items-center gap-2">
          <span className="text-sm text-muted-foreground">{t('tags.filter_by_tag')}:</span>
          <Badge variant="secondary" className="cursor-pointer gap-1" onClick={() => handleTagFilter(null)}>
            {tagFilter}
            <X className="h-3 w-3" />
          </Badge>
        </div>
      )}

      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <div className="space-y-3 p-6">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          ) : error ? (
            <div className="flex flex-col items-center gap-2 py-10 text-center">
              <AlertCircle className="h-8 w-8 text-destructive" />
              <p className="text-sm text-destructive">{t('tunnels.failed_load')}</p>
              <Button variant="outline" size="sm" onClick={() => refetch()}>
                {t('common.retry')}
              </Button>
            </div>
          ) : tunnels.length === 0 ? (
            <div className="flex flex-col items-center gap-3 py-10 text-center">
              <Server className="h-10 w-10 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">
                {t('tunnels.empty')}
              </p>
              <Button onClick={() => setShowCreate(true)}>
                <Plus className="mr-1 h-4 w-4" />
                {t('dashboard.create_first')}
              </Button>
            </div>
          ) : (
            <>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-10">
                      <input
                        type="checkbox"
                        checked={allSelected}
                        onChange={toggleSelectAll}
                        className="h-4 w-4 rounded border-muted-foreground"
                      />
                    </TableHead>
                    <TableHead>{t('tunnels.table.name')}</TableHead>
                    <TableHead>{t('tunnels.table.protocol')}</TableHead>
                    <TableHead>{t('tunnels.table.status')}</TableHead>
                    <TableHead>{t('tunnels.table.domain')}</TableHead>
                    <TableHead>{t('tags.no_tags')}</TableHead>
                    <TableHead>{t('tunnels.table.traffic')}</TableHead>
                    <TableHead className="text-right">{t('tunnels.table.actions')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {paginatedTunnels.map((tunnel) => (
                    <TableRow key={tunnel.id}>
                      <TableCell>
                        <input
                          type="checkbox"
                          checked={selectedIds.has(tunnel.id)}
                          onChange={() => toggleSelect(tunnel.id)}
                          className="h-4 w-4 rounded border-muted-foreground"
                        />
                      </TableCell>
                      <TableCell className="font-medium">
                        <Link
                          to={`/tunnels/${tunnel.id}`}
                          className="hover:underline"
                        >
                          {tunnel.name}
                        </Link>
                      </TableCell>
                      <TableCell>
                        <Badge variant="secondary" className="uppercase">
                          {t(`tunnels.protocols.${tunnel.protocol}`)}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <Badge variant={statusVariant[tunnel.status]}>
                          {t(`tunnels.status.${tunnel.status}`)}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        {tunnel.domain ? (
                          <button
                            type="button"
                            onClick={() => copyDomain(tunnel.domain!)}
                            className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
                          >
                            {tunnel.domain}
                            {copiedDomain === tunnel.domain ? (
                              <Check className="h-3 w-3 text-emerald-500" />
                            ) : (
                              <Copy className="h-3 w-3" />
                            )}
                          </button>
                        ) : (
                          <span className="text-muted-foreground">—</span>
                        )}
                      </TableCell>
                      <TableCell>
                        {tunnel.tags && tunnel.tags.length > 0 ? (
                          <div className="flex flex-wrap gap-1">
                            {tunnel.tags.map((tag) => (
                              <button
                                key={tag}
                                onClick={() => handleTagFilter(tag)}
                                className={`rounded-full px-2 py-0.5 text-xs font-medium cursor-pointer ${TAG_COLORS[Math.abs(hashCode(tag)) % TAG_COLORS.length]}`}
                              >
                                {tag}
                              </button>
                            ))}
                          </div>
                        ) : (
                          <span className="text-xs text-muted-foreground">—</span>
                        )}
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {formatBytes((Number((tunnel as any).bytes_in_total) || 0) + (Number((tunnel as any).bytes_out_total) || 0))}
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex items-center justify-end gap-1">
                          {tunnel.status === 'active' ? (
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => stopTunnel.mutate(tunnel.id)}
                              disabled={stopTunnel.isPending}
                            >
                              <Square className="h-4 w-4 text-amber-500" />
                            </Button>
                          ) : (
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => startTunnel.mutate(tunnel.id)}
                              disabled={startTunnel.isPending}
                            >
                              <Play className="h-4 w-4 text-emerald-500" />
                            </Button>
                          )}
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => handleClone(tunnel)}
                          >
                            <Copy className="h-4 w-4" />
                          </Button>
                          <Link
                            to={`/tunnels/${tunnel.id}`}
                            className="inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring hover:bg-accent hover:text-accent-foreground h-8 w-8"
                          >
                            <ExternalLink className="h-4 w-4" />
                          </Link>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => setDeleteTarget(tunnel)}
                          >
                            <Trash2 className="h-4 w-4 text-destructive" />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>

              {/* Pagination */}
              {filteredTunnels.length > PAGE_SIZE && (
                <div className="flex items-center justify-between border-t p-4">
                  <p className="text-sm text-muted-foreground">
                    {filteredTunnels.length === tunnels.length
                      ? `${t('tunnels.pagination.showing')} ${page * PAGE_SIZE + 1}–${Math.min((page + 1) * PAGE_SIZE, filteredTunnels.length)} ${t('tunnels.pagination.of')} ${filteredTunnels.length}`
                      : `${filteredTunnels.length} ${t('tunnels.pagination.of')} ${tunnels.length}`
                    }
                  </p>
                  <div className="flex gap-2">
                    <Button variant="outline" size="sm" disabled={page === 0} onClick={() => setPage((p) => p - 1)}>
                      {t('tunnels.pagination.previous')}
                    </Button>
                    <Button variant="outline" size="sm" disabled={!hasMore} onClick={() => setPage((p) => p + 1)}>
                      {t('tunnels.pagination.next')}
                    </Button>
                  </div>
                </div>
              )}
            </>
          )}
        </CardContent>
      </Card>

      {/* Create Dialog */}
      <Dialog
        open={showCreate}
        onClose={() => {
          setShowCreate(false)
          setCreateTab('scratch')
        }}
        title={t('tunnels.create_dialog.title')}
        description={t('tunnels.create_dialog.description')}
      >
        {/* Tabs */}
        <div className="flex gap-1 border-b mb-4">
          <button
            onClick={() => setCreateTab('scratch')}
            className={`flex items-center gap-1 border-b-2 px-3 py-1.5 text-sm font-medium transition-colors ${
              createTab === 'scratch'
                ? 'border-primary text-primary'
                : 'border-transparent text-muted-foreground hover:text-foreground'
            }`}
          >
            {t('templates.tab_from_scratch')}
          </button>
          <button
            onClick={() => setCreateTab('templates')}
            className={`flex items-center gap-1 border-b-2 px-3 py-1.5 text-sm font-medium transition-colors ${
              createTab === 'templates'
                ? 'border-primary text-primary'
                : 'border-transparent text-muted-foreground hover:text-foreground'
            }`}
          >
            {t('templates.tab_templates')}
          </button>
        </div>

        {createTab === 'templates' ? (
          <div className="grid grid-cols-2 gap-3">
            {TUNNEL_TEMPLATES.map((tmpl) => {
              const Icon = templateIcons[tmpl.icon]
              return (
                <button
                  key={tmpl.id}
                  onClick={() => applyTemplate(tmpl)}
                  className="flex flex-col items-start gap-2 rounded-lg border p-4 text-left hover:bg-accent hover:text-accent-foreground transition-colors"
                >
                  <div className="flex items-center gap-2">
                    {Icon && <Icon className="h-5 w-5 text-primary" />}
                    <span className="font-medium text-sm">{tmpl.name}</span>
                  </div>
                  <div className="text-xs text-muted-foreground">{tmpl.description}</div>
                  <div className="flex gap-2 mt-1">
                    <Badge variant="secondary" className="text-xs uppercase">{tmpl.protocol}</Badge>
                    <Badge variant="secondary" className="text-xs">:{tmpl.local_port}</Badge>
                    {tmpl.compression && <Badge variant="outline" className="text-xs">compressed</Badge>}
                  </div>
                </button>
              )
            })}
          </div>
        ) : (
          <form
            onSubmit={(e) => {
              e.preventDefault()
              handleCreate()
            }}
            className="flex flex-col gap-4"
          >
            <Input
              label={t('tunnels.create_dialog.name')}
              placeholder={t('tunnels.create_dialog.name_placeholder')}
              value={newTunnel.name}
              onChange={(e) => setNewTunnel((p) => ({ ...p, name: e.target.value }))}
            />
            <div className="flex gap-3">
              <div className="flex-1">
                <label className="text-sm font-medium">{t('tunnels.create_dialog.protocol')}</label>
                <select
                  value={newTunnel.protocol}
                  onChange={(e) =>
                    setNewTunnel((p) => ({
                      ...p,
                      protocol: e.target.value as Tunnel['protocol'],
                    }))
                  }
                  className="mt-1.5 flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                >
                  <option value="tcp">{t('tunnels.protocols.tcp')}</option>
                  <option value="http">{t('tunnels.protocols.http')}</option>
                  <option value="https">{t('tunnels.protocols.https')}</option>
                </select>
              </div>
              <Input
                label={t('tunnels.create_dialog.domain')}
                placeholder={t('tunnels.create_dialog.domain_placeholder')}
                value={newTunnel.domain}
                onChange={(e) => setNewTunnel((p) => ({ ...p, domain: e.target.value }))}
                className="flex-1"
              />
            </div>
            <div className="flex gap-3">
              <Input
                label={t('tunnels.create_dialog.local_port')}
                type="number"
                placeholder="8080"
                value={newTunnel.local_port || ''}
                onChange={(e) => setNewTunnel((p) => ({ ...p, local_port: Number(e.target.value) }))}
              />
              <Input
                label={t('tunnels.create_dialog.remote_port')}
                type="number"
                placeholder="443"
                value={newTunnel.remote_port || ''}
                onChange={(e) => setNewTunnel((p) => ({ ...p, remote_port: Number(e.target.value) }))}
              />
            </div>

            {/* Tags Input */}
            <div>
              <label className="text-sm font-medium">{t('tags.placeholder')}</label>
              <div className="mt-1.5 flex gap-2">
                <Input
                  placeholder={t('tags.placeholder')}
                  value={tagInput}
                  onChange={(e) => setTagInput(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') { e.preventDefault(); handleAddTag() }
                  }}
                  className="flex-1"
                />
                <Button type="button" variant="outline" size="sm" onClick={handleAddTag}>
                  <Plus className="h-4 w-4" />
                </Button>
              </div>
              {newTunnel.tags.length > 0 && (
                <div className="flex flex-wrap gap-1 mt-2">
                  {newTunnel.tags.map((tag) => (
                    <Badge key={tag} variant="secondary" className="gap-1">
                      {tag}
                      <button type="button" onClick={() => handleRemoveTag(tag)}>
                        <X className="h-3 w-3" />
                      </button>
                    </Badge>
                  ))}
                </div>
              )}
            </div>

            {createTunnel.error && (
              <p className="text-sm text-destructive">
                {(createTunnel.error as Error)?.message ?? t('tunnels.create_dialog.failed_create')}
              </p>
            )}
            <div className="flex justify-end gap-2 pt-2">
              <Button variant="outline" type="button" onClick={() => setShowCreate(false)}>
                {t('tunnels.create_dialog.cancel')}
              </Button>
              <Button type="submit" disabled={createTunnel.isPending}>
                {createTunnel.isPending ? t('tunnels.create_dialog.creating') : t('tunnels.create_dialog.submit')}
              </Button>
            </div>
          </form>
        )}
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        title={t('tunnels.delete_dialog.title')}
        description={t('tunnels.delete_dialog.message')}
      >
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="outline" onClick={() => setDeleteTarget(null)}>
            {t('tunnels.delete_dialog.cancel')}
          </Button>
          <Button variant="destructive" onClick={handleDelete} disabled={deleteTunnel.isPending}>
            {deleteTunnel.isPending ? t('tunnels.delete_dialog.deleting') : t('tunnels.delete_dialog.confirm')}
          </Button>
        </div>
      </Dialog>

      {/* Batch Delete Confirmation Dialog */}
      <Dialog
        open={batchDeleteTargets !== null}
        onClose={() => setBatchDeleteTargets(null)}
        title={t('batch.batch_delete_title')}
        description={t('batch.batch_delete_message', { count: batchDeleteTargets?.length || 0 })}
      >
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="outline" onClick={() => setBatchDeleteTargets(null)}>
            {t('tunnels.delete_dialog.cancel')}
          </Button>
          <Button variant="destructive" onClick={handleBatchDelete} disabled={batchProcessing}>
            {batchProcessing ? t('tunnels.delete_dialog.deleting') : t('tunnels.delete_dialog.confirm')}
          </Button>
        </div>
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

function hashCode(str: string): number {
  let hash = 0
  for (let i = 0; i < str.length; i++) {
    hash = ((hash << 5) - hash) + str.charCodeAt(i)
    hash |= 0
  }
  return Math.abs(hash)
}
