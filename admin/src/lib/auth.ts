export interface JwtPayload {
  sub: string
  role: string
  exp: number
}

export function getAccessToken(): string | null {
  return localStorage.getItem('admin_access_token')
}

export function getRefreshToken(): string | null {
  return localStorage.getItem('admin_refresh_token')
}

export function setTokens(access: string, refresh: string): void {
  localStorage.setItem('admin_access_token', access)
  localStorage.setItem('admin_refresh_token', refresh)
}

export function clearTokens(): void {
  localStorage.removeItem('admin_access_token')
  localStorage.removeItem('admin_refresh_token')
}

export function isAuthenticated(): boolean {
  const token = getAccessToken()
  if (!token) return false
  const payload = parseJWT(token)
  if (!payload) return false
  return payload.exp * 1000 > Date.now()
}

export function parseJWT(token: string): JwtPayload | null {
  try {
    const parts = token.split('.')
    if (parts.length !== 3) return null
    const payload = JSON.parse(atob(parts[1]))
    if (
      typeof payload.sub !== 'string' ||
      typeof payload.exp !== 'number'
    ) {
      return null
    }
    return {
      sub: payload.sub,
      role: payload.role ?? 'operator',
      exp: payload.exp,
    }
  } catch {
    return null
  }
}
