import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Badge } from '@/components/ui/Badge'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/Card'
import { useAuth } from '@/hooks/useAuth'
import { Loader2, Shield } from 'lucide-react'

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

export default function Login() {
  const navigate = useNavigate()
  const { login, isLoading, error, clearError } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [validation, setValidation] = useState<{ email?: string; password?: string }>({})
  const { t } = useTranslation()

  const validate = (): boolean => {
    const errs: { email?: string; password?: string } = {}
    if (!email.trim()) errs.email = t('auth.login.emailRequired')
    else if (!EMAIL_RE.test(email)) errs.email = t('auth.login.invalidEmail')
    if (!password.trim()) errs.password = t('auth.login.passwordRequired')
    setValidation(errs)
    return Object.keys(errs).length === 0
  }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    clearError()
    if (!validate()) return

    try {
      await login(email, password)
      navigate('/')
    } catch {
      // error is set in the store
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-slate-950 via-slate-900 to-slate-950 p-4">
      <Card className="w-full max-w-md border-slate-700 bg-slate-900/80 backdrop-blur">
        <CardHeader className="text-center">
          <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-primary/20">
            <Shield className="h-6 w-6 text-primary" />
          </div>
          <CardTitle className="text-2xl text-white">{t('auth.login.title')}</CardTitle>
          <CardDescription className="text-slate-400">
            {t('auth.login.subtitle')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <Input
              label={t('auth.login.email')}
              type="email"
              placeholder={t('auth.login.emailPlaceholder')}
              value={email}
              onChange={(e) => {
                setEmail(e.target.value)
                if (validation.email) setValidation((v) => ({ ...v, email: undefined }))
              }}
              autoComplete="email"
              error={validation.email}
            />
            <Input
              label={t('auth.login.password')}
              type="password"
              placeholder={t('auth.login.passwordPlaceholder')}
              value={password}
              onChange={(e) => {
                setPassword(e.target.value)
                if (validation.password) setValidation((v) => ({ ...v, password: undefined }))
              }}
              autoComplete="current-password"
              error={validation.password}
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
                  {t('auth.login.signingIn')}
                </>
              ) : (
                t('auth.login.signIn')
              )}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
