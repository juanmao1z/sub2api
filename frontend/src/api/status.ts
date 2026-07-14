import { apiClient } from './client'

export interface RealtimeConcurrencyStatus {
  current: number
  updated_at: string
}

export interface HomepageStatus {
  started_at: string | null
  active_users_1h: number
  success_rate_today: number | null
  total_tokens: number
  updated_at: string
}

async function getRealtimeConcurrency(signal?: AbortSignal): Promise<RealtimeConcurrencyStatus> {
  const { data } = await apiClient.get<RealtimeConcurrencyStatus>('/status/concurrency', { signal })
  return data
}

async function getHomepageStatus(signal?: AbortSignal): Promise<HomepageStatus> {
  const { data } = await apiClient.get<HomepageStatus>('/status/homepage', { signal })
  return data
}

export const statusAPI = { getRealtimeConcurrency, getHomepageStatus }
