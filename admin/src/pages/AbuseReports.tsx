import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiRequest } from '@/lib/api'
import { Card, CardContent } from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Skeleton } from '@/components/ui/Skeleton'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Search, CheckCircle, XCircle, AlertTriangle } from 'lucide-react'

interface AbuseReport {
  id: string
  org_id: string
  reporter_id: string
  tunnel_id: string
  reason: string
  description: string
  status: 'pending' | 'investigating' | 'resolved' | 'dismissed'
  resolution?: string
  created_at: string
  updated_at: string
}

const statusBadge: Record<string, 'warning' | 'default' | 'success' | 'destructive'> = {
  pending: 'warning',
  investigating: 'default',
  resolved: 'success',
  dismissed: 'destructive',
}

export default function AbuseReports() {
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')

  const reportsQuery = useQuery<{ reports: AbuseReport[]; total: number }>({
    queryKey: ['admin', 'abuse', 'reports'],
    queryFn: () => apiRequest('/api/admin/v1/abuse/reports'),
  })

  const resolveMutation = useMutation({
    mutationFn: ({ id, resolution }: { id: string; resolution: string }) =>
      apiRequest(`/api/admin/v1/abuse/reports/${id}/resolve`, {
        method: 'POST',
        body: JSON.stringify({ resolution }),
      }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'abuse', 'reports'] }),
  })

  const dismissMutation = useMutation({
    mutationFn: ({ id, reason }: { id: string; reason: string }) =>
      apiRequest(`/api/admin/v1/abuse/reports/${id}/dismiss`, {
        method: 'POST',
        body: JSON.stringify({ reason }),
      }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'abuse', 'reports'] }),
  })

  const reports = reportsQuery.data?.reports ?? []

  const filtered = reports.filter(
    (r) =>
      r.reason.toLowerCase().includes(search.toLowerCase()) ||
      r.org_id.toLowerCase().includes(search.toLowerCase()) ||
      r.tunnel_id.toLowerCase().includes(search.toLowerCase()) ||
      (r.description && r.description.toLowerCase().includes(search.toLowerCase())),
  )

  if (reportsQuery.isLoading) {
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
          <h1 className="text-2xl font-bold">Abuse Reports</h1>
          <p className="text-sm text-muted-foreground">Review and manage reported abuse incidents</p>
        </div>
        <AlertTriangle className="h-8 w-8 text-amber-500" />
      </div>

      <div className="relative max-w-sm">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          placeholder="Search reports..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="pl-9"
        />
      </div>

      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Report ID</TableHead>
                <TableHead>Org</TableHead>
                <TableHead>Tunnel</TableHead>
                <TableHead>Reason</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Created</TableHead>
                <TableHead>Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7} className="py-8 text-center text-muted-foreground">
                    No abuse reports found
                  </TableCell>
                </TableRow>
              ) : (
                filtered.map((report) => (
                  <TableRow key={report.id}>
                    <TableCell className="font-mono text-xs">{report.id.slice(0, 8)}</TableCell>
                    <TableCell className="font-mono text-xs">{report.org_id.slice(0, 8)}</TableCell>
                    <TableCell className="font-mono text-xs">{report.tunnel_id.slice(0, 8)}</TableCell>
                    <TableCell>
                      <div>
                        <p className="font-medium text-sm">{report.reason}</p>
                        {report.description && (
                          <p className="text-xs text-muted-foreground truncate max-w-[200px]">
                            {report.description}
                          </p>
                        )}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={statusBadge[report.status] ?? 'secondary'}>
                        {report.status}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground text-xs">
                      {new Date(report.created_at).toLocaleString()}
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1">
                        {report.status === 'pending' || report.status === 'investigating' ? (
                          <>
                            <Button
                              size="sm"
                              variant="outline"
                              className="h-7 text-xs text-emerald-600"
                              disabled={resolveMutation.isPending}
                              onClick={() =>
                                resolveMutation.mutate({
                                  id: report.id,
                                  resolution: 'resolved by admin',
                                })
                              }
                            >
                              <CheckCircle className="mr-1 h-3 w-3" />
                              Resolve
                            </Button>
                            <Button
                              size="sm"
                              variant="outline"
                              className="h-7 text-xs text-destructive"
                              disabled={dismissMutation.isPending}
                              onClick={() =>
                                dismissMutation.mutate({
                                  id: report.id,
                                  reason: 'dismissed by admin',
                                })
                              }
                            >
                              <XCircle className="mr-1 h-3 w-3" />
                              Dismiss
                            </Button>
                          </>
                        ) : report.status === 'resolved' ? (
                          <span className="text-xs text-muted-foreground">
                            {report.resolution ?? 'Resolved'}
                          </span>
                        ) : (
                          <span className="text-xs text-muted-foreground">
                            {report.resolution ?? 'Dismissed'}
                          </span>
                        )}
                      </div>
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
