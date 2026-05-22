import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { apiRequest } from '@/lib/api'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Skeleton } from '@/components/ui/Skeleton'
import { AlertCircle, Check, Pencil, Save, X } from 'lucide-react'

interface PricingPlan {
  id: string
  name: string
  price_monthly_usd: number
  max_tunnels: number
  max_bandwidth_gb: number
  features: string[]
}

interface PricingData {
  plans: PricingPlan[]
  annual_discount: number
}

const planColors: Record<string, string> = {
  free: 'border-slate-500/30',
  pro: 'border-blue-500/30',
  team: 'border-purple-500/30',
  business: 'border-amber-500/30',
}

export default function PricingConfig() {
  const queryClient = useQueryClient()
  const { t } = useTranslation()
  const [editPlan, setEditPlan] = useState<PricingPlan | null>(null)
  const [annualDiscount, setAnnualDiscount] = useState(20)
  const [editForm, setEditForm] = useState<PricingPlan | null>(null)

  const { data, isLoading, isError, refetch } = useQuery<PricingData>({
    queryKey: ['admin', 'pricing'],
    queryFn: () => apiRequest('/api/admin/v1/pricing'),
  })

  useEffect(() => {
    if (data) {
      setAnnualDiscount(Math.round((data.annual_discount ?? 0.2) * 100))
    }
  }, [data])

  const saveMutation = useMutation({
    mutationFn: (payload: { plans: PricingPlan[]; annual_discount: number }) =>
      apiRequest('/api/admin/v1/pricing', {
        method: 'PUT',
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'pricing'] })
      setEditPlan(null)
      setEditForm(null)
    },
  })

  const handleEdit = (plan: PricingPlan) => {
    setEditPlan(plan)
    setEditForm({ ...plan, features: [...plan.features] })
  }

  const handleCancelEdit = () => {
    setEditPlan(null)
    setEditForm(null)
  }

  const handleSave = () => {
    if (!editForm || !data) return
    const plans = data.plans.map((p) => (p.id === editForm.id ? { ...editForm } : p))
    saveMutation.mutate({
      plans,
      annual_discount: annualDiscount / 100,
    })
  }

  const updateFeature = (index: number, value: string) => {
    if (!editForm) return
    const features = [...editForm.features]
    features[index] = value
    setEditForm({ ...editForm, features })
  }

  const addFeature = () => {
    if (!editForm) return
    setEditForm({ ...editForm, features: [...editForm.features, ''] })
  }

  const removeFeature = (index: number) => {
    if (!editForm) return
    const features = editForm.features.filter((_, i) => i !== index)
    setEditForm({ ...editForm, features })
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">{t('pricing.title')}</h1>
          <p className="text-sm text-muted-foreground">{t('pricing.subtitle')}</p>
        </div>
        {saveMutation.isSuccess && (
          <Badge variant="success" className="px-3 py-1">
            <Check className="mr-1 h-3 w-3" />
            {t('pricing.saved')}
          </Badge>
        )}
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t('pricing.annualDiscount')}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-center gap-4">
            <input
              type="range"
              min={0}
              max={50}
              value={annualDiscount}
              onChange={(e) => setAnnualDiscount(Number(e.target.value))}
              className="h-2 flex-1 cursor-pointer appearance-none rounded-full bg-secondary accent-primary"
            />
            <span className="min-w-[3rem] text-right text-lg font-bold">{annualDiscount}%</span>
          </div>
          <p className="mt-1 text-xs text-muted-foreground">
            {t('pricing.annualHint')}
          </p>
        </CardContent>
      </Card>

      {isError ? (
        <div className="flex flex-col items-center gap-2 rounded-lg border border-destructive/50 p-6 text-center">
          <AlertCircle className="h-8 w-8 text-destructive" />
          <p className="text-sm text-destructive">{t('pricing.failedToLoad')}</p>
          <Button variant="outline" size="sm" onClick={() => refetch()}>
            {t('common.retry')}
          </Button>
        </div>
      ) : (
        <div className="grid gap-6 md:grid-cols-2 xl:grid-cols-4">
          {(data?.plans ?? []).map((plan) => {
            const isEditing = editPlan?.id === plan.id
            const form = isEditing ? editForm : null

            return (
              <Card key={plan.id} className={`relative border-2 ${planColors[plan.id] ?? ''}`}>
                <CardHeader>
                  <CardTitle className="flex items-center justify-between">
                    <span>{plan.name}</span>
                    <Badge variant={plan.id === 'free' ? 'secondary' : 'default'}>
                      {plan.id === 'free' ? 'Free' : `$${plan.price_monthly_usd / 100}/mo`}
                    </Badge>
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-3">
                  {isEditing && form ? (
                    <>
                      <Input
                        label={t('pricing.monthlyPrice')}
                        type="number"
                        value={form.price_monthly_usd}
                        onChange={(e) => setEditForm({ ...form, price_monthly_usd: Number(e.target.value) })}
                      />
                      <Input
                        label={t('pricing.maxTunnels')}
                        type="number"
                        value={form.max_tunnels}
                        onChange={(e) => setEditForm({ ...form, max_tunnels: Number(e.target.value) })}
                      />
                      <Input
                        label={t('pricing.maxBandwidth')}
                        type="number"
                        value={form.max_bandwidth_gb}
                        onChange={(e) => setEditForm({ ...form, max_bandwidth_gb: Number(e.target.value) })}
                      />
                      <div>
                        <label className="text-sm font-medium text-foreground">{t('pricing.features')}</label>
                        <div className="mt-1.5 space-y-1.5">
                          {form.features.map((feat, idx) => (
                            <div key={idx} className="flex gap-1">
                              <input
                                className="flex h-8 w-full rounded-md border border-input bg-transparent px-2 py-1 text-xs shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                                value={feat}
                                onChange={(e) => updateFeature(idx, e.target.value)}
                              />
                              <Button variant="ghost" size="sm" onClick={() => removeFeature(idx)}>
                                <X className="h-3 w-3" />
                              </Button>
                            </div>
                          ))}
                          <Button variant="outline" size="sm" onClick={addFeature}>
                            {t('pricing.addFeature')}
                          </Button>
                        </div>
                      </div>
                      <div className="flex gap-2 pt-2">
                        <Button size="sm" onClick={handleSave} disabled={saveMutation.isPending}>
                          <Save className="mr-1 h-3.5 w-3.5" />
                          {t('common.save')}
                        </Button>
                        <Button variant="outline" size="sm" onClick={handleCancelEdit}>
                          {t('common.cancel')}
                        </Button>
                      </div>
                    </>
                  ) : (
                    <>
                      <div className="space-y-1.5 text-sm">
                        <div className="flex justify-between">
                          <span className="text-muted-foreground">{t('pricing.tunnels')}</span>
                          <span className="font-medium">
                            {plan.max_tunnels >= 999999 ? t('pricing.unlimited') : plan.max_tunnels}
                          </span>
                        </div>
                        <div className="flex justify-between">
                          <span className="text-muted-foreground">{t('pricing.bandwidth')}</span>
                          <span className="font-medium">
                            {plan.max_bandwidth_gb >= 5000 ? `${plan.max_bandwidth_gb / 1024} TB` : `${plan.max_bandwidth_gb} GB`}
                          </span>
                        </div>
                      </div>
                      {plan.features && plan.features.length > 0 && (
                        <div>
                          <p className="mb-1 text-xs font-medium text-muted-foreground">{t('pricing.features')}</p>
                          <ul className="space-y-1">
                            {plan.features.map((feat, idx) => (
                              <li key={idx} className="flex items-start gap-1.5 text-xs">
                                <Check className="mt-0.5 h-3 w-3 flex-shrink-0 text-emerald-400" />
                                {feat}
                              </li>
                            ))}
                          </ul>
                        </div>
                      )}
                      <Button
                        variant="outline"
                        size="sm"
                        className="w-full"
                        onClick={() => handleEdit(plan)}
                      >
                        <Pencil className="mr-1 h-3.5 w-3.5" />
                        {t('common.edit')}
                      </Button>
                    </>
                  )}
                </CardContent>
              </Card>
            )
          })}
        </div>
      )}

      {isLoading && (
        <div className="grid gap-6 md:grid-cols-2 xl:grid-cols-4">
          {[1, 2, 3, 4].map((i) => (
            <Card key={i}>
              <CardHeader>
                <Skeleton className="h-5 w-20" />
              </CardHeader>
              <CardContent className="space-y-3">
                <Skeleton className="h-4 w-full" />
                <Skeleton className="h-4 w-3/4" />
                <Skeleton className="h-4 w-1/2" />
                <Skeleton className="h-8 w-full" />
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}
