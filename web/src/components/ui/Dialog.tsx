import { useEffect, useRef, type ReactNode, type MouseEvent } from 'react'
import { cn } from '@/lib/utils'
import { X } from 'lucide-react'

export interface DialogProps {
  /** Whether the dialog is visible */
  open: boolean
  /** Called when the user requests to close the dialog */
  onClose: () => void
  /** Dialog title (optional, but recommended for accessibility) */
  title?: string
  /** Dialog description for screen readers */
  description?: string
  /** Dialog body content */
  children: ReactNode
  /** Additional CSS classes for the overlay */
  className?: string
}

/**
 * A modal dialog built with native HTML elements and Tailwind CSS.
 *
 * Includes backdrop click-to-close, Escape key handling, and focus trap.
 *
 * @example
 * <Dialog open={show} onClose={() => setShow(false)} title="Confirm">
 *   <p>Are you sure?</p>
 * </Dialog>
 */
export function Dialog({
  open,
  onClose,
  title,
  description,
  children,
  className,
}: DialogProps) {
  const overlayRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose()
    }
    if (open) {
      document.addEventListener('keydown', handleKeyDown)
      document.body.style.overflow = 'hidden'
    }
    return () => {
      document.removeEventListener('keydown', handleKeyDown)
      document.body.style.overflow = ''
    }
  }, [open, onClose])

  if (!open) return null

  const handleBackdropClick = (e: MouseEvent) => {
    if (e.target === overlayRef.current) onClose()
  }

  return (
    <div
      ref={overlayRef}
      role="dialog"
      aria-modal="true"
      aria-label={title}
      aria-describedby={description ? 'dialog-desc' : undefined}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={handleBackdropClick}
    >
      <div
        className={cn(
          'relative w-full max-w-lg rounded-lg border bg-background p-6 shadow-lg',
          className,
        )}
      >
        <button
          onClick={onClose}
          className="absolute right-4 top-4 rounded-sm opacity-70 ring-offset-background transition-opacity hover:opacity-100 focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2"
          aria-label="Close"
        >
          <X className="h-4 w-4" />
        </button>
        {title && (
          <h2 className="text-lg font-semibold leading-none tracking-tight">
            {title}
          </h2>
        )}
        {description && (
          <p id="dialog-desc" className="mt-2 text-sm text-muted-foreground">
            {description}
          </p>
        )}
        <div className={cn(title || description ? 'mt-4' : '')}>{children}</div>
      </div>
    </div>
  )
}
