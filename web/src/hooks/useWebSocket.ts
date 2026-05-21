import { useEffect, useRef, useState, useCallback } from 'react'
import { OmniTunWS } from '@/lib/websocket'
import { getAccessToken } from '@/lib/auth'

/**
 * React hook for WebSocket communication.
 * Automatically connects, reconnects, and cleans up on unmount.
 *
 * @example
 * const { state, send } = useWebSocket('ws://localhost:8080/ws', {
 *   'tunnel.status': (data) => console.log('Status:', data),
 * })
 */
export function useWebSocket(
  url: string,
  handlers: Record<string, (data: unknown) => void> = {},
) {
  const wsRef = useRef<OmniTunWS | null>(null)
  const [connectionState, setConnectionState] = useState<
    'connecting' | 'connected' | 'disconnected' | 'reconnecting'
  >('disconnected')

  useEffect(() => {
    const token = getAccessToken()
    if (!token) return

    let ws: OmniTunWS | null = null
    // Delay connection to avoid React StrictMode double-render issues
    const timer = setTimeout(() => {
      ws = new OmniTunWS(url, token)
      wsRef.current = ws
      ws.onStateChange((state) => setConnectionState(state))
      Object.entries(handlers).forEach(([channel, handler]) => {
        ws!.on(channel, handler)
      })
    }, 500)

    return () => {
      clearTimeout(timer)
      if (ws) ws.close()
      wsRef.current = null
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [url])

  const send = useCallback((data: unknown) => {
    wsRef.current?.send(data)
  }, [])

  return {
    connectionState,
    send,
  }
}
