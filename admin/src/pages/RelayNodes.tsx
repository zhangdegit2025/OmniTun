import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Card, CardContent } from '@/components/ui/Card'
import { Input } from '@/components/ui/Input'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Search } from 'lucide-react'

interface RelayRow {
  id: string
  name: string
  hostname: string
  region: string
  status: 'online' | 'offline' | 'maintenance'
  version: string
  cpu_usage: number
  memory_usage: number
  active_connections: number
  last_seen_at: string
}

const mockRelays: RelayRow[] = [
  { id: '1', name: 'relay-us-east-1', hostname: 'us-east-1.omnitun.io', region: 'US East', status: 'online', version: '1.2.0', cpu_usage: 23, memory_usage: 45, active_connections: 1450, last_seen_at: '2026-05-21T11:30:00Z' },
  { id: '2', name: 'relay-us-west-1', hostname: 'us-west-1.omnitun.io', region: 'US West', status: 'online', version: '1.2.0', cpu_usage: 18, memory_usage: 38, active_connections: 980, last_seen_at: '2026-05-21T11:29:00Z' },
  { id: '3', name: 'relay-eu-central-1', hostname: 'eu-central-1.omnitun.io', region: 'EU Central', status: 'online', version: '1.1.9', cpu_usage: 35, memory_usage: 62, active_connections: 2100, last_seen_at: '2026-05-21T11:28:00Z' },
  { id: '4', name: 'relay-ap-southeast-1', hostname: 'ap-se-1.omnitun.io', region: 'Asia Pacific', status: 'offline', version: '1.1.8', cpu_usage: 0, memory_usage: 0, active_connections: 0, last_seen_at: '2026-05-20T22:15:00Z' },
  { id: '5', name: 'relay-sa-east-1', hostname: 'sa-east-1.omnitun.io', region: 'South America', status: 'maintenance', version: '1.2.0', cpu_usage: 5, memory_usage: 12, active_connections: 45, last_seen_at: '2026-05-21T10:00:00Z' },
]

const statusVariant: Record<string, 'success' | 'destructive' | 'warning'> = {
  online: 'success',
  offline: 'destructive',
  maintenance: 'warning',
}

export default function RelayNodes() {
  const [search, setSearch] = useState('')

  const filtered = mockRelays.filter(
    (r) =>
      r.name.toLowerCase().includes(search.toLowerCase()) ||
      r.hostname.toLowerCase().includes(search.toLowerCase()) ||
      r.region.toLowerCase().includes(search.toLowerCase()),
  )

  const onlineCount = mockRelays.filter((r) => r.status === 'online').length

  return (
    <div className="space-y-6 p-6">
      <div>
        <h1 className="text-2xl font-bold">Relay Nodes</h1>
        <p className="text-sm text-muted-foreground">
          {onlineCount} of {mockRelays.length} nodes online
        </p>
      </div>

      <div className="relative max-w-sm">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          placeholder="Search relay nodes..."
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
                <TableHead>Region</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Version</TableHead>
                <TableHead>CPU</TableHead>
                <TableHead>Memory</TableHead>
                <TableHead>Connections</TableHead>
                <TableHead>Last Seen</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map((relay) => (
                <TableRow key={relay.id}>
                  <TableCell>
                    <Link to={`/relays/${relay.id}`} className="font-medium text-primary hover:underline">
                      {relay.name}
                    </Link>
                    <p className="text-xs text-muted-foreground">{relay.hostname}</p>
                  </TableCell>
                  <TableCell>{relay.region}</TableCell>
                  <TableCell>
                    <Badge variant={statusVariant[relay.status] ?? 'secondary'}>{relay.status}</Badge>
                  </TableCell>
                  <TableCell>
                    <code className="rounded bg-muted px-1 py-0.5 text-xs">{relay.version}</code>
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <div className="h-2 w-16 rounded-full bg-muted">
                        <div
                          className="h-2 rounded-full bg-primary transition-all"
                          style={{ width: `${relay.cpu_usage}%` }}
                        />
                      </div>
                      <span className="text-xs text-muted-foreground">{relay.cpu_usage}%</span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <div className="h-2 w-16 rounded-full bg-muted">
                        <div
                          className="h-2 rounded-full bg-amber-500 transition-all"
                          style={{ width: `${relay.memory_usage}%` }}
                        />
                      </div>
                      <span className="text-xs text-muted-foreground">{relay.memory_usage}%</span>
                    </div>
                  </TableCell>
                  <TableCell>{relay.active_connections.toLocaleString()}</TableCell>
                  <TableCell className="text-muted-foreground">
                    {new Date(relay.last_seen_at).toLocaleString()}
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
