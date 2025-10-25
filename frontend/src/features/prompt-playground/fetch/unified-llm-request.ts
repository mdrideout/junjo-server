import { API_HOST } from '../../../config'
import {
  UnifiedLLMRequest,
  UnifiedLLMResponseSchema,
  ProviderInfoSchema,
  ModelInfoSchema,
  type ProviderInfo,
  type ModelInfo
} from '../schemas/unified-request'
import { z } from 'zod'

/**
 * Send a unified LLM generation request to the backend
 */
export const unifiedLLMRequest = async (payload: UnifiedLLMRequest) => {
  const response = await fetch(`${API_HOST}/llm/v2/generate`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
    credentials: 'include',
  })

  if (!response.ok) {
    // Try to extract error message from response body
    let errorMessage = 'Failed to generate content'
    try {
      const errorData = await response.json()
      if (errorData.error) {
        errorMessage = errorData.error
      }
    } catch {
      // If parsing fails, use default message
    }
    throw new Error(errorMessage)
  }

  const data = await response.json()
  return UnifiedLLMResponseSchema.parse(data)
}

/**
 * Fetch list of available providers
 */
export const fetchProviders = async (): Promise<ProviderInfo[]> => {
  const response = await fetch(`${API_HOST}/llm/providers`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
    credentials: 'include',
  })

  if (!response.ok) {
    throw new Error('Failed to fetch providers')
  }

  const data = await response.json()
  return z.array(ProviderInfoSchema).parse(data)
}

/**
 * Fetch list of all available models
 */
export const fetchAllModels = async (): Promise<ModelInfo[]> => {
  const response = await fetch(`${API_HOST}/llm/models`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
    credentials: 'include',
  })

  if (!response.ok) {
    throw new Error('Failed to fetch models')
  }

  const data = await response.json()
  return z.array(ModelInfoSchema).parse(data)
}

/**
 * Fetch models for a specific provider
 */
export const fetchModelsByProvider = async (provider: string): Promise<ModelInfo[]> => {
  const response = await fetch(`${API_HOST}/llm/providers/${provider}/models`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
    credentials: 'include',
  })

  if (!response.ok) {
    throw new Error(`Failed to fetch models for provider: ${provider}`)
  }

  const data = await response.json()
  return z.array(ModelInfoSchema).parse(data)
}

/**
 * Refresh models for a specific provider (forces API fetch, bypasses cache)
 */
export const refreshModelsByProvider = async (provider: string): Promise<ModelInfo[]> => {
  const response = await fetch(`${API_HOST}/llm/providers/${provider}/models/refresh`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    credentials: 'include',
  })

  if (!response.ok) {
    throw new Error(`Failed to refresh models for provider: ${provider}`)
  }

  const data = await response.json()
  return z.array(ModelInfoSchema).parse(data)
}
