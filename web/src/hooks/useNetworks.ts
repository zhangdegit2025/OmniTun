import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiRequest } from '@/lib/api'
import type { MeshNetwork, MeshNode, MeshInvite } from '@/lib/types'

interface CreateNetworkInput {
  name: string
  cidr: string
}

interface JoinNetworkInput {
  invite_code: string
}

const NETWORKS_KEY = ['networks'] as const

export function useNetworks() {
  const queryClient = useQueryClient()

  const networksQuery = useQuery<MeshNetwork[]>({
    queryKey: NETWORKS_KEY,
    queryFn: () =>
      apiRequest<MeshNetwork[] | { networks: MeshNetwork[] }>('/v1/networks').then((data) => {
        if (Array.isArray(data)) return data
        if (data && Array.isArray((data as { networks: MeshNetwork[] }).networks))
          return (data as { networks: MeshNetwork[] }).networks
        return []
      }),
  })

  const createNetwork = useMutation<MeshNetwork, Error, CreateNetworkInput>({
    mutationFn: (input) =>
      apiRequest<MeshNetwork>('/v1/networks', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: NETWORKS_KEY })
    },
  })

  const joinNetwork = useMutation<MeshNode, Error, JoinNetworkInput>({
    mutationFn: (input) =>
      apiRequest<MeshNode>('/v1/networks/join', {
        method: 'POST',
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: NETWORKS_KEY })
    },
  })

  const deleteNetwork = useMutation<void, Error, string>({
    mutationFn: (id) => apiRequest<void>(`/v1/networks/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: NETWORKS_KEY })
    },
  })

  return {
    networks: networksQuery.data ?? [],
    isLoading: networksQuery.isLoading,
    error: networksQuery.error,
    refetch: networksQuery.refetch,
    createNetwork,
    joinNetwork,
    deleteNetwork,
  }
}

export function useNetwork(id: string | undefined) {
  return useQuery<MeshNetwork & { nodes: MeshNode[] }>({
    queryKey: ['network', id],
    queryFn: () => apiRequest<MeshNetwork & { nodes: MeshNode[] }>(`/v1/networks/${id}`),
    enabled: !!id,
  })
}

export function useNetworkInvite(networkId: string | undefined) {
  const queryClient = useQueryClient()

  const inviteQuery = useQuery<MeshInvite>({
    queryKey: ['network', networkId, 'invite'],
    queryFn: () => apiRequest<MeshInvite>(`/v1/networks/${networkId}/invite`),
    enabled: !!networkId,
  })

  const createInvite = useMutation<MeshInvite, Error, string>({
    mutationFn: (id) =>
      apiRequest<MeshInvite>(`/v1/networks/${id}/invite`, { method: 'POST' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['network', networkId, 'invite'] })
    },
  })

  return {
    invite: inviteQuery.data,
    isLoading: inviteQuery.isLoading,
    refetch: inviteQuery.refetch,
    createInvite,
  }
}

export function useRemoveNode(networkId: string | undefined) {
  const queryClient = useQueryClient()

  return useMutation<void, Error, string>({
    mutationFn: (nodeId) =>
      apiRequest<void>(`/v1/networks/${networkId}/nodes/${nodeId}`, { method: 'DELETE' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['network', networkId] })
      queryClient.invalidateQueries({ queryKey: NETWORKS_KEY })
    },
  })
}
