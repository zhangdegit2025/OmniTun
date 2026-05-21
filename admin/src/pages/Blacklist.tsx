import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiRequest } from '@/lib/api'
import { Card, CardContent } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Badge } from '@/components/ui/Badge'
import { Skeleton } from '@/components/ui/Skeleton'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Plus, Trash2, ShieldBan } from 'lucide-react'

interface BlacklistEntry {
  id: string
  cidr: string
  reason: string
  created_by: string
  created_at: string
  expires_at?: string
}

export default function Blacklist() {
  const queryClient = useQueryClient()
  const [newCidr, setNewCidr] = useState('')
  const [newReason, setNewReason] = useState('')
  const [addError, setAddError] = useState('')

  const blacklistQuery = useQuery<{ entries: BlacklistEntry[] }>({
    queryKey: ['admin', 'abuse', 'blacklist'],
    queryFn: () => apiRequest('/api/admin/v1/abuse/blacklist'),
  })

  const addMutation = useMutation({
    mutationFn: ({ cidr, reason }: { cidr: string; reason: string }) =>
      apiRequest('/api/admin/v1/abuse/blacklist', {
        method: 'POST',
        body: JSON.stringify({ cidr, reason }),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'abuse', 'blacklist'] })
      setNewCidr('')
      setNewReason('')
      setAddError('')
    },
    onError: (err: { message?: string }) => {
      setAddError(err.message ?? 'Failed to add entry')
    },
  })

  const removeMutation = useMutation({
    mutationFn: (id: string) =>
      apiRequest(`/api/admin/v1/abuse/blacklist/${id}`, {
        method: 'DELETE',
      }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'abuse', 'blacklist'] }),
  })

  const entries = blacklistQuery.data?.entries ?? []

  const handleAdd = () => {
    if (!newCidr.trim()) {
      setAddError('CIDR is required')
      return
    }
    addMutation.mutate({
      cidr: newCidr.trim(),
      reason: newReason.trim() || 'manual block',
    })
  }

  if (blacklistQuery.isLoading) {
    return (
      <div className="space-y-6 p-6">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-64 w-full" />
      </div>
    )
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">IP Blacklist</h1>
          <p className="text-sm text-muted-foreground">Manage blocked IP addresses and CIDR ranges</p>
        </div>
        <ShieldBan className="h-8 w-8 text-destructive" />
      </div>

      <Card>
        <CardContent className="p-6">
          <div className="flex items-end gap-3">
            <div className="flex-1">
              <Input
                label="CIDR"
                placeholder="e.g. 192.168.0.0/24 or 10.0.0.1/32"
                value={newCidr}
                onChange={(e) => {
                  setNewCidr(e.target.value)
                  setAddError('')
                }}
                error={addError}
              />
            </div>
            <div className="flex-1">
              <Input
                label="Reason"
                placeholder="e.g. DDoS attack, spam"
                value={newReason}
                onChange={(e) => setNewReason(e.target.value)}
              />
            </div>
            <Button
              onClick={handleAdd}
              disabled={addMutation.isPending}
              className="h-9"
            >
              <Plus className="mr-1 h-4 w-4" />
              {addMutation.isPending ? 'Adding...' : 'Add'}
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>CIDR</TableHead>
                <TableHead>Reason</TableHead>
                <TableHead>Added By</TableHead>
                <TableHead>Added At</TableHead>
                <TableHead>Expires</TableHead>
                <TableHead className="w-[80px]">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {entries.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} className="py-8 text-center text-muted-foreground">
                    No blacklist entries. Add a CIDR to start blocking.
                  </TableCell>
                </TableRow>
              ) : (
                entries.map((entry) => (
                  <TableRow key={entry.id}>
                    <TableCell>
                      <Badge variant="destructive" className="font-mono text-xs">
                        {entry.cidr}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-sm">{entry.reason}</TableCell>
                    <TableCell className="text-sm text-muted-foreground">{entry.created_by}</TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {new Date(entry.created_at).toLocaleString()}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {entry.expires_at ? new Date(entry.expires_at).toLocaleString() : 'Never'}
                    </TableCell>
                    <TableCell>
                      <Button
                        size="sm"
                        variant="ghost"
                        className="h-7 text-xs text-destructive"
                        disabled={removeMutation.isPending}
                        onClick={() => removeMutation.mutate(entry.id)}
                      >
                        <Trash2 className="h-3 w-3" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}
