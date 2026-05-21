import { useQuery } from '@tanstack/react-query'
import { apiRequest } from '@/lib/api'
import type { MRRData, ChurnData, FunnelData } from '@/lib/types'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card'
import { Skeleton } from '@/components/ui/Skeleton'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
  BarChart,
  Bar,
  Cell,
} from 'recharts'
import { AlertCircle, TrendingUp, Users, DollarSign, Percent, ArrowUpRight, ArrowDownRight } from 'lucide-react'

const COLORS = ['#22c55e', '#3b82f6', '#f59e0b', '#ef4444']

function formatCurrency(val: number): string {
  return '$' + val.toLocaleString('en-US', { minimumFractionDigits: 0, maximumFractionDigits: 0 })
}

function formatPercent(val: number): string {
  return val.toFixed(1) + '%'
}

export default function RevenueDashboard() {
  const mrrQuery = useQuery<MRRData>({
    queryKey: ['admin', 'revenue', 'mrr'],
    queryFn: () => apiRequest<MRRData>('/api/admin/v1/revenue/mrr'),
  })

  const churnQuery = useQuery<ChurnData>({
    queryKey: ['admin', 'revenue', 'churn'],
    queryFn: () => apiRequest<ChurnData>('/api/admin/v1/revenue/churn'),
  })

  const funnelQuery = useQuery<FunnelData>({
    queryKey: ['admin', 'revenue', 'funnel'],
    queryFn: () => apiRequest<FunnelData>('/api/admin/v1/revenue/funnel'),
  })

  const mrrData = mrrQuery.data
  const churnData = churnQuery.data
  const funnelData = funnelQuery.data
  const loading = mrrQuery.isLoading || churnQuery.isLoading || funnelQuery.isLoading
  const error = mrrQuery.isError || churnQuery.isError || funnelQuery.isError

  if (error) {
    return (
      <div className="flex flex-col items-center gap-2 rounded-lg border border-destructive/50 p-6 text-center">
        <AlertCircle className="h-8 w-8 text-destructive" />
        <p className="text-sm text-destructive">Failed to load revenue data</p>
        <Button variant="outline" size="sm" onClick={() => { mrrQuery.refetch(); churnQuery.refetch(); funnelQuery.refetch() }}>
          Retry
        </Button>
      </div>
    )
  }

  return (
    <div className="space-y-6 p-6">
      <div>
        <h1 className="text-2xl font-bold">Revenue Dashboard</h1>
        <p className="text-sm text-muted-foreground">Revenue metrics, trends, and forecasting</p>
      </div>

      <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {loading ? (
          <>
            <Skeleton className="h-28" />
            <Skeleton className="h-28" />
            <Skeleton className="h-28" />
            <Skeleton className="h-28" />
          </>
        ) : (
          <>
            <Card>
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardTitle className="text-sm font-medium text-muted-foreground">MRR</CardTitle>
                <DollarSign className="h-4 w-4 text-emerald-500" />
              </CardHeader>
              <CardContent>
                <div className="flex items-baseline gap-2">
                  <p className="text-2xl font-bold">{formatCurrency(mrrData?.mrr ?? 0)}</p>
                  <Badge variant="success" className="flex items-center gap-0.5">
                    <TrendingUp className="h-3 w-3" /> 6.2%
                  </Badge>
                </div>
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardTitle className="text-sm font-medium text-muted-foreground">ARR</CardTitle>
                <DollarSign className="h-4 w-4 text-primary" />
              </CardHeader>
              <CardContent>
                <p className="text-2xl font-bold">{formatCurrency(mrrData?.arr ?? 0)}</p>
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardTitle className="text-sm font-medium text-muted-foreground">Active Subscriptions</CardTitle>
                <Users className="h-4 w-4 text-blue-500" />
              </CardHeader>
              <CardContent>
                <div className="flex items-baseline gap-2">
                  <p className="text-2xl font-bold">{mrrData?.active_subscriptions ?? 0}</p>
                  <Badge variant="success" className="flex items-center gap-0.5">
                    <ArrowUpRight className="h-3 w-3" /> 12
                  </Badge>
                </div>
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardTitle className="text-sm font-medium text-muted-foreground">Churn Rate</CardTitle>
                <Percent className="h-4 w-4 text-amber-500" />
              </CardHeader>
              <CardContent>
                <div className="flex items-baseline gap-2">
                  <p className="text-2xl font-bold">{formatPercent(churnData?.churn_rate ?? 0)}</p>
                  <Badge variant="warning" className="flex items-center gap-0.5">
                    <ArrowDownRight className="h-3 w-3" /> 0.4pp
                  </Badge>
                </div>
              </CardContent>
            </Card>
          </>
        )}
      </section>

      <section className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>MRR Trend Breakdown</CardTitle>
          </CardHeader>
          <CardContent>
            {loading ? (
              <Skeleton className="h-72 w-full" />
            ) : (
              <ResponsiveContainer width="100%" height={300}>
                <LineChart data={mrrData?.trend ?? []}>
                  <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
                  <XAxis dataKey="month" className="text-xs" />
                  <YAxis className="text-xs" tickFormatter={(v) => '$' + (v / 1000).toFixed(0) + 'k'} />
                  <Tooltip
                    formatter={(value: number) => [formatCurrency(value), '']}
                    contentStyle={{ borderRadius: '8px', border: '1px solid hsl(var(--border))' }}
                  />
                  <Legend />
                  <Line type="monotone" dataKey="mrr" name="MRR" stroke="#8b5cf6" strokeWidth={2} dot={{ r: 4 }} />
                  <Line type="monotone" dataKey="new" name="New" stroke="#22c55e" strokeWidth={1.5} dot={false} />
                  <Line type="monotone" dataKey="expansion" name="Expansion" stroke="#3b82f6" strokeWidth={1.5} dot={false} />
                  <Line type="monotone" dataKey="contraction" name="Contraction" stroke="#f59e0b" strokeWidth={1.5} dot={false} />
                  <Line type="monotone" dataKey="churn" name="Churn" stroke="#ef4444" strokeWidth={1.5} dot={false} />
                </LineChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Revenue Forecast</CardTitle>
          </CardHeader>
          <CardContent>
            {loading ? (
              <Skeleton className="h-72 w-full" />
            ) : (
              <div className="space-y-4">
                {[
                  { label: '30 Day', value: mrrData?.forecast.days_30 ?? 0 },
                  { label: '60 Day', value: mrrData?.forecast.days_60 ?? 0 },
                  { label: '90 Day', value: mrrData?.forecast.days_90 ?? 0 },
                ].map((item, i) => (
                  <div key={item.label} className="flex items-center gap-4">
                    <span className="w-20 text-sm font-medium text-muted-foreground">{item.label}</span>
                    <div className="flex-1">
                      <div className="h-8 rounded-md bg-muted overflow-hidden">
                        <div
                          className="h-full rounded-md bg-gradient-to-r from-emerald-500 to-blue-500 transition-all"
                          style={{ width: `${Math.min((item.value / 70000) * 100, 100)}%` }}
                        />
                      </div>
                    </div>
                    <span className="w-24 text-right text-sm font-semibold">{formatCurrency(item.value)}</span>
                  </div>
                ))}
                <div className="flex items-center gap-4 pt-2">
                  <span className="w-20 text-sm font-medium text-muted-foreground">Current</span>
                  <div className="flex-1">
                    <div className="h-8 rounded-md bg-muted overflow-hidden">
                      <div
                        className="h-full rounded-md bg-primary transition-all"
                        style={{ width: `${Math.min(((mrrData?.mrr ?? 0) / 70000) * 100, 100)}%` }}
                      />
                    </div>
                  </div>
                  <span className="w-24 text-right text-sm font-semibold">{formatCurrency(mrrData?.mrr ?? 0)}</span>
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      </section>

      <section className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Conversion Funnel</CardTitle>
          </CardHeader>
          <CardContent>
            {loading ? (
              <Skeleton className="h-72 w-full" />
            ) : (
              <ResponsiveContainer width="100%" height={300}>
                <BarChart data={funnelData?.stages ?? []} layout="vertical">
                  <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
                  <XAxis type="number" className="text-xs" />
                  <YAxis dataKey="name" type="category" className="text-xs" width={80} />
                  <Tooltip
                    formatter={(value: number) => [value.toLocaleString(), 'Count']}
                    contentStyle={{ borderRadius: '8px', border: '1px solid hsl(var(--border))' }}
                  />
                  <Bar dataKey="count" name="Users" radius={[0, 4, 4, 0]}>
                    {(funnelData?.stages ?? []).map((_, idx) => (
                      <Cell key={idx} fill={COLORS[idx % COLORS.length]} />
                    ))}
                  </Bar>
                </BarChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Churn Metrics</CardTitle>
          </CardHeader>
          <CardContent>
            {loading ? (
              <Skeleton className="h-72 w-full" />
            ) : (
              <div className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div className="rounded-lg border border-border p-4">
                    <p className="text-xs text-muted-foreground">Voluntary Churn</p>
                    <p className="text-xl font-bold text-destructive">{formatPercent(churnData?.voluntary_churn ?? 0)}</p>
                  </div>
                  <div className="rounded-lg border border-border p-4">
                    <p className="text-xs text-muted-foreground">Involuntary Churn</p>
                    <p className="text-xl font-bold text-amber-500">{formatPercent(churnData?.involuntary_churn ?? 0)}</p>
                  </div>
                  <div className="rounded-lg border border-border p-4">
                    <p className="text-xs text-muted-foreground">Retention Rate</p>
                    <p className="text-xl font-bold text-emerald-500">{formatPercent(churnData?.retention_rate ?? 0)}</p>
                  </div>
                  <div className="rounded-lg border border-border p-4">
                    <p className="text-xs text-muted-foreground">At-Risk Customers</p>
                    <p className="text-xl font-bold text-amber-500">{churnData?.at_risk_customers ?? 0}</p>
                  </div>
                </div>
                <ResponsiveContainer width="100%" height={160}>
                  <LineChart data={churnData?.monthly_trend ?? []}>
                    <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
                    <XAxis dataKey="month" className="text-xs" />
                    <YAxis domain={[2, 4]} className="text-xs" tickFormatter={(v) => v + '%'} />
                    <Tooltip
                      formatter={(value: number) => [formatPercent(value), 'Churn Rate']}
                      contentStyle={{ borderRadius: '8px', border: '1px solid hsl(var(--border))' }}
                    />
                    <Line type="monotone" dataKey="rate" stroke="#f59e0b" strokeWidth={2} dot={{ r: 4 }} />
                  </LineChart>
                </ResponsiveContainer>
              </div>
            )}
          </CardContent>
        </Card>
      </section>
    </div>
  )
}
