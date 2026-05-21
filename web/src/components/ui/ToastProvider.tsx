import { type ReactNode, useState, useCallback } from 'react'
import { ToastContext, type Toast } from './useToast'
import { ToastContainer } from './Toast'

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])
  const dismiss = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id))
  }, [])
  const toast = useCallback(
    (t: Omit<Toast, 'id'>) => {
      const id = crypto.randomUUID()
      const duration = t.duration ?? 4000
      setToasts((prev) => [...prev, { ...t, id }])
      if (duration > 0) {
        setTimeout(() => dismiss(id), duration)
      }
    },
    [dismiss],
  )
  return (
    <ToastContext.Provider value={{ toasts, toast, dismiss }}>
      {children}
      <ToastContainer toasts={toasts} onDismiss={dismiss} />
    </ToastContext.Provider>
  )
}
