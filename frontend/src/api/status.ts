import { apiClient } from './client'

export interface RealtimeConcurrencyStatus {
  current: number
  updated_at: string
}

async function getRealtimeConcurrency(): Promise<RealtimeConcurrencyStatus> {
  const { data } = await apiClient.get<RealtimeConcurrencyStatus>('/status/concurrency')
  return data
}

export const statusAPI = { getRealtimeConcurrency }
