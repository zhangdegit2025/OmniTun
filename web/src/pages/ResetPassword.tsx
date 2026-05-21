import { useState, type FormEvent, useMemo } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Badge } from '@/components/ui/Badge'
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from '@/components/ui/Card'
import { Loader2, AlertTriangle, CheckCircle } from 'lucide-react'

type Strength = 'weak' | 'medium' | 'strong'

function getStrength(pw: string): Strength {
  let score = 0
  if (pw.length >= 8) score++
  if (/[A-Z]/.test(pw) && /[a-z]/.test(pw)) score++
  if (/\d/.test(pw)) score++
  if (/[^A-Za-z0-9]/.test(pw)) score++
  if (score <= 1) return 'weak'
  if (score <= 2) return 'medium'
  return 'strong'
}

const strengthColors: Record<Strength, string> = {
  weak: 'bg-red-500',
  medium: 'bg-amber-500',
  strong: 'bg-emerald-500',
}

export default function ResetPassword() {
  const { t } = useTranslation()
  const [searchParams] = useSearchParams()
  const token = searchParams.get('token')

  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [validation, setValidation] = useState<{
    password?: string
    confirm?: string
  }>({})
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)

  const strength = useMemo(() => getStrength(newPassword), [newPassword])

  const strengthLabels: Record<Strength, string> = {
    weak: t('auth.reset.password_strength.weak'),
    medium: t('auth.reset.password_strength.medium'),
    strong: t('auth.reset.password_strength.strong'),
  }

  if (!token) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-slate-900 via-blue-950 to-slate-900 p-4">
        <Card className="w-full max-w-md border-slate-700 bg-slate-900/80 backdrop-blur">
          <CardContent className="flex flex-col items-center gap-4 py-8">
            <AlertTriangle className="h-12 w-12 text-amber-500" />
            <p className="text-center text-slate-300">{t('auth.reset.token_missing')}</p>
            <Link
              to="/login"
              className="text-sm font-medium text-primary hover:underline"
            >
              {t('auth.reset.back')}
            </Link>
          </CardContent>
        </Card>
      </div>
    )
  }

  const validate = (): boolean => {
    const errs: typeof validation = {}
    if (!newPassword.trim()) errs.password = t('auth.login.password_required')
    else if (newPassword.length < 8) errs.password = t('auth.reset.password_rule')
    if (newPassword !== confirmPassword) errs.confirm = t('auth.reset.password_mismatch')
    setValidation(errs)
    return Object.keys(errs).length === 0
  }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    if (!validate()) return

    setIsLoading(true)
    try {
      const res = await fetch('/v1/auth/password/reset', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token, new_password: newPassword }),
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
            <p className="text-center text-slate-300">{t('auth.reset.success')}</p>
            <Link
              to="/login"
              className="text-sm font-medium text-primary hover:underline"
            >
              {t('auth.reset.back')}
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
          <CardTitle className="text-2xl text-white">{t('auth.reset.title')}</CardTitle>
          <CardDescription className="text-slate-400">
            {t('auth.reset.description')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <div>
              <Input
                label={t('auth.reset.new_password')}
                type="password"
                placeholder={t('auth.reset.password_min')}
                value={newPassword}
                onChange={(e) => {
                  setNewPassword(e.target.value)
                  if (validation.password) setValidation((v) => ({ ...v, password: undefined }))
                }}
                autoComplete="new-password"
                error={validation.password}
              />
              {newPassword.length > 0 && (
                <div className="mt-2 space-y-1">
                  <div className="flex h-1.5 gap-1">
                    <div className={`flex-1 rounded-full transition-colors ${strengthColors[strength]}`} />
                    <div className={`flex-1 rounded-full transition-colors ${strength !== 'weak' ? strengthColors[strength] : 'bg-muted'}`} />
                    <div className={`flex-1 rounded-full transition-colors ${strength === 'strong' ? strengthColors[strength] : 'bg-muted'}`} />
                  </div>
                  <p className="text-xs text-muted-foreground">
                    {t('auth.reset.password_strength.label')}{' '}
                    <span className="font-medium">{strengthLabels[strength]}</span>
                  </p>
                </div>
              )}
            </div>
            <Input
              label={t('auth.reset.confirm_password')}
              type="password"
              placeholder={t('auth.reset.confirm_password_placeholder')}
              value={confirmPassword}
              onChange={(e) => {
                setConfirmPassword(e.target.value)
                if (validation.confirm) setValidation((v) => ({ ...v, confirm: undefined }))
              }}
              autoComplete="new-password"
              error={validation.confirm}
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
                  {t('auth.reset.resetting')}
                </>
              ) : (
                t('auth.reset.submit')
              )}
            </Button>
          </form>
        </CardContent>
        <CardFooter className="justify-center">
          <Link to="/login" className="text-sm text-muted-foreground hover:underline">
            {t('auth.reset.back')}
          </Link>
        </CardFooter>
      </Card>
    </div>
  )
}
