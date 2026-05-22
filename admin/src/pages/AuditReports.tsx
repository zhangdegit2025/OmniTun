import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { apiRequest } from '@/lib/api'
import type { AuditReportHistory, AuditReportTemplate } from '@/lib/types'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Skeleton } from '@/components/ui/Skeleton'
import { AlertCircle, FileText, Download, Clock, FileJson } from 'lucide-react'

const formatIcons: Record<string, typeof FileText> = {
  pdf: FileText,
  csv: FileText,
  json: FileJson,
}

export default function AuditReports() {
  const queryClient = useQueryClient()
  const { t } = useTranslation()
  const [selectedTemplate, setSelectedTemplate] = useState('')
  const [fromDate, setFromDate] = useState('')
  const [toDate, setToDate] = useState('')
  const [orgFilter, setOrgFilter] = useState('')
  const [format, setFormat] = useState('json')
  const [activeTab, setActiveTab] = useState<'generate' | 'history'>('generate')

  const historyQuery = useQuery<{ reports: AuditReportHistory[]; templates: AuditReportTemplate[] }>({
    queryKey: ['admin', 'reports', 'history'],
    queryFn: () => apiRequest('/api/admin/v1/reports/history'),
  })

  const generateMutation = useMutation({
    mutationFn: () => {
      const params = new URLSearchParams()
      params.set('template', selectedTemplate)
      params.set('from', fromDate)
      params.set('to', toDate)
      if (orgFilter) params.set('org_id', orgFilter)
      params.set('format', format)
      return apiRequest(`/api/admin/v1/reports/audit?${params.toString()}`)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'reports', 'history'] })
    },
  })

  const templates = historyQuery.data?.templates ?? []
  const reports = historyQuery.data?.reports ?? []

  const handleGenerate = () => {
    if (!selectedTemplate || !fromDate || !toDate) return
    generateMutation.mutate()
  }

  return (
    <div className="space-y-6 p-6">
      <div>
        <h1 className="text-2xl font-bold">{t('auditReports.title')}</h1>
        <p className="text-sm text-muted-foreground">
          {t('auditReports.subtitle')}
        </p>
      </div>

      <div className="flex gap-2">
        <Button
          variant={activeTab === 'generate' ? 'default' : 'outline'}
          size="sm"
          onClick={() => setActiveTab('generate')}
        >
          <FileText className="mr-1 h-4 w-4" />
          {t('auditReports.generateReport')}
        </Button>
        <Button
          variant={activeTab === 'history' ? 'default' : 'outline'}
          size="sm"
          onClick={() => setActiveTab('history')}
        >
          <Clock className="mr-1 h-4 w-4" />
          {t('auditReports.reportHistory')}
        </Button>
      </div>

      {activeTab === 'generate' && (
        <div className="grid gap-6 lg:grid-cols-3">
          <div className="lg:col-span-2 space-y-6">
            <Card>
              <CardHeader>
                <CardTitle>{t('auditReports.reportConfig')}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div>
                  <label className="mb-1 block text-sm font-medium">{t('auditReports.reportTemplate')}</label>
                  <select
                    className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                    value={selectedTemplate}
                    onChange={(e) => setSelectedTemplate(e.target.value)}
                  >
                    <option value="">{t('auditReports.selectTemplate')}</option>
                    {templates.map((t) => (
                      <option key={t.id} value={t.id}>
                        {t.name}
                      </option>
                    ))}
                  </select>
                </div>

                <div className="grid gap-4 sm:grid-cols-2">
                  <Input
                    label={t('auditReports.fromDate')}
                    type="date"
                    value={fromDate}
                    onChange={(e) => setFromDate(e.target.value)}
                  />
                  <Input
                    label={t('auditReports.toDate')}
                    type="date"
                    value={toDate}
                    onChange={(e) => setToDate(e.target.value)}
                  />
                </div>

                <div className="grid gap-4 sm:grid-cols-2">
                  <Input
                    label={t('auditReports.orgFilter')}
                    placeholder={t('auditReports.allOrganizations')}
                    value={orgFilter}
                    onChange={(e) => setOrgFilter(e.target.value)}
                  />
                  <div>
                    <label className="mb-1 block text-sm font-medium">{t('auditReports.exportFormat')}</label>
                    <select
                      className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                      value={format}
                      onChange={(e) => setFormat(e.target.value)}
                    >
                      <option value="json">JSON</option>
                      <option value="csv">CSV</option>
                      <option value="pdf">PDF</option>
                    </select>
                  </div>
                </div>

                <Button
                  onClick={handleGenerate}
                  disabled={generateMutation.isPending || !selectedTemplate || !fromDate || !toDate}
                  className="w-full"
                >
                  <Download className="mr-2 h-4 w-4" />
                  {generateMutation.isPending ? t('auditReports.generating') : t('auditReports.generateReport')}
                </Button>

                {generateMutation.isError && (
                  <div className="rounded-md border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive">
                    {(generateMutation.error as Error)?.message || t('auditReports.generateFailed')}
                  </div>
                )}

                {generateMutation.isSuccess && (
                  <div className="rounded-md border border-green-200 bg-green-50 p-3 text-sm text-green-800 dark:border-green-800 dark:bg-green-950 dark:text-green-200">
                    {t('auditReports.generateSuccess')}
                  </div>
                )}
              </CardContent>
            </Card>
          </div>

          <div>
            <Card>
              <CardHeader>
                <CardTitle>{t('auditReports.availableTemplates')}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                {historyQuery.isLoading ? (
                  Array.from({ length: 5 }).map((_, i) => (
                    <Skeleton key={i} className="h-14 w-full" />
                  ))
                ) : (
                  templates.map((t) => (
                    <button
                      key={t.id}
                      onClick={() => setSelectedTemplate(t.id)}
                      className={`w-full rounded-md border p-3 text-left transition-colors hover:bg-accent ${
                        selectedTemplate === t.id ? 'border-primary bg-accent' : 'border-border'
                      }`}
                    >
                      <p className="text-sm font-medium">{t.name}</p>
                      <p className="text-xs text-muted-foreground">{t.description}</p>
                    </button>
                  ))
                )}
              </CardContent>
            </Card>
          </div>
        </div>
      )}

      {activeTab === 'history' && (
        <Card>
          <CardHeader>
            <CardTitle>
              {t('auditReports.previousReports')}
              {reports.length > 0 && (
                <span className="ml-2 text-sm font-normal text-muted-foreground">
                  ({reports.length})
                </span>
              )}
            </CardTitle>
          </CardHeader>
          <CardContent>
            {historyQuery.isLoading ? (
              <div className="space-y-2">
                {Array.from({ length: 3 }).map((_, i) => (
                  <Skeleton key={i} className="h-12 w-full" />
                ))}
              </div>
            ) : historyQuery.isError ? (
              <div className="flex flex-col items-center gap-2 py-8 text-center">
                <AlertCircle className="h-8 w-8 text-destructive" />
                <p className="text-sm text-destructive">{t('auditReports.failedToLoad')}</p>
                <Button variant="outline" size="sm" onClick={() => historyQuery.refetch()}>
                  {t('common.retry')}
                </Button>
              </div>
            ) : reports.length === 0 ? (
              <div className="flex flex-col items-center gap-3 py-8 text-center">
                <FileText className="h-12 w-12 text-muted-foreground" />
                <p className="text-sm text-muted-foreground">{t('auditReports.noReports')}</p>
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('auditReports.report')}</TableHead>
                    <TableHead>{t('auditReports.template')}</TableHead>
                    <TableHead>{t('auditReports.dateRange')}</TableHead>
                    <TableHead>{t('auditReports.format')}</TableHead>
                    <TableHead>{t('auditReports.generated')}</TableHead>
                    <TableHead>{t('auditReports.size')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {reports.map((rpt) => {
                    const FormatIcon = formatIcons[rpt.format] ?? FileText
                    return (
                      <TableRow key={rpt.id}>
                        <TableCell>
                          <div className="flex items-center gap-2">
                            <FormatIcon className="h-4 w-4 text-muted-foreground" />
                            <span className="font-mono text-xs">{rpt.id}</span>
                          </div>
                        </TableCell>
                        <TableCell>
                          <Badge variant="secondary">{rpt.template}</Badge>
                        </TableCell>
                        <TableCell className="whitespace-nowrap text-sm text-muted-foreground">
                          {rpt.date_range.from} to {rpt.date_range.to}
                        </TableCell>
                        <TableCell>
                          <Badge variant="secondary" className="uppercase">
                            {rpt.format}
                          </Badge>
                        </TableCell>
                        <TableCell className="whitespace-nowrap text-sm text-muted-foreground">
                          {new Date(rpt.generated_at).toLocaleString()}
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                          {(rpt.size_bytes / 1024).toFixed(1)} KB
                        </TableCell>
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  )
}
