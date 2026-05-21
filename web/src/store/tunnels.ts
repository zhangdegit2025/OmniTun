import { create } from 'zustand'

interface TunnelStore {
  selectedTunnelId: string | null
  selectTunnel: (id: string | null) => void
}

export const useTunnelStore = create<TunnelStore>((set) => ({
  selectedTunnelId: null,
  selectTunnel: (id) => set({ selectedTunnelId: id }),
}))
