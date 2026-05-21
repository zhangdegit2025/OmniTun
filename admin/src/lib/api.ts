export interface ApiError {
  code: string
  message: string
  details?: Record<string, unknown>
}

function readToken(): string | null {
  return localStorage.getItem('admin_access_token')
}

function writeToken(token: string) {
  localStorage.setItem('admin_access_token', token)
}

function removeToken() {
  localStorage.removeItem('admin_access_token')
}

class ApiClient {
  private token: string | null = null
  private refreshPromise: Promise<string | null> | null = null
  private baseUrl: string

  constructor() {
    this.token = readToken()
    this.baseUrl = import.meta.env.PROD
      ? (import.meta.env.VITE_ADMIN_API_URL ?? '')
      : ''
  }

  setToken(token: string) {
    this.token = token
    writeToken(token)
  }

  clearToken() {
    this.token = null
    removeToken()
  }

  private async refreshToken(): Promise<string | null> {
    if (this.refreshPromise) return this.refreshPromise
    this.refreshPromise = this._doRefresh()
    const result = await this.refreshPromise
    this.refreshPromise = null
    return result
  }

  private async _doRefresh(): Promise<string | null> {
    try {
      const refresh = localStorage.getItem('admin_refresh_token')
      if (!refresh) return null
      const url = this.buildUrl('/api/admin/v1/auth/refresh')
      const res = await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_token: refresh }),
      })
      if (!res.ok) {
        this.clearToken()
        return null
      }
      const data = await res.json()
      if (data.access_token) {
        this.setToken(data.access_token)
      }
      if (data.refresh_token) {
        localStorage.setItem('admin_refresh_token', data.refresh_token)
      }
      return data.access_token ?? null
    } catch {
      this.clearToken()
      return null
    }
  }

  private buildUrl(path: string): string {
    if (this.baseUrl) return `${this.baseUrl}${path}`
    return path
  }

  async request<T>(path: string, options: RequestInit = {}): Promise<T> {
    if (!this.token) this.token = readToken()

    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...((options.headers as Record<string, string>) || {}),
    }

    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`
    }

    let url = this.buildUrl(path)
    let res = await fetch(url, { ...options, headers })

    if (res.status === 401 && this.token) {
      const newToken = await this.refreshToken()
      if (newToken) {
        headers['Authorization'] = `Bearer ${newToken}`
        url = this.buildUrl(path)
        res = await fetch(url, { ...options, headers })
      }
    }

    if (!res.ok) {
      let error: ApiError
      try {
        const body = await res.json()
        const errObj = body.error || body
        error = {
          code: errObj.code ?? 'UNKNOWN',
          message: errObj.message ?? res.statusText,
          details: body.details,
        }
      } catch {
        error = {
          code: 'HTTP_ERROR',
          message: res.statusText,
        }
      }
      throw error
    }

    if (res.status === 204) return undefined as T

    return res.json()
  }
}

const client = new ApiClient()

export async function apiRequest<T>(
  path: string,
  options?: RequestInit,
): Promise<T> {
  return client.request<T>(path, options)
}

export function setAuthToken(token: string): void {
  client.setToken(token)
}

export function clearAuthToken(): void {
  client.clearToken()
}
