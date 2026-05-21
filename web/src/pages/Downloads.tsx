import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import {
  Download,
  Copy,
  Check,
  Monitor,
  Apple,
  Terminal,
  Cpu,
  Package,
  ChevronDown,
  ChevronRight,
} from 'lucide-react'
import { cn } from '@/lib/utils'

interface PlatformArch {
  os: string
  icon: typeof Monitor
  arches: string[]
}

interface VersionRelease {
  version: string
  date: string
  changes: string[]
  latest: boolean
}

const platformMatrix: PlatformArch[] = [
  { os: 'macOS', icon: Apple, arches: ['amd64 (Intel)', 'arm64 (Apple Silicon)'] },
  { os: 'Linux', icon: Terminal, arches: ['amd64', 'arm64'] },
  { os: 'Windows', icon: Monitor, arches: ['amd64', 'arm64'] },
]

const versions: VersionRelease[] = [
  {
    version: 'v1.3.0',
    date: '2026-05-15',
    latest: true,
    changes: [
      'Mesh network topology visualization',
      'Webhook delivery retry with configurable backoff',
      'DNS verification status auto-refresh',
    ],
  },
  {
    version: 'v1.2.1',
    date: '2026-04-28',
    latest: false,
    changes: [
      'Fix TLS certificate renewal for custom domains',
      'Improve tunnel reconnection stability under high load',
    ],
  },
  {
    version: 'v1.2.0',
    date: '2026-04-10',
    latest: false,
    changes: [
      'Custom domain support with automatic DNS verification',
      'Batch tunnel operations (start/stop/delete)',
      'Request inspector waterfall view',
    ],
  },
  {
    version: 'v1.1.0',
    date: '2026-03-20',
    latest: false,
    changes: [
      'Mesh networking with WireGuard integration',
      'OAuth / SAML SSO for enterprise authentication',
      'Command palette with search',
    ],
  },
  {
    version: 'v1.0.0',
    date: '2026-02-14',
    latest: false,
    changes: [
      'Initial release',
      'TCP / HTTP / HTTPS tunnel support',
      'Web dashboard with real-time traffic',
      'CLI tool with install scripts',
    ],
  },
]

const cliVersion = versions.find((v) => v.latest)?.version ?? 'v1.3.0'

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = () => {
    navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <Button variant="ghost" size="sm" onClick={handleCopy}>
      {copied ? <Check className="h-3.5 w-3.5 text-emerald-500" /> : <Copy className="h-3.5 w-3.5" />}
    </Button>
  )
}

function InstallCode({ code, label }: { code: string; label: string }) {
  return (
    <div className="flex items-center justify-between rounded-md bg-muted/50 border px-3 py-2">
      <div className="flex items-center gap-2 min-w-0">
        <span className="text-xs font-medium text-muted-foreground shrink-0">{label}</span>
        <code className="text-sm truncate">{code}</code>
      </div>
      <CopyButton text={code} />
    </div>
  )
}

function VersionHistory() {
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})
  const toggle = (v: string) => setExpanded((p) => ({ ...p, [v]: !p[v] }))

  return (
    <Card>
      <CardHeader>
        <CardTitle>Version History</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="space-y-1">
          {versions.map((v) => {
            const isOpen = expanded[v.version] ?? false
            return (
              <div key={v.version} className="border-b last:border-b-0">
                <button
                  onClick={() => toggle(v.version)}
                  className="flex w-full items-center gap-3 py-2.5 text-left hover:bg-muted/30 transition-colors rounded-sm px-2"
                >
                  {isOpen ? (
                    <ChevronDown className="h-4 w-4 text-muted-foreground shrink-0" />
                  ) : (
                    <ChevronRight className="h-4 w-4 text-muted-foreground shrink-0" />
                  )}
                  <span className="font-mono text-sm font-medium">{v.version}</span>
                  {v.latest && <Badge variant="success" className="text-xs">latest</Badge>}
                  <span className="text-xs text-muted-foreground ml-auto">{v.date}</span>
                </button>
                {isOpen && (
                  <ul className="ml-9 mb-2 space-y-1">
                    {v.changes.map((c, i) => (
                      <li key={i} className="text-sm text-muted-foreground flex items-start gap-2">
                        <span className="text-primary/60 mt-1.5 block h-1.5 w-1.5 rounded-full bg-primary/60 shrink-0" />
                        {c}
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            )
          })}
        </div>
      </CardContent>
    </Card>
  )
}

export default function Downloads() {
  const { t } = useTranslation()
  const [selectedOS, setSelectedOS] = useState<string>('macOS')

  const selected = platformMatrix.find((p) => p.os === selectedOS) ?? platformMatrix[0]
  const Icon = selected.icon

  const macCodes = [
    { label: 'curl', code: `curl -fsSL https://cli.omnitun.dev/install.sh | bash` },
    { label: 'brew', code: `brew install omnitun/tap/omnitun` },
  ]

  const linuxCodes = [
    { label: 'curl', code: `curl -fsSL https://cli.omnitun.dev/install.sh | bash` },
    { label: 'apt', code: `sudo apt-get install omnitun` },
  ]

  const winCodes = [
    { label: 'PowerShell', code: `irm https://cli.omnitun.dev/install.ps1 | iex` },
    { label: 'winget', code: `winget install OmniTun.CLI` },
    { label: 'choco', code: `choco install omnitun` },
  ]

  const getInstallCommands = () => {
    switch (selectedOS) {
      case 'macOS': return macCodes
      case 'Linux': return linuxCodes
      case 'Windows': return winCodes
      default: return macCodes
    }
  }

  const getPlatformFilename = (os: string, arch: string): string => {
    const ext = os === 'Windows' ? '.zip' : '.tar.gz'
    const osName = os.toLowerCase()
    const cleanArch = arch.replace(/ \(.*\)/, '')
    return `omnitun_${cliVersion}_${osName}_${cleanArch}${ext}`
  }

  return (
    <div className="space-y-6 p-6">
      <div>
        <div className="flex items-center gap-3">
          <Download className="h-6 w-6 text-primary" />
          <h1 className="text-2xl font-bold">{t('downloads.title', 'Downloads')}</h1>
        </div>
        <p className="mt-1 text-sm text-muted-foreground">
          {t('downloads.subtitle', 'Download the OmniTun CLI for your platform.')}
        </p>
      </div>

      {/* Platform Selection */}
      <div className="flex gap-2">
        {platformMatrix.map((p) => (
          <button
            key={p.os}
            onClick={() => setSelectedOS(p.os)}
            className={cn(
              'flex items-center gap-2 rounded-lg border px-4 py-2.5 text-sm font-medium transition-colors',
              selectedOS === p.os
                ? 'border-primary bg-primary/10 text-primary'
                : 'border-border bg-card text-muted-foreground hover:bg-accent',
            )}
          >
            <p.icon className="h-4 w-4" />
            {p.os}
          </button>
        ))}
      </div>

      {/* Platform Matrix */}
      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <div className="flex items-center gap-2">
              <Icon className="h-5 w-5 text-primary" />
              <CardTitle>{selectedOS}</CardTitle>
            </div>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              {selected.arches.map((arch) => {
                const filename = getPlatformFilename(selectedOS, arch)
                return (
                  <div
                    key={arch}
                    className="flex items-center justify-between rounded-md border px-3 py-2.5"
                  >
                    <div className="flex items-center gap-2">
                      <Cpu className="h-4 w-4 text-muted-foreground" />
                      <span className="text-sm font-mono">{arch}</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className="hidden sm:inline text-xs text-muted-foreground font-mono">
                        {filename}
                      </span>
                      <Button variant="outline" size="sm">
                        <Download className="mr-1 h-3.5 w-3.5" />
                        <span className="hidden sm:inline">Download</span>
                      </Button>
                    </div>
                  </div>
                )
              })}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <div className="flex items-center gap-2">
              <Terminal className="h-5 w-5 text-primary" />
              <CardTitle>Install</CardTitle>
            </div>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              {getInstallCommands().map((cmd) => (
                <InstallCode key={cmd.label} label={cmd.label} code={cmd.code} />
              ))}
            </div>

            <p className="mt-4 text-xs text-muted-foreground">
              After installation, verify with:
            </p>
            <div className="mt-1 flex items-center justify-between rounded-md bg-muted/50 border px-3 py-2">
              <code className="text-sm">omnitun version</code>
              <CopyButton text="omnitun version" />
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Quick Start */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Package className="h-5 w-5 text-primary" />
            <CardTitle>{t('downloads.quick_start', 'Quick Start')}</CardTitle>
          </div>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            <div>
              <h4 className="text-sm font-semibold mb-1">1. Authenticate</h4>
              <div className="flex items-center justify-between rounded-md bg-muted/50 border px-3 py-2">
                <code className="text-sm">omnitun login</code>
                <CopyButton text="omnitun login" />
              </div>
            </div>
            <div>
              <h4 className="text-sm font-semibold mb-1">2. Create a tunnel</h4>
              <div className="flex items-center justify-between rounded-md bg-muted/50 border px-3 py-2">
                <code className="text-sm">omnitun tunnel create my-api --protocol http --port 3000</code>
                <CopyButton text="omnitun tunnel create my-api --protocol http --port 3000" />
              </div>
            </div>
            <div>
              <h4 className="text-sm font-semibold mb-1">3. View status</h4>
              <div className="flex items-center justify-between rounded-md bg-muted/50 border px-3 py-2">
                <code className="text-sm">omnitun tunnel list</code>
                <CopyButton text="omnitun tunnel list" />
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Version History */}
      <VersionHistory />
    </div>
  )
}
