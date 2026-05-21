import { useCallback } from 'react'
import { useAuthStore } from '@/store/auth'

export function useAuth() {
  const login = useAuthStore((s) => s.login)
  const logout = useAuthStore((s) => s.logout)
  const user = useAuthStore((s) => s.user)
  const isLoggedIn = useAuthStore((s) => s.isLoggedIn)
  const isLoading = useAuthStore((s) => s.isLoading)
  const error = useAuthStore((s) => s.error)
  const clearError = useAuthStore((s) => s.clearError)
  const fetchUser = useAuthStore((s) => s.fetchUser)

  const handleLogin = useCallback(
    async (email: string, password: string) => {
      await login(email, password)
    },
    [login],
  )

  const handleLogout = useCallback(() => {
    logout()
  }, [logout])

  return {
    user,
    isLoggedIn,
    isLoading,
    error,
    login: handleLogin,
    logout: handleLogout,
    clearError,
    fetchUser,
  }
}
