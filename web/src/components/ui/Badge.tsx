import { type HTMLAttributes, forwardRef } from 'react'
import { cn } from '@/lib/utils'

/**
 * Visual variants for the Badge component.
 */
const badgeVariants = {
  default: 'bg-primary text-primary-foreground',
  secondary: 'bg-secondary text-secondary-foreground',
  destructive: 'bg-destructive text-destructive-foreground',
  outline: 'border text-foreground',
  success: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900 dark:text-emerald-200',
  warning: 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200',
} as const

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  /** Visual variant of the badge */
  variant?: keyof typeof badgeVariants
}

/**
 * A small badge used to display status, labels, or counts.
 *
 * @example
 * <Badge variant="success">Active</Badge>
 * <Badge variant="destructive">Stopped</Badge>
 */
export const Badge = forwardRef<HTMLSpanElement, BadgeProps>(
  ({ className, variant = 'default', ...props }, ref) => {
    return (
      <span
        ref={ref}
        className={cn(
          'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-semibold transition-colors',
          badgeVariants[variant],
          className,
        )}
        {...props}
      />
    )
  },
)

Badge.displayName = 'Badge'
