import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Skeleton } from '@/components/ui/Skeleton'
import { apiRequest } from '@/lib/api'
import { Plus, Pencil, Trash2, Shield, Copy } from 'lucide-react'

interface CustomRole {
  id: string
  name: string
  permissions: string[]
  assigned_users: number
  created_at: string
  updated_at: string
}

interface RoleTemplates {
  [key: string]: string[]
}

interface ListRolesResponse {
  roles: CustomRole[]
  templates: RoleTemplates
}

const PERMISSION_CATEGORIES = [
  {
    name: 'Tunnels',
    permissions: ['tunnels:create', 'tunnels:read', 'tunnels:update', 'tunnels:delete', 'tunnels:start', 'tunnels:stop'],
  },
  {
    name: 'Domains',
    permissions: ['domains:create', 'domains:read', 'domains:update', 'domains:delete'],
  },
  {
    name: 'Networks',
    permissions: ['networks:create', 'networks:read', 'networks:update', 'networks:delete', 'networks:join', 'networks:leave'],
  },
  {
    name: 'Billing',
    permissions: ['billing:read', 'billing:manage'],
  },
  {
    name: 'Members',
    permissions: ['members:read', 'members:invite', 'members:remove', 'members:change_role'],
  },
  {
    name: 'Settings',
    permissions: ['settings:read', 'settings:update'],
  },
]

function permissionLabel(perm: string): string {
  const parts = perm.split(':')
  const resource = parts[0].charAt(0).toUpperCase() + parts[0].slice(1)
  const action = parts[1].replace('_', ' ')
  return `${action} ${resource}`
}

export default function Roles() {
  const { t } = useTranslation()
  const [roles, setRoles] = useState<CustomRole[]>([])
  const [templates, setTemplates] = useState<RoleTemplates>({})
  const [loading, setLoading] = useState(true)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<CustomRole | null>(null)

  const [formName, setFormName] = useState('')
  const [formPerms, setFormPerms] = useState<string[]>([])
  const [saving, setSaving] = useState(false)

  const fetchRoles = useCallback(async () => {
    setLoading(true)
    try {
      const data = await apiRequest<ListRolesResponse>('/api/admin/v1/roles')
      setRoles(data.roles || [])
      setTemplates(data.templates || {})
    } catch {
      setRoles([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchRoles()
  }, [fetchRoles])

  function openCreate() {
    setEditing(null)
    setFormName('')
    setFormPerms([])
    setDialogOpen(true)
  }

  function openEdit(role: CustomRole) {
    setEditing(role)
    setFormName(role.name)
    setFormPerms([...role.permissions])
    setDialogOpen(true)
  }

  function applyTemplate(templateName: string) {
    const perms = templates[templateName]
    if (perms) {
      setFormPerms([...perms])
    }
  }

  function togglePermission(perm: string) {
    setFormPerms((prev) => {
      if (prev.includes(perm)) {
        return prev.filter((p) => p !== perm)
      }
      return [...prev, perm]
    })
  }

  function toggleCategory(perms: string[], checked: boolean) {
    setFormPerms((prev) => {
      if (checked) {
        const toAdd = perms.filter((p) => !prev.includes(p))
        return [...prev, ...toAdd]
      }
      return prev.filter((p) => !perms.includes(p))
    })
  }

  function allCategoryChecked(perms: string[]): boolean {
    return perms.every((p) => formPerms.includes(p))
  }

  function someCategoryChecked(perms: string[]): boolean {
    return perms.some((p) => formPerms.includes(p)) && !allCategoryChecked(perms)
  }

  async function handleSave() {
    if (!formName.trim() || formPerms.length === 0) return
    setSaving(true)
    try {
      const body = {
        name: formName.trim(),
        permissions: formPerms,
      }
      if (editing) {
        await apiRequest(`/api/admin/v1/roles/${encodeURIComponent(editing.id)}`, {
          method: 'PUT',
          body: JSON.stringify(body),
        })
      } else {
        await apiRequest('/api/admin/v1/roles', {
          method: 'POST',
          body: JSON.stringify(body),
        })
      }
      setDialogOpen(false)
      fetchRoles()
    } catch {
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(role: CustomRole) {
    if (!confirm(t('roles.deleteConfirm', { name: role.name }))) return
    try {
      await apiRequest(`/api/admin/v1/roles/${encodeURIComponent(role.id)}`, {
        method: 'DELETE',
      })
      fetchRoles()
    } catch (e) {
      const err = e as { code?: string; message?: string }
      if (err.code === 'has_users') {
        alert(`Cannot delete: ${err.message}`)
      }
    }
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">{t('roles.title')}</h1>
          <p className="text-sm text-muted-foreground">{t('roles.subtitle_other', { count: roles.length })}</p>
        </div>
        <Button onClick={openCreate}>
          <Plus className="mr-1 h-4 w-4" />
          {t('roles.newRole')}
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
                  <TableHead>{t('roles.name')}</TableHead>
                  <TableHead>{t('roles.permissions')}</TableHead>
                  <TableHead>{t('roles.assignedUsers')}</TableHead>
                  <TableHead>{t('common.created')}</TableHead>
                  <TableHead className="w-[120px]">{t('roles.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {roles.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={5} className="text-center text-muted-foreground py-8">
                      <Shield className="mx-auto mb-2 h-8 w-8 opacity-20" />
                      {t('roles.noRoles')}
                    </TableCell>
                  </TableRow>
                )}
                {roles.map((role) => (
                  <TableRow key={role.id}>
                    <TableCell>
                      <div className="font-medium">{role.name}</div>
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1 max-w-[300px]">
                        {role.permissions.slice(0, 4).map((p) => (
                          <Badge key={p} variant="secondary" className="text-[10px]">
                            {p}
                          </Badge>
                        ))}
                        {role.permissions.length > 4 && (
                          <Badge variant="secondary" className="text-[10px]">
                            {t('roles.more', { count: role.permissions.length - 4 })}
                          </Badge>
                        )}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={role.assigned_users > 0 ? 'default' : 'secondary'}>
                        {role.assigned_users}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {new Date(role.created_at).toLocaleDateString()}
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1">
                        <Button variant="ghost" size="sm" onClick={() => openEdit(role)}>
                          <Pencil className="h-3.5 w-3.5" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => handleDelete(role)}
                          disabled={role.assigned_users > 0}
                        >
                          <Trash2 className={`h-3.5 w-3.5 ${role.assigned_users > 0 ? 'text-muted-foreground' : 'text-destructive'}`} />
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
          <div className="w-full max-w-2xl max-h-[85vh] overflow-y-auto rounded-lg border bg-card shadow-lg">
            <CardHeader>
              <CardTitle>{editing ? t('roles.editRole') : t('roles.createRole')}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <Input
                label={t('roles.roleName')}
                placeholder={t('roles.roleNamePlaceholder')}
                value={formName}
                onChange={(e) => setFormName(e.target.value)}
              />

              {Object.keys(templates).length > 0 && (
                <div>
                  <label className="text-sm font-medium text-foreground">{t('roles.templates')}</label>
                  <div className="mt-1.5 flex flex-wrap gap-2">
                    {Object.keys(templates).map((name) => (
                      <Button
                        key={name}
                        variant="outline"
                        size="sm"
                        onClick={() => applyTemplate(name)}
                      >
                        <Copy className="mr-1 h-3 w-3" />
                        {name}
                      </Button>
                    ))}
                  </div>
                </div>
              )}

              <div>
                <label className="text-sm font-medium text-foreground">{t('roles.permissions')}</label>
                <div className="mt-2 space-y-3">
                  {PERMISSION_CATEGORIES.map((cat) => (
                    <fieldset key={cat.name} className="rounded-md border border-border p-3">
                      <legend className="px-1 text-sm font-medium">
                        <label className="flex items-center gap-2 cursor-pointer">
                          <input
                            type="checkbox"
                            checked={allCategoryChecked(cat.permissions)}
                            ref={(el) => {
                              if (el) el.indeterminate = someCategoryChecked(cat.permissions)
                            }}
                            onChange={(e) => toggleCategory(cat.permissions, e.target.checked)}
                            className="h-4 w-4 rounded border-input"
                          />
                          {cat.name}
                        </label>
                      </legend>
                      <div className="mt-1.5 grid grid-cols-2 gap-x-4 gap-y-1">
                        {cat.permissions.map((perm) => (
                          <label key={perm} className="flex items-center gap-2 text-sm cursor-pointer">
                            <input
                              type="checkbox"
                              checked={formPerms.includes(perm)}
                              onChange={() => togglePermission(perm)}
                              className="h-3.5 w-3.5 rounded border-input"
                            />
                            <span className="text-muted-foreground">{permissionLabel(perm)}</span>
                          </label>
                        ))}
                      </div>
                    </fieldset>
                  ))}
                </div>
              </div>

              <div className="flex justify-between pt-2 text-sm text-muted-foreground">
                <span>{t('roles.permissionsSelected_other', { count: formPerms.length })}</span>
                <div className="flex gap-2">
                  <Button variant="outline" onClick={() => setDialogOpen(false)}>
                    {t('common.cancel')}
                  </Button>
                  <Button onClick={handleSave} disabled={saving || !formName.trim() || formPerms.length === 0}>
                    {saving ? t('common.saving') : t('common.save')}
                  </Button>
                </div>
              </div>
            </CardContent>
          </div>
        </div>
      )}
    </div>
  )
}
