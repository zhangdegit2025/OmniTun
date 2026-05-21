import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiRequest } from '@/lib/api'
import type { Tunnel } from '@/lib/types'

interface CreateTunnelInput {
  name: string
  protocol: 'tcp' | 'http' | 'https'
  local_port: number
  remote_port: number
  domain?: string
}

interface UpdateTunnelInput extends Partial<CreateTunnelInput> {
  id: string
  auth_mode?: 'none' | 'basic' | 'oauth'
  max_connections?: number
}

const TUNNELS_KEY = ['tunnels'] as const

export function useTunnels() {
  const queryClient = useQueryClient()

  const tunnelsQuery = useQuery<Tunnel[]>({
    queryKey: TUNNELS_KEY,
    queryFn: () => apiRequest<Tunnel[] | { tunnels: Tunnel[] }>('/v1/tunnels').then(data => {
      if (Array.isArray(data)) return data
      if (data && Array.isArray((data as { tunnels: Tunnel[] }).tunnels)) return (data as { tunnels: Tunnel[] }).tunnels
      return []
    }),
  })

  const createTunnel = useMutation<Tunnel, Error, CreateTunnelInput>({
    mutationFn: (input) =>
      apiRequest<Tunnel>('/v1/tunnels', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: TUNNELS_KEY })
    },
  })

  const updateTunnel = useMutation<Tunnel, Error, UpdateTunnelInput>({
    mutationFn: ({ id, ...input }) =>
      apiRequest<Tunnel>(`/v1/tunnels/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: TUNNELS_KEY })
    },
  })

  const deleteTunnel = useMutation<void, Error, string>({
    mutationFn: (id) =>
      apiRequest<void>(`/v1/tunnels/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: TUNNELS_KEY })
    },
  })

  const startTunnel = useMutation<Tunnel, Error, string>({
    mutationFn: (id) =>
      apiRequest<Tunnel>(`/v1/tunnels/${id}/start`, { method: 'POST' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: TUNNELS_KEY })
    },
  })

  const stopTunnel = useMutation<Tunnel, Error, string>({
    mutationFn: (id) =>
      apiRequest<Tunnel>(`/v1/tunnels/${id}/stop`, { method: 'POST' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: TUNNELS_KEY })
    },
  })

  const restartTunnel = useMutation<Tunnel, Error, string>({
    mutationFn: (id) =>
      apiRequest<Tunnel>(`/v1/tunnels/${id}/restart`, { method: 'POST' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: TUNNELS_KEY })
    },
  })

  return {
    tunnels: tunnelsQuery.data ?? [],
    isLoading: tunnelsQuery.isLoading,
    error: tunnelsQuery.error,
    refetch: tunnelsQuery.refetch,
    createTunnel,
    updateTunnel,
    deleteTunnel,
    startTunnel,
    stopTunnel,
    restartTunnel,
  }
}

export function useTunnel(id: string | undefined) {
  return useQuery<Tunnel>({
    queryKey: ['tunnel', id],
    queryFn: () => apiRequest<Tunnel>(`/v1/tunnels/${id}`),
    enabled: !!id,
  })
}
