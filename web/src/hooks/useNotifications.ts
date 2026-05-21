import { useState, useCallback, useEffect } from 'react'

export interface Notification {
  id: string
  type: string
  category: 'tunnels' | 'billing' | 'system'
  title: string
  description: string
  read: boolean
  severity: 'info' | 'warning' | 'error' | 'success'
  created_at: string
  link?: string
}

function generateMockNotifications(): Notification[] {
  const now = Date.now()
  const items: Notification[] = [
    {
      id: 'n1',
      type: 'tunnel_created',
      category: 'tunnels',
      title: 'Tunnel "api-gateway" created',
      description: 'A new HTTP tunnel was created on port 8080.',
      read: false,
      severity: 'success',
      created_at: new Date(now - 2 * 60 * 1000).toISOString(),
      link: '/tunnels',
    },
    {
      id: 'n2',
      type: 'tunnel_error',
      category: 'tunnels',
      title: 'Tunnel "db-proxy" connection error',
      description: 'The tunnel failed to establish a connection to the remote server.',
      read: false,
      severity: 'error',
      created_at: new Date(now - 15 * 60 * 1000).toISOString(),
      link: '/tunnels',
    },
    {
      id: 'n3',
      type: 'domain_verified',
      category: 'tunnels',
      title: 'Domain verified: app.example.com',
      description: 'Your custom domain has been verified and is now active.',
      read: false,
      severity: 'success',
      created_at: new Date(now - 60 * 60 * 1000).toISOString(),
      link: '/domains',
    },
    {
      id: 'n4',
      type: 'billing_invoice',
      category: 'billing',
      title: 'Invoice #INV-2026-05 generated',
      description: 'Your monthly invoice for May 2026 is now available.',
      read: true,
      severity: 'info',
      created_at: new Date(now - 2 * 60 * 60 * 1000).toISOString(),
      link: '/billing',
    },
    {
      id: 'n5',
      type: 'billing_limit',
      category: 'billing',
      title: 'Bandwidth usage at 80%',
      description: 'You have used 8 GB of your 10 GB monthly bandwidth limit.',
      read: true,
      severity: 'warning',
      created_at: new Date(now - 5 * 60 * 60 * 1000).toISOString(),
      link: '/billing',
    },
    {
      id: 'n6',
      type: 'system_update',
      category: 'system',
      title: 'OmniTun v2.4.0 released',
      description: 'New features include enhanced mesh networking and improved performance.',
      read: true,
      severity: 'info',
      created_at: new Date(now - 24 * 60 * 60 * 1000).toISOString(),
    },
    {
      id: 'n7',
      type: 'system_maintenance',
      category: 'system',
      title: 'Scheduled maintenance on June 1',
      description: 'We will be performing scheduled maintenance. Expect up to 5 minutes of downtime.',
      read: true,
      severity: 'warning',
      created_at: new Date(now - 2 * 24 * 60 * 60 * 1000).toISOString(),
    },
    {
      id: 'n8',
      type: 'tunnel_started',
      category: 'tunnels',
      title: 'Tunnel "web-app" started',
      description: 'The tunnel has been started and is now accepting connections.',
      read: true,
      severity: 'success',
      created_at: new Date(now - 3 * 24 * 60 * 60 * 1000).toISOString(),
      link: '/tunnels',
    },
  ]
  return items
}

let mockStore = generateMockNotifications()

export function useNotifications() {
  const [notifications, setNotifications] = useState<Notification[]>(mockStore)
  const [isLoading, setIsLoading] = useState(false)

  useEffect(() => {
    const interval = setInterval(() => {
      mockStore = generateMockNotifications()
      setNotifications([...mockStore])
    }, 30_000)
    return () => clearInterval(interval)
  }, [])

  const markAllRead = useCallback(() => {
    mockStore = mockStore.map((n) => ({ ...n, read: true }))
    setNotifications([...mockStore])
  }, [])

  const markOneRead = useCallback((id: string) => {
    mockStore = mockStore.map((n) => (n.id === id ? { ...n, read: true } : n))
    setNotifications([...mockStore])
  }, [])

  const refetch = useCallback(() => {
    setIsLoading(true)
    mockStore = generateMockNotifications()
    setNotifications([...mockStore])
    setIsLoading(false)
  }, [])

  const unreadCount = notifications.filter((n) => !n.read).length

  return {
    notifications,
    isLoading,
    unreadCount,
    markAllRead,
    markOneRead,
    refetch,
  }
}
