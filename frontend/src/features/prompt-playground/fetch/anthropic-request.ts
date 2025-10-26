import { API_HOST } from '../../../config'
import { AnthropicRequest, AnthropicResponseSchema, type AnthropicResponse } from '../schemas/anthropic-request'

/**
 * Send a request to Anthropic's Messages API via our backend
 */
export const anthropicRequest = async (payload: AnthropicRequest): Promise<AnthropicResponse> => {
  const response = await fetch(`${API_HOST}/llm/anthropic/generate`, {
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
  return AnthropicResponseSchema.parse(data)
}
