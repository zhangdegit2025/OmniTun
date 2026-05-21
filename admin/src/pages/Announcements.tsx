import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiRequest } from '@/lib/api'
import type { Announcement } from '@/lib/types'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Skeleton } from '@/components/ui/Skeleton'
import { AlertCircle, Plus, Pencil, Trash2, Megaphone } from 'lucide-react'

const severityBadgeVariant: Record<string, 'default' | 'success' | 'warning' | 'destructive' | 'secondary'> = {
  info: 'default',
  warning: 'warning',
  critical: 'destructive',
}

const targetLabels: Record<string, string> = {
  all: 'All Plans',
  free: 'Free',
  pro: 'Pro',
  team: 'Team',
  business: 'Business',
  enterprise: 'Enterprise',
}

export default function Announcements() {
  const queryClient = useQueryClient()
  const [showForm, setShowForm] = useState(false)
  const [editing, setEditing] = useState<Announcement | null>(null)
  const [form, setForm] = useState({
    title: '',
    body: '',
    severity: 'info' as 'info' | 'warning' | 'critical',
    target: 'all',
    active: true,
    start_at: '',
    end_at: '',
  })

  const listQuery = useQuery<{ announcements: Announcement[] }>({
    queryKey: ['admin', 'announcements'],
    queryFn: () => apiRequest('/api/admin/v1/announcements'),
  })

  const createMutation = useMutation({
    mutationFn: (data: typeof form) =>
      apiRequest('/api/admin/v1/announcements', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'announcements'] })
      resetForm()
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: typeof form }) =>
      apiRequest(`/api/admin/v1/announcements/${id}`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'announcements'] })
      resetForm()
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) =>
      apiRequest(`/api/admin/v1/announcements/${id}`, { method: 'DELETE' }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'announcements'] }),
  })

  const resetForm = () => {
    setShowForm(false)
    setEditing(null)
    setForm({ title: '', body: '', severity: 'info', target: 'all', active: true, start_at: '', end_at: '' })
  }

  const handleEdit = (a: Announcement) => {
    setEditing(a)
    setForm({
      title: a.title,
      body: a.body,
      severity: a.severity,
      target: a.target,
      active: a.active,
      start_at: a.start_at ?? '',
      end_at: a.end_at ?? '',
    })
    setShowForm(true)
  }

  const handleSubmit = () => {
    if (!form.title.trim()) return
    if (editing) {
      updateMutation.mutate({ id: editing.id, data: form })
    } else {
      createMutation.mutate(form)
    }
  }

  const data = listQuery.data
  const showSkeleton = listQuery.isLoading

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Announcements</h1>
          <p className="text-sm text-muted-foreground">Manage system-wide announcements for user dashboards</p>
        </div>
        <Button size="sm" onClick={() => { resetForm(); setShowForm(true) }}>
          <Plus className="mr-2 h-4 w-4" />
          New Announcement
        </Button>
      </div>

      {showForm && (
        <Card>
          <CardHeader>
            <CardTitle>{editing ? 'Edit Announcement' : 'Create Announcement'}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid gap-4">
              <Input
                label="Title"
                value={form.title}
                onChange={(e) => setForm((f) => ({ ...f, title: e.target.value }))}
                placeholder="Announcement title"
              />
              <div className="flex flex-col gap-1.5">
                <label className="text-sm font-medium text-foreground">Body (Markdown)</label>
                <textarea
                  className="flex min-h-[120px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                  value={form.body}
                  onChange={(e) => setForm((f) => ({ ...f, body: e.target.value }))}
                  placeholder="Markdown content..."
                />
              </div>
              <div className="grid gap-4 sm:grid-cols-2">
                <div className="flex flex-col gap-1.5">
                  <label className="text-sm font-medium text-foreground">Severity</label>
                  <select
                    className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                    value={form.severity}
                    onChange={(e) => setForm((f) => ({ ...f, severity: e.target.value as 'info' | 'warning' | 'critical' }))}
                  >
                    <option value="info">Info</option>
                    <option value="warning">Warning</option>
                    <option value="critical">Critical</option>
                  </select>
                </div>
                <div className="flex flex-col gap-1.5">
                  <label className="text-sm font-medium text-foreground">Target Plan</label>
                  <select
                    className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                    value={form.target}
                    onChange={(e) => setForm((f) => ({ ...f, target: e.target.value }))}
                  >
                    <option value="all">All Plans</option>
                    <option value="free">Free</option>
                    <option value="pro">Pro</option>
                    <option value="team">Team</option>
                    <option value="business">Business</option>
                    <option value="enterprise">Enterprise</option>
                  </select>
                </div>
              </div>
              <div className="grid gap-4 sm:grid-cols-2">
                <Input
                  label="Start At"
                  type="datetime-local"
                  value={form.start_at}
                  onChange={(e) => setForm((f) => ({ ...f, start_at: e.target.value }))}
                />
                <Input
                  label="End At"
                  type="datetime-local"
                  value={form.end_at}
                  onChange={(e) => setForm((f) => ({ ...f, end_at: e.target.value }))}
                />
              </div>
              <div className="flex items-center gap-2">
                <input
                  type="checkbox"
                  id="active-toggle"
                  checked={form.active}
                  onChange={(e) => setForm((f) => ({ ...f, active: e.target.checked }))}
                  className="h-4 w-4 rounded border-input"
                />
                <label htmlFor="active-toggle" className="text-sm font-medium text-foreground">
                  Active
                </label>
              </div>
              <div className="flex gap-2">
                <Button onClick={handleSubmit} disabled={createMutation.isPending || updateMutation.isPending}>
                  {editing ? 'Update' : 'Create'}
                </Button>
                <Button variant="outline" onClick={resetForm}>
                  Cancel
                </Button>
              </div>
              {(createMutation.isError || updateMutation.isError) && (
                <Badge variant="destructive" className="w-full justify-center py-1.5">
                  {(createMutation.error as { message?: string })?.message ??
                    (updateMutation.error as { message?: string })?.message ??
                    'Operation failed'}
                </Badge>
              )}
            </div>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>
            All Announcements
            {data?.announcements && (
              <span className="text-sm font-normal text-muted-foreground">
                ({data.announcements.length})
              </span>
            )}
          </CardTitle>
        </CardHeader>
        <CardContent>
          {listQuery.isError ? (
            <div className="flex flex-col items-center gap-2 rounded-lg border border-destructive/50 p-6 text-center">
              <AlertCircle className="h-8 w-8 text-destructive" />
              <p className="text-sm text-destructive">Failed to load announcements</p>
              <Button variant="outline" size="sm" onClick={() => listQuery.refetch()}>
                Retry
              </Button>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Title</TableHead>
                  <TableHead>Severity</TableHead>
                  <TableHead>Target</TableHead>
                  <TableHead>Active</TableHead>
                  <TableHead>Schedule</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead className="w-[100px]">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {showSkeleton
                  ? Array.from({ length: 3 }).map((_, i) => (
                      <TableRow key={i}>
                        <TableCell><Skeleton className="h-4 w-32" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-10" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                      </TableRow>
                    ))
                  : (data?.announcements ?? []).map((a) => (
                      <TableRow key={a.id}>
                        <TableCell className="font-medium">{a.title}</TableCell>
                        <TableCell>
                          <Badge variant={severityBadgeVariant[a.severity] ?? 'secondary'}>
                            {a.severity}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-xs text-muted-foreground">
                          {targetLabels[a.target] ?? a.target}
                        </TableCell>
                        <TableCell>
                          <Badge variant={a.active ? 'success' : 'secondary'}>
                            {a.active ? 'Active' : 'Inactive'}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-xs text-muted-foreground">
                          {a.start_at || a.end_at ? (
                            <>
                              {a.start_at ? new Date(a.start_at).toLocaleDateString() : 'Now'}
                              {' \u2013 '}
                              {a.end_at ? new Date(a.end_at).toLocaleDateString() : 'Forever'}
                            </>
                          ) : (
                            'Always'
                          )}
                        </TableCell>
                        <TableCell className="text-xs text-muted-foreground">
                          {new Date(a.created_at).toLocaleDateString()}
                        </TableCell>
                        <TableCell>
                          <div className="flex gap-1">
                            <Button variant="ghost" size="sm" onClick={() => handleEdit(a)}>
                              <Pencil className="h-3.5 w-3.5" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => {
                                if (window.confirm('Delete this announcement?')) {
                                  deleteMutation.mutate(a.id)
                                }
                              }}
                            >
                              <Trash2 className="h-3.5 w-3.5 text-destructive" />
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
              </TableBody>
            </Table>
          )}

          {data?.announcements && data.announcements.length === 0 && !showSkeleton && (
            <div className="py-8 text-center text-sm text-muted-foreground">
              <Megaphone className="mx-auto mb-2 h-6 w-6" />
              No announcements created yet
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
