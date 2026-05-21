import { type HTMLAttributes } from 'react'
import { cn } from '@/lib/utils'

const variantClasses = {
  default: 'bg-primary/20 text-primary-foreground',
  success: 'bg-emerald-500/20 text-emerald-400',
  warning: 'bg-amber-500/20 text-amber-400',
  destructive: 'bg-destructive/20 text-destructive',
  secondary: 'bg-secondary text-secondary-foreground',
} as const

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  variant?: keyof typeof variantClasses
}

export function Badge({ className, variant = 'default', ...props }: BadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium transition-colors',
        variantClasses[variant],
        className,
      )}
      {...props}
    />
  )
}
