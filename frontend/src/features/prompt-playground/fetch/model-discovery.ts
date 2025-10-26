import { API_HOST } from '../../../config'

export interface ModelInfo {
  id: string
  name: string
  provider: string
  description?: string
  contextWindow?: number
  maxOutputTokens?: number
  createdAt?: string
  capabilities?: string[]
  metadata?: Record<string, string>
}

interface ModelsResponse {
  models: ModelInfo[]
}

/**
 * Fetch all available models from all providers
 */
export const fetchAllModels = async (): Promise<ModelInfo[]> => {
  const response = await fetch(`${API_HOST}/llm/models`, {
    method: 'GET',
    credentials: 'include',
  })

  if (!response.ok) {
    throw new Error(`Failed to fetch models: ${response.statusText}`)
  }

  const data: ModelsResponse = await response.json()
  return data.models
}

/**
 * Fetch models for a specific provider
 */
export const fetchModelsByProvider = async (provider: string): Promise<ModelInfo[]> => {
  const response = await fetch(`${API_HOST}/llm/providers/${provider}/models`, {
    method: 'GET',
    credentials: 'include',
  })

  if (!response.ok) {
    throw new Error(`Failed to fetch models for ${provider}: ${response.statusText}`)
  }

  const data: ModelsResponse = await response.json()
  return data.models
}

/**
 * Refresh models from provider API
 */
export const refreshModelsByProvider = async (provider: string): Promise<ModelInfo[]> => {
  const response = await fetch(`${API_HOST}/llm/providers/${provider}/models/refresh`, {
    method: 'POST',
    credentials: 'include',
  })

  if (!response.ok) {
    throw new Error(`Failed to refresh models for ${provider}: ${response.statusText}`)
  }

  // After refresh, fetch the updated models
  return fetchModelsByProvider(provider)
}
