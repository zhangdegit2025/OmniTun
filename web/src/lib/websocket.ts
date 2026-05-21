type MessageHandler = (data: unknown) => void

type ConnectionState = 'connecting' | 'connected' | 'disconnected' | 'reconnecting'

/**
 * WebSocket client for OmniTun real-time communication.
 *
 * Features:
 * - Automatic reconnection with exponential backoff
 * - Message routing by type field
 * - Connection state tracking
 *
 * @example
 * const ws = new OmniTunWS('ws://localhost:8080/ws', token)
 * ws.on('tunnel.status', (data) => console.log(data))
 */
export class OmniTunWS {
  private url: string
  private token: string
  private ws: WebSocket | null = null
  private handlers = new Map<string, Set<MessageHandler>>()
  private reconnectAttempts = 0
  private maxReconnectAttempts = 10
  private baseDelay = 1000
  private maxDelay = 30_000
  private timer: ReturnType<typeof setTimeout> | null = null
  private destroyed = false
  private _state: ConnectionState = 'disconnected'
  private stateListeners = new Set<(s: ConnectionState) => void>()

  constructor(url: string, token: string) {
    this.url = url
    this.token = token
    this.connect()
  }

  private setState(state: ConnectionState) {
    this._state = state
    this.stateListeners.forEach((fn) => fn(state))
  }

  get state(): ConnectionState {
    return this._state
  }

  /** Registers a listener for connection state changes. Returns an unsubscribe function. */
  onStateChange(fn: (s: ConnectionState) => void): () => void {
    this.stateListeners.add(fn)
    return () => this.stateListeners.delete(fn)
  }

  private connect() {
    if (this.destroyed) return
    this.setState('connecting')

    try {
      this.ws = new WebSocket(`${this.url}?token=${encodeURIComponent(this.token)}`)
    } catch {
      this.scheduleReconnect()
      return
    }

    this.ws.onopen = () => {
      this.reconnectAttempts = 0
      this.setState('connected')
    }

    this.ws.onmessage = (event: MessageEvent) => {
      try {
        const msg = JSON.parse(event.data as string)
        const type: string | undefined = msg.type
        if (type && this.handlers.has(type)) {
          this.handlers.get(type)!.forEach((fn) => fn(msg))
        }
        if (this.handlers.has('*')) {
          this.handlers.get('*')!.forEach((fn) => fn(msg))
        }
      } catch {
        // Ignore non-JSON messages
      }
    }

    this.ws.onclose = () => {
      if (!this.destroyed) {
        this.setState('disconnected')
        this.scheduleReconnect()
      }
    }

    this.ws.onerror = () => {
      // Connection errors are expected when backend is unavailable.
      // The onclose handler handles reconnection.
    }
  }

  private scheduleReconnect() {
    if (this.destroyed) return
    if (this.reconnectAttempts >= this.maxReconnectAttempts) return

    this.setState('reconnecting')
    const delay = Math.min(
      this.baseDelay * Math.pow(2, this.reconnectAttempts),
      this.maxDelay,
    )
    this.reconnectAttempts++

    this.timer = setTimeout(() => this.connect(), delay)
  }

  /**
   * Register a handler for messages with the given type.
   * Use `*` to match all message types.
   */
  on(channel: string, handler: MessageHandler): void {
    if (!this.handlers.has(channel)) {
      this.handlers.set(channel, new Set())
    }
    this.handlers.get(channel)!.add(handler)
  }

  /**
   * Remove a previously registered handler.
   */
  off(channel: string, handler: MessageHandler): void {
    this.handlers.get(channel)?.delete(handler)
  }

  /**
   * Send a JSON message through the WebSocket.
   */
  send(data: unknown): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(data))
    }
  }

  /**
   * Close the WebSocket connection permanently (no reconnection).
   */
  close(): void {
    this.destroyed = true
    if (this.timer) clearTimeout(this.timer)
    this.ws?.close()
    this.setState('disconnected')
  }
}
