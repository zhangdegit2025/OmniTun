import { useState, useCallback, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/Card'
import { useToast } from '@/components/ui/useToast'
import { apiRequest } from '@/lib/api'
import {
  ArrowRight,
  ArrowLeft,
  Check,
  Copy,
  Globe,
  Network,
  Terminal,
  Server,
  PartyPopper,
  ExternalLink,
  Loader2,
  Apple,
  Monitor,
} from 'lucide-react'

const STEPS = 5

const CLI_COMMANDS: Record<string, { install: string; verify: string }> = {
  macos: {
    install: 'brew install omnitun/tap/omnitun',
    verify: 'omnitun --version',
  },
  linux: {
    install: 'curl -fsSL https://get.omnitun.dev | bash',
    verify: 'omnitun --version',
  },
  windows: {
    install: 'irm https://get.omnitun.dev/install.ps1 | iex',
    verify: 'omnitun --version',
  },
}

function slugify(text: string): string {
  return text
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 64)
}

function ProgressBar({ step }: { step: number }) {
  return (
    <div className="flex items-center justify-center gap-2">
      {Array.from({ length: STEPS }, (_, i) => (
        <div key={i} className="flex items-center">
          <div
            className={`flex h-8 w-8 items-center justify-center rounded-full text-sm font-semibold transition-all duration-300 ${
              i + 1 <= step
                ? 'bg-blue-600 text-white shadow-lg shadow-blue-600/30'
                : 'bg-slate-700 text-slate-400'
            }`}
          >
            {i + 1 < step ? <Check className="h-4 w-4" /> : i + 1}
          </div>
          {i < STEPS - 1 && (
            <div
              className={`h-0.5 w-8 transition-colors duration-300 ${
                i + 1 < step ? 'bg-blue-600' : 'bg-slate-700'
              }`}
            />
          )}
        </div>
      ))}
    </div>
  )
}

export default function Onboarding() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { toast } = useToast()
  const [step, setStep] = useState(1)

  const [orgName, setOrgName] = useState('')
  const [orgSlug, setOrgSlug] = useState('')
  const [orgCreating, setOrgCreating] = useState(false)
  const [orgError, setOrgError] = useState('')

  const [cliPlatform, setCliPlatform] = useState<'macos' | 'linux' | 'windows'>('macos')
  const [copiedPlatform, setCopiedPlatform] = useState<string | null>(null)

  const [tunnelPort, setTunnelPort] = useState('')
  const [tunnelProtocol, setTunnelProtocol] = useState('http')
  const [tunnelName, setTunnelName] = useState('')
  const [tunnelCreating, setTunnelCreating] = useState(false)
  const [createdTunnel, setCreatedTunnel] = useState<{
    id: string
    slug: string
    domain: string
  } | null>(null)

  const nextStep = useCallback(() => {
    setStep((s) => Math.min(s + 1, STEPS))
  }, [])

  const prevStep = useCallback(() => {
    setStep((s) => Math.max(s - 1, 1))
  }, [])

  const handleSkip = async () => {
    try {
      await apiRequest('/v1/org/onboarding/complete', { method: 'POST' })
    } catch {
      // proceed anyway
    }
    navigate('/')
  }

  const handleFinish = async () => {
    try {
      await apiRequest('/v1/org/onboarding/complete', { method: 'POST' })
    } catch {
      // proceed anyway
    }
    navigate('/')
  }

  const handleCreateOrg = async (e: FormEvent) => {
    e.preventDefault()
    if (!orgName.trim()) {
      setOrgError(t('onboarding.step2_name_required'))
      return
    }
    setOrgError('')
    setOrgCreating(true)
    try {
      await apiRequest('/v1/org/', {
        method: 'PATCH',
        body: JSON.stringify({ name: orgName.trim(), slug: orgSlug || slugify(orgName) }),
      })
      nextStep()
    } catch {
      setOrgError(t('onboarding.failed_create_org'))
    } finally {
      setOrgCreating(false)
    }
  }

  const handleCopy = (text: string, key: string) => {
    navigator.clipboard.writeText(text)
    setCopiedPlatform(key)
    toast({ title: t('onboarding.copied_cli'), variant: 'success' })
    setTimeout(() => setCopiedPlatform(null), 2000)
  }

  const handleCreateTunnel = async (e: FormEvent) => {
    e.preventDefault()
    setTunnelCreating(true)
    try {
      const data = await apiRequest<{
        id: string
        slug: string
        domain: string
        protocol: string
        local_port: number
      }>('/v1/tunnels', {
        method: 'POST',
        body: JSON.stringify({
          name: tunnelName || t('onboarding.step4_name_placeholder'),
          protocol: tunnelProtocol,
          local_port: parseInt(tunnelPort) || 3000,
        }),
      })
      toast({ title: t('onboarding.step4_created'), variant: 'success' })
      setCreatedTunnel({
        id: data.id,
        slug: data.slug,
        domain: data.domain || `${data.slug}.omnitun.io`,
      })
      nextStep()
    } catch {
      toast({ title: t('onboarding.failed_create_tunnel'), variant: 'error' })
    } finally {
      setTunnelCreating(false)
    }
  }

  return (
    <div className="flex min-h-screen flex-col items-center bg-gradient-to-br from-slate-900 via-blue-950 to-slate-900">
      <div className="flex w-full items-center justify-between px-6 py-4">
        <div className="flex items-center gap-2">
          <Server className="h-6 w-6 text-blue-400" />
          <span className="text-lg font-bold text-white">OmniTun</span>
        </div>
        <button
          onClick={handleSkip}
          className="text-sm text-slate-400 transition-colors hover:text-slate-300"
        >
          {t('onboarding.skip')}
        </button>
      </div>

      <div className="mt-8 mb-2">
        <ProgressBar step={step} />
      </div>

      <div className="mt-6 w-full max-w-2xl px-4 pb-16">
        {step === 1 && (
          <div className="space-y-8 text-center">
            <div className="mx-auto flex h-20 w-20 items-center justify-center rounded-2xl bg-blue-600/20">
              <Globe className="h-10 w-10 text-blue-400" />
            </div>
            <div>
              <h1 className="text-3xl font-bold text-white">{t('onboarding.step1_title')}</h1>
              <p className="mt-3 text-slate-400">{t('onboarding.step1_desc')}</p>
            </div>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
              <Card className="border-slate-700 bg-slate-800/50 text-center backdrop-blur">
                <CardContent className="pt-6">
                  <div className="mx-auto mb-3 flex h-10 w-10 items-center justify-center rounded-lg bg-blue-600/20">
                    <Terminal className="h-5 w-5 text-blue-400" />
                  </div>
                  <h3 className="font-semibold text-white">{t('onboarding.step1_feature1_title')}</h3>
                  <p className="mt-1 text-sm text-slate-400">{t('onboarding.step1_feature1_desc')}</p>
                </CardContent>
              </Card>
              <Card className="border-slate-700 bg-slate-800/50 text-center backdrop-blur">
                <CardContent className="pt-6">
                  <div className="mx-auto mb-3 flex h-10 w-10 items-center justify-center rounded-lg bg-emerald-600/20">
                    <Globe className="h-5 w-5 text-emerald-400" />
                  </div>
                  <h3 className="font-semibold text-white">{t('onboarding.step1_feature2_title')}</h3>
                  <p className="mt-1 text-sm text-slate-400">{t('onboarding.step1_feature2_desc')}</p>
                </CardContent>
              </Card>
              <Card className="border-slate-700 bg-slate-800/50 text-center backdrop-blur">
                <CardContent className="pt-6">
                  <div className="mx-auto mb-3 flex h-10 w-10 items-center justify-center rounded-lg bg-purple-600/20">
                    <Network className="h-5 w-5 text-purple-400" />
                  </div>
                  <h3 className="font-semibold text-white">{t('onboarding.step1_feature3_title')}</h3>
                  <p className="mt-1 text-sm text-slate-400">{t('onboarding.step1_feature3_desc')}</p>
                </CardContent>
              </Card>
            </div>
          </div>
        )}

        {step === 2 && (
          <Card className="border-slate-700 bg-slate-800/80 backdrop-blur">
            <CardHeader className="text-center">
              <CardTitle className="text-2xl text-white">{t('onboarding.step2_title')}</CardTitle>
              <CardDescription className="text-slate-400">
                {t('onboarding.step2_desc')}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <form onSubmit={handleCreateOrg} className="flex flex-col gap-4">
                <Input
                  label={t('onboarding.step2_name_label')}
                  placeholder={t('onboarding.step2_name_placeholder')}
                  value={orgName}
                  onChange={(e) => {
                    setOrgName(e.target.value)
                    setOrgSlug(slugify(e.target.value))
                    if (orgError) setOrgError('')
                  }}
                  error={orgError}
                />
                <Input
                  label={t('onboarding.step2_slug_label')}
                  placeholder={t('onboarding.step2_slug_placeholder')}
                  value={orgSlug}
                  onChange={(e) => setOrgSlug(slugify(e.target.value))}
                  disabled
                />
                <Button type="submit" className="w-full" disabled={orgCreating}>
                  {orgCreating ? (
                    <>
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                      {t('onboarding.creating_org')}
                    </>
                  ) : (
                    t('onboarding.next')
                  )}
                </Button>
              </form>
            </CardContent>
          </Card>
        )}

        {step === 3 && (
          <div className="space-y-6">
            <div className="text-center">
              <div className="mx-auto flex h-20 w-20 items-center justify-center rounded-2xl bg-blue-600/20">
                <Terminal className="h-10 w-10 text-blue-400" />
              </div>
              <h1 className="mt-4 text-2xl font-bold text-white">{t('onboarding.step3_title')}</h1>
              <p className="mt-2 text-slate-400">{t('onboarding.step3_desc')}</p>
            </div>

            <div className="flex justify-center gap-2">
              {(['macos', 'linux', 'windows'] as const).map((platform) => {
                const icons: Record<string, React.ReactNode> = {
                  macos: <Apple className="h-4 w-4" />,
                  linux: <Terminal className="h-4 w-4" />,
                  windows: <Monitor className="h-4 w-4" />,
                }
                return (
                  <button
                    key={platform}
                    onClick={() => setCliPlatform(platform)}
                    className={`flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium transition-colors ${
                      cliPlatform === platform
                        ? 'bg-blue-600 text-white'
                        : 'bg-slate-800 text-slate-400 hover:text-slate-200'
                    }`}
                  >
                    {icons[platform]}
                    {t(`onboarding.step3_${platform}`)}
                  </button>
                )
              })}
            </div>

            <Card className="border-slate-700 bg-slate-800/80 backdrop-blur">
              <CardContent className="space-y-4 pt-6">
                <div>
                  <p className="mb-2 text-sm text-slate-400">{t('onboarding.step3_install_note')}</p>
                  <div className="flex items-center gap-2 rounded-lg bg-slate-900 p-3 font-mono text-sm text-slate-200">
                    <code className="flex-1 break-all">{CLI_COMMANDS[cliPlatform].install}</code>
                    <button
                      onClick={() => handleCopy(CLI_COMMANDS[cliPlatform].install, `install-${cliPlatform}`)}
                      className="shrink-0 rounded p-1.5 text-slate-400 transition-colors hover:bg-slate-700 hover:text-white"
                    >
                      {copiedPlatform === `install-${cliPlatform}` ? (
                        <Check className="h-4 w-4 text-emerald-400" />
                      ) : (
                        <Copy className="h-4 w-4" />
                      )}
                    </button>
                  </div>
                </div>
                <div>
                  <p className="mb-2 text-sm text-slate-400">{t('onboarding.step3_verify')}</p>
                  <div className="flex items-center gap-2 rounded-lg bg-slate-900 p-3 font-mono text-sm text-slate-200">
                    <code className="flex-1">{CLI_COMMANDS[cliPlatform].verify}</code>
                    <button
                      onClick={() => handleCopy(CLI_COMMANDS[cliPlatform].verify, `verify-${cliPlatform}`)}
                      className="shrink-0 rounded p-1.5 text-slate-400 transition-colors hover:bg-slate-700 hover:text-white"
                    >
                      {copiedPlatform === `verify-${cliPlatform}` ? (
                        <Check className="h-4 w-4 text-emerald-400" />
                      ) : (
                        <Copy className="h-4 w-4" />
                      )}
                    </button>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
        )}

        {step === 4 && (
          <Card className="border-slate-700 bg-slate-800/80 backdrop-blur">
            <CardHeader className="text-center">
              <CardTitle className="text-2xl text-white">{t('onboarding.step4_title')}</CardTitle>
              <CardDescription className="text-slate-400">
                {t('onboarding.step4_desc')}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <form onSubmit={handleCreateTunnel} className="flex flex-col gap-4">
                <Input
                  label={t('onboarding.step4_name_label')}
                  placeholder={t('onboarding.step4_name_placeholder')}
                  value={tunnelName}
                  onChange={(e) => setTunnelName(e.target.value)}
                />
                <div className="grid grid-cols-2 gap-4">
                  <Input
                    label={t('onboarding.step4_port_label')}
                    type="number"
                    placeholder={t('onboarding.step4_port_placeholder')}
                    value={tunnelPort}
                    onChange={(e) => setTunnelPort(e.target.value)}
                  />
                  <div className="flex flex-col gap-1.5">
                    <label className="text-sm font-medium text-slate-300">
                      {t('onboarding.step4_protocol_label')}
                    </label>
                    <select
                      value={tunnelProtocol}
                      onChange={(e) => setTunnelProtocol(e.target.value)}
                      className="h-10 rounded-lg border border-slate-600 bg-slate-900 px-3 text-sm text-slate-200 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                    >
                      <option value="http">HTTP</option>
                      <option value="https">HTTPS</option>
                      <option value="tcp">TCP</option>
                    </select>
                  </div>
                </div>
                <div className="rounded-lg bg-slate-900 p-4 text-center">
                  <p className="text-sm text-slate-400">{t('onboarding.step4_preview_title')}</p>
                  <p className="mt-1 font-mono text-base text-blue-400">
                    https://{tunnelName ? slugify(tunnelName) : 'my-tunnel'}.omnitun.dev
                  </p>
                </div>
                <Button type="submit" className="w-full" disabled={tunnelCreating}>
                  {tunnelCreating ? (
                    <>
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                      {t('onboarding.step4_creating')}
                    </>
                  ) : (
                    t('onboarding.step4_create')
                  )}
                </Button>
              </form>
            </CardContent>
          </Card>
        )}

        {step === 5 && (
          <div className="space-y-8 text-center">
            <div className="mx-auto flex h-24 w-24 items-center justify-center rounded-full bg-emerald-600/20">
              <PartyPopper className="h-12 w-12 text-emerald-400" />
            </div>
            <div>
              <h1 className="text-3xl font-bold text-white">{t('onboarding.step5_title')}</h1>
              <p className="mt-3 text-slate-400">{t('onboarding.step5_desc')}</p>
            </div>

            {createdTunnel && (
              <Card className="border-slate-700 bg-slate-800/80 backdrop-blur">
                <CardContent className="pt-6">
                  <p className="text-sm text-slate-400">{t('onboarding.step5_tunnel_url')}</p>
                  <div className="mt-2 flex items-center justify-center gap-2">
                    <a
                      href={`https://${createdTunnel.domain}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="flex items-center gap-1 font-mono text-lg text-blue-400 hover:underline"
                    >
                      {createdTunnel.domain}
                      <ExternalLink className="h-4 w-4" />
                    </a>
                    <button
                      onClick={() => {
                        navigator.clipboard.writeText(`https://${createdTunnel.domain}`)
                        toast({ title: t('onboarding.step5_copy_url'), variant: 'success' })
                      }}
                      className="rounded p-1.5 text-slate-400 transition-colors hover:bg-slate-700 hover:text-white"
                    >
                      <Copy className="h-4 w-4" />
                    </button>
                  </div>
                </CardContent>
              </Card>
            )}

            <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
              <button
                onClick={handleFinish}
                className="flex items-center justify-center gap-2 rounded-lg bg-blue-600 px-4 py-3 font-medium text-white transition-colors hover:bg-blue-700"
              >
                {t('onboarding.step5_link_dashboard')}
                <ArrowRight className="h-4 w-4" />
              </button>
              <button
                onClick={async () => {
                  await handleFinish()
                  navigate('/tunnels')
                }}
                className="flex items-center justify-center gap-2 rounded-lg bg-slate-700 px-4 py-3 font-medium text-slate-200 transition-colors hover:bg-slate-600"
              >
                {t('onboarding.step5_link_tunnels')}
              </button>
              <a
                href="https://docs.omnitun.dev"
                target="_blank"
                rel="noopener noreferrer"
                onClick={handleFinish}
                className="flex items-center justify-center gap-2 rounded-lg bg-slate-700 px-4 py-3 font-medium text-slate-200 transition-colors hover:bg-slate-600"
              >
                <ExternalLink className="h-4 w-4" />
                {t('onboarding.step5_link_docs')}
              </a>
            </div>
          </div>
        )}

        {step < 5 && (
          <div className="mt-8 flex justify-between">
            <Button
              variant="outline"
              onClick={step === 1 ? handleSkip : prevStep}
              className="border-slate-600 text-slate-300 hover:bg-slate-800"
            >
              <ArrowLeft className="mr-2 h-4 w-4" />
              {step === 1 ? t('onboarding.skip') : t('onboarding.back')}
            </Button>
            {step === 1 && (
              <Button onClick={nextStep}>
                {t('onboarding.get_started')}
                <ArrowRight className="ml-2 h-4 w-4" />
              </Button>
            )}
            {step === 3 && (
              <Button onClick={nextStep}>
                {t('onboarding.next')}
                <ArrowRight className="ml-2 h-4 w-4" />
              </Button>
            )}
            {step === 4 && (
              <Button onClick={nextStep} disabled={!createdTunnel}>
                {t('onboarding.next')}
                <ArrowRight className="ml-2 h-4 w-4" />
              </Button>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
