import { useQuery, useMutation } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { apiRequest } from '@/lib/api'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card'
import { Skeleton } from '@/components/ui/Skeleton'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { useToast } from '@/components/ui/useToast'
import {
  CreditCard,
  Check,
  ArrowUpRight,
  AlertCircle,
  Zap,
  Server,
  HardDrive,
} from 'lucide-react'

interface BillingPlanData {
  id: string
  name: string
  price_monthly_usd: number
  max_tunnels: number
  max_bandwidth_gb: number
  features: string[]
}

interface PlanResponse {
  current_plan: BillingPlanData
  available_plans: BillingPlanData[]
}

interface UsageResponse {
  plan: string
  tunnels_used: number
  tunnels_limit: number
  bandwidth_bytes: number
  bandwidth_limit: number
  bandwidth_gb: number
  bandwidth_limit_gb: number
  period_start: string
  period_end: string
}

interface Invoice {
  id: string
  plan_id: string
  amount_usd: number
  status: string
  created_at: string
}

const planColors: Record<string, string> = {
  free: 'bg-slate-100 border-slate-200',
  pro: 'bg-blue-50 border-blue-200',
  team: 'bg-purple-50 border-purple-200',
  business: 'bg-amber-50 border-amber-200',
}

const planBadgeVariant: Record<string, 'default' | 'success' | 'warning'> = {
  free: 'default',
  pro: 'success',
  team: 'warning',
  business: 'default',
}

export default function Billing() {
  const { t } = useTranslation()
  const { toast } = useToast()

  const planQuery = useQuery<PlanResponse>({
    queryKey: ['billing', 'plan'],
    queryFn: () => apiRequest<PlanResponse>('/v1/billing/plan'),
  })

  const usageQuery = useQuery<UsageResponse>({
    queryKey: ['billing', 'usage'],
    queryFn: () => apiRequest<UsageResponse>('/v1/billing/usage'),
  })

  const invoicesQuery = useQuery<Invoice[]>({
    queryKey: ['billing', 'invoices'],
    queryFn: () => apiRequest<Invoice[]>('/v1/billing/invoices'),
  })

  const checkoutMutation = useMutation({
    mutationFn: (planId: string) =>
      apiRequest<{ url: string }>('/v1/billing/checkout', {
        method: 'POST',
        body: JSON.stringify({
          plan_id: planId,
          success_url: window.location.origin + '/billing?success=true',
          cancel_url: window.location.origin + '/billing?canceled=true',
        }),
      }),
    onSuccess: (data) => {
      if (data.url) {
        window.location.href = data.url
      }
    },
    onError: () => {
      toast({ title: t('billing.checkout_failed'), variant: 'error' })
    },
  })

  const isLoading = planQuery.isLoading || usageQuery.isLoading

  const currentPlan = planQuery.data?.current_plan
  const availablePlans = planQuery.data?.available_plans ?? []
  const usage = usageQuery.data

  const tunnelPercent = usage
    ? Math.min(100, (usage.tunnels_used / Math.max(1, usage.tunnels_limit)) * 100)
    : 0
  const bandwidthPercent = usage
    ? Math.min(100, (usage.bandwidth_gb / Math.max(0.01, usage.bandwidth_limit_gb)) * 100)
    : 0

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">{t('billing.title')}</h1>
          <p className="text-sm text-muted-foreground">{t('billing.subtitle')}</p>
        </div>
      </div>

      {/* Current Plan */}
      {isLoading ? (
        <Card>
          <CardContent className="p-6">
            <Skeleton className="h-24 w-full" />
          </CardContent>
        </Card>
      ) : currentPlan ? (
        <Card className={`border-2 ${planColors[currentPlan.id] ?? 'border-slate-200'}`}>
          <CardHeader className="flex flex-row items-center justify-between">
            <div>
              <CardTitle className="text-lg">{t('billing.current_plan')}</CardTitle>
              <p className="text-sm text-muted-foreground">{t('billing.current_plan_desc')}</p>
            </div>
            <Badge variant={planBadgeVariant[currentPlan.id] ?? 'secondary'}>
              {currentPlan.name}
            </Badge>
          </CardHeader>
          <CardContent className="space-y-4">
            {/* Usage bars */}
            <div className="space-y-3">
              <div>
                <div className="flex items-center justify-between text-sm mb-1">
                  <span className="flex items-center gap-1">
                    <Server className="h-3.5 w-3.5" />
                    {t('billing.tunnels_usage')}
                  </span>
                  <span className="text-muted-foreground">
                    {usage?.tunnels_used ?? 0} / {usage?.tunnels_limit ?? 1}
                  </span>
                </div>
                <div className="h-2 rounded-full bg-muted overflow-hidden">
                  <div
                    className={`h-full rounded-full transition-all ${tunnelPercent > 80 ? 'bg-destructive' : 'bg-primary'}`}
                    style={{ width: `${tunnelPercent}%` }}
                  />
                </div>
              </div>
              <div>
                <div className="flex items-center justify-between text-sm mb-1">
                  <span className="flex items-center gap-1">
                    <HardDrive className="h-3.5 w-3.5" />
                    {t('billing.bandwidth_usage')}
                  </span>
                  <span className="text-muted-foreground">
                    {(usage?.bandwidth_gb ?? 0).toFixed(1)} / {(usage?.bandwidth_limit_gb ?? 0).toFixed(0)} GB
                  </span>
                </div>
                <div className="h-2 rounded-full bg-muted overflow-hidden">
                  <div
                    className={`h-full rounded-full transition-all ${bandwidthPercent > 80 ? 'bg-destructive' : 'bg-primary'}`}
                    style={{ width: `${bandwidthPercent}%` }}
                  />
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      ) : null}

      {/* Available Plans */}
      <div>
        <h2 className="text-lg font-semibold mb-4">{t('billing.available_plans')}</h2>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {availablePlans.map((plan) => {
            const isCurrent = plan.id === currentPlan?.id
            const isFree = plan.id === 'free'
            return (
              <Card
                key={plan.id}
                className={`relative border-2 ${isCurrent ? 'ring-2 ring-primary border-primary' : 'border-border'}`}
              >
                {isCurrent && (
                  <div className="absolute -top-2 left-4">
                    <Badge variant="success">{t('billing.current')}</Badge>
                  </div>
                )}
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <CreditCard className="h-5 w-5" />
                    {plan.name}
                  </CardTitle>
                  <div className="mt-2">
                    {plan.price_monthly_usd === 0 ? (
                      <span className="text-3xl font-bold">{t('billing.free')}</span>
                    ) : (
                      <span className="text-3xl font-bold">
                        ${(plan.price_monthly_usd / 100).toFixed(0)}
                        <span className="text-sm font-normal text-muted-foreground">/mo</span>
                      </span>
                    )}
                  </div>
                </CardHeader>
                <CardContent className="space-y-3">
                  <ul className="space-y-2 text-sm">
                    <li className="flex items-center gap-2">
                      <Server className="h-4 w-4 text-muted-foreground" />
                      {t('billing.tunnels_count', { count: plan.max_tunnels >= 999999 ? '∞' : plan.max_tunnels })}
                    </li>
                    <li className="flex items-center gap-2">
                      <HardDrive className="h-4 w-4 text-muted-foreground" />
                      {t('billing.bandwidth_count', { gb: plan.max_bandwidth_gb >= 999 ? '∞' : plan.max_bandwidth_gb })}
                    </li>
                    {plan.features?.map((f, i) => (
                      <li key={i} className="flex items-center gap-2">
                        <Check className="h-4 w-4 text-emerald-500" />
                        {f}
                      </li>
                    ))}
                  </ul>
                  {!isCurrent && !isFree && (
                    <Button
                      className="w-full"
                      onClick={() => checkoutMutation.mutate(plan.id)}
                      disabled={checkoutMutation.isPending}
                    >
                      {checkoutMutation.isPending ? (
                        t('billing.upgrading')
                      ) : (
                        <>
                          <ArrowUpRight className="mr-1 h-4 w-4" />
                          {t('billing.upgrade_to', { plan: plan.name })}
                        </>
                      )}
                    </Button>
                  )}
                  {isFree && !isCurrent && (
                    <Button variant="outline" className="w-full" onClick={() => checkoutMutation.mutate('free')}>
                      {t('billing.downgrade')}
                    </Button>
                  )}
                </CardContent>
              </Card>
            )
          })}
        </div>
      </div>

      {/* Usage Details */}
      {usageQuery.isLoading ? (
        <Card>
          <CardContent className="p-6">
            <Skeleton className="h-32 w-full" />
          </CardContent>
        </Card>
      ) : usage ? (
        <Card>
          <CardHeader>
            <CardTitle>{t('billing.usage_details')}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <div className="rounded-lg border p-4">
                <div className="flex items-center gap-2 text-sm text-muted-foreground mb-1">
                  <Zap className="h-4 w-4" />
                  {t('billing.usage_tunnels')}
                </div>
                <p className="text-2xl font-bold">
                  {usage.tunnels_used}
                  <span className="text-sm font-normal text-muted-foreground">
                    {' '}/ {usage.tunnels_limit}
                  </span>
                </p>
              </div>
              <div className="rounded-lg border p-4">
                <div className="flex items-center gap-2 text-sm text-muted-foreground mb-1">
                  <HardDrive className="h-4 w-4" />
                  {t('billing.usage_bandwidth')}
                </div>
                <p className="text-2xl font-bold">
                  {usage.bandwidth_gb.toFixed(1)}
                  <span className="text-sm font-normal text-muted-foreground"> GB</span>
                </p>
              </div>
              <div className="rounded-lg border p-4">
                <div className="flex items-center gap-2 text-sm text-muted-foreground mb-1">
                  <CreditCard className="h-4 w-4" />
                  {t('billing.period_start')}
                </div>
                <p className="text-lg font-bold">
                  {new Date(usage.period_start).toLocaleDateString()}
                </p>
              </div>
              <div className="rounded-lg border p-4">
                <div className="flex items-center gap-2 text-sm text-muted-foreground mb-1">
                  <CreditCard className="h-4 w-4" />
                  {t('billing.period_end')}
                </div>
                <p className="text-lg font-bold">
                  {new Date(usage.period_end).toLocaleDateString()}
                </p>
              </div>
            </div>
          </CardContent>
        </Card>
      ) : usageQuery.isError ? (
        <Card>
          <CardContent className="flex flex-col items-center gap-2 py-6">
            <AlertCircle className="h-8 w-8 text-destructive" />
            <p className="text-sm text-destructive">{t('billing.failed_usage')}</p>
            <Button variant="outline" size="sm" onClick={() => usageQuery.refetch()}>
              {t('common.retry')}
            </Button>
          </CardContent>
        </Card>
      ) : null}

      {/* Invoice History */}
      <Card>
        <CardHeader>
          <CardTitle>{t('billing.invoice_history')}</CardTitle>
        </CardHeader>
        <CardContent>
          {invoicesQuery.isLoading ? (
            <div className="space-y-3">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          ) : invoicesQuery.isError ? (
            <div className="flex flex-col items-center gap-2 py-6">
              <AlertCircle className="h-8 w-8 text-destructive" />
              <p className="text-sm text-destructive">{t('billing.failed_invoices')}</p>
              <Button variant="outline" size="sm" onClick={() => invoicesQuery.refetch()}>
                {t('common.retry')}
              </Button>
            </div>
          ) : invoicesQuery.data?.length === 0 ? (
            <div className="flex flex-col items-center gap-3 py-10 text-center">
              <CreditCard className="h-10 w-10 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">{t('billing.no_invoices')}</p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('billing.table.invoice')}</TableHead>
                  <TableHead>{t('billing.table.plan')}</TableHead>
                  <TableHead>{t('billing.table.amount')}</TableHead>
                  <TableHead>{t('billing.table.status')}</TableHead>
                  <TableHead>{t('billing.table.date')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {invoicesQuery.data?.map((inv) => (
                  <TableRow key={inv.id}>
                    <TableCell className="font-mono text-xs">{inv.id}</TableCell>
                    <TableCell>
                      <Badge variant="secondary">{inv.plan_id}</Badge>
                    </TableCell>
                    <TableCell>${inv.amount_usd.toFixed(2)}</TableCell>
                    <TableCell>
                      <Badge variant={inv.status === 'paid' ? 'success' : 'warning'}>
                        {inv.status}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {new Date(inv.created_at).toLocaleDateString()}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
