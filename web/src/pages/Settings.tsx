import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiRequest } from '@/lib/api'
import type { ApiKey, OrgUsage, Invitation, OrgEvent } from '@/lib/types'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/Card'
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '@/components/ui/Table'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Dialog } from '@/components/ui/Dialog'
import { Badge } from '@/components/ui/Badge'
import { Skeleton } from '@/components/ui/Skeleton'
import { useAuth } from '@/hooks/useAuth'
import { useToast } from '@/components/ui/useToast'
import { AlertCircle, Copy, Plus, Trash2, Check, Users, Key, Building2, BarChart3, Shield, ClipboardList, Clock, Link2 } from 'lucide-react'
import { cn } from '@/lib/utils'

type Tab = 'organization' | 'security' | 'apikeys' | 'members' | 'activity' | 'sessions'

export default function Settings() {
  const { t } = useTranslation()
  const { user, fetchUser } = useAuth()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const [activeTab, setActiveTab] = useState<Tab>('organization')

  const tabItems: { key: Tab; label: string; icon: React.FC<{ className?: string }> }[] = [
    { key: 'organization', label: t('settings.tabs.organization'), icon: Building2 },
    { key: 'security', label: t('settings.tabs.security'), icon: Shield },
    { key: 'members', label: t('settings.tabs.members'), icon: Users },
    { key: 'apikeys', label: t('settings.tabs.api_keys'), icon: Key },
    { key: 'activity', label: t('activity.title'), icon: ClipboardList },
    { key: 'sessions', label: t('sessions.title'), icon: Clock },
  ]

  return (
    <div className="space-y-6 p-6">
      <div>
        <h1 className="text-2xl font-bold">{t('settings.title')}</h1>
        <p className="text-sm text-muted-foreground">{t('settings.subtitle')}</p>
      </div>

      {/* Tab navigation */}
      <div className="flex flex-wrap gap-1 border-b">
        {tabItems.map((tab) => {
          const Icon = tab.icon
          return (
            <button
              key={tab.key}
              onClick={() => setActiveTab(tab.key)}
              className={cn(
                'flex items-center gap-2 border-b-2 px-4 py-2 text-sm font-medium transition-colors',
                activeTab === tab.key
                  ? 'border-primary text-primary'
                  : 'border-transparent text-muted-foreground hover:text-foreground',
              )}
            >
              <Icon className="h-4 w-4" />
              {tab.label}
            </button>
          )
        })}
      </div>

      {activeTab === 'organization' && <OrganizationTab user={user} />}
      {activeTab === 'security' && <SecurityTab user={user} fetchUser={fetchUser} toast={toast} />}
      {activeTab === 'apikeys' && <ApiKeysTab toast={toast} queryClient={queryClient} />}
      {activeTab === 'members' && <MembersTab toast={toast} />}
      {activeTab === 'activity' && <ActivityTab />}
      {activeTab === 'sessions' && <SessionsTab toast={toast} />}
    </div>
  )
}

function OrganizationTab({ user }: { user: { name: string; email: string; role: string; org_id: string } | null }) {
  const { t } = useTranslation()
  const usageQuery = useQuery<OrgUsage>({
    queryKey: ['org', 'usage'],
    queryFn: () => apiRequest<OrgUsage>('/v1/org/usage'),
  })

  const usage = usageQuery.data

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>{t('settings.organization.title')}</CardTitle>
          <CardDescription>{t('settings.organization.description')}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <label className="text-sm font-medium">{t('settings.organization.name')}</label>
              <Input defaultValue={user?.name ?? ''} className="mt-1.5" />
            </div>
            <div>
              <label className="text-sm font-medium">{t('settings.organization.slug')}</label>
              <Input defaultValue={user?.org_id ?? ''} className="mt-1.5" disabled />
              <p className="mt-1 text-xs text-muted-foreground">{t('settings.organization.slug_help')}</p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <span className="text-sm text-muted-foreground">{t('settings.organization.plan')}:</span>
            <Badge variant="success">{t('settings.organization.plan_free')}</Badge>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <BarChart3 className="h-5 w-5" />
            {t('settings.organization.usage')}
          </CardTitle>
          <CardDescription>{t('settings.organization.usage_description')}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          {usageQuery.isLoading ? (
            <>
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
            </>
          ) : usageQuery.isError ? (
            <div className="flex flex-col items-center gap-2 py-4 text-center">
              <AlertCircle className="h-6 w-6 text-destructive" />
              <p className="text-sm text-destructive">{t('settings.organization.failed_usage')}</p>
              <Button variant="outline" size="sm" onClick={() => usageQuery.refetch()}>
                {t('common.retry')}
              </Button>
            </div>
          ) : (
            <>
              <div>
                <div className="mb-1 flex items-center justify-between text-sm">
                  <span className="text-muted-foreground">{t('settings.organization.bandwidth_usage')}</span>
                  <span className="font-medium">
                    {formatBytes(usage?.bandwidth_used ?? 0)} / {formatBytes(usage?.bandwidth_limit ?? 0)}
                  </span>
                </div>
                <div className="h-3 overflow-hidden rounded-full bg-muted">
                  <div
                    className="h-full rounded-full bg-primary transition-all"
                    style={{
                      width: `${usage ? Math.min((usage.bandwidth_used / usage.bandwidth_limit) * 100, 100) : 0}%`,
                    }}
                  />
                </div>
              </div>

              <div>
                <div className="mb-1 flex items-center justify-between text-sm">
                  <span className="text-muted-foreground">{t('settings.organization.tunnels_usage')}</span>
                  <span className="font-medium">
                    {usage?.tunnels_used ?? 0} / {usage?.tunnels_limit ?? 0}
                  </span>
                </div>
                <div className="h-3 overflow-hidden rounded-full bg-muted">
                  <div
                    className="h-full rounded-full bg-emerald-500 transition-all"
                    style={{
                      width: `${usage ? Math.min((usage.tunnels_used / usage.tunnels_limit) * 100, 100) : 0}%`,
                    }}
                  />
                </div>
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function ApiKeysTab({
  toast,
  queryClient,
}: {
  toast: (t: { title: string; description?: string; variant?: 'default' | 'success' | 'error' | 'warning' }) => void
  queryClient: ReturnType<typeof useQueryClient>
}) {
  const { t } = useTranslation()
  const [showCreateKey, setShowCreateKey] = useState(false)
  const [newKeyName, setNewKeyName] = useState('')
  const [copiedKey, setCopiedKey] = useState<string | null>(null)
  const [createdKey, setCreatedKey] = useState<string | null>(null)
  const [revokeTarget, setRevokeTarget] = useState<ApiKey | null>(null)

  const apiKeysQuery = useQuery<ApiKey[]>({
    queryKey: ['apikeys'],
    queryFn: () => apiRequest<ApiKey[]>('/v1/api-keys'),
  })

  const createKeyMutation = useMutation<
    { key: string; api_key: ApiKey },
    Error,
    string
  >({
    mutationFn: (name) =>
      apiRequest('/v1/api-keys', {
        method: 'POST',
        body: JSON.stringify({ name }),
      }),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['apikeys'] })
      setCreatedKey(data.key)
    },
  })

  const revokeKeyMutation = useMutation<void, Error, string>({
    mutationFn: (id) =>
      apiRequest(`/v1/api-keys/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['apikeys'] })
      toast({ title: t('settings.api_keys.created'), variant: 'success' })
      setRevokeTarget(null)
    },
  })

  const handleCreateKey = async () => {
    if (!newKeyName.trim()) return
    await createKeyMutation.mutateAsync(newKeyName.trim())
    setNewKeyName('')
  }

  const handleCloseCreatedKey = () => {
    setCreatedKey(null)
    setShowCreateKey(false)
  }

  const copyToClipboard = async (text: string) => {
    await navigator.clipboard.writeText(text)
    setCopiedKey(text)
    toast({ title: t('common.copied') })
    setTimeout(() => setCopiedKey(null), 2000)
  }

  return (
    <div>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle>{t('settings.api_keys.title')}</CardTitle>
            <CardDescription>{t('settings.api_keys.description')}</CardDescription>
          </div>
          <Button size="sm" onClick={() => setShowCreateKey(true)}>
            <Plus className="mr-1 h-4 w-4" />
            {t('settings.api_keys.create')}
          </Button>
        </CardHeader>
        <CardContent>
          {apiKeysQuery.isLoading ? (
            <div className="space-y-3">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          ) : apiKeysQuery.isError ? (
            <div className="flex flex-col items-center gap-2 py-6 text-center">
              <AlertCircle className="h-8 w-8 text-destructive" />
              <p className="text-sm text-destructive">{t('settings.api_keys.failed_load')}</p>
              <Button variant="outline" size="sm" onClick={() => apiKeysQuery.refetch()}>
                {t('common.retry')}
              </Button>
            </div>
          ) : apiKeysQuery.data?.length === 0 ? (
            <div className="py-6 text-center text-sm text-muted-foreground">
              {t('settings.api_keys.empty')}
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('settings.api_keys.table.name')}</TableHead>
                  <TableHead>{t('settings.api_keys.table.prefix')}</TableHead>
                  <TableHead>{t('settings.api_keys.table.created')}</TableHead>
                  <TableHead>{t('settings.api_keys.table.last_used')}</TableHead>
                  <TableHead>{t('settings.api_keys.table.status')}</TableHead>
                  <TableHead className="text-right">{t('settings.api_keys.table.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {apiKeysQuery.data?.map((apikey) => (
                  <TableRow key={apikey.id}>
                    <TableCell className="font-medium">{apikey.name}</TableCell>
                    <TableCell>
                      <code className="rounded bg-muted px-2 py-0.5 text-xs">
                        {apikey.key_prefix}...
                      </code>
                    </TableCell>
                    <TableCell className="text-muted-foreground text-sm">
                      {new Date(apikey.created_at).toLocaleDateString()}
                    </TableCell>
                    <TableCell className="text-muted-foreground text-sm">
                      {apikey.last_used_at
                        ? new Date(apikey.last_used_at).toLocaleDateString()
                        : t('settings.api_keys.never')}
                    </TableCell>
                    <TableCell>
                      <Badge variant={apikey.status === 'active' ? 'success' : 'destructive'}>
                        {apikey.status === 'active' ? t('common.active') : apikey.status}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setRevokeTarget(apikey)}
                        disabled={apikey.status === 'revoked'}
                      >
                        <Trash2 className="h-4 w-4 text-destructive" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Dialog
        open={showCreateKey}
        onClose={() => {
          setShowCreateKey(false)
          setCreatedKey(null)
        }}
        title={t('settings.api_keys.create_dialog.title')}
        description={t('settings.api_keys.create_dialog.description')}
      >
        {createdKey ? (
          <div className="flex flex-col gap-4">
            <div className="rounded-lg border border-emerald-500 bg-emerald-50 p-4 dark:bg-emerald-950">
              <p className="text-sm font-medium text-emerald-900 dark:text-emerald-100">
                {t('settings.api_keys.created_dialog.warning')}
              </p>
              <div className="mt-3 flex items-center gap-2">
                <code className="flex-1 break-all rounded bg-background px-3 py-2 text-sm font-mono">
                  {createdKey}
                </code>
                <Button variant="outline" size="sm" onClick={() => copyToClipboard(createdKey)}>
                  {copiedKey === createdKey ? (
                    <Check className="h-4 w-4 text-emerald-600" />
                  ) : (
                    <Copy className="h-4 w-4" />
                  )}
                </Button>
              </div>
            </div>
            <Button onClick={handleCloseCreatedKey}>{t('settings.api_keys.created_dialog.done')}</Button>
          </div>
        ) : (
          <form
            onSubmit={(e) => {
              e.preventDefault()
              handleCreateKey()
            }}
            className="flex flex-col gap-4"
          >
            <Input
              label={t('settings.api_keys.create_dialog.name')}
              placeholder="e.g. CI/CD Pipeline"
              value={newKeyName}
              onChange={(e) => setNewKeyName(e.target.value)}
            />
            {createKeyMutation.error && (
              <p className="text-sm text-destructive">
                {(createKeyMutation.error as Error)?.message ?? t('settings.api_keys.create_dialog.failed_create')}
              </p>
            )}
            <div className="flex justify-end gap-2 pt-2">
              <Button variant="outline" type="button" onClick={() => setShowCreateKey(false)}>
                {t('settings.api_keys.create_dialog.cancel')}
              </Button>
              <Button type="submit" disabled={createKeyMutation.isPending}>
                {createKeyMutation.isPending ? t('settings.api_keys.create_dialog.creating') : t('settings.api_keys.create_dialog.submit')}
              </Button>
            </div>
          </form>
        )}
      </Dialog>

      <Dialog
        open={!!revokeTarget}
        onClose={() => setRevokeTarget(null)}
        title={t('settings.api_keys.revoke_dialog.title')}
        description={t('settings.api_keys.revoke_dialog.message')}
      >
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="outline" onClick={() => setRevokeTarget(null)}>
            {t('settings.api_keys.revoke_dialog.cancel')}
          </Button>
          <Button
            variant="destructive"
            onClick={() => revokeTarget && revokeKeyMutation.mutate(revokeTarget.id)}
            disabled={revokeKeyMutation.isPending}
          >
            {revokeKeyMutation.isPending ? t('settings.api_keys.revoke_dialog.revoking') : t('settings.api_keys.revoke_dialog.confirm')}
          </Button>
        </div>
      </Dialog>
    </div>
  )
}

function SecurityTab({
  user,
  fetchUser,
  toast,
}: {
  user: { name: string; email: string; role: string; org_id: string; mfa_enabled?: boolean } | null
  fetchUser: () => Promise<void>
  toast: (t: { title: string; description?: string; variant?: 'default' | 'success' | 'error' | 'warning' }) => void
}) {
  const { t } = useTranslation()
  const [enrolling, setEnrolling] = useState(false)
  const [secret, setSecret] = useState('')
  const [qrCodeUrl, setQrCodeUrl] = useState('')
  const [verifyCode, setVerifyCode] = useState('')
  const [verifying, setVerifying] = useState(false)
  const [disabling, setDisabling] = useState(false)
  const mfaEnabled = user?.mfa_enabled ?? false

  const handleEnrollMFA = async () => {
    setEnrolling(true)
    try {
      const data = await apiRequest<{ secret: string; qr_code_url: string }>('/v1/auth/mfa/enroll')
      setSecret(data.secret)
      setQrCodeUrl(data.qr_code_url)
    } catch {
      toast({ title: t('settings.security.enroll_failed'), variant: 'error' })
    } finally {
      setEnrolling(false)
    }
  }

  const handleVerifyMFA = async () => {
    if (!verifyCode || verifyCode.length !== 6) return
    setVerifying(true)
    try {
      const data = await apiRequest<{ success: boolean }>('/v1/auth/mfa/verify', {
        method: 'POST',
        body: JSON.stringify({ code: verifyCode }),
      })
      if (data.success) {
        toast({ title: t('settings.security.mfa_enabled'), variant: 'success' })
        setSecret('')
        setQrCodeUrl('')
        setVerifyCode('')
        await fetchUser()
      } else {
        toast({ title: t('settings.security.verify_failed'), variant: 'error' })
      }
    } catch {
      toast({ title: t('settings.security.verify_failed'), variant: 'error' })
    } finally {
      setVerifying(false)
    }
  }

  const handleDisableMFA = async () => {
    setDisabling(true)
    try {
      await apiRequest<{ success: boolean }>('/v1/auth/mfa/disable', { method: 'POST' })
      toast({ title: t('settings.security.mfa_disabled'), variant: 'success' })
      await fetchUser()
    } catch {
      toast({ title: t('settings.security.disable_failed'), variant: 'error' })
    } finally {
      setDisabling(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Shield className="h-5 w-5" />
          {t('settings.security.title')}
        </CardTitle>
        <CardDescription>{t('settings.security.description')}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm font-medium">{t('settings.security.mfa_status')}</p>
            <p className="text-sm text-muted-foreground">
              {mfaEnabled ? t('settings.security.mfa_enabled_desc') : t('settings.security.mfa_disabled_desc')}
            </p>
          </div>
          <Badge variant={mfaEnabled ? 'success' : 'secondary'}>
            {mfaEnabled ? t('settings.security.enabled') : t('settings.security.disabled')}
          </Badge>
        </div>

        {mfaEnabled ? (
          <Button variant="destructive" onClick={handleDisableMFA} disabled={disabling}>
            {disabling ? t('settings.security.disabling') : t('settings.security.disable')}
          </Button>
        ) : (
          <>
            {!secret ? (
              <Button onClick={handleEnrollMFA} disabled={enrolling}>
                {enrolling ? t('settings.security.enrolling') : t('settings.security.enable_mfa')}
              </Button>
            ) : (
              <div className="space-y-4 rounded-lg border p-4">
                <p className="text-sm font-medium">{t('settings.security.scan_qr')}</p>
                <div className="rounded bg-muted p-3">
                  <pre className="text-xs break-all whitespace-pre-wrap font-mono">{qrCodeUrl}</pre>
                </div>
                <p className="text-sm text-muted-foreground">
                  {t('settings.security.manual_secret')}: <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{secret}</code>
                </p>
                <div className="flex items-center gap-2">
                  <Input
                    label={t('settings.security.verify_code')}
                    placeholder="000000"
                    value={verifyCode}
                    onChange={(e) => setVerifyCode(e.target.value.replace(/[^0-9]/g, ''))}
                    maxLength={6}
                  />
                  <Button onClick={handleVerifyMFA} disabled={verifying || verifyCode.length !== 6}>
                    {verifying ? t('settings.security.verifying') : t('settings.security.verify')}
                  </Button>
                </div>
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>
  )
}

function MembersTab({ toast }: { toast: (t: { title: string; description?: string; variant?: 'default' | 'success' | 'error' | 'warning' }) => void }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [generating, setGenerating] = useState(false)
  const [copied, setCopied] = useState<string | null>(null)

  const invitationsQuery = useQuery<{ invitations: Invitation[] }>({
    queryKey: ['invitations'],
    queryFn: () => apiRequest<{ invitations: Invitation[] }>('/v1/org/invitations'),
  })

  const generateMutation = useMutation<Invitation, Error, void>({
    mutationFn: () =>
      apiRequest<Invitation>('/v1/org/invitations', {
        method: 'POST',
        body: JSON.stringify({ max_uses: 0, expires_in: 86400 * 7 }),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['invitations'] })
      toast({ title: t('invitations.generate'), variant: 'success' })
    },
  })

  const deleteMutation = useMutation<void, Error, string>({
    mutationFn: (id) =>
      apiRequest(`/v1/org/invitations/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['invitations'] })
      toast({ title: t('invitations.deleted'), variant: 'success' })
    },
  })

  const generateLink = async () => {
    setGenerating(true)
    try {
      await generateMutation.mutateAsync()
    } finally {
      setGenerating(false)
    }
  }

  const copyLink = async (code: string) => {
    const link = `${window.location.origin}/join?code=${code}`
    await navigator.clipboard.writeText(link)
    setCopied(code)
    toast({ title: t('invitations.link_copied') })
    setTimeout(() => setCopied(null), 2000)
  }

  const invitations = invitationsQuery.data?.invitations || []

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle className="flex items-center gap-2">
              <Link2 className="h-5 w-5" />
              {t('invitations.active_links')}
            </CardTitle>
            <CardDescription>{t('settings.members.description')}</CardDescription>
          </div>
          <Button size="sm" onClick={generateLink} disabled={generating}>
            <Plus className="mr-1 h-4 w-4" />
            {generating ? t('invitations.generating') : t('invitations.generate')}
          </Button>
        </CardHeader>
        <CardContent>
          {invitationsQuery.isLoading ? (
            <Skeleton className="h-10 w-full" />
          ) : invitations.length === 0 ? (
            <div className="py-6 text-center text-sm text-muted-foreground">
              <Link2 className="mx-auto h-10 w-10 text-muted-foreground" />
              <p className="mt-2">{t('invitations.no_invites')}</p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('invitations.copy_link')}</TableHead>
                  <TableHead>{t('invitations.uses')}</TableHead>
                  <TableHead>{t('invitations.expires')}</TableHead>
                  <TableHead className="text-right">{t('tunnels.table.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {invitations.map((inv) => (
                  <TableRow key={inv.id}>
                    <TableCell>
                      <button
                        type="button"
                        onClick={() => copyLink(inv.code)}
                        className="flex items-center gap-2 text-sm font-mono hover:text-primary"
                      >
                        <code className="rounded bg-muted px-2 py-1 text-xs">{inv.code}</code>
                        {copied === inv.code ? (
                          <Check className="h-3.5 w-3.5 text-emerald-500" />
                        ) : (
                          <Copy className="h-3.5 w-3.5" />
                        )}
                      </button>
                    </TableCell>
                    <TableCell className="text-sm">
                      {inv.max_uses === 0
                        ? t('invitations.unlimited_uses')
                        : `${inv.use_count} / ${inv.max_uses}`}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {inv.expires_at
                        ? new Date(inv.expires_at).toLocaleDateString()
                        : t('invitations.never_expires')}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => deleteMutation.mutate(inv.id)}
                      >
                        <Trash2 className="h-4 w-4 text-destructive" />
                      </Button>
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

function ActivityTab() {
  const { t } = useTranslation()

  const activityQuery = useQuery<{ events: OrgEvent[] }>({
    queryKey: ['org', 'activity'],
    queryFn: () => apiRequest<{ events: OrgEvent[] }>('/v1/org/activity'),
    refetchInterval: 30000,
  })

  const events = activityQuery.data?.events || []

  const formatAction = (action: string): string => {
    const parts = action.split('.')
    if (parts.length >= 2) {
      const resource = parts[0]
      const verb = parts[1]
      return `${resource} ${verb}`
    }
    return action
  }

  const getInitials = (name: string, email: string): string => {
    if (name) return name.charAt(0).toUpperCase()
    if (email) return email.charAt(0).toUpperCase()
    return '?'
  }

  const AVATAR_COLORS = [
    'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-200',
    'bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-200',
    'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-200',
    'bg-purple-100 text-purple-700 dark:bg-purple-900 dark:text-purple-200',
    'bg-pink-100 text-pink-700 dark:bg-pink-900 dark:text-pink-200',
    'bg-cyan-100 text-cyan-700 dark:bg-cyan-900 dark:text-cyan-200',
  ]

  const formatTimeAgo = (dateStr: string): string => {
    const date = new Date(dateStr)
    const now = new Date()
    const diffMs = now.getTime() - date.getTime()
    const diffMins = Math.floor(diffMs / 60000)
    if (diffMins < 1) return t('notifications.ago.now')
    if (diffMins < 60) return t('notifications.ago.minute', { n: diffMins })
    const diffHours = Math.floor(diffMins / 60)
    if (diffHours < 24) return t('notifications.ago.hour', { n: diffHours })
    return t('notifications.ago.day', { n: Math.floor(diffHours / 24) })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <ClipboardList className="h-5 w-5" />
          {t('activity.title')}
        </CardTitle>
        <CardDescription>{t('activity.description')}</CardDescription>
      </CardHeader>
      <CardContent>
        {activityQuery.isLoading ? (
          <div className="space-y-3">
            <Skeleton className="h-12 w-full" />
            <Skeleton className="h-12 w-full" />
            <Skeleton className="h-12 w-full" />
          </div>
        ) : activityQuery.isError ? (
          <div className="flex flex-col items-center gap-2 py-4 text-center">
            <AlertCircle className="h-6 w-6 text-destructive" />
            <p className="text-sm text-destructive">{t('activity.failed_load')}</p>
          </div>
        ) : events.length === 0 ? (
          <div className="py-6 text-center text-sm text-muted-foreground">
            <ClipboardList className="mx-auto h-10 w-10 text-muted-foreground" />
            <p className="mt-2">{t('activity.empty')}</p>
          </div>
        ) : (
          <div className="space-y-4">
            {events.map((event) => {
              const initials = getInitials(event.user_name, event.user_email)
              const colorIndex = Math.abs(event.user_id.split('').reduce((a, c) => a + c.charCodeAt(0), 0)) % AVATAR_COLORS.length

              return (
                <div key={event.id} className="flex items-start gap-3">
                  <div className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-xs font-bold ${AVATAR_COLORS[colorIndex]}`}>
                    {initials}
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-sm">
                      <span className="font-medium">{event.user_name || event.user_email}</span>{' '}
                      <span className="text-muted-foreground">
                        {formatAction(event.action)}
                      </span>
                      {event.resource_type && (
                        <span className="text-muted-foreground"> {event.resource_type}</span>
                      )}
                      {event.resource_id && (
                        <code className="ml-1 rounded bg-muted px-1 py-0.5 text-xs font-mono">
                          {event.resource_id.substring(0, 8)}...
                        </code>
                      )}
                    </p>
                    <p className="text-xs text-muted-foreground mt-0.5">
                      {formatTimeAgo(event.created_at)}
                    </p>
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function SessionsTab({ toast }: { toast: (t: { title: string; description?: string; variant?: 'default' | 'success' | 'error' | 'warning' }) => void }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [revokeTarget, setRevokeTarget] = useState<string | null>(null)

  const sessionsQuery = useQuery<{ sessions: import('@/lib/types').Session[] }>({
    queryKey: ['sessions'],
    queryFn: () => apiRequest<{ sessions: import('@/lib/types').Session[] }>('/v1/sessions'),
  })

  const revokeMutation = useMutation<void, Error, string>({
    mutationFn: (id) =>
      apiRequest(`/v1/sessions/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sessions'] })
      toast({ title: t('sessions.revoked'), variant: 'success' })
      setRevokeTarget(null)
    },
  })

  const handleRevoke = async () => {
    if (!revokeTarget) return
    await revokeMutation.mutateAsync(revokeTarget)
  }

  const sessions = sessionsQuery.data?.sessions || []

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Clock className="h-5 w-5" />
          {t('sessions.title')}
        </CardTitle>
        <CardDescription>{t('sessions.subtitle')}</CardDescription>
      </CardHeader>
      <CardContent>
        {sessionsQuery.isLoading ? (
          <div className="space-y-3">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </div>
        ) : sessionsQuery.isError ? (
          <div className="flex flex-col items-center gap-2 py-4 text-center">
            <AlertCircle className="h-6 w-6 text-destructive" />
            <p className="text-sm text-destructive">{t('sessions.failed_load')}</p>
          </div>
        ) : sessions.length === 0 ? (
          <div className="py-6 text-center text-sm text-muted-foreground">
            <Clock className="mx-auto h-10 w-10 text-muted-foreground" />
            <p className="mt-2">{t('sessions.no_sessions')}</p>
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('sessions.table.browser')}</TableHead>
                <TableHead>{t('sessions.table.ip')}</TableHead>
                <TableHead>{t('sessions.table.location')}</TableHead>
                <TableHead>{t('sessions.table.last_active')}</TableHead>
                <TableHead className="text-right">{t('sessions.table.actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {sessions.map((session) => (
                <TableRow key={session.id}>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <span className="text-sm">{session.browser} / {session.os}</span>
                      {session.current && (
                        <Badge variant="success" className="text-xs">{t('sessions.current_session')}</Badge>
                      )}
                    </div>
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground font-mono">
                    {session.ip || '—'}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {session.location || '—'}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {new Date(session.last_active).toLocaleString()}
                  </TableCell>
                  <TableCell className="text-right">
                    {!session.current && (
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setRevokeTarget(session.id)}
                      >
                        <Trash2 className="h-4 w-4 text-destructive" />
                      </Button>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>

      <Dialog
        open={!!revokeTarget}
        onClose={() => setRevokeTarget(null)}
        title={t('sessions.revoke')}
        description={t('sessions.revoke_confirm')}
      >
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="outline" onClick={() => setRevokeTarget(null)}>
            {t('common.cancel')}
          </Button>
          <Button variant="destructive" onClick={handleRevoke} disabled={revokeMutation.isPending}>
            {revokeMutation.isPending ? t('sessions.revoking') : t('sessions.revoke')}
          </Button>
        </div>
      </Dialog>
    </Card>
  )
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
}
