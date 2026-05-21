import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Card, CardContent } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Search, Plus } from 'lucide-react'

interface OrgRow {
  id: string
  name: string
  email: string
  plan: string
  status: 'active' | 'suspended' | 'trial'
  tunnel_count: number
  user_count: number
  created_at: string
}

const mockOrgs: OrgRow[] = [
  { id: '1', name: 'Acme Corp', email: 'admin@acme.com', plan: 'pro', status: 'active', tunnel_count: 12, user_count: 25, created_at: '2026-01-15' },
  { id: '2', name: 'Globex Inc', email: 'hello@globex.io', plan: 'enterprise', status: 'active', tunnel_count: 48, user_count: 120, created_at: '2025-11-01' },
  { id: '3', name: 'Initech', email: 'ops@initech.dev', plan: 'free', status: 'trial', tunnel_count: 1, user_count: 3, created_at: '2026-05-18' },
  { id: '4', name: 'Umbrella', email: 'support@umbrella.com', plan: 'pro', status: 'suspended', tunnel_count: 0, user_count: 8, created_at: '2025-08-22' },
  { id: '5', name: 'Stark Industries', email: 'tony@stark.net', plan: 'enterprise', status: 'active', tunnel_count: 67, user_count: 340, created_at: '2024-03-10' },
]

const statusVariant: Record<string, 'success' | 'destructive' | 'warning'> = {
  active: 'success',
  suspended: 'destructive',
  trial: 'warning',
}

const planVariant: Record<string, 'default' | 'success' | 'secondary'> = {
  free: 'secondary',
  pro: 'default',
  enterprise: 'success',
}

export default function Organizations() {
  const [search, setSearch] = useState('')

  const filtered = mockOrgs.filter(
    (o) =>
      o.name.toLowerCase().includes(search.toLowerCase()) ||
      o.email.toLowerCase().includes(search.toLowerCase()),
  )

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Organizations</h1>
          <p className="text-sm text-muted-foreground">Manage all tenant organizations</p>
        </div>
        <Button>
          <Plus className="mr-1 h-4 w-4" />
          New Organization
        </Button>
      </div>

      <div className="relative max-w-sm">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          placeholder="Search organizations..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="pl-9"
        />
      </div>

      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Plan</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Tunnels</TableHead>
                <TableHead>Users</TableHead>
                <TableHead>Created</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map((org) => (
                <TableRow key={org.id}>
                  <TableCell>
                    <Link to={`/organizations/${org.id}`} className="font-medium text-primary hover:underline">
                      {org.name}
                    </Link>
                    <p className="text-xs text-muted-foreground">{org.email}</p>
                  </TableCell>
                  <TableCell>
                    <Badge variant={planVariant[org.plan] ?? 'secondary'}>{org.plan}</Badge>
                  </TableCell>
                  <TableCell>
                    <Badge variant={statusVariant[org.status] ?? 'secondary'}>{org.status}</Badge>
                  </TableCell>
                  <TableCell>{org.tunnel_count}</TableCell>
                  <TableCell>{org.user_count}</TableCell>
                  <TableCell className="text-muted-foreground">
                    {new Date(org.created_at).toLocaleDateString()}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}
