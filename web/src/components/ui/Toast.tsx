import { cn } from '@/lib/utils'
import { X } from 'lucide-react'
import type { Toast as ToastType, ToastVariant } from './useToast'

const variantClasses: Record<ToastVariant, string> = {
  default: 'border bg-background text-foreground',
  success:
    'border-emerald-500 bg-emerald-50 text-emerald-900 dark:bg-emerald-950 dark:text-emerald-100',
  error:
    'border-destructive bg-red-50 text-red-900 dark:bg-red-950 dark:text-red-100',
  warning:
    'border-amber-500 bg-amber-50 text-amber-900 dark:bg-amber-950 dark:text-amber-100',
}

function ToastItem({ toast: t, onDismiss }: { toast: ToastType; onDismiss: () => void }) {
  return (
    <div
      role="alert"
      className={cn(
        'flex items-start gap-3 rounded-lg border px-4 py-3 shadow-md transition-all',
        variantClasses[t.variant ?? 'default'],
      )}
    >
      <div className="flex-1 text-sm">
        <p className="font-semibold">{t.title}</p>
        {t.description && (
          <p className="text-muted-foreground">{t.description}</p>
        )}
      </div>
      <button
        onClick={onDismiss}
        className="shrink-0 rounded-sm opacity-70 hover:opacity-100"
        aria-label="Dismiss"
      >
        <X className="h-4 w-4" />
      </button>
    </div>
  )
}

export interface ToastContainerProps {
  toasts: ToastType[]
  onDismiss: (id: string) => void
}

/**
 * Renders the toast notification list. Used internally by ToastProvider.
 */
export function ToastContainer({ toasts, onDismiss }: ToastContainerProps) {
  return (
    <div
      aria-live="polite"
      className="fixed bottom-4 right-4 z-50 flex flex-col gap-2"
    >
      {toasts.map((t) => (
        <ToastItem key={t.id} toast={t} onDismiss={() => onDismiss(t.id)} />
      ))}
    </div>
  )
}
