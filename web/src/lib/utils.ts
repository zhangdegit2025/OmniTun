import { type ClassValue, clsx } from 'clsx'

/**
 * Combines class names using clsx, supporting conditional and merged classes.
 */
export function cn(...inputs: ClassValue[]): string {
  return clsx(inputs)
}
