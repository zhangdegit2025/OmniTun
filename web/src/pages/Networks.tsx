import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useNetworks } from '@/hooks/useNetworks'
import type { MeshNetwork } from '@/lib/types'
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
  ExternalLink,
  Share2,
  LogIn,
} from 'lucide-react'

export default function Networks() {
  const { t } = useTranslation()
  const { toast } = useToast()
  const { networks, isLoading, error, refetch, createNetwork, joinNetwork, deleteNetwork } =
    useNetworks()

  const [showCreate, setShowCreate] = useState(false)
  const [showJoin, setShowJoin] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<MeshNetwork | null>(null)
  const [newNetwork, setNewNetwork] = useState({ name: '', cidr: '' })
  const [inviteCode, setInviteCode] = useState('')

  const handleCreate = async () => {
    try {
      await createNetwork.mutateAsync({ name: newNetwork.name, cidr: newNetwork.cidr })
      setShowCreate(false)
      setNewNetwork({ name: '', cidr: '' })
      toast({ title: t('networks.created'), variant: 'success' })
    } catch {
      // handled by mutation state
    }
  }

  const handleJoin = async () => {
    try {
      await joinNetwork.mutateAsync({ invite_code: inviteCode })
      setShowJoin(false)
      setInviteCode('')
      toast({ title: t('networks.joined'), variant: 'success' })
    } catch {
      // handled by mutation state
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    try {
      await deleteNetwork.mutateAsync(deleteTarget.id)
      setDeleteTarget(null)
      toast({ title: t('networks.deleted'), variant: 'success' })
    } catch {
      // handled by mutation state
    }
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">{t('networks.title')}</h1>
          <p className="text-sm text-muted-foreground">{t('networks.subtitle')}</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => setShowJoin(true)}>
            <LogIn className="mr-1 h-4 w-4" />
            {t('networks.join')}
          </Button>
          <Button onClick={() => setShowCreate(true)}>
            <Plus className="mr-1 h-4 w-4" />
            {t('networks.create')}
          </Button>
        </div>
      </div>

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
              <p className="text-sm text-destructive">{t('networks.failed_load')}</p>
              <Button variant="outline" size="sm" onClick={() => refetch()}>
                {t('common.retry')}
              </Button>
            </div>
          ) : networks.length === 0 ? (
            <div className="flex flex-col items-center gap-3 py-10 text-center">
              <Share2 className="h-10 w-10 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">{t('networks.empty')}</p>
              <div className="flex gap-2">
                <Button variant="outline" onClick={() => setShowJoin(true)}>
                  <LogIn className="mr-1 h-4 w-4" />
                  {t('networks.join')}
                </Button>
                <Button onClick={() => setShowCreate(true)}>
                  <Plus className="mr-1 h-4 w-4" />
                  {t('networks.create')}
                </Button>
              </div>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('networks.table.name')}</TableHead>
                  <TableHead>{t('networks.table.cidr')}</TableHead>
                  <TableHead>{t('networks.table.nodes')}</TableHead>
                  <TableHead>{t('networks.table.status')}</TableHead>
                  <TableHead className="text-right">{t('networks.table.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {networks.map((network) => (
                  <TableRow key={network.id}>
                    <TableCell className="font-medium">
                      <Link to={`/networks/${network.id}`} className="hover:underline">
                        {network.name}
                      </Link>
                    </TableCell>
                    <TableCell className="font-mono text-sm">{network.cidr}</TableCell>
                    <TableCell>{network.node_count}</TableCell>
                    <TableCell>
                      <Badge variant="success">{t('networks.active')}</Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex items-center justify-end gap-1">
                        <Link
                          to={`/networks/${network.id}`}
                          className="inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring hover:bg-accent hover:text-accent-foreground h-8 w-8"
                        >
                          <ExternalLink className="h-4 w-4" />
                        </Link>
                        <Button variant="ghost" size="sm" onClick={() => setDeleteTarget(network)}>
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

      {/* Create Dialog */}
      <Dialog
        open={showCreate}
        onClose={() => setShowCreate(false)}
        title={t('networks.create_dialog.title')}
        description={t('networks.create_dialog.description')}
      >
        <form
          onSubmit={(e) => {
            e.preventDefault()
            handleCreate()
          }}
          className="flex flex-col gap-4"
        >
          <Input
            label={t('networks.create_dialog.name')}
            placeholder={t('networks.create_dialog.name_placeholder')}
            value={newNetwork.name}
            onChange={(e) => setNewNetwork((p) => ({ ...p, name: e.target.value }))}
          />
          <Input
            label={t('networks.create_dialog.cidr')}
            placeholder="192.168.100.0/24"
            value={newNetwork.cidr}
            onChange={(e) => setNewNetwork((p) => ({ ...p, cidr: e.target.value }))}
          />
          {createNetwork.error && (
            <p className="text-sm text-destructive">
              {(createNetwork.error as Error)?.message ?? t('networks.create_dialog.failed_create')}
            </p>
          )}
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="outline" type="button" onClick={() => setShowCreate(false)}>
              {t('networks.create_dialog.cancel')}
            </Button>
            <Button type="submit" disabled={createNetwork.isPending}>
              {createNetwork.isPending
                ? t('networks.create_dialog.creating')
                : t('networks.create_dialog.submit')}
            </Button>
          </div>
        </form>
      </Dialog>

      {/* Join Dialog */}
      <Dialog
        open={showJoin}
        onClose={() => setShowJoin(false)}
        title={t('networks.join_dialog.title')}
        description={t('networks.join_dialog.description')}
      >
        <form
          onSubmit={(e) => {
            e.preventDefault()
            handleJoin()
          }}
          className="flex flex-col gap-4"
        >
          <Input
            label={t('networks.join_dialog.code')}
            placeholder={t('networks.join_dialog.code_placeholder')}
            value={inviteCode}
            onChange={(e) => setInviteCode(e.target.value)}
          />
          {joinNetwork.error && (
            <p className="text-sm text-destructive">
              {(joinNetwork.error as Error)?.message ?? t('networks.join_dialog.failed_join')}
            </p>
          )}
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="outline" type="button" onClick={() => setShowJoin(false)}>
              {t('networks.join_dialog.cancel')}
            </Button>
            <Button type="submit" disabled={joinNetwork.isPending}>
              {joinNetwork.isPending
                ? t('networks.join_dialog.joining')
                : t('networks.join_dialog.submit')}
            </Button>
          </div>
        </form>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        title={t('networks.delete_dialog.title')}
        description={t('networks.delete_dialog.message')}
      >
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="outline" onClick={() => setDeleteTarget(null)}>
            {t('networks.delete_dialog.cancel')}
          </Button>
          <Button variant="destructive" onClick={handleDelete} disabled={deleteNetwork.isPending}>
            {deleteNetwork.isPending
              ? t('networks.delete_dialog.deleting')
              : t('networks.delete_dialog.confirm')}
          </Button>
        </div>
      </Dialog>
    </div>
  )
}
