import { useState } from 'react'
import { NavLink, Outlet, useLocation } from 'react-router-dom'
import { useAuth } from '@/hooks/useAuth'
import { Button } from '@/components/ui/Button'
import { useTranslation } from 'react-i18next'
import {
  LayoutDashboard,
  Building2,
  Users,
  Server,
  Menu,
  LogOut,
  Shield,
  FileText,
  Megaphone,
  Key,
  Receipt,
  DollarSign,
  Percent,
  Globe,
  Trash2,
  BarChart3,
  ScrollText,
} from 'lucide-react'
import { cn } from '@/lib/utils'

export default function AdminLayout() {
  const { user, logout } = useAuth()
  const location = useLocation()
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const { t } = useTranslation()

  const navItems = [
    { to: '/', label: t('nav.dashboard'), icon: LayoutDashboard },
    { to: '/organizations', label: t('nav.organizations'), icon: Building2 },
    { to: '/users', label: t('nav.users'), icon: Users },
    { to: '/relays', label: t('nav.relayNodes'), icon: Server },
    { to: '/audit-logs', label: t('nav.auditLogs'), icon: FileText },
    { to: '/announcements', label: t('nav.announcements'), icon: Megaphone },
    { to: '/certificates', label: t('nav.certificates'), icon: Key },
    { to: '/revenue', label: t('nav.revenue'), icon: DollarSign },
    { to: '/invoices', label: t('nav.invoices'), icon: Receipt },
    { to: '/pricing', label: t('nav.pricing'), icon: DollarSign },
    { to: '/discounts', label: t('nav.discounts'), icon: Percent },
    { to: '/ip-whitelist', label: t('nav.ipWhitelist'), icon: Globe },
    { to: '/data-retention', label: t('nav.dataRetention'), icon: Trash2 },
    { to: '/sla', label: t('nav.slaDashboard'), icon: BarChart3 },
    { to: '/audit-reports', label: t('nav.auditReports'), icon: ScrollText },
    { to: '/roles', label: t('nav.roles'), icon: Shield },
  ]

  return (
    <div className="flex min-h-screen bg-background">
      {sidebarOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/50 lg:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      <aside
        className={cn(
          'fixed inset-y-0 left-0 z-50 flex w-64 flex-col border-r border-border bg-card transition-transform lg:static lg:translate-x-0',
          sidebarOpen ? 'translate-x-0' : '-translate-x-full',
        )}
      >
        <div className="flex h-14 items-center gap-2 border-b border-border px-4">
          <Shield className="h-6 w-6 text-primary" />
          <span className="text-lg font-bold">{t('nav.omniTunAdmin')}</span>
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

        <div className="border-t border-border p-3">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Shield className="h-4 w-4" />
            <span className="truncate">{user?.name ?? user?.email}</span>
          </div>
        </div>
      </aside>

      <div className="flex flex-1 flex-col">
        <header className="sticky top-0 z-30 flex h-14 items-center gap-4 border-b border-border bg-background px-4">
          <Button
            variant="ghost"
            size="sm"
            className="lg:hidden"
            onClick={() => setSidebarOpen(true)}
          >
            <Menu className="h-5 w-5" />
          </Button>

          <div className="flex-1" />

          <span className="hidden text-sm text-muted-foreground sm:inline">
            {user?.name ?? user?.email}
          </span>
          <Button variant="outline" size="sm" onClick={logout}>
            <LogOut className="mr-1 h-4 w-4" />
            {t('nav.signOut')}
          </Button>
        </header>

        <main className="flex-1 overflow-auto">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
