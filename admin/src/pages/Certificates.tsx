import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiRequest } from '@/lib/api'
import type { SystemCertificate, TenantCertificate } from '@/lib/types'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Skeleton } from '@/components/ui/Skeleton'
import { AlertCircle, RefreshCw, XCircle, Shield } from 'lucide-react'

type Tab = 'system' | 'tenants'

export default function Certificates() {
  const queryClient = useQueryClient()
  const [tab, setTab] = useState<Tab>('system')

  const systemQuery = useQuery<{ certificates: SystemCertificate[] }>({
    queryKey: ['admin', 'certificates', 'system'],
    queryFn: () => apiRequest('/api/admin/v1/certificates/system'),
    enabled: tab === 'system',
  })

  const tenantsQuery = useQuery<{ certificates: TenantCertificate[] }>({
    queryKey: ['admin', 'certificates', 'tenants'],
    queryFn: () => apiRequest('/api/admin/v1/certificates/tenants'),
    enabled: tab === 'tenants',
  })

  const renewMutation = useMutation({
    mutationFn: (id: string) =>
      apiRequest(`/api/admin/v1/certificates/${id}/renew`, { method: 'POST' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'certificates'] })
    },
  })

  const revokeMutation = useMutation({
    mutationFn: (id: string) =>
      apiRequest(`/api/admin/v1/certificates/${id}/revoke`, { method: 'POST' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'certificates'] })
    },
  })

  const activeQuery = tab === 'system' ? systemQuery : tenantsQuery
  const data = activeQuery.data?.certificates ?? []
  const showSkeleton = activeQuery.isLoading

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'valid':
        return <Badge variant="success">Valid</Badge>
      case 'expiring_soon':
        return <Badge variant="warning">Expiring Soon</Badge>
      case 'expired':
        return <Badge variant="destructive">Expired</Badge>
      default:
        return <Badge variant="secondary">{status}</Badge>
    }
  }

  const getExpiryColor = (days: number) => {
    if (days < 0) return 'text-destructive'
    if (days < 30) return 'text-amber-400'
    return 'text-emerald-400'
  }

  return (
    <div className="space-y-6 p-6">
      <div>
        <h1 className="text-2xl font-bold">Certificate Management</h1>
        <p className="text-sm text-muted-foreground">Monitor and manage TLS certificates</p>
      </div>

      <div className="flex gap-2 border-b border-border">
        <Button
          variant={tab === 'system' ? 'default' : 'ghost'}
          size="sm"
          onClick={() => setTab('system')}
          className="rounded-b-none"
        >
          System Certs
        </Button>
        <Button
          variant={tab === 'tenants' ? 'default' : 'ghost'}
          size="sm"
          onClick={() => setTab('tenants')}
          className="rounded-b-none"
        >
          Tenant Certs
        </Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>
            {tab === 'system' ? 'System Certificates' : 'Tenant Certificates'}
            {activeQuery.isSuccess && (
              <span className="text-sm font-normal text-muted-foreground">({data.length})</span>
            )}
          </CardTitle>
        </CardHeader>
        <CardContent>
          {activeQuery.isError ? (
            <div className="flex flex-col items-center gap-2 rounded-lg border border-destructive/50 p-6 text-center">
              <AlertCircle className="h-8 w-8 text-destructive" />
              <p className="text-sm text-destructive">Failed to load certificates</p>
              <Button variant="outline" size="sm" onClick={() => activeQuery.refetch()}>
                Retry
              </Button>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Domain</TableHead>
                  {tab === 'tenants' && <TableHead>Organization</TableHead>}
                  <TableHead>Issuer</TableHead>
                  <TableHead>Expires</TableHead>
                  <TableHead>Days Left</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Auto Renew</TableHead>
                  <TableHead className="w-[140px]">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {showSkeleton
                  ? Array.from({ length: 5 }).map((_, i) => (
                      <TableRow key={i}>
                        <TableCell><Skeleton className="h-4 w-40" /></TableCell>
                        {tab === 'tenants' && <TableCell><Skeleton className="h-4 w-24" /></TableCell>}
                        <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-12" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-20" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-12" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-20" /></TableCell>
                      </TableRow>
                    ))
                  : data.length === 0 && !showSkeleton ? (
                    <TableRow>
                      <TableCell colSpan={tab === 'tenants' ? 8 : 7} className="py-8 text-center text-muted-foreground">
                        <Shield className="mx-auto mb-2 h-6 w-6" />
                        No certificates found
                      </TableCell>
                    </TableRow>
                  ) : tab === 'system' ? (
                    (data as SystemCertificate[]).map((cert) => (
                      <TableRow key={cert.id}>
                        <TableCell className="font-medium font-mono text-sm">{cert.domain}</TableCell>
                        <TableCell className="text-xs text-muted-foreground">{cert.issuer}</TableCell>
                        <TableCell className="whitespace-nowrap text-xs">
                          {new Date(cert.not_after).toLocaleDateString()}
                        </TableCell>
                        <TableCell>
                          <span className={`font-mono font-bold ${getExpiryColor(cert.days_remaining)}`}>
                            {cert.days_remaining}d
                          </span>
                        </TableCell>
                        <TableCell>{getStatusBadge(cert.status)}</TableCell>
                        <TableCell>
                          <Badge variant={cert.auto_renew ? 'success' : 'secondary'}>
                            {cert.auto_renew ? 'Yes' : 'No'}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          <div className="flex gap-1">
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => renewMutation.mutate(cert.id)}
                              disabled={renewMutation.isPending}
                            >
                              <RefreshCw className="mr-1 h-3.5 w-3.5" />
                              Renew
                            </Button>
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => {
                                if (window.confirm(`Revoke certificate for ${cert.domain}?`)) {
                                  revokeMutation.mutate(cert.id)
                                }
                              }}
                              disabled={revokeMutation.isPending}
                            >
                              <XCircle className="h-3.5 w-3.5 text-destructive" />
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))
                  ) : (
                    (data as TenantCertificate[]).map((cert) => (
                      <TableRow key={cert.id}>
                        <TableCell className="font-medium font-mono text-sm">{cert.domain}</TableCell>
                        <TableCell className="text-xs">{cert.org_name || cert.org_id?.slice(0, 8)}</TableCell>
                        <TableCell className="text-xs text-muted-foreground">{cert.issuer}</TableCell>
                        <TableCell className="whitespace-nowrap text-xs">
                          {new Date(cert.not_after).toLocaleDateString()}
                        </TableCell>
                        <TableCell>
                          <span className={`font-mono font-bold ${getExpiryColor(cert.days_remaining)}`}>
                            {cert.days_remaining}d
                          </span>
                        </TableCell>
                        <TableCell>{getStatusBadge(cert.status)}</TableCell>
                        <TableCell>
                          <Badge variant={cert.auto_renew ? 'success' : 'secondary'}>
                            {cert.auto_renew ? 'Yes' : 'No'}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          <div className="flex gap-1">
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => renewMutation.mutate(cert.id)}
                              disabled={renewMutation.isPending}
                            >
                              <RefreshCw className="mr-1 h-3.5 w-3.5" />
                              Renew
                            </Button>
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => {
                                if (window.confirm(`Revoke certificate for ${cert.domain}?`)) {
                                  revokeMutation.mutate(cert.id)
                                }
                              }}
                              disabled={revokeMutation.isPending}
                            >
                              <XCircle className="h-3.5 w-3.5 text-destructive" />
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
