import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { Sun, Moon, Monitor } from 'lucide-react'
import { Button } from '@/components/ui/Button'

type Theme = 'light' | 'dark' | 'system'

const STORAGE_KEY = 'omnitun-theme'

function getStoredTheme(): Theme {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored === 'dark' || stored === 'light' || stored === 'system') return stored
  } catch {
    // localStorage unavailable
  }
  return 'system'
}

function applyTheme(theme: Theme) {
  const isDark =
    theme === 'dark' ||
    (theme === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.classList.toggle('dark', isDark)
}

const iconMap = {
  light: Sun,
  dark: Moon,
  system: Monitor,
}

const themeCycle: Theme[] = ['light', 'dark', 'system']

export function ThemeToggle() {
  const { t } = useTranslation()
  const [theme, setTheme] = useState<Theme>(getStoredTheme)

  useEffect(() => {
    applyTheme(theme)
  }, [theme])

  useEffect(() => {
    if (theme !== 'system') return
    const mq = window.matchMedia('(prefers-color-scheme: dark)')
    function handleChange() {
      applyTheme('system')
    }
    mq.addEventListener('change', handleChange)
    return () => mq.removeEventListener('change', handleChange)
  }, [theme])

  const cycle = useCallback(() => {
    setTheme((prev) => {
      const idx = themeCycle.indexOf(prev)
      const next = themeCycle[(idx + 1) % themeCycle.length]
      try {
        localStorage.setItem(STORAGE_KEY, next)
      } catch {
        // ignore
      }
      return next
    })
  }, [])

  const Icon = iconMap[theme]

  return (
    <Button
      variant="ghost"
      size="sm"
      onClick={cycle}
      title={t(`theme.${theme}`)}
    >
      <Icon className="h-4 w-4" />
      <span className="hidden sm:inline">
        {t(`theme.${theme}`)}
      </span>
    </Button>
  )
}
