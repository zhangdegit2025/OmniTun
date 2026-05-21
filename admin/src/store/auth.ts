import { create } from 'zustand'
import type { AdminUser } from '@/lib/types'
import { apiRequest, setAuthToken, clearAuthToken } from '@/lib/api'
import { setTokens, clearTokens, isAuthenticated } from '@/lib/auth'

interface AuthState {
  user: AdminUser | null
  isLoggedIn: boolean
  isLoading: boolean
  error: string | null

  login: (email: string, password: string) => Promise<void>
  logout: () => void
  fetchUser: () => Promise<void>
  clearError: () => void
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isLoggedIn: isAuthenticated(),
  isLoading: false,
  error: null,

  login: async (email, password) => {
    set({ isLoading: true, error: null })
    try {
      const data = await apiRequest<{
        access_token: string
        refresh_token: string
        user: AdminUser
      }>('/api/admin/v1/auth/login', {
        method: 'POST',
        body: JSON.stringify({ email, password }),
      })
      setTokens(data.access_token, data.refresh_token)
      setAuthToken(data.access_token)
      set({ user: data.user, isLoggedIn: true, isLoading: false })
    } catch (err: unknown) {
      const message =
        err && typeof err === 'object' && 'message' in err
          ? (err as { message: string }).message
          : 'Login failed'
      set({ error: message, isLoading: false })
      throw err
    }
  },

  logout: () => {
    clearTokens()
    clearAuthToken()
    set({ user: null, isLoggedIn: false, error: null })
  },

  fetchUser: async () => {
    set({ isLoading: true })
    try {
      const user = await apiRequest<AdminUser>('/api/admin/v1/auth/me')
      set({ user, isLoggedIn: true, isLoading: false })
    } catch {
      clearTokens()
      clearAuthToken()
      set({ user: null, isLoggedIn: false, isLoading: false })
    }
  },

  clearError: () => set({ error: null }),
}))
