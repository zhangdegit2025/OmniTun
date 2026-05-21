import { useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Badge } from '@/components/ui/Badge'
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from '@/components/ui/Card'
import { useAuth } from '@/hooks/useAuth'
import { Loader2 } from 'lucide-react'

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

export default function Login() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { login: loginAction, isLoading, error, clearError } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [mfaRequired, setMfaRequired] = useState(false)
  const [mfaCode, setMfaCode] = useState('')
  const [validation, setValidation] = useState<{ email?: string; password?: string; mfa?: string }>({})

  const validate = (): boolean => {
    const errs: { email?: string; password?: string; mfa?: string } = {}
    if (!mfaRequired) {
      if (!email.trim()) errs.email = t('auth.login.email_required')
      else if (!EMAIL_RE.test(email)) errs.email = t('auth.login.invalid_email')
      if (!password.trim()) errs.password = t('auth.login.password_required')
    } else {
      if (!mfaCode.trim() || mfaCode.length !== 6) errs.mfa = t('auth.login.mfa_code_required')
    }
    setValidation(errs)
    return Object.keys(errs).length === 0
  }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    clearError()
    if (!validate()) return

    try {
      if (mfaRequired) {
        await loginAction(email, password, mfaCode)
      } else {
        await loginAction(email, password)
      }
      navigate('/')
    } catch (err: unknown) {
      const code = err && typeof err === 'object' && 'code' in err ? (err as { code: string }).code : ''
      if (code === 'mfa_required') {
        setMfaRequired(true)
        clearError()
      }
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-slate-900 via-blue-950 to-slate-900 p-4">
      <Card className="w-full max-w-md border-slate-700 bg-slate-900/80 backdrop-blur">
        <CardHeader className="text-center">
          <CardTitle className="text-2xl text-white">{t('auth.login.title')}</CardTitle>
          <CardDescription className="text-slate-400">
            {mfaRequired ? t('auth.login.mfa_description') : t('auth.login.description')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            {!mfaRequired ? (
              <>
                <Input
                  label={t('auth.login.email')}
                  type="email"
                  placeholder={t('auth.login.email_placeholder')}
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
                  placeholder={t('auth.login.password_placeholder')}
                  value={password}
                  onChange={(e) => {
                    setPassword(e.target.value)
                    if (validation.password) setValidation((v) => ({ ...v, password: undefined }))
                  }}
                  autoComplete="current-password"
                  error={validation.password}
                />
              </>
            ) : (
              <>
                <Input
                  label={t('auth.login.mfa_code')}
                  type="text"
                  placeholder="000000"
                  value={mfaCode}
                  onChange={(e) => {
                    setMfaCode(e.target.value.replace(/[^0-9]/g, ''))
                    if (validation.mfa) setValidation((v) => ({ ...v, mfa: undefined }))
                  }}
                  autoComplete="one-time-code"
                  maxLength={6}
                  error={validation.mfa}
                />
                <button
                  type="button"
                  onClick={() => { setMfaRequired(false); setMfaCode('') }}
                  className="text-xs text-primary hover:underline"
                >
                  {t('auth.login.back_to_login')}
                </button>
              </>
            )}
            {error && (
              <Badge variant="destructive" className="w-full justify-center py-1.5">
                {error}
              </Badge>
            )}
            <Button type="submit" className="w-full" disabled={isLoading}>
              {isLoading ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" />
                  {t('auth.login.signing_in')}
                </>
              ) : (
                mfaRequired ? t('auth.login.verify_mfa') : t('auth.login.submit')
              )}
            </Button>
          </form>

          {!mfaRequired && (
            <>
              <div className="relative my-6">
                <div className="absolute inset-0 flex items-center">
                  <span className="w-full border-t border-slate-700" />
                </div>
                <div className="relative flex justify-center text-xs uppercase">
                  <span className="bg-slate-900 px-2 text-slate-400">
                    {t('auth.login.or_continue')}
                  </span>
                </div>
              </div>

              <div className="flex gap-3">
                <Button
                  variant="outline"
                  className="w-full border-slate-700 text-slate-300 hover:bg-slate-800"
                  type="button"
                  onClick={() => {
                    window.location.href = '/v1/auth/github'
                  }}
                >
                  <svg className="mr-2 h-4 w-4" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M12 0C5.37 0 0 5.37 0 12c0 5.3 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61-.546-1.385-1.335-1.755-1.335-1.755-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 21.795 24 17.295 24 12 24 5.37 18.63 0 12 0z" />
                  </svg>
                  {t('auth.login.github')}
                </Button>
                <Button
                  variant="outline"
                  className="w-full border-slate-700 text-slate-300 hover:bg-slate-800"
                  type="button"
                  onClick={() => {
                    window.location.href = '/v1/auth/google'
                  }}
                >
                  <svg className="mr-2 h-4 w-4" viewBox="0 0 24 24">
                    <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z" fill="#4285F4" />
                    <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853" />
                    <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05" />
                    <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335" />
                  </svg>
                  {t('auth.login.google')}
                </Button>
              </div>

              <div className="mt-3">
                <Button
                  variant="outline"
                  className="w-full border-slate-700 text-slate-300 hover:bg-slate-800"
                  type="button"
                  onClick={() => {
                    window.location.href = '/v1/auth/saml/login'
                  }}
                >
                  <svg className="mr-2 h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                    <path d="M9 12l2 2 4-4" /><path d="M5 12a7 7 0 0114 0" /><circle cx="12" cy="12" r="3" />
                  </svg>
                  {t('auth.login.saml')}
                </Button>
              </div>
              <Button
                variant="outline"
                className="mt-3 w-full border-slate-700 text-slate-300 hover:bg-slate-800"
                type="button"
                onClick={() => {
                  window.location.href = '/v1/auth/oidc/login'
                }}
              >
                <svg className="mr-2 h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M9 12h6" />
                  <path d="M12 9v6" />
                  <circle cx="12" cy="12" r="9" />
                </svg>
                {t('auth.login.oidc')}
              </Button>
            </>
          )}
        </CardContent>
        {!mfaRequired && (
          <CardFooter className="justify-center">
            <p className="text-sm text-slate-400">
              {t('auth.login.no_account')}{' '}
              <Link to="/register" className="font-medium text-primary hover:underline">
                {t('auth.login.register_link')}
              </Link>
            </p>
          </CardFooter>
        )}
      </Card>
    </div>
  )
}