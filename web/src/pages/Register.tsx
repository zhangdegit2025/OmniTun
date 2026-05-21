import { useState, type FormEvent, useMemo } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Badge } from '@/components/ui/Badge'
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from '@/components/ui/Card'
import { useAuth } from '@/hooks/useAuth'
import { useToast } from '@/components/ui/useToast'
import { Loader2 } from 'lucide-react'

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

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

export default function Register() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { toast } = useToast()
  const { register: registerAction, isLoading, error, clearError } = useAuth()
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [validation, setValidation] = useState<{
    name?: string
    email?: string
    password?: string
    confirm?: string
  }>({})

  const strength = useMemo(() => getStrength(password), [password])

  const strengthLabels: Record<Strength, string> = {
    weak: t('auth.register.password_strength.weak'),
    medium: t('auth.register.password_strength.medium'),
    strong: t('auth.register.password_strength.strong'),
  }

  const validate = (): boolean => {
    const errs: typeof validation = {}
    if (!name.trim()) errs.name = t('auth.register.name_required')
    if (!email.trim()) errs.email = t('auth.register.email_required')
    else if (!EMAIL_RE.test(email)) errs.email = t('auth.register.invalid_email')
    if (!password.trim()) errs.password = t('auth.login.password_required')
    else if (password.length < 8) errs.password = t('auth.register.password_rule')
    if (password !== confirmPassword) errs.confirm = t('auth.register.password_mismatch')
    setValidation(errs)
    return Object.keys(errs).length === 0
  }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    clearError()
    if (!validate()) return

    try {
      await registerAction(email, password, name)
      toast({ title: t('auth.register.created'), variant: 'success' })
      navigate('/onboarding')
    } catch {
      // Error handled by store
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-slate-900 via-blue-950 to-slate-900 p-4">
      <Card className="w-full max-w-md border-slate-700 bg-slate-900/80 backdrop-blur">
        <CardHeader className="text-center">
          <CardTitle className="text-2xl text-white">{t('auth.register.title')}</CardTitle>
          <CardDescription className="text-slate-400">
            {t('auth.register.description')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <Input
              label={t('auth.register.name')}
              type="text"
              placeholder={t('auth.register.name_placeholder')}
              value={name}
              onChange={(e) => {
                setName(e.target.value)
                if (validation.name) setValidation((v) => ({ ...v, name: undefined }))
              }}
              autoComplete="name"
              error={validation.name}
            />
            <Input
              label={t('auth.register.email')}
              type="email"
              placeholder={t('auth.register.email_placeholder')}
              value={email}
              onChange={(e) => {
                setEmail(e.target.value)
                if (validation.email) setValidation((v) => ({ ...v, email: undefined }))
              }}
              autoComplete="email"
              error={validation.email}
            />
            <div>
              <Input
                label={t('auth.register.password')}
                type="password"
                placeholder={t('auth.register.password_min')}
                value={password}
                onChange={(e) => {
                  setPassword(e.target.value)
                  if (validation.password) setValidation((v) => ({ ...v, password: undefined }))
                }}
                autoComplete="new-password"
                error={validation.password}
              />
              {password.length > 0 && (
                <div className="mt-2 space-y-1">
                  <div className="flex h-1.5 gap-1">
                    <div className={`flex-1 rounded-full transition-colors ${strengthColors[strength]}`} />
                    <div className={`flex-1 rounded-full transition-colors ${strength !== 'weak' ? strengthColors[strength] : 'bg-muted'}`} />
                    <div className={`flex-1 rounded-full transition-colors ${strength === 'strong' ? strengthColors[strength] : 'bg-muted'}`} />
                  </div>
                  <p className="text-xs text-muted-foreground">
                    {t('auth.register.password_strength.label')}{' '}
                    <span className="font-medium">{strengthLabels[strength]}</span>
                  </p>
                </div>
              )}
            </div>
            <Input
              label={t('auth.register.confirm_password')}
              type="password"
              placeholder={t('auth.register.confirm_password_placeholder')}
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
                  {t('auth.register.creating')}
                </>
              ) : (
                t('auth.register.submit')
              )}
            </Button>
          </form>
        </CardContent>
        <CardFooter className="justify-center">
          <p className="text-sm text-slate-400">
            {t('auth.register.has_account')}{' '}
            <Link to="/login" className="font-medium text-primary hover:underline">
              {t('auth.register.sign_in_link')}
            </Link>
          </p>
        </CardFooter>
      </Card>
    </div>
  )
}
