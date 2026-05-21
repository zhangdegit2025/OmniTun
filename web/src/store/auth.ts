import { create } from 'zustand'
import type { User } from '@/lib/types'
import { apiRequest, setAuthToken, clearAuthToken } from '@/lib/api'
import { setTokens, clearTokens, isAuthenticated } from '@/lib/auth'

interface AuthState {
  user: User | null
  isLoggedIn: boolean
  isLoading: boolean
  error: string | null

  login: (email: string, password: string, mfaCode?: string) => Promise<void>
  register: (email: string, password: string, name: string) => Promise<void>
  logout: () => void
  fetchUser: () => Promise<void>
  clearError: () => void
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isLoggedIn: isAuthenticated(),
  isLoading: false,
  error: null,

  login: async (email, password, mfaCode) => {
    set({ isLoading: true, error: null })
    try {
      const body: Record<string, string> = { email, password }
      if (mfaCode) {
        body.mfa_code = mfaCode
      }
      const data = await apiRequest<{
        access_token: string
        refresh_token: string
        user: User
      }>('/v1/auth/login', {
        method: 'POST',
        body: JSON.stringify(body),
      })
      setTokens(data.access_token, data.refresh_token)
      setAuthToken(data.access_token)
      set({ user: data.user, isLoggedIn: true, isLoading: false })
    } catch (err: unknown) {
      const message =
        err && typeof err === 'object' && 'message' in err
          ? (err as { message: string }).message
          : 'Login failed'
      const code =
        err && typeof err === 'object' && 'code' in err
          ? (err as { code: string }).code
          : ''
      set({ error: message, isLoading: false })
      throw { message, code }
    }
  },

  register: async (email, password, name) => {
    set({ isLoading: true, error: null })
    try {
      const data = await apiRequest<{
        access_token: string
        refresh_token: string
        user: User
      }>('/v1/auth/register', {
        method: 'POST',
        body: JSON.stringify({ email, password, name }),
      })
      setTokens(data.access_token, data.refresh_token)
      setAuthToken(data.access_token)
      set({ user: data.user, isLoggedIn: true, isLoading: false })
    } catch (err: unknown) {
      const message =
        err && typeof err === 'object' && 'message' in err
          ? (err as { message: string }).message
          : 'Registration failed'
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
      const user = await apiRequest<User>('/v1/auth/me')
      set({ user, isLoggedIn: true, isLoading: false })
    } catch {
      clearTokens()
      clearAuthToken()
      set({ user: null, isLoggedIn: false, isLoading: false })
    }
  },

  clearError: () => set({ error: null }),
}))
