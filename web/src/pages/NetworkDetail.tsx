import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useNetwork, useNetworkInvite, useRemoveNode } from '@/hooks/useNetworks'
import type { MeshNode } from '@/lib/types'
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
import { Dialog } from '@/components/ui/Dialog'
import { Skeleton } from '@/components/ui/Skeleton'
import { useToast } from '@/components/ui/useToast'
import {
  AlertCircle,
  ArrowLeft,
  Copy,
  Check,
  Network,
  UserX,
  Plus,
} from 'lucide-react'
import { format, formatDistanceToNow } from 'date-fns'

const statusVariant = {
  online: 'success' as const,
  offline: 'secondary' as const,
}

export default function NetworkDetail() {
  const { id } = useParams<{ id: string }>()
  const { t } = useTranslation()
  const { toast } = useToast()
  const { data: network, isLoading, error, refetch } = useNetwork(id)
  const { invite, createInvite, isLoading: inviteLoading } = useNetworkInvite(id)
  const removeNode = useRemoveNode(id)
  const [removeTarget, setRemoveTarget] = useState<MeshNode | null>(null)
  const [copiedCode, setCopiedCode] = useState(false)

  const handleCreateInvite = async () => {
    if (!id) return
    try {
      await createInvite.mutateAsync(id)
      toast({ title: t('network_detail.invite_created'), variant: 'success' })
    } catch {
      // handled by mutation state
    }
  }

  const handleCopyInvite = async (code: string) => {
    await navigator.clipboard.writeText(code)
    setCopiedCode(true)
    toast({ title: t('common.copied'), variant: 'success' })
    setTimeout(() => setCopiedCode(false), 2000)
  }

  const handleRemoveNode = async () => {
    if (!removeTarget) return
    try {
      await removeNode.mutateAsync(removeTarget.id)
      setRemoveTarget(null)
      toast({ title: t('network_detail.node_removed'), variant: 'success' })
    } catch {
      // handled by mutation state
    }
  }

  return (
    <div className="space-y-6 p-6">
      {/* Breadcrumb */}
      <nav className="flex items-center gap-2 text-sm text-muted-foreground">
        <Link to="/networks" className="hover:text-foreground">
          {t('network_detail.breadcrumb')}
        </Link>
        <span>/</span>
        <span className="text-foreground">{network?.name ?? '...'}</span>
      </nav>

      <Link
        to="/networks"
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft className="h-4 w-4" />
        {t('network_detail.back')}
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
            <p className="text-sm text-destructive">{t('network_detail.failed_load')}</p>
            <Button variant="outline" size="sm" onClick={() => refetch()}>
              {t('common.retry')}
            </Button>
          </CardContent>
        </Card>
      ) : !network ? (
        <Card>
          <CardContent className="py-10 text-center text-sm text-muted-foreground">
            {t('network_detail.not_found')}
          </CardContent>
        </Card>
      ) : (
        <>
          {/* Network Info Card */}
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle className="text-2xl">{network.name}</CardTitle>
                  <CardDescription className="mt-1">
                    {t('network_detail.created')}{' '}
                    {format(new Date(network.created_at), 'PPP')}{' '}
                    &middot;{' '}
                    {formatDistanceToNow(new Date(network.created_at), {
                      addSuffix: true,
                    })}
                  </CardDescription>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <dl className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
                <div>
                  <dt className="text-sm text-muted-foreground">{t('networks.table.cidr')}</dt>
                  <dd className="mt-1 font-mono text-sm">{network.cidr}</dd>
                </div>
                <div>
                  <dt className="text-sm text-muted-foreground">{t('networks.table.nodes')}</dt>
                  <dd className="mt-1 text-sm">{network.nodes?.length ?? 0}</dd>
                </div>
                <div>
                  <dt className="text-sm text-muted-foreground">
                    {t('network_detail.online_nodes')}
                  </dt>
                  <dd className="mt-1 text-sm">
                    {network.nodes?.filter((n) => n.status === 'online').length ?? 0}
                  </dd>
                </div>
              </dl>
            </CardContent>
          </Card>

          {/* Invite Code Card */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-lg">
                <Plus className="h-5 w-5" />
                {t('network_detail.invite_code')}
              </CardTitle>
              <CardDescription>{t('network_detail.invite_description')}</CardDescription>
            </CardHeader>
            <CardContent>
              {invite ? (
                <div className="flex items-center gap-3">
                  <code className="flex-1 rounded-md bg-muted px-4 py-2 font-mono text-sm">
                    {invite.code}
                  </code>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => handleCopyInvite(invite.code)}
                  >
                    {copiedCode ? (
                      <Check className="h-4 w-4 text-emerald-500" />
                    ) : (
                      <Copy className="h-4 w-4" />
                    )}
                    <span className="ml-1">{t('network_detail.copy_code')}</span>
                  </Button>
                </div>
              ) : (
                <div className="flex items-center justify-between">
                  <p className="text-sm text-muted-foreground">
                    {t('network_detail.no_invite')}
                  </p>
                  <Button size="sm" onClick={handleCreateInvite} disabled={inviteLoading || createInvite.isPending}>
                    {createInvite.isPending
                      ? t('network_detail.generating')
                      : t('network_detail.generate_invite')}
                  </Button>
                </div>
              )}
            </CardContent>
          </Card>

          {/* Topology Visualization */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-lg">
                <Network className="h-5 w-5" />
                {t('network_detail.topology')}
              </CardTitle>
              <CardDescription>{t('network_detail.topology_description')}</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="flex flex-wrap items-center justify-center gap-6 py-6">
                {(network.nodes ?? []).map((node, i) => (
                  <span key={node.id}>
                    <div className="flex flex-col items-center gap-2">
                      <div
                        className={`flex h-16 w-16 items-center justify-center rounded-full border-2 text-xs font-medium ${
                          node.status === 'online'
                            ? 'border-emerald-500 bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300'
                            : 'border-muted-foreground/30 bg-muted text-muted-foreground'
                        }`}
                      >
                        {node.name.slice(0, 2).toUpperCase()}
                      </div>
                      <div className="text-center">
                        <p className="text-xs font-medium">{node.name}</p>
                        <p className="font-mono text-[10px] text-muted-foreground">
                          {node.ip_address}
                        </p>
                      </div>
                    </div>
                    {i < (network.nodes ?? []).length - 1 && (
                      <div className="flex h-0.5 w-12 items-center justify-center text-muted-foreground/50">
                        <svg width="24" height="24" viewBox="0 0 24 24">
                          <line
                            x1="0"
                            y1="12"
                            x2="24"
                            y2="12"
                            stroke="currentColor"
                            strokeWidth="2"
                            strokeDasharray="4 4"
                          />
                        </svg>
                      </div>
                    )}
                  </span>
                ))}
                {(network.nodes ?? []).length === 0 && (
                  <p className="text-sm text-muted-foreground">
                    {t('network_detail.no_nodes')}
                  </p>
                )}
              </div>
            </CardContent>
          </Card>

          {/* Node List */}
          <Card>
            <CardHeader>
              <CardTitle className="text-lg">{t('network_detail.nodes')}</CardTitle>
            </CardHeader>
            <CardContent className="p-0">
              {(network.nodes ?? []).length === 0 ? (
                <div className="py-10 text-center text-sm text-muted-foreground">
                  {t('network_detail.no_nodes')}
                </div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('network_detail.table.name')}</TableHead>
                      <TableHead>{t('network_detail.table.ip')}</TableHead>
                      <TableHead>{t('network_detail.table.public_key')}</TableHead>
                      <TableHead>{t('network_detail.table.nat_type')}</TableHead>
                      <TableHead>{t('network_detail.table.endpoint')}</TableHead>
                      <TableHead>{t('network_detail.table.status')}</TableHead>
                      <TableHead className="text-right">{t('common.actions', 'Actions')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(network.nodes ?? []).map((node) => (
                      <TableRow key={node.id}>
                        <TableCell className="font-medium">{node.name}</TableCell>
                        <TableCell className="font-mono text-sm">{node.ip_address}</TableCell>
                        <TableCell className="font-mono text-xs max-w-[150px] truncate">
                          {node.public_key.slice(0, 16)}...
                        </TableCell>
                        <TableCell>
                          <Badge variant="secondary" className="text-xs">
                            {node.nat_type || '—'}
                          </Badge>
                        </TableCell>
                        <TableCell className="font-mono text-xs max-w-[150px] truncate">
                          {(node.endpoints ?? [])[0] || '—'}
                        </TableCell>
                        <TableCell>
                          <Badge variant={statusVariant[node.status]}>
                            {t(`network_detail.status.${node.status}`)}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-right">
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => setRemoveTarget(node)}
                          >
                            <UserX className="h-4 w-4 text-destructive" />
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        </>
      )}

      {/* Remove Node Confirmation Dialog */}
      <Dialog
        open={!!removeTarget}
        onClose={() => setRemoveTarget(null)}
        title={t('network_detail.remove_dialog.title')}
        description={t('network_detail.remove_dialog.message')}
      >
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="outline" onClick={() => setRemoveTarget(null)}>
            {t('network_detail.remove_dialog.cancel')}
          </Button>
          <Button
            variant="destructive"
            onClick={handleRemoveNode}
            disabled={removeNode.isPending}
          >
            {removeNode.isPending
              ? t('network_detail.remove_dialog.removing')
              : t('network_detail.remove_dialog.confirm')}
          </Button>
        </div>
      </Dialog>
    </div>
  )
}
