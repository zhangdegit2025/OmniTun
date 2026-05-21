import { useCallback } from 'react'
import { useAuthStore } from '@/store/auth'

/**
 * Hook providing login, register, and logout actions with loading/error state.
 * Uses the Zustand auth store underneath.
 */
export function useAuth() {
  const login = useAuthStore((s) => s.login)
  const register = useAuthStore((s) => s.register)
  const logout = useAuthStore((s) => s.logout)
  const user = useAuthStore((s) => s.user)
  const isLoggedIn = useAuthStore((s) => s.isLoggedIn)
  const isLoading = useAuthStore((s) => s.isLoading)
  const error = useAuthStore((s) => s.error)
  const clearError = useAuthStore((s) => s.clearError)
  const fetchUser = useAuthStore((s) => s.fetchUser)

  const handleLogin = useCallback(
    async (email: string, password: string, mfaCode?: string) => {
      await login(email, password, mfaCode)
    },
    [login],
  )

  const handleRegister = useCallback(
    async (email: string, password: string, name: string) => {
      await register(email, password, name)
    },
    [register],
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
    register: handleRegister,
    logout: handleLogout,
    clearError,
    fetchUser,
  }
}
