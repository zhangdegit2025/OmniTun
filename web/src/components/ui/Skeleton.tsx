import { cn } from '@/lib/utils'

export interface SkeletonProps {
  /** Additional CSS classes */
  className?: string
}

/**
 * A skeleton loader component for indicating loading states.
 *
 * @example
 * <Skeleton className="h-4 w-[200px]" />
 */
export function Skeleton({ className }: SkeletonProps) {
  return (
    <div
      role="status"
      aria-label="Loading"
      className={cn('animate-pulse rounded-md bg-muted', className)}
    />
  )
}
