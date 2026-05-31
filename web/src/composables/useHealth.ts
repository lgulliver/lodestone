import { useQuery } from '@tanstack/vue-query'

export interface Health {
  status: string
  service: string
  version: string
  commit: string
}

async function fetchHealth(): Promise<Health> {
  const res = await fetch('/health')
  if (!res.ok) throw new Error(`health check failed: ${res.status}`)
  return res.json()
}

export function useHealth() {
  return useQuery({
    queryKey: ['health'],
    queryFn: fetchHealth,
    staleTime: 60_000,
    retry: 1,
  })
}
