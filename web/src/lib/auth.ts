export interface JwtPayload {
  sub: string
  org_id: string
  role: string
  exp: number
}

/**
 * Retrieves the access token from localStorage.
 */
export function getAccessToken(): string | null {
  return localStorage.getItem('access_token')
}

/**
 * Retrieves the refresh token from localStorage.
 */
export function getRefreshToken(): string | null {
  return localStorage.getItem('refresh_token')
}

/**
 * Stores both access and refresh tokens in localStorage.
 */
export function setTokens(access: string, refresh: string): void {
  localStorage.setItem('access_token', access)
  localStorage.setItem('refresh_token', refresh)
}

/**
 * Removes all tokens from localStorage.
 */
export function clearTokens(): void {
  localStorage.removeItem('access_token')
  localStorage.removeItem('refresh_token')
}

/**
 * Returns true if an access token is present and not expired.
 */
export function isAuthenticated(): boolean {
  const token = getAccessToken()
  if (!token) return false
  const payload = parseJWT(token)
  if (!payload) return false
  return payload.exp * 1000 > Date.now()
}

/**
 * Parses a JWT and returns its payload. Returns null if the token is malformed.
 */
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
      org_id: payload.org_id ?? '',
      role: payload.role ?? 'member',
      exp: payload.exp,
    }
  } catch {
    return null
  }
}
