import { apiClient } from './client'

export interface RealtimeConcurrencyStatus {
  current: number
  updated_at: string
}

async function getRealtimeConcurrency(signal: AbortSignal): Promise<RealtimeConcurrencyStatus> {
  const { data } = await apiClient.get<RealtimeConcurrencyStatus>('/status/concurrency', { signal })
  return data
}

export const statusAPI = { getRealtimeConcurrency }
