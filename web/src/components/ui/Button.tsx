import { type ButtonHTMLAttributes, forwardRef } from 'react'
import { cn } from '@/lib/utils'

/**
 * Variant styles for the Button component.
 */
const variantClasses = {
  default:
    'bg-primary text-primary-foreground shadow hover:bg-primary/90',
  destructive:
    'bg-destructive text-destructive-foreground shadow-sm hover:bg-destructive/90',
  outline:
    'border border-input bg-transparent shadow-sm hover:bg-accent hover:text-accent-foreground',
  ghost: 'hover:bg-accent hover:text-accent-foreground',
} as const

/**
 * Size styles for the Button component.
 */
const sizeClasses = {
  sm: 'h-8 rounded-md px-3 text-xs',
  default: 'h-9 rounded-md px-4 py-2 text-sm',
  lg: 'h-10 rounded-md px-8 text-base',
} as const

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  /** Visual variant of the button */
  variant?: keyof typeof variantClasses
  /** Size of the button */
  size?: keyof typeof sizeClasses
}

/**
 * A versatile button component with multiple visual variants and sizes.
 *
 * @example
 * <Button variant="destructive" size="lg">Delete</Button>
 */
export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant = 'default', size = 'default', ...props }, ref) => {
    return (
      <button
        ref={ref}
        className={cn(
          'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md font-medium transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50',
          variantClasses[variant],
          sizeClasses[size],
          className,
        )}
        {...props}
      />
    )
  },
)

Button.displayName = 'Button'
