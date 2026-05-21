import { useParams } from 'react-router-dom'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { ArrowLeft, Mail, Building2, Calendar, Shield } from 'lucide-react'

export default function UserDetail() {
  const { id } = useParams()

  const user = {
    id: id ?? '1',
    name: 'John Doe',
    email: 'john@acme.com',
    org_id: '1',
    org_name: 'Acme Corp',
    role: 'admin',
    status: 'active' as const,
    mfa_enabled: true,
    created_at: '2026-01-15T08:00:00Z',
    last_login_at: '2026-05-21T10:30:00Z',
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="sm" onClick={() => window.history.back()}>
          <ArrowLeft className="mr-1 h-4 w-4" />
          Back
        </Button>
        <div>
          <h1 className="text-2xl font-bold">{user.name}</h1>
          <p className="text-sm text-muted-foreground">User #{user.id}</p>
        </div>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Profile Information</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-center gap-3">
              <Mail className="h-4 w-4 text-muted-foreground" />
              <div>
                <p className="text-sm text-muted-foreground">Email</p>
                <p className="font-medium">{user.email}</p>
              </div>
            </div>
            <div className="flex items-center gap-3">
              <Building2 className="h-4 w-4 text-muted-foreground" />
              <div>
                <p className="text-sm text-muted-foreground">Organization</p>
                <p className="font-medium">{user.org_name}</p>
              </div>
            </div>
            <div className="flex items-center gap-3">
              <Shield className="h-4 w-4 text-muted-foreground" />
              <div>
                <p className="text-sm text-muted-foreground">Role</p>
                <Badge variant="default">{user.role}</Badge>
              </div>
            </div>
            <div className="flex items-center gap-3">
              <Calendar className="h-4 w-4 text-muted-foreground" />
              <div>
                <p className="text-sm text-muted-foreground">Joined</p>
                <p className="font-medium">{new Date(user.created_at).toLocaleDateString()}</p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Account Status</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-center justify-between rounded-md border border-border p-3">
              <span className="text-sm font-medium">Status</span>
              <Badge variant={user.status === 'active' ? 'success' : 'destructive'}>{user.status}</Badge>
            </div>
            <div className="flex items-center justify-between rounded-md border border-border p-3">
              <span className="text-sm font-medium">MFA Enabled</span>
              <Badge variant={user.mfa_enabled ? 'success' : 'secondary'}>
                {user.mfa_enabled ? 'Yes' : 'No'}
              </Badge>
            </div>
            <div className="flex items-center justify-between rounded-md border border-border p-3">
              <span className="text-sm font-medium">Last Login</span>
              <span className="text-sm text-muted-foreground">
                {user.last_login_at ? new Date(user.last_login_at).toLocaleString() : 'Never'}
              </span>
            </div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Actions</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex gap-3">
            <Button variant="outline">Reset Password</Button>
            <Button variant="outline">Disable Account</Button>
            <Button variant="destructive">Delete User</Button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
