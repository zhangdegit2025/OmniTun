import { lazy, Suspense } from 'react'
import { useParams } from 'react-router-dom'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Skeleton } from '@/components/ui/Skeleton'
import { ArrowLeft, Cpu, HardDrive, Activity, Globe } from 'lucide-react'

const TrafficLineChart = lazy(() => import('@/components/ui/TrafficLineChart'))

export default function RelayNodeDetail() {
  const { id } = useParams()

  const relay = {
    id: id ?? '1',
    name: 'relay-us-east-1',
    hostname: 'us-east-1.omnitun.io',
    region: 'US East',
    status: 'online' as const,
    version: '1.2.0',
    public_ip: '203.0.113.10',
    cpu_usage: 23,
    memory_usage: 45,
    disk_usage: 32,
    bandwidth_in: 2.4 * 1024 * 1024 * 1024,
    bandwidth_out: 1.8 * 1024 * 1024 * 1024,
    active_connections: 1450,
    total_tunnels: 89,
    uptime_seconds: 2592000,
    last_seen_at: '2026-05-21T11:30:00Z',
  }

  function formatBytes(bytes: number): string {
    if (bytes === 0) return '0 B'
    const units = ['B', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(1024))
    return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
  }

  function formatUptime(seconds: number): string {
    const days = Math.floor(seconds / 86400)
    const hours = Math.floor((seconds % 86400) / 3600)
    return `${days}d ${hours}h`
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="sm" onClick={() => window.history.back()}>
          <ArrowLeft className="mr-1 h-4 w-4" />
          Back
        </Button>
        <div>
          <h1 className="text-2xl font-bold">{relay.name}</h1>
          <p className="text-sm text-muted-foreground">{relay.hostname}</p>
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">Status</CardTitle>
            <Activity className="h-4 w-4 text-emerald-500" />
          </CardHeader>
          <CardContent>
            <Badge variant="success">online</Badge>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">CPU Usage</CardTitle>
            <Cpu className="h-4 w-4 text-primary" />
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">{relay.cpu_usage}%</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">Memory Usage</CardTitle>
            <HardDrive className="h-4 w-4 text-amber-500" />
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">{relay.memory_usage}%</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">Uptime</CardTitle>
            <Globe className="h-4 w-4 text-primary" />
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">{formatUptime(relay.uptime_seconds)}</p>
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Node Information</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-center justify-between rounded-md border border-border p-3">
              <span className="text-sm font-medium">Region</span>
              <span className="text-sm text-muted-foreground">{relay.region}</span>
            </div>
            <div className="flex items-center justify-between rounded-md border border-border p-3">
              <span className="text-sm font-medium">Version</span>
              <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{relay.version}</code>
            </div>
            <div className="flex items-center justify-between rounded-md border border-border p-3">
              <span className="text-sm font-medium">Public IP</span>
              <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{relay.public_ip}</code>
            </div>
            <div className="flex items-center justify-between rounded-md border border-border p-3">
              <span className="text-sm font-medium">Disk Usage</span>
              <span className="text-sm text-muted-foreground">{relay.disk_usage}%</span>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Traffic Statistics</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-center justify-between rounded-md border border-border p-3">
              <span className="text-sm font-medium">Active Connections</span>
              <span className="text-sm font-medium">{relay.active_connections.toLocaleString()}</span>
            </div>
            <div className="flex items-center justify-between rounded-md border border-border p-3">
              <span className="text-sm font-medium">Total Tunnels</span>
              <span className="text-sm font-medium">{relay.total_tunnels}</span>
            </div>
            <div className="flex items-center justify-between rounded-md border border-border p-3">
              <span className="text-sm font-medium">Bandwidth In</span>
              <span className="text-sm text-muted-foreground">{formatBytes(relay.bandwidth_in)}</span>
            </div>
            <div className="flex items-center justify-between rounded-md border border-border p-3">
              <span className="text-sm font-medium">Bandwidth Out</span>
              <span className="text-sm text-muted-foreground">{formatBytes(relay.bandwidth_out)}</span>
            </div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Traffic History</CardTitle>
        </CardHeader>
        <CardContent>
          <Suspense fallback={<Skeleton className="h-64 w-full" />}>
            <TrafficLineChart />
          </Suspense>
        </CardContent>
      </Card>
    </div>
  )
}
