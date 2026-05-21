import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
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

  const validate = (): boolean => {
    const errs: { email?: string; password?: string } = {}
    if (!email.trim()) errs.email = 'Email is required'
    else if (!EMAIL_RE.test(email)) errs.email = 'Invalid email address'
    if (!password.trim()) errs.password = 'Password is required'
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
          <CardTitle className="text-2xl text-white">Admin Console</CardTitle>
          <CardDescription className="text-slate-400">
            Sign in to manage OmniTun
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <Input
              label="Email"
              type="email"
              placeholder="admin@omnitun.io"
              value={email}
              onChange={(e) => {
                setEmail(e.target.value)
                if (validation.email) setValidation((v) => ({ ...v, email: undefined }))
              }}
              autoComplete="email"
              error={validation.email}
            />
            <Input
              label="Password"
              type="password"
              placeholder="Enter your password"
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
                  Signing in...
                </>
              ) : (
                'Sign In'
              )}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
