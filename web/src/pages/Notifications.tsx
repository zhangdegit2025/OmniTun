import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useNotifications, type Notification } from '@/hooks/useNotifications'
import { Card, CardContent } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Skeleton } from '@/components/ui/Skeleton'
import { cn } from '@/lib/utils'
import {
  Bell,
  CheckCheck,
  Server,
  CreditCard,
  Settings2,
  ChevronRight,
  CheckCircle2,
  XCircle,
  Info,
  AlertTriangle,
} from 'lucide-react'

type Category = 'all' | 'tunnels' | 'billing' | 'system'

const categoryTabs: { key: Category; icon: typeof Bell }[] = [
  { key: 'all', icon: Bell },
  { key: 'tunnels', icon: Server },
  { key: 'billing', icon: CreditCard },
  { key: 'system', icon: Settings2 },
]

const severityIcon = {
  success: CheckCircle2,
  error: XCircle,
  warning: AlertTriangle,
  info: Info,
}

const severityColor = {
  success: 'text-emerald-500',
  error: 'text-destructive',
  warning: 'text-amber-500',
  info: 'text-primary',
}

function timeAgo(dateStr: string, t: (key: string, options?: Record<string, unknown>) => string): string {
  const now = Date.now()
  const diff = now - new Date(dateStr).getTime()
  const minutes = Math.floor(diff / 60000)
  if (minutes < 1) return t('notifications.ago.now')
  if (minutes < 60) return t('notifications.ago.minute', { n: minutes })
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return t('notifications.ago.hour', { n: hours })
  const days = Math.floor(hours / 24)
  return t('notifications.ago.day', { n: days })
}

export default function Notifications() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { notifications, isLoading, markAllRead, markOneRead } = useNotifications()
  const [category, setCategory] = useState<Category>('all')

  const filtered =
    category === 'all'
      ? notifications
      : notifications.filter((n) => n.category === category)

  const hasUnread = notifications.some((n) => !n.read)

  const handleView = (n: Notification) => {
    if (!n.read) markOneRead(n.id)
    if (n.link) navigate(n.link)
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">{t('notifications.title')}</h1>
          <p className="text-sm text-muted-foreground">{t('notifications.subtitle')}</p>
        </div>
        {hasUnread && (
          <Button variant="outline" size="sm" onClick={markAllRead}>
            <CheckCheck className="mr-1 h-4 w-4" />
            {t('notifications.mark_all_read')}
          </Button>
        )}
      </div>

      <div className="flex gap-2 border-b pb-3">
        {categoryTabs.map(({ key, icon: Icon }) => {
          const count =
            key === 'all'
              ? notifications.filter((n) => !n.read).length
              : notifications.filter((n) => n.category === key && !n.read).length
          return (
            <button
              key={key}
              type="button"
              onClick={() => setCategory(key)}
              className={cn(
                'inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition-colors',
                category === key
                  ? 'bg-primary text-primary-foreground'
                  : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground',
              )}
            >
              <Icon className="h-3.5 w-3.5" />
              {key === 'all' ? t('notifications.all') : t(`notifications.${key === 'billing' ? 'billing_cat' : key}`)}
              {count > 0 && (
                <Badge
                  variant={category === key ? 'default' : 'secondary'}
                  className="ml-0.5 h-4 min-w-4 px-1 text-[10px]"
                >
                  {count}
                </Badge>
              )}
            </button>
          )
        })}
      </div>

      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <div className="space-y-3 p-6">
              <Skeleton className="h-16 w-full" />
              <Skeleton className="h-16 w-full" />
              <Skeleton className="h-16 w-full" />
            </div>
          ) : filtered.length === 0 ? (
            <div className="flex flex-col items-center gap-3 py-10 text-center">
              <Bell className="h-10 w-10 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">{t('notifications.empty')}</p>
            </div>
          ) : (
            <div className="divide-y">
              {filtered.map((n) => {
                const SevIcon = severityIcon[n.severity]
                return (
                  <div
                    key={n.id}
                    className={cn(
                      'flex items-start gap-3 px-4 py-3 transition-colors hover:bg-muted/50',
                      !n.read && 'bg-muted/30',
                    )}
                  >
                    <div className="mt-0.5 flex-shrink-0">
                      <SevIcon className={cn('h-5 w-5', severityColor[n.severity])} />
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <p
                          className={cn(
                            'text-sm leading-tight',
                            !n.read && 'font-semibold',
                          )}
                        >
                          {n.title}
                        </p>
                        {!n.read && (
                          <span className="h-2 w-2 flex-shrink-0 rounded-full bg-primary" />
                        )}
                      </div>
                      <p className="mt-0.5 text-xs text-muted-foreground">{n.description}</p>
                      <p className="mt-1 text-xs text-muted-foreground/70">
                        {timeAgo(n.created_at, t)}
                      </p>
                    </div>
                    {n.link && (
                      <Button
                        variant="ghost"
                        size="sm"
                        className="mt-0.5 flex-shrink-0"
                        onClick={() => handleView(n)}
                      >
                        <span className="text-xs">{t('notifications.view')}</span>
                        <ChevronRight className="ml-0.5 h-3 w-3" />
                      </Button>
                    )}
                  </div>
                )
              })}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
