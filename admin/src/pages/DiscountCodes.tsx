import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { apiRequest } from '@/lib/api'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Skeleton } from '@/components/ui/Skeleton'
import { Plus, Pencil, Trash2, ToggleLeft, ToggleRight, AlertCircle, Tag } from 'lucide-react'

interface DiscountCode {
  id: string
  code: string
  type: 'percentage' | 'fixed'
  value: number
  uses: number
  max_uses: number
  active: boolean
  expires_at: string
  created_at: string
  applicable_plans: string
}

interface DiscountListResponse {
  discounts: DiscountCode[]
}

export default function DiscountCodes() {
  const queryClient = useQueryClient()
  const { t } = useTranslation()
  const [showForm, setShowForm] = useState(false)
  const [editing, setEditing] = useState<string | null>(null)
  const [form, setForm] = useState({
    code: '',
    type: 'percentage' as 'percentage' | 'fixed',
    value: 0,
    max_uses: 0,
    expires_at: '',
    applicable_plans: '',
  })

  const listQuery = useQuery<DiscountListResponse>({
    queryKey: ['admin', 'discounts'],
    queryFn: () => apiRequest('/api/admin/v1/discounts'),
  })

  const createMutation = useMutation({
    mutationFn: (data: typeof form) =>
      apiRequest('/api/admin/v1/discounts', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'discounts'] })
      resetForm()
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<DiscountCode> }) =>
      apiRequest(`/api/admin/v1/discounts/${id}`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'discounts'] })
      resetForm()
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) =>
      apiRequest(`/api/admin/v1/discounts/${id}`, { method: 'DELETE' }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'discounts'] }),
  })

  const resetForm = () => {
    setShowForm(false)
    setEditing(null)
    setForm({ code: '', type: 'percentage', value: 0, max_uses: 0, expires_at: '', applicable_plans: '' })
  }

  const handleCreate = () => {
    setEditing(null)
    resetForm()
    setShowForm(true)
  }

  const handleEdit = (d: DiscountCode) => {
    setEditing(d.id)
    setForm({
      code: d.code,
      type: d.type,
      value: d.value,
      max_uses: d.max_uses,
      expires_at: d.expires_at ? new Date(d.expires_at).toISOString().slice(0, 16) : '',
      applicable_plans: d.applicable_plans || '',
    })
    setShowForm(true)
  }

  const handleSubmit = () => {
    if (!form.code.trim() || form.value <= 0) return
    if (editing) {
      updateMutation.mutate({ id: editing, data: { active: undefined, ...form } })
    } else {
      createMutation.mutate(form)
    }
  }

  const handleToggle = (d: DiscountCode) => {
    updateMutation.mutate({ id: d.id, data: { active: !d.active } })
  }

  const data = listQuery.data
  const showSkeleton = listQuery.isLoading

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">{t('discountCodes.title')}</h1>
          <p className="text-sm text-muted-foreground">{t('discountCodes.subtitle')}</p>
        </div>
        <Button size="sm" onClick={handleCreate}>
          <Plus className="mr-2 h-4 w-4" />
          {t('discountCodes.newDiscount')}
        </Button>
      </div>

      {showForm && (
        <Card>
          <CardHeader>
            <CardTitle>{editing ? t('discountCodes.editDiscount') : t('discountCodes.createDiscount')}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid gap-4">
              <Input
                label={t('discountCodes.code')}
                value={form.code}
                onChange={(e) => setForm((f) => ({ ...f, code: e.target.value }))}
                placeholder="e.g. SUMMER2026"
              />
              <div className="grid gap-4 sm:grid-cols-2">
                <div className="flex flex-col gap-1.5">
                  <label className="text-sm font-medium text-foreground">{t('discountCodes.type')}</label>
                  <select
                    className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                    value={form.type}
                    onChange={(e) => setForm((f) => ({ ...f, type: e.target.value as 'percentage' | 'fixed' }))}
                  >
                    <option value="percentage">{t('discountCodes.percentage')}</option>
                    <option value="fixed">{t('discountCodes.fixedAmount')}</option>
                  </select>
                </div>
                <Input
                  label={form.type === 'percentage' ? t('discountCodes.valuePercent') : t('discountCodes.valueDollar')}
                  type="number"
                  value={form.value}
                  onChange={(e) => setForm((f) => ({ ...f, value: Number(e.target.value) }))}
                  placeholder={form.type === 'percentage' ? '10' : '500'}
                />
              </div>
              <div className="grid gap-4 sm:grid-cols-2">
                <Input
                  label={t('discountCodes.maxUses')}
                  type="number"
                  value={form.max_uses}
                  onChange={(e) => setForm((f) => ({ ...f, max_uses: Number(e.target.value) }))}
                />
                <Input
                  label={t('discountCodes.expiresAt')}
                  type="datetime-local"
                  value={form.expires_at}
                  onChange={(e) => setForm((f) => ({ ...f, expires_at: e.target.value }))}
                />
              </div>
              <Input
                label={t('discountCodes.applicablePlans')}
                value={form.applicable_plans}
                onChange={(e) => setForm((f) => ({ ...f, applicable_plans: e.target.value }))}
                placeholder="pro,team,business"
              />
              <div className="flex gap-2">
                <Button onClick={handleSubmit} disabled={createMutation.isPending || updateMutation.isPending}>
                  {editing ? t('common.update') : t('common.create')}
                </Button>
                <Button variant="outline" onClick={resetForm}>
                  {t('common.cancel')}
                </Button>
              </div>
              {(createMutation.isError || updateMutation.isError) && (
                <Badge variant="destructive" className="w-full justify-center py-1.5">
                  {(createMutation.error as { message?: string })?.message ??
                    (updateMutation.error as { message?: string })?.message ??
                    t('discountCodes.operationFailed')}
                </Badge>
              )}
            </div>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>
            {t('discountCodes.allDiscounts')}
            {data?.discounts && (
              <span className="text-sm font-normal text-muted-foreground"> ({data.discounts.length})</span>
            )}
          </CardTitle>
        </CardHeader>
        <CardContent>
          {listQuery.isError ? (
            <div className="flex flex-col items-center gap-2 rounded-lg border border-destructive/50 p-6 text-center">
              <AlertCircle className="h-8 w-8 text-destructive" />
              <p className="text-sm text-destructive">{t('discountCodes.failedToLoad')}</p>
              <Button variant="outline" size="sm" onClick={() => listQuery.refetch()}>
                {t('common.retry')}
              </Button>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('discountCodes.code')}</TableHead>
                  <TableHead>{t('discountCodes.type')}</TableHead>
                  <TableHead>{t('discountCodes.value')}</TableHead>
                  <TableHead>{t('discountCodes.usesMax')}</TableHead>
                  <TableHead>{t('discountCodes.status')}</TableHead>
                  <TableHead>{t('discountCodes.expires')}</TableHead>
                  <TableHead>{t('discountCodes.plans')}</TableHead>
                  <TableHead className="w-[120px]">{t('discountCodes.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {showSkeleton
                  ? Array.from({ length: 3 }).map((_, i) => (
                      <TableRow key={i}>
                        <TableCell><Skeleton className="h-4 w-20" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-10" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-10" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-20" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                      </TableRow>
                    ))
                  : (data?.discounts ?? []).map((d) => (
                      <TableRow key={d.id}>
                        <TableCell className="font-mono font-medium">{d.code}</TableCell>
                        <TableCell>
                          <Badge variant={d.type === 'percentage' ? 'default' : 'secondary'}>
                            {d.type}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          {d.type === 'percentage' ? `${d.value}%` : `$${d.value}`}
                        </TableCell>
                        <TableCell className="text-muted-foreground">
                          {d.uses ?? 0}
                          {d.max_uses > 0 ? ` / ${d.max_uses}` : ' / \u221e'}
                        </TableCell>
                        <TableCell>
                          {d.active ? (
                            <Badge variant="success">{t('common.active')}</Badge>
                          ) : (
                            <Badge variant="secondary">{t('common.disabled')}</Badge>
                          )}
                        </TableCell>
                        <TableCell className="text-xs text-muted-foreground">
                          {d.expires_at
                            ? new Date(d.expires_at).toLocaleDateString()
                            : t('common.never')}
                        </TableCell>
                        <TableCell className="text-xs text-muted-foreground">
                          {d.applicable_plans || t('common.all')}
                        </TableCell>
                        <TableCell>
                          <div className="flex gap-1">
                            <Button variant="ghost" size="sm" onClick={() => handleToggle(d)}>
                              {d.active
                                ? <ToggleRight className="h-3.5 w-3.5 text-emerald-400" />
                                : <ToggleLeft className="h-3.5 w-3.5" />
                              }
                            </Button>
                            <Button variant="ghost" size="sm" onClick={() => handleEdit(d)}>
                              <Pencil className="h-3.5 w-3.5" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => {
                                if (window.confirm(t('discountCodes.deleteConfirm'))) {
                                  deleteMutation.mutate(d.id)
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

          {data?.discounts && data.discounts.length === 0 && !showSkeleton && (
            <div className="py-8 text-center text-sm text-muted-foreground">
              <Tag className="mx-auto mb-2 h-6 w-6" />
              {t('discountCodes.noDiscounts')}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
