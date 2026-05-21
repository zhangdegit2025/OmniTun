import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts'

function generateMockData() {
  const data = []
  const now = new Date()
  for (let i = 23; i >= 0; i--) {
    const t = new Date(now.getTime() - i * 60 * 60 * 1000)
    data.push({
      time: t.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
      in: Math.floor(Math.random() * 500 + 100),
      out: Math.floor(Math.random() * 400 + 50),
    })
  }
  return data
}

export default function TrafficLineChart() {
  const data = generateMockData()

  return (
    <ResponsiveContainer width="100%" height={320}>
      <LineChart data={data}>
        <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
        <XAxis dataKey="time" stroke="hsl(var(--muted-foreground))" fontSize={12} />
        <YAxis stroke="hsl(var(--muted-foreground))" fontSize={12} />
        <Tooltip
          contentStyle={{
            backgroundColor: 'hsl(var(--card))',
            border: '1px solid hsl(var(--border))',
            borderRadius: '0.5rem',
          }}
        />
        <Line
          type="monotone"
          dataKey="in"
          stroke="hsl(var(--primary))"
          strokeWidth={2}
          dot={false}
          name="Inbound"
        />
        <Line
          type="monotone"
          dataKey="out"
          stroke="#f59e0b"
          strokeWidth={2}
          dot={false}
          name="Outbound"
        />
      </LineChart>
    </ResponsiveContainer>
  )
}
