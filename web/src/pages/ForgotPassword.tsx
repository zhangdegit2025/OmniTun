import { useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Badge } from '@/components/ui/Badge'
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from '@/components/ui/Card'
import { Loader2, CheckCircle } from 'lucide-react'

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

export default function ForgotPassword() {
  const { t } = useTranslation()
  const [email, setEmail] = useState('')
  const [validation, setValidation] = useState<{ email?: string }>({})
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)

  const validate = (): boolean => {
    const errs: { email?: string } = {}
    if (!email.trim()) errs.email = t('auth.forgot.email_required')
    else if (!EMAIL_RE.test(email)) errs.email = t('auth.forgot.invalid_email')
    setValidation(errs)
    return Object.keys(errs).length === 0
  }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    if (!validate()) return

    setIsLoading(true)
    try {
      const res = await fetch('/v1/auth/password/forgot', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email }),
      })
      if (!res.ok) {
        const body = await res.json().catch(() => ({}))
        throw new Error(body?.error?.message || body?.message || 'Request failed')
      }
      setSuccess(true)
    } catch (err: unknown) {
      const message =
        err && typeof err === 'object' && 'message' in err
          ? (err as { message: string }).message
          : 'Something went wrong'
      setError(message)
    } finally {
      setIsLoading(false)
    }
  }

  if (success) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-slate-900 via-blue-950 to-slate-900 p-4">
        <Card className="w-full max-w-md border-slate-700 bg-slate-900/80 backdrop-blur">
          <CardContent className="flex flex-col items-center gap-4 py-8">
            <CheckCircle className="h-12 w-12 text-emerald-500" />
            <p className="text-center text-slate-300">{t('auth.forgot.success')}</p>
            <Link
              to="/login"
              className="text-sm font-medium text-primary hover:underline"
            >
              {t('auth.forgot.back')}
            </Link>
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-slate-900 via-blue-950 to-slate-900 p-4">
      <Card className="w-full max-w-md border-slate-700 bg-slate-900/80 backdrop-blur">
        <CardHeader className="text-center">
          <CardTitle className="text-2xl text-white">{t('auth.forgot.title')}</CardTitle>
          <CardDescription className="text-slate-400">
            {t('auth.forgot.description')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <Input
              label={t('auth.forgot.email')}
              type="email"
              placeholder={t('auth.forgot.email_placeholder')}
              value={email}
              onChange={(e) => {
                setEmail(e.target.value)
                if (validation.email) setValidation((v) => ({ ...v, email: undefined }))
              }}
              autoComplete="email"
              error={validation.email}
            />
            {error && (
              <Badge variant="destructive" className="w-full justify-center py-1.5">
                {error}
              </Badge>
            )}
            <Button type="submit" className="w-full" disabled={isLoading}>
              {isLoading ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" />
                  {t('auth.forgot.sending')}
                </>
              ) : (
                t('auth.forgot.submit')
              )}
            </Button>
          </form>
        </CardContent>
        <CardFooter className="justify-center">
          <Link to="/login" className="text-sm text-muted-foreground hover:underline">
            {t('auth.forgot.back')}
          </Link>
        </CardFooter>
      </Card>
    </div>
  )
}
