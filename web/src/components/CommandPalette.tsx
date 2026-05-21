import {
  createContext,
  useContext,
  useState,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  type ReactNode,
} from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useQueryClient } from '@tanstack/react-query'
import { cn } from '@/lib/utils'
import type { Tunnel } from '@/lib/types'
import { Search, Server, Globe, Settings2, BookOpen, Share2 } from 'lucide-react'

interface SearchResult {
  id: string
  icon: typeof Server
  name: string
  subtitle: string
  group: string
  to: string
}

interface CommandPaletteState {
  open: boolean
  setOpen: (open: boolean) => void
  toggle: () => void
}

const CommandPaletteContext = createContext<CommandPaletteState>({
  open: false,
  setOpen: () => {},
  toggle: () => {},
})

export function useCommandPalette() {
  return useContext(CommandPaletteContext)
}

export function CommandPaletteProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false)
  const toggle = useCallback(() => setOpen((prev) => !prev), [])

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault()
        setOpen((prev) => !prev)
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [])

  return (
    <CommandPaletteContext.Provider value={{ open, setOpen, toggle }}>
      {children}
      <CommandPaletteOverlay />
    </CommandPaletteContext.Provider>
  )
}

interface DomainItem {
  id: string
  domain: string
  tunnel_name?: string
  tunnel_id?: string
  verification_status?: string
}

interface MeshNetworkItem {
  id: string
  name: string
  cidr?: string
}

function fuzzyMatch(query: string, text: string): boolean {
  if (!query) return true
  const lowerQuery = query.toLowerCase()
  const lowerText = text.toLowerCase()
  if (lowerText.includes(lowerQuery)) return true
  let qi = 0
  for (let ti = 0; ti < lowerText.length; ti++) {
    if (lowerText[ti] === lowerQuery[qi]) {
      qi++
      if (qi === lowerQuery.length) return true
    }
  }
  return false
}

function CommandPaletteOverlay() {
  const { open, setOpen } = useCommandPalette()
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const inputRef = useRef<HTMLInputElement>(null)
  const [query, setQuery] = useState('')
  const [selectedIndex, setSelectedIndex] = useState(0)
  const overlayRef = useRef<HTMLDivElement>(null)

  const tunnels = queryClient.getQueryData<Tunnel[]>(['tunnels']) ?? []
  const domains = queryClient.getQueryData<DomainItem[]>(['domains']) ?? []
  const meshNetworks = queryClient.getQueryData<MeshNetworkItem[]>(['networks']) ?? []

  const settingsRoutes = [
    { name: 'nav.settings', slug: 'settings', to: '/settings' },
    { name: 'settings.tabs.organization', slug: 'settings-organization', to: '/settings' },
    { name: 'settings.tabs.security', slug: 'settings-security', to: '/settings' },
    { name: 'settings.tabs.api_keys', slug: 'settings-api', to: '/settings' },
    { name: 'billing.title', slug: 'billing', to: '/billing' },
    { name: 'notifications.title', slug: 'notifications', to: '/notifications' },
  ]

  const docsLinks = [
    { slug: 'docs-getting-started', to: '#', icon: BookOpen, group: 'docs' },
  ]

  const results = useMemo((): SearchResult[] => {
    const items: SearchResult[] = []

    tunnels.forEach((tunnel) => {
      if (fuzzyMatch(query, tunnel.name) || fuzzyMatch(query, tunnel.domain ?? '')) {
        items.push({
          id: `tunnel-${tunnel.id}`,
          icon: Server,
          name: tunnel.name,
          subtitle: `${tunnel.protocol.toUpperCase()} — ${tunnel.domain || `Port ${tunnel.remote_port}`}`,
          group: 'tunnels',
          to: `/tunnels/${tunnel.id}`,
        })
      }
    })

    domains.forEach((d) => {
      if (fuzzyMatch(query, d.domain) || fuzzyMatch(query, d.tunnel_name ?? '')) {
        items.push({
          id: `domain-${d.id}`,
          icon: Globe,
          name: d.domain,
          subtitle: d.tunnel_name ?? d.tunnel_id?.slice(0, 8) ?? '',
          group: 'domains',
          to: '/domains',
        })
      }
    })

    meshNetworks.forEach((n) => {
      if (fuzzyMatch(query, n.name)) {
        items.push({
          id: `network-${n.id}`,
          icon: Share2,
          name: n.name,
          subtitle: n.cidr ?? '',
          group: 'networks',
          to: `/networks/${n.id}`,
        })
      }
    })

    settingsRoutes.forEach((s) => {
      const label = t(s.name, s.name)
      if (fuzzyMatch(query, label) || fuzzyMatch(query, s.slug)) {
        items.push({
          id: `settings-${s.slug}`,
          icon: Settings2,
          name: label,
          subtitle: t('nav.settings'),
          group: 'settings',
          to: s.to,
        })
      }
    })

    if (query) {
      docsLinks.forEach((d) => {
        if (fuzzyMatch(query, d.slug)) {
          items.push({
            id: d.slug,
            icon: d.icon,
            name: d.slug.replace('docs-', '').replace(/-/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase()),
            subtitle: '',
            group: d.group,
            to: d.to,
          })
        }
      })
    }

    return items
  }, [query, tunnels, domains, meshNetworks, t])

  const grouped = useMemo(() => {
    const map = new Map<string, SearchResult[]>()
    results.forEach((r) => {
      const list = map.get(r.group) ?? []
      list.push(r)
      map.set(r.group, list)
    })
    return Array.from(map.entries())
  }, [results])

  const flatList = useMemo(() => results, [results])

  const groupLabels: Record<string, string> = {
    tunnels: t('search.groups.tunnels'),
    domains: t('search.groups.domains'),
    settings: t('search.groups.settings'),
    networks: t('search.groups.networks'),
    docs: t('search.groups.docs'),
  }

  useEffect(() => {
    if (open) {
      setQuery('')
      setSelectedIndex(0)
      setTimeout(() => inputRef.current?.focus(), 50)
    }
  }, [open])

  useEffect(() => {
    setSelectedIndex(0)
  }, [query])

  useEffect(() => {
    function handleEscape(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false)
    }
    if (open) {
      document.addEventListener('keydown', handleEscape)
      document.body.style.overflow = 'hidden'
    }
    return () => {
      document.removeEventListener('keydown', handleEscape)
      document.body.style.overflow = ''
    }
  }, [open, setOpen])

  const execute = useCallback(
    (to: string) => {
      setOpen(false)
      if (to.startsWith('/')) navigate(to)
    },
    [setOpen, navigate],
  )

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setSelectedIndex((prev) => Math.min(prev + 1, flatList.length - 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setSelectedIndex((prev) => Math.max(prev - 1, 0))
    } else if (e.key === 'Enter') {
      e.preventDefault()
      if (flatList[selectedIndex]) execute(flatList[selectedIndex].to)
    }
  }

  if (!open) return null

  return (
    <div
      ref={overlayRef}
      className="fixed inset-0 z-[100] flex items-start justify-center bg-black/50 pt-[15vh]"
      onClick={(e) => {
        if (e.target === overlayRef.current) setOpen(false)
      }}
    >
      <div className="w-full max-w-lg rounded-lg border bg-card shadow-2xl">
        <div className="flex items-center gap-2 border-b px-4">
          <Search className="h-4 w-4 text-muted-foreground" />
          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={t('search.placeholder')}
            className="flex-1 bg-transparent py-3 text-sm outline-none placeholder:text-muted-foreground"
          />
          <kbd className="hidden rounded border bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground sm:inline-block">
            ESC
          </kbd>
        </div>
        <div className="max-h-[320px] overflow-y-auto p-2">
          {grouped.length === 0 && query ? (
            <p className="px-3 py-6 text-center text-sm text-muted-foreground">
              {t('search.no_results')}
            </p>
          ) : (
            grouped.map(([group, items]) => (
              <div key={group}>
                <p className="px-3 py-1.5 text-[10px] font-semibold uppercase text-muted-foreground">
                  {groupLabels[group] ?? group}
                </p>
                {items.map((item) => {
                  const idx = flatList.indexOf(item)
                  const isSelected = idx === selectedIndex
                  const Icon = item.icon
                  return (
                    <button
                      key={item.id}
                      type="button"
                      className={cn(
                        'flex w-full items-center gap-3 rounded-md px-3 py-2 text-left text-sm transition-colors',
                        isSelected
                          ? 'bg-accent text-accent-foreground'
                          : 'text-foreground hover:bg-accent/50',
                      )}
                      onClick={() => execute(item.to)}
                      onMouseEnter={() => setSelectedIndex(idx)}
                    >
                      <Icon className="h-4 w-4 flex-shrink-0 text-muted-foreground" />
                      <div className="min-w-0 flex-1">
                        <p className="truncate">{item.name}</p>
                        {item.subtitle && (
                          <p className="truncate text-xs text-muted-foreground">
                            {item.subtitle}
                          </p>
                        )}
                      </div>
                    </button>
                  )
                })}
              </div>
            ))
          )}
        </div>
        <div className="border-t px-4 py-2 text-[10px] text-muted-foreground">
          <span className="inline-flex items-center gap-1">
            <kbd className="rounded border bg-muted px-1 py-0.5">↑↓</kbd> navigate
          </span>
          <span className="mx-2">·</span>
          <span className="inline-flex items-center gap-1">
            <kbd className="rounded border bg-muted px-1 py-0.5">↵</kbd> select
          </span>
        </div>
      </div>
    </div>
  )
}
