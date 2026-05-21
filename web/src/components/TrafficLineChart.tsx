import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts'
import type { TrafficPoint } from '@/lib/types'

interface TrafficLineChartProps {
  data: TrafficPoint[]
}

export default function TrafficLineChart({ data }: TrafficLineChartProps) {
  return (
    <ResponsiveContainer width="100%" height={250}>
      <LineChart data={data}>
        <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
        <XAxis
          dataKey="timestamp"
          tick={{ fontSize: 11 }}
          className="text-muted-foreground"
          tickFormatter={(v: string) => new Date(v).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
        />
        <YAxis tick={{ fontSize: 11 }} className="text-muted-foreground" tickFormatter={(v: number) => formatBytes(v)} />
        <Tooltip
          contentStyle={{
            backgroundColor: 'hsl(var(--card))',
            border: '1px solid hsl(var(--border))',
            borderRadius: 'var(--radius)',
          }}
          formatter={(value: number) => [formatBytes(value), '']}
        />
        <Line type="monotone" dataKey="bytes_in" stroke="hsl(var(--primary))" strokeWidth={2} dot={false} name="Inbound" />
        <Line type="monotone" dataKey="bytes_out" stroke="#22c55e" strokeWidth={2} dot={false} name="Outbound" />
      </LineChart>
    </ResponsiveContainer>
  )
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
}
