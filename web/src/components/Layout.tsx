import { useState, type ReactNode } from 'react'
import { NavLink, useLocation, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import i18n from '@/i18n'
import { useAuth } from '@/hooks/useAuth'
import { useNotifications } from '@/hooks/useNotifications'
import { Button } from '@/components/ui/Button'
import { ThemeToggle } from '@/components/ThemeToggle'
import { prefetchPage } from '@/lib/prefetch'
import {
  LayoutDashboard,
  Server,
  Settings,
  Menu,
  X,
  LogOut,
  User,
  Globe,
  Share2,
  CreditCard,
  Bell,
  BookOpen,
  Download,
} from 'lucide-react'
import { cn } from '@/lib/utils'

export default function Layout({ children }: { children: ReactNode }) {
  const { t } = useTranslation()
  const { user, logout } = useAuth()
  const location = useLocation()
  const navigate = useNavigate()
  const { unreadCount } = useNotifications()
  const [sidebarOpen, setSidebarOpen] = useState(false)

  const navItems = [
    { to: '/', label: t('nav.dashboard'), icon: LayoutDashboard, prefetch: 'dashboard' as const },
    { to: '/tunnels', label: t('nav.tunnels'), icon: Server, prefetch: 'tunnels' as const },
    { to: '/domains', label: t('nav.domains'), icon: Globe, prefetch: 'domains' as const },
    { to: '/networks', label: t('nav.networks'), icon: Share2, prefetch: 'networks' as const },
    { to: '/billing', label: t('nav.billing'), icon: CreditCard, prefetch: 'billing' as const },
    { to: '/settings', label: t('nav.settings'), icon: Settings, prefetch: 'settings' as const },
    { to: '/notifications', label: t('nav.notifications'), icon: Bell, prefetch: 'notifications' as const },
    { to: '/docs/api', label: t('nav.api_docs'), icon: BookOpen, prefetch: 'settings' as const },
    { to: '/downloads', label: t('nav.downloads'), icon: Download, prefetch: 'settings' as const },
  ]

  return (
    <div className="flex min-h-screen bg-background">
      {/* Mobile overlay */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/50 lg:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      {/* Sidebar */}
      <aside
        className={cn(
          'fixed inset-y-0 left-0 z-50 flex w-64 flex-col border-r bg-card transition-transform lg:static lg:translate-x-0',
          sidebarOpen ? 'translate-x-0' : '-translate-x-full',
        )}
      >
        <div className="flex h-14 items-center gap-2 border-b px-4">
          <Server className="h-6 w-6 text-primary" />
          <span className="text-lg font-bold">{t('app.title')}</span>
        </div>

        <nav className="flex-1 space-y-1 p-3">
          {navItems.map((item) => {
            const Icon = item.icon
            const isActive =
              item.to === '/'
                ? location.pathname === '/'
                : location.pathname.startsWith(item.to)
            return (
              <NavLink
                key={item.to}
                to={item.to}
                onClick={() => setSidebarOpen(false)}
                onMouseEnter={() => prefetchPage(item.prefetch)}
                className={cn(
                  'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors',
                  isActive
                    ? 'bg-primary text-primary-foreground'
                    : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground',
                )}
              >
                <Icon className="h-4 w-4" />
                {item.label}
              </NavLink>
            )
          })}
        </nav>

        <div className="border-t p-3">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <User className="h-4 w-4" />
            <span className="truncate">{user?.name ?? user?.email}</span>
          </div>
        </div>
      </aside>

      {/* Main area */}
      <div className="flex flex-1 flex-col">
        {/* Top header */}
        <header className="sticky top-0 z-30 flex h-14 items-center gap-4 border-b bg-background px-4">
          <Button
            variant="ghost"
            size="sm"
            className="lg:hidden"
            onClick={() => setSidebarOpen(true)}
          >
            <Menu className="h-5 w-5" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className={cn('lg:hidden', !sidebarOpen && 'hidden')}
            onClick={() => setSidebarOpen(false)}
          >
            <X className="h-5 w-5" />
          </Button>

          <div className="flex-1" />

          <span className="hidden text-sm text-muted-foreground sm:inline">
            {user?.name ?? user?.email}
          </span>
          <Button
            variant="ghost"
            size="sm"
            className="relative"
            onClick={() => navigate('/notifications')}
            title={t('nav.notifications')}
          >
            <Bell className="h-4 w-4" />
            {unreadCount > 0 && (
              <span className="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-0.5 text-[10px] font-bold text-destructive-foreground">
                {unreadCount > 9 ? '9+' : unreadCount}
              </span>
            )}
          </Button>
          <ThemeToggle />
          <Button
            variant="ghost"
            size="sm"
            onClick={() => i18n.changeLanguage(i18n.language === 'en' ? 'zh' : 'en')}
          >
            <Globe className="mr-1 h-4 w-4" />
            {i18n.language === 'en' ? '中文' : 'EN'}
          </Button>
          <Button variant="outline" size="sm" onClick={logout}>
            <LogOut className="mr-1 h-4 w-4" />
            {t('nav.sign_out')}
          </Button>
        </header>

        {/* Page content */}
        <main className="flex-1 overflow-auto">{children}</main>
      </div>
    </div>
  )
}
