import { useParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { ArrowLeft, Building2, Users, Server, BarChart3 } from 'lucide-react'

export default function OrganizationDetail() {
  const { id } = useParams()
  const { t } = useTranslation()

  const org = {
    id: id ?? '1',
    name: 'Acme Corp',
    email: 'admin@acme.com',
    plan: 'pro',
    status: 'active' as const,
    tunnel_count: 12,
    user_count: 25,
    bandwidth_used: 2.4 * 1024 * 1024 * 1024,
    bandwidth_limit: 10 * 1024 * 1024 * 1024,
    created_at: '2026-01-15T08:00:00Z',
  }

  const users = [
    { id: '1', name: 'John Doe', email: 'john@acme.com', role: 'admin', status: 'active' as const },
    { id: '2', name: 'Jane Smith', email: 'jane@acme.com', role: 'member', status: 'active' as const },
  ]

  const tunnels = [
    { id: '1', name: 'web-prod', protocol: 'https', status: 'active' as const, traffic: 324 * 1024 * 1024 },
    { id: '2', name: 'api-staging', protocol: 'tcp', status: 'stopped' as const, traffic: 0 },
  ]

  function formatBytes(bytes: number): string {
    if (bytes === 0) return '0 B'
    const units = ['B', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(1024))
    return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="sm" onClick={() => window.history.back()}>
          <ArrowLeft className="mr-1 h-4 w-4" />
          {t('organizationDetail.back')}
        </Button>
        <div>
          <h1 className="text-2xl font-bold">{org.name}</h1>
          <p className="text-sm text-muted-foreground">{t('organizationDetail.organization', { id: org.id })}</p>
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">{t('organizationDetail.status')}</CardTitle>
            <Building2 className="h-4 w-4 text-primary" />
          </CardHeader>
          <CardContent>
            <Badge variant="success">{org.status}</Badge>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">{t('organizationDetail.users')}</CardTitle>
            <Users className="h-4 w-4 text-primary" />
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">{org.user_count}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">{t('organizationDetail.tunnels')}</CardTitle>
            <Server className="h-4 w-4 text-primary" />
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">{org.tunnel_count}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">{t('organizationDetail.bandwidth')}</CardTitle>
            <BarChart3 className="h-4 w-4 text-primary" />
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">
              {formatBytes(org.bandwidth_used)} / {formatBytes(org.bandwidth_limit)}
            </p>
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>{t('organizationDetail.users')}</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('organizationDetail.status')}</TableHead>
                  <TableHead>{t('organizationDetail.role')}</TableHead>
                  <TableHead>{t('organizationDetail.status')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {users.map((u) => (
                  <TableRow key={u.id}>
                    <TableCell>
                      <p className="font-medium">{u.name}</p>
                      <p className="text-xs text-muted-foreground">{u.email}</p>
                    </TableCell>
                    <TableCell><Badge variant="secondary">{u.role}</Badge></TableCell>
                    <TableCell><Badge variant="success">{u.status}</Badge></TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t('organizationDetail.tunnels')}</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('common.name')}</TableHead>
                  <TableHead>{t('organizationDetail.protocol')}</TableHead>
                  <TableHead>{t('organizationDetail.status')}</TableHead>
                  <TableHead>{t('organizationDetail.traffic')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {tunnels.map((t) => (
                  <TableRow key={t.id}>
                    <TableCell className="font-medium">{t.name}</TableCell>
                    <TableCell><code className="rounded bg-muted px-1 py-0.5 text-xs">{t.protocol}</code></TableCell>
                    <TableCell>
                      <Badge variant={t.status === 'active' ? 'success' : 'secondary'}>{t.status}</Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground">{formatBytes(t.traffic)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
