import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Skeleton } from '@/components/ui/Skeleton'
import { apiRequest } from '@/lib/api'
import { Plus, Pencil, Trash2, Flag } from 'lucide-react'

interface Flag {
  id: string
  key: string
  name: string
  description: string
  type: string
  value: string
  enabled: boolean
  created_at: string
  updated_at: string
}

const typeVariant: Record<string, 'success' | 'warning' | 'default'> = {
  boolean: 'success',
  percentage: 'warning',
  whitelist: 'default',
}

export default function FeatureFlags() {
  const { t } = useTranslation()
  const [flags, setFlags] = useState<Flag[]>([])
  const [loading, setLoading] = useState(true)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<Flag | null>(null)

  const [formKey, setFormKey] = useState('')
  const [formName, setFormName] = useState('')
  const [formDesc, setFormDesc] = useState('')
  const [formType, setFormType] = useState('boolean')
  const [formValue, setFormValue] = useState('{"enabled":true}')
  const [formEnabled, setFormEnabled] = useState(true)
  const [saving, setSaving] = useState(false)

  const fetchFlags = useCallback(async () => {
    setLoading(true)
    try {
      const data = await apiRequest<{ flags: Flag[] }>('/api/admin/v1/feature-flags')
      setFlags(data.flags || [])
    } catch {
      setFlags([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchFlags()
  }, [fetchFlags])

  function openCreate() {
    setEditing(null)
    setFormKey('')
    setFormName('')
    setFormDesc('')
    setFormType('boolean')
    setFormValue('{"enabled":true}')
    setFormEnabled(true)
    setDialogOpen(true)
  }

  function openEdit(flag: Flag) {
    setEditing(flag)
    setFormKey(flag.key)
    setFormName(flag.name)
    setFormDesc(flag.description)
    setFormType(flag.type)
    setFormValue(flag.value)
    setFormEnabled(flag.enabled)
    setDialogOpen(true)
  }

  async function handleSave() {
    if (!formKey.trim() || !formName.trim()) return
    setSaving(true)
    try {
      const body = {
        key: formKey.trim(),
        name: formName.trim(),
        description: formDesc.trim(),
        type: formType,
        value: formValue,
        enabled: formEnabled,
      }

      if (editing) {
        await apiRequest(`/api/admin/v1/feature-flags/${encodeURIComponent(editing.key)}`, {
          method: 'PUT',
          body: JSON.stringify(body),
        })
      } else {
        await apiRequest('/api/admin/v1/feature-flags', {
          method: 'POST',
          body: JSON.stringify(body),
        })
      }
      setDialogOpen(false)
      fetchFlags()
    } catch {
    } finally {
      setSaving(false)
    }
  }

  async function handleToggle(flag: Flag) {
    try {
      await apiRequest(`/api/admin/v1/feature-flags/${encodeURIComponent(flag.key)}`, {
        method: 'PUT',
        body: JSON.stringify({
          key: flag.key,
          name: flag.name,
          description: flag.description,
          type: flag.type,
          value: flag.value,
          enabled: !flag.enabled,
        }),
      })
      fetchFlags()
    } catch {
    }
  }

  async function handleDelete(flag: Flag) {
    if (!confirm(t('featureFlags.deleteConfirm', { name: flag.name }))) return
    try {
      await apiRequest(`/api/admin/v1/feature-flags/${encodeURIComponent(flag.key)}`, {
        method: 'DELETE',
      })
      fetchFlags()
    } catch {
    }
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">{t('featureFlags.title')}</h1>
          <p className="text-sm text-muted-foreground">{t('featureFlags.subtitle', { count: flags.length })}</p>
        </div>
        <Button onClick={openCreate}>
          <Plus className="mr-1 h-4 w-4" />
          {t('featureFlags.newFlag')}
        </Button>
      </div>

      <Card>
        {loading ? (
          <CardContent className="p-6 space-y-2">
            <Skeleton className="h-6 w-full" />
            <Skeleton className="h-6 w-full" />
            <Skeleton className="h-6 w-full" />
          </CardContent>
        ) : (
          <CardContent className="p-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('featureFlags.key')}</TableHead>
                  <TableHead>{t('featureFlags.name')}</TableHead>
                  <TableHead>{t('featureFlags.type')}</TableHead>
                  <TableHead>{t('common.status')}</TableHead>
                  <TableHead className="w-[120px]">{t('featureFlags.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {flags.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={5} className="text-center text-muted-foreground py-8">
                      <Flag className="mx-auto mb-2 h-8 w-8 opacity-20" />
                      {t('featureFlags.noFlags')}
                    </TableCell>
                  </TableRow>
                )}
                {flags.map((flag) => (
                  <TableRow key={flag.id}>
                    <TableCell>
                      <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{flag.key}</code>
                    </TableCell>
                    <TableCell>
                      <div className="font-medium">{flag.name}</div>
                      {flag.description && (
                        <p className="text-xs text-muted-foreground">{flag.description}</p>
                      )}
                    </TableCell>
                    <TableCell>
                      <Badge variant={typeVariant[flag.type] ?? 'secondary'}>{flag.type}</Badge>
                    </TableCell>
                    <TableCell>
                      <button
                        onClick={() => handleToggle(flag)}
                        className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                          flag.enabled ? 'bg-emerald-500' : 'bg-muted'
                        }`}
                      >
                        <span
                          className={`inline-block h-3.5 w-3.5 rounded-full bg-white shadow transition-transform ${
                            flag.enabled ? 'translate-x-[18px]' : 'translate-x-[3px]'
                          }`}
                        />
                      </button>
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1">
                        <Button variant="ghost" size="sm" onClick={() => openEdit(flag)}>
                          <Pencil className="h-3.5 w-3.5" />
                        </Button>
                        <Button variant="ghost" size="sm" onClick={() => handleDelete(flag)}>
                          <Trash2 className="h-3.5 w-3.5 text-destructive" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        )}
      </Card>

      {dialogOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="w-full max-w-lg rounded-lg border bg-card shadow-lg">
            <CardHeader>
              <CardTitle>{editing ? t('featureFlags.editFlag') : t('featureFlags.createFlag')}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <Input
                label={t('featureFlags.key')}
                placeholder={t('featureFlags.keyPlaceholder')}
                value={formKey}
                onChange={(e) => setFormKey(e.target.value)}
                disabled={!!editing}
              />
              <Input
                label={t('featureFlags.name')}
                placeholder={t('featureFlags.namePlaceholder')}
                value={formName}
                onChange={(e) => setFormName(e.target.value)}
              />
              <Input
                label={t('featureFlags.description')}
                placeholder={t('featureFlags.descriptionPlaceholder')}
                value={formDesc}
                onChange={(e) => setFormDesc(e.target.value)}
              />
              <div className="flex flex-col gap-1.5">
                <label className="text-sm font-medium text-foreground">{t('featureFlags.type')}</label>
                <select
                  value={formType}
                  onChange={(e) => {
                    setFormType(e.target.value)
                    if (e.target.value === 'boolean') setFormValue('{"enabled":true}')
                    else if (e.target.value === 'percentage') setFormValue('{"percentage":50}')
                    else if (e.target.value === 'whitelist') setFormValue('{"orgs":[]}')
                  }}
                  className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                >
                  <option value="boolean">{t('featureFlags.boolean')}</option>
                  <option value="percentage">{t('featureFlags.percentage')}</option>
                  <option value="whitelist">{t('featureFlags.whitelist')}</option>
                </select>
              </div>
              <div className="flex flex-col gap-1.5">
                <label className="text-sm font-medium text-foreground">{t('featureFlags.value')}</label>
                <textarea
                  value={formValue}
                  onChange={(e) => setFormValue(e.target.value)}
                  rows={4}
                  className="flex min-h-[80px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm transition-colors placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring font-mono"
                />
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm font-medium">{t('common.enabled')}</span>
                <button
                  type="button"
                  onClick={() => setFormEnabled(!formEnabled)}
                  className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                    formEnabled ? 'bg-emerald-500' : 'bg-muted'
                  }`}
                >
                  <span
                    className={`inline-block h-3.5 w-3.5 rounded-full bg-white shadow transition-transform ${
                      formEnabled ? 'translate-x-[18px]' : 'translate-x-[3px]'
                    }`}
                  />
                </button>
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <Button variant="outline" onClick={() => setDialogOpen(false)}>
                  {t('common.cancel')}
                </Button>
                <Button onClick={handleSave} disabled={saving}>
                  {saving ? t('common.saving') : t('common.save')}
                </Button>
              </div>
            </CardContent>
          </div>
        </div>
      )}
    </div>
  )
}
