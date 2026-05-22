import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { apiRequest } from '@/lib/api'
import type { CustomerDetail as CustomerDetailType, CustomerHealth } from '@/lib/types'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Skeleton } from '@/components/ui/Skeleton'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts'
import {
  ArrowLeft,
  AlertCircle,
  Building2,
  Mail,
  CreditCard,
  Activity,
  Users,
  Wifi,
} from 'lucide-react'

const tabs = ['Overview', 'Billing', 'Usage', 'Activity'] as const

function healthColor(score: number): string {
  if (score >= 80) return 'bg-emerald-500'
  if (score >= 50) return 'bg-amber-500'
  return 'bg-red-500'
}

function healthTextColor(score: number): string {
  if (score >= 80) return 'text-emerald-500'
  if (score >= 50) return 'text-amber-500'
  return 'text-red-500'
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

const invoiceStatusVariant: Record<string, 'success' | 'warning' | 'destructive' | 'secondary'> = {
  paid: 'success',
  pending: 'warning',
  failed: 'destructive',
}

export default function CustomerDetail() {
  const { id } = useParams<{ id: string }>()
  const [activeTab, setActiveTab] = useState<string>('Overview')
  const { t } = useTranslation()

  const customerQuery = useQuery<CustomerDetailType>({
    queryKey: ['admin', 'customer', id],
    queryFn: () => apiRequest<CustomerDetailType>(`/api/admin/v1/customers/${id}`),
    enabled: !!id,
  })

  const healthQuery = useQuery<CustomerHealth>({
    queryKey: ['admin', 'customer', id, 'health'],
    queryFn: () => apiRequest<CustomerHealth>(`/api/admin/v1/customers/${id}/health`),
    enabled: !!id,
  })

  const customer = customerQuery.data
  const health = healthQuery.data
  const loading = customerQuery.isLoading
  const error = customerQuery.isError

  if (error) {
    return (
      <div className="flex flex-col items-center gap-2 rounded-lg border border-destructive/50 p-6 text-center m-6">
        <AlertCircle className="h-8 w-8 text-destructive" />
        <p className="text-sm text-destructive">{t('customerDetail.failedToLoad')}</p>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={() => customerQuery.refetch()}>{t('common.retry')}</Button>
          <Link to="/customers" className="inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring border border-input bg-transparent shadow-sm hover:bg-accent hover:text-accent-foreground h-8 px-3 text-xs">
            <ArrowLeft className="h-4 w-4" />{t('customerDetail.backToCustomers')}
          </Link>
        </div>
      </div>
    )
  }

  if (loading) {
    return (
      <div className="space-y-6 p-6">
        <Skeleton className="h-8 w-64" />
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Skeleton className="h-28" />
          <Skeleton className="h-28" />
          <Skeleton className="h-28" />
          <Skeleton className="h-28" />
        </div>
        <Skeleton className="h-96 w-full" />
      </div>
    )
  }

  if (!customer) return null

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center gap-4">
        <Link to="/customers" className="inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring border border-input bg-transparent shadow-sm hover:bg-accent hover:text-accent-foreground h-8 px-3 text-xs">
          <ArrowLeft className="h-4 w-4" />{t('customerDetail.back')}
        </Link>
        <div>
          <h1 className="text-2xl font-bold">{customer.org_name}</h1>
          <p className="text-sm text-muted-foreground">{t('customerDetail.customerView')}</p>
        </div>
      </div>

      <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">{t('customerDetail.healthScore')}</CardTitle>
            <Activity className="h-4 w-4 text-emerald-500" />
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              <div className={`h-3 w-3 rounded-full ${healthColor(customer.health_score)}`} />
              <p className={`text-2xl font-bold ${healthTextColor(customer.health_score)}`}>
                {customer.health_score}
              </p>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">{t('customerDetail.mrr')}</CardTitle>
            <CreditCard className="h-4 w-4 text-primary" />
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">
              ${customer.mrr.toLocaleString('en-US', { minimumFractionDigits: 0, maximumFractionDigits: 0 })}
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">{t('customerDetail.users')}</CardTitle>
            <Users className="h-4 w-4 text-blue-500" />
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">{customer.user_count}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">{t('customerDetail.tunnels')}</CardTitle>
            <Wifi className="h-4 w-4 text-emerald-500" />
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">{customer.tunnel_count}</p>
          </CardContent>
        </Card>
      </section>

      <div className="flex gap-1 border-b border-border">
        {tabs.map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
              activeTab === tab
                ? 'border-primary text-primary'
                : 'border-transparent text-muted-foreground hover:text-foreground'
            }`}
          >
            {tab}
          </button>
        ))}
      </div>

      {activeTab === 'Overview' && (
        <div className="grid gap-6 lg:grid-cols-2">
          <Card>
            <CardHeader>
              <CardTitle>{t('customerDetail.orgInfo')}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="flex items-center justify-between text-sm">
                <span className="text-muted-foreground">{t('customerDetail.name')}</span>
                <span className="font-medium">{customer.org_name}</span>
              </div>
              <div className="flex items-center justify-between text-sm">
                <span className="text-muted-foreground">{t('customerDetail.plan')}</span>
                <Badge variant={planVariant[customer.plan] ?? 'secondary'}>{customer.plan}</Badge>
              </div>
              <div className="flex items-center justify-between text-sm">
                <span className="text-muted-foreground">{t('customerDetail.status')}</span>
                <Badge variant={statusVariant[customer.status] ?? 'secondary'}>{customer.status}</Badge>
              </div>
              <div className="flex items-center justify-between text-sm">
                <span className="text-muted-foreground">{t('customerDetail.customerSince')}</span>
                <span>{new Date(customer.created_at).toLocaleDateString()}</span>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>{t('customerDetail.contacts')}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              {customer.contacts.map((contact, i) => (
                <div key={i} className="flex items-center gap-3 rounded-md border border-border p-3">
                  <Building2 className="h-5 w-5 text-muted-foreground" />
                  <div>
                    <p className="text-sm font-medium">{contact.name}</p>
                    <div className="flex items-center gap-1 text-xs text-muted-foreground">
                      <Mail className="h-3 w-3" />
                      {contact.email}
                    </div>
                  </div>
                  <Badge variant="secondary" className="ml-auto">{contact.role}</Badge>
                </div>
              ))}
            </CardContent>
          </Card>

          <Card className="lg:col-span-2">
            <CardHeader>
              <CardTitle>{t('customerDetail.healthScoreTrend')}</CardTitle>
            </CardHeader>
            <CardContent>
              {healthQuery.isLoading ? (
                <Skeleton className="h-64 w-full" />
              ) : (
                <ResponsiveContainer width="100%" height={250}>
                  <LineChart data={health?.trend ?? []}>
                    <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
                    <XAxis dataKey="date" className="text-xs" />
                    <YAxis domain={[70, 100]} className="text-xs" />
                    <Tooltip
                      contentStyle={{ borderRadius: '8px', border: '1px solid hsl(var(--border))' }}
                    />
                    <Line
                      type="monotone"
                      dataKey="score"
                      stroke="#22c55e"
                      strokeWidth={2}
                      dot={{ r: 4 }}
                      name="Health Score"
                    />
                  </LineChart>
                </ResponsiveContainer>
              )}
            </CardContent>
          </Card>
        </div>
      )}

      {activeTab === 'Billing' && (
        <div className="grid gap-6 lg:grid-cols-2">
          <Card>
            <CardHeader>
              <CardTitle>{t('customerDetail.subscriptionHistory')}</CardTitle>
            </CardHeader>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('customerDetail.plan')}</TableHead>
                    <TableHead>{t('customerDetail.status')}</TableHead>
                    <TableHead>{t('customerDetail.start')}</TableHead>
                    <TableHead>{t('customerDetail.end')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {customer.subscriptions.map((sub) => (
                    <TableRow key={sub.id}>
                      <TableCell>
                        <Badge variant={planVariant[sub.plan] ?? 'secondary'}>{sub.plan}</Badge>
                      </TableCell>
                      <TableCell>
                        <Badge variant={sub.status === 'active' ? 'success' : 'secondary'}>{sub.status}</Badge>
                      </TableCell>
                      <TableCell className="text-muted-foreground">{sub.start}</TableCell>
                      <TableCell className="text-muted-foreground">{sub.end || '-'}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>{t('customerDetail.invoices')}</CardTitle>
            </CardHeader>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('customerDetail.invoice')}</TableHead>
                    <TableHead>{t('customerDetail.amount')}</TableHead>
                    <TableHead>{t('customerDetail.date')}</TableHead>
                    <TableHead>{t('customerDetail.status')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {customer.invoices.map((inv) => (
                    <TableRow key={inv.id}>
                      <TableCell className="font-mono text-xs">{inv.id}</TableCell>
                      <TableCell className="font-medium">
                        ${inv.amount.toLocaleString('en-US', { minimumFractionDigits: 2 })}
                      </TableCell>
                      <TableCell className="text-muted-foreground">{inv.date}</TableCell>
                      <TableCell>
                        <Badge variant={invoiceStatusVariant[inv.status] ?? 'secondary'}>{inv.status}</Badge>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </div>
      )}

      {activeTab === 'Usage' && (
        <div className="grid gap-6 lg:grid-cols-2">
          <Card>
            <CardHeader>
              <CardTitle>{t('customerDetail.tunnels')}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-2 gap-4">
                <div className="rounded-lg border border-border p-4">
                  <p className="text-xs text-muted-foreground">{t('customerDetail.activeTunnels')}</p>
                  <p className="text-2xl font-bold">{customer.usage.tunnels}</p>
                </div>
                <div className="rounded-lg border border-border p-4">
                  <p className="text-xs text-muted-foreground">{t('customerDetail.bandwidthUsed')}</p>
                  <p className="text-2xl font-bold">{customer.usage.bandwidth_gb.toFixed(1)} GB</p>
                </div>
                <div className="rounded-lg border border-border p-4">
                  <p className="text-xs text-muted-foreground">{t('customerDetail.activeUsers')}</p>
                  <p className="text-2xl font-bold">{customer.usage.active_users}</p>
                </div>
                <div className="rounded-lg border border-border p-4">
                  <p className="text-xs text-muted-foreground">{t('customerDetail.connections')}</p>
                  <p className="text-2xl font-bold">{customer.usage.connections}</p>
                </div>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>{t('customerDetail.healthFactors')}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              {healthQuery.isLoading ? (
                <Skeleton className="h-48 w-full" />
              ) : (
                (health?.factors ?? []).map((factor) => (
                  <div key={factor.factor} className="flex items-center gap-3">
                    <span className="w-36 text-sm text-muted-foreground">{factor.factor}</span>
                    <div className="flex-1">
                      <div className="h-4 rounded-full bg-muted overflow-hidden">
                        <div
                          className={`h-full rounded-full transition-all ${
                            factor.score >= 80 ? 'bg-emerald-500' : factor.score >= 50 ? 'bg-amber-500' : 'bg-red-500'
                          }`}
                          style={{ width: `${factor.score}%` }}
                        />
                      </div>
                    </div>
                    <span className="w-10 text-right text-sm font-medium">{factor.score}</span>
                  </div>
                ))
              )}
            </CardContent>
          </Card>
        </div>
      )}

      {activeTab === 'Activity' && (
        <Card>
          <CardHeader>
            <CardTitle>{t('customerDetail.eventTimeline')}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="relative space-y-0">
              {customer.activity.map((event, i) => (
                <div key={i} className="flex gap-4 pb-4">
                  <div className="relative">
                    <div className={`h-2.5 w-2.5 rounded-full mt-1.5 ${
                      event.action.includes('created') || event.action.includes('verified') || event.action.includes('upgraded') || event.action.includes('paid')
                        ? 'bg-emerald-500'
                        : event.action.includes('invited')
                          ? 'bg-blue-500'
                          : 'bg-muted-foreground'
                    }`} />
                    {i < customer.activity.length - 1 && (
                      <div className="absolute top-4 left-1/2 h-full w-px -translate-x-1/2 bg-border" />
                    )}
                  </div>
                  <div className="flex-1">
                    <div className="flex items-center gap-2">
                      <Badge variant="secondary" className="text-xs">{event.action}</Badge>
                      <span className="text-xs text-muted-foreground">
                        {new Date(event.timestamp).toLocaleString()}
                      </span>
                    </div>
                    <p className="mt-1 text-sm">{event.detail}</p>
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
