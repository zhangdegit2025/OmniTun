import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { apiRequest } from '@/lib/api'
import type { CustomerListResponse } from '@/lib/types'
import { Card, CardContent } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Skeleton } from '@/components/ui/Skeleton'
import { AlertCircle, Search, ExternalLink } from 'lucide-react'

function healthColor(score: number): string {
  if (score >= 80) return 'bg-emerald-500'
  if (score >= 50) return 'bg-amber-500'
  return 'bg-red-500'
}

const statusVariant: Record<string, 'success' | 'destructive' | 'warning' | 'secondary'> = {
  active: 'success',
  suspended: 'destructive',
  trial: 'warning',
}

const planVariant: Record<string, 'default' | 'success' | 'secondary'> = {
  free: 'secondary',
  pro: 'default',
  enterprise: 'success',
}

export default function CustomerCRM() {
  const { t } = useTranslation()
  const [search, setSearch] = useState('')
  const [planFilter, setPlanFilter] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [healthFilter, setHealthFilter] = useState('')

  const buildQuery = () => {
    const params = new URLSearchParams()
    if (planFilter) params.set('plan', planFilter)
    if (statusFilter) params.set('status', statusFilter)
    if (healthFilter === 'healthy') { params.set('health_min', '80') }
    else if (healthFilter === 'at_risk') { params.set('health_min', '50'); params.set('health_max', '79') }
    else if (healthFilter === 'critical') { params.set('health_max', '49') }
    return params.toString()
  }

  const customersQuery = useQuery<CustomerListResponse>({
    queryKey: ['admin', 'customers', planFilter, statusFilter, healthFilter],
    queryFn: () => apiRequest<CustomerListResponse>(`/api/admin/v1/customers?${buildQuery()}`),
  })

  const data = customersQuery.data
  const showSkeleton = customersQuery.isLoading

  const filtered = (data?.customers ?? []).filter((c) =>
    !search || c.org_name.toLowerCase().includes(search.toLowerCase()) || c.contact_email.toLowerCase().includes(search.toLowerCase()),
  )

  return (
    <div className="space-y-6 p-6">
      <div>
        <h1 className="text-2xl font-bold">{t('customerCRM.title')}</h1>
        <p className="text-sm text-muted-foreground">{t('customerCRM.subtitle')}</p>
      </div>

      <Card>
        <CardContent className="pt-6">
          <div className="flex flex-wrap items-center gap-4">
            <div className="relative flex-1 min-w-[200px] max-w-sm">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                placeholder={t('customerCRM.searchPlaceholder')}
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="pl-9"
              />
            </div>
            <select
              className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
              value={planFilter}
              onChange={(e) => setPlanFilter(e.target.value)}
            >
              <option value="">{t('customerCRM.allPlans')}</option>
              <option value="free">Free</option>
              <option value="pro">Pro</option>
              <option value="enterprise">Enterprise</option>
            </select>
            <select
              className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
            >
              <option value="">{t('customerCRM.allStatus')}</option>
              <option value="active">Active</option>
              <option value="suspended">Suspended</option>
              <option value="trial">Trial</option>
            </select>
            <select
              className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
              value={healthFilter}
              onChange={(e) => setHealthFilter(e.target.value)}
            >
              <option value="">{t('customerCRM.allHealth')}</option>
              <option value="healthy">{t('customerCRM.healthy')}</option>
              <option value="at_risk">{t('customerCRM.atRisk')}</option>
              <option value="critical">{t('customerCRM.critical')}</option>
            </select>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardContent className="p-0">
          {customersQuery.isError ? (
            <div className="flex flex-col items-center gap-2 rounded-lg border border-destructive/50 p-6 text-center">
              <AlertCircle className="h-8 w-8 text-destructive" />
              <p className="text-sm text-destructive">{t('customerCRM.failedToLoad')}</p>
              <Button variant="outline" size="sm" onClick={() => customersQuery.refetch()}>
                {t('common.retry')}
              </Button>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('customerCRM.organization')}</TableHead>
                  <TableHead>{t('customerCRM.plan')}</TableHead>
                  <TableHead>{t('customerCRM.mrr')}</TableHead>
                  <TableHead>{t('customerCRM.status')}</TableHead>
                  <TableHead>{t('customerCRM.health')}</TableHead>
                  <TableHead>{t('customerCRM.created')}</TableHead>
                  <TableHead className="w-16" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {showSkeleton ? (
                  Array.from({ length: 5 }).map((_, i) => (
                    <TableRow key={i}>
                      <TableCell><Skeleton className="h-4 w-32" /></TableCell>
                      <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                      <TableCell><Skeleton className="h-4 w-20" /></TableCell>
                      <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                      <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                      <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                      <TableCell><Skeleton className="h-4 w-4" /></TableCell>
                    </TableRow>
                  ))
                ) : filtered.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={7}>
                      <div className="py-8 text-center text-sm text-muted-foreground">
                        <Search className="mx-auto mb-2 h-6 w-6" />
                        {t('customerCRM.noCustomers')}
                      </div>
                    </TableCell>
                  </TableRow>
                ) : (
                  filtered.map((customer) => (
                    <TableRow key={customer.id}>
                      <TableCell>
                        <Link to={`/customers/${customer.id}`} className="font-medium text-primary hover:underline">
                          {customer.org_name}
                        </Link>
                        <p className="text-xs text-muted-foreground">{customer.contact_email}</p>
                      </TableCell>
                      <TableCell>
                        <Badge variant={planVariant[customer.plan] ?? 'secondary'}>{customer.plan}</Badge>
                      </TableCell>
                      <TableCell className="font-mono text-sm">
                        {customer.mrr > 0 ? `$${customer.mrr.toLocaleString()}` : '-'}
                      </TableCell>
                      <TableCell>
                        <Badge variant={statusVariant[customer.status] ?? 'secondary'}>{customer.status}</Badge>
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          <div className={`h-2.5 w-2.5 rounded-full ${healthColor(customer.health_score)}`} />
                          <span className="text-sm font-medium">{customer.health_score}</span>
                        </div>
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {new Date(customer.created_at).toLocaleDateString()}
                      </TableCell>
                      <TableCell>
                        <Link to={`/customers/${customer.id}`} className="text-muted-foreground hover:text-foreground">
                          <ExternalLink className="h-4 w-4" />
                        </Link>
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
