import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Card, CardContent } from '@/components/ui/Card'
import { Input } from '@/components/ui/Input'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Search } from 'lucide-react'

interface UserRow {
  id: string
  name: string
  email: string
  org_name: string
  role: string
  status: 'active' | 'disabled'
  created_at: string
  last_login_at?: string
}

const mockUsers: UserRow[] = [
  { id: '1', name: 'John Doe', email: 'john@acme.com', org_name: 'Acme Corp', role: 'admin', status: 'active', created_at: '2026-01-15', last_login_at: '2026-05-21T10:00:00Z' },
  { id: '2', name: 'Jane Smith', email: 'jane@acme.com', org_name: 'Acme Corp', role: 'member', status: 'active', created_at: '2026-02-01', last_login_at: '2026-05-20T16:30:00Z' },
  { id: '3', name: 'Bob Wilson', email: 'bob@globex.io', org_name: 'Globex Inc', role: 'admin', status: 'active', created_at: '2025-11-15', last_login_at: '2026-05-21T08:00:00Z' },
  { id: '4', name: 'Alice Lee', email: 'alice@initech.dev', org_name: 'Initech', role: 'member', status: 'disabled', created_at: '2026-05-18' },
  { id: '5', name: 'Tony Stark', email: 'tony@stark.net', org_name: 'Stark Industries', role: 'admin', status: 'active', created_at: '2024-03-15', last_login_at: '2026-05-21T09:30:00Z' },
]

export default function Users() {
  const [search, setSearch] = useState('')

  const filtered = mockUsers.filter(
    (u) =>
      u.name.toLowerCase().includes(search.toLowerCase()) ||
      u.email.toLowerCase().includes(search.toLowerCase()) ||
      u.org_name.toLowerCase().includes(search.toLowerCase()),
  )

  return (
    <div className="space-y-6 p-6">
      <div>
        <h1 className="text-2xl font-bold">Users</h1>
        <p className="text-sm text-muted-foreground">Global user management across all organizations</p>
      </div>

      <div className="relative max-w-sm">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          placeholder="Search users..."
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
                <TableHead>User</TableHead>
                <TableHead>Organization</TableHead>
                <TableHead>Role</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Last Login</TableHead>
                <TableHead>Joined</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map((user) => (
                <TableRow key={user.id}>
                  <TableCell>
                    <Link to={`/users/${user.id}`} className="font-medium text-primary hover:underline">
                      {user.name}
                    </Link>
                    <p className="text-xs text-muted-foreground">{user.email}</p>
                  </TableCell>
                  <TableCell>{user.org_name}</TableCell>
                  <TableCell>
                    <Badge variant={user.role === 'admin' ? 'default' : 'secondary'}>{user.role}</Badge>
                  </TableCell>
                  <TableCell>
                    <Badge variant={user.status === 'active' ? 'success' : 'destructive'}>{user.status}</Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {user.last_login_at
                      ? new Date(user.last_login_at).toLocaleString()
                      : 'Never'}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {new Date(user.created_at).toLocaleDateString()}
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
