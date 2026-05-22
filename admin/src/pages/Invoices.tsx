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
import { Search, CheckCircle, Ban, Mail, ChevronDown, ChevronUp, AlertCircle } from 'lucide-react'

interface InvoiceItem {
  description: string
  quantity: number
  unit_price: number
  total: number
}

interface Invoice {
  id: string
  customer: string
  amount: number
  subtotal: number
  tax: number
  status: 'paid' | 'pending' | 'overdue' | 'void'
  date: string
  due_date: string
  payment_method: string
  items: InvoiceItem[]
  organization?: { id: string; name: string }
}

interface InvoiceListResponse {
  invoices: Invoice[]
  total: number
}

const statusBadgeVariant: Record<string, 'success' | 'warning' | 'destructive' | 'secondary'> = {
  paid: 'success',
  pending: 'warning',
  overdue: 'destructive',
  void: 'secondary',
}

export default function Invoices() {
  const queryClient = useQueryClient()
  const { t } = useTranslation()
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const [detailData, setDetailData] = useState<Invoice | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)

  const listQuery = useQuery<InvoiceListResponse>({
    queryKey: ['admin', 'invoices', statusFilter, search],
    queryFn: () => {
      const params = new URLSearchParams()
      if (statusFilter) params.set('status', statusFilter)
      if (search) params.set('customer', search)
      return apiRequest(`/api/admin/v1/invoices?${params.toString()}`)
    },
  })

  const markPaidMutation = useMutation({
    mutationFn: (id: string) =>
      apiRequest(`/api/admin/v1/invoices/${id}/mark-paid`, { method: 'POST' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'invoices'] })
      setExpandedId(null)
      setDetailData(null)
    },
  })

  const voidMutation = useMutation({
    mutationFn: (id: string) =>
      apiRequest(`/api/admin/v1/invoices/${id}/void`, { method: 'POST' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'invoices'] })
      setExpandedId(null)
      setDetailData(null)
    },
  })

  const loadDetail = async (id: string) => {
    if (expandedId === id) {
      setExpandedId(null)
      setDetailData(null)
      return
    }
    setExpandedId(id)
    setDetailLoading(true)
    try {
      const data = await apiRequest<Invoice>(`/api/admin/v1/invoices/${id}`)
      setDetailData(data)
    } catch {
      setDetailData(null)
    } finally {
      setDetailLoading(false)
    }
  }

  const showSkeleton = listQuery.isLoading
  const data = listQuery.data

  return (
    <div className="space-y-6 p-6">
      <div>
        <h1 className="text-2xl font-bold">{t('invoices.title')}</h1>
        <p className="text-sm text-muted-foreground">{t('invoices.subtitle')}</p>
      </div>

      <div className="flex flex-wrap gap-3">
        <div className="relative max-w-sm">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder={t('invoices.searchPlaceholder')}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9"
          />
        </div>
        <select
          className="flex h-9 rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
        >
          <option value="">{t('invoices.allStatuses')}</option>
          <option value="paid">Paid</option>
          <option value="pending">Pending</option>
          <option value="overdue">Overdue</option>
          <option value="void">Void</option>
        </select>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>
            {t('invoices.allInvoices')}
            {data?.total !== undefined && (
              <span className="text-sm font-normal text-muted-foreground"> ({data.total})</span>
            )}
          </CardTitle>
        </CardHeader>
        <CardContent>
          {listQuery.isError ? (
            <div className="flex flex-col items-center gap-2 rounded-lg border border-destructive/50 p-6 text-center">
              <AlertCircle className="h-8 w-8 text-destructive" />
              <p className="text-sm text-destructive">{t('invoices.failedToLoad')}</p>
              <Button variant="outline" size="sm" onClick={() => listQuery.refetch()}>
                {t('common.retry')}
              </Button>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-[80px]">{t('invoices.id')}</TableHead>
                  <TableHead>{t('invoices.customer')}</TableHead>
                  <TableHead>{t('invoices.amount')}</TableHead>
                  <TableHead>{t('invoices.status')}</TableHead>
                  <TableHead>{t('invoices.date')}</TableHead>
                  <TableHead>{t('invoices.dueDate')}</TableHead>
                  <TableHead className="w-[140px]">{t('invoices.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {showSkeleton
                  ? Array.from({ length: 4 }).map((_, i) => (
                      <TableRow key={i}>
                        <TableCell><Skeleton className="h-4 w-12" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-32" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-14" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-20" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                      </TableRow>
                    ))
                  : (data?.invoices ?? []).map((inv) => (
                      <>
                        <TableRow key={inv.id}>
                          <TableCell className="font-mono text-xs">{inv.id?.slice(0, 8)}</TableCell>
                          <TableCell className="font-medium">{inv.customer || '—'}</TableCell>
                          <TableCell>${inv.amount?.toFixed(2)}</TableCell>
                          <TableCell>
                            <Badge variant={statusBadgeVariant[inv.status] ?? 'secondary'}>
                              {inv.status}
                            </Badge>
                          </TableCell>
                          <TableCell className="text-muted-foreground">
                            {inv.date ? new Date(inv.date).toLocaleDateString() : '—'}
                          </TableCell>
                          <TableCell className="text-muted-foreground">
                            {inv.due_date ?? '—'}
                          </TableCell>
                          <TableCell>
                            <div className="flex gap-1">
                              <Button
                                variant="ghost"
                                size="sm"
                                onClick={() => loadDetail(inv.id)}
                              >
                                {expandedId === inv.id
                                  ? <ChevronUp className="h-3.5 w-3.5" />
                                  : <ChevronDown className="h-3.5 w-3.5" />
                                }
                              </Button>
                              {inv.status !== 'paid' && inv.status !== 'void' && (
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => {
                                    if (window.confirm(t('invoices.markPaidConfirm'))) {
                                      markPaidMutation.mutate(inv.id)
                                    }
                                  }}
                                >
                                  <CheckCircle className="h-3.5 w-3.5 text-emerald-400" />
                                </Button>
                              )}
                              {inv.status !== 'void' && inv.status !== 'paid' && (
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => {
                                    if (window.confirm(t('invoices.voidConfirm'))) {
                                      voidMutation.mutate(inv.id)
                                    }
                                  }}
                                >
                                  <Ban className="h-3.5 w-3.5 text-destructive" />
                                </Button>
                              )}
                              <Button variant="ghost" size="sm">
                                <Mail className="h-3.5 w-3.5" />
                              </Button>
                            </div>
                          </TableCell>
                        </TableRow>
                        {expandedId === inv.id && (
                          <TableRow key={`${inv.id}-detail`}>
                            <TableCell colSpan={7} className="bg-muted/30">
                              {detailLoading ? (
                                <div className="space-y-2 py-2">
                                  <Skeleton className="h-4 w-48" />
                                  <Skeleton className="h-4 w-64" />
                                  <Skeleton className="h-4 w-32" />
                                </div>
                              ) : detailData ? (
                                <div className="grid gap-3 py-2 text-sm">
                                  <div className="flex flex-wrap gap-x-8 gap-y-1">
                                    <div>
                                      <span className="text-muted-foreground">{t('invoices.subtotal')}:</span>{' '}
                                      <span className="font-medium">${detailData.subtotal?.toFixed(2)}</span>
                                    </div>
                                    <div>
                                      <span className="text-muted-foreground">{t('invoices.tax')}:</span>{' '}
                                      <span className="font-medium">${detailData.tax?.toFixed(2)}</span>
                                    </div>
                                    <div>
                                      <span className="text-muted-foreground">{t('invoices.paymentMethod')}:</span>{' '}
                                      <span className="font-medium">{detailData.payment_method || t('invoices.na')}</span>
                                    </div>
                                  </div>
                                  {detailData.items && detailData.items.length > 0 && (
                                    <div>
                                      <p className="mb-1 font-medium">{t('invoices.invoiceItems')}</p>
                                      <div className="rounded border">
                                        <table className="w-full text-xs">
                                          <thead>
                                            <tr className="border-b bg-muted/50">
                                              <th className="px-3 py-1.5 text-left">{t('invoices.description')}</th>
                                              <th className="px-3 py-1.5 text-right">{t('invoices.qty')}</th>
                                              <th className="px-3 py-1.5 text-right">{t('invoices.unitPrice')}</th>
                                              <th className="px-3 py-1.5 text-right">{t('invoices.total')}</th>
                                            </tr>
                                          </thead>
                                          <tbody>
                                            {detailData.items.map((item, idx) => (
                                              <tr key={idx} className="border-b last:border-0">
                                                <td className="px-3 py-1.5">{item.description}</td>
                                                <td className="px-3 py-1.5 text-right">{item.quantity}</td>
                                                <td className="px-3 py-1.5 text-right">${item.unit_price?.toFixed(2)}</td>
                                                <td className="px-3 py-1.5 text-right font-medium">${item.total?.toFixed(2)}</td>
                                              </tr>
                                            ))}
                                          </tbody>
                                        </table>
                                      </div>
                                    </div>
                                  )}
                                </div>
                              ) : (
                                <p className="py-2 text-sm text-muted-foreground">{t('invoices.failedToLoadDetail')}</p>
                              )}
                            </TableCell>
                          </TableRow>
                        )}
                      </>
                    ))}
              </TableBody>
            </Table>
          )}

          {data?.invoices && data.invoices.length === 0 && !showSkeleton && (
            <div className="py-8 text-center text-sm text-muted-foreground">
              <FileText className="mx-auto mb-2 h-6 w-6" />
              {t('invoices.noInvoices')}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function FileText({ className }: { className?: string }) {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className={className}>
      <path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/>
      <path d="M14 2v4a2 2 0 0 0 2 2h4"/>
      <path d="M10 9H8"/>
      <path d="M16 13H8"/>
      <path d="M16 17H8"/>
    </svg>
  )
}
