import { API_HOST } from '../../../config'
import { OpenAIRequest, OpenAIResponseSchema, type OpenAIResponse } from '../schemas/openai-request'

/**
 * Send a request to OpenAI's chat completions endpoint via our backend
 */
export const openaiRequest = async (payload: OpenAIRequest): Promise<OpenAIResponse> => {
  const response = await fetch(`${API_HOST}/llm/openai/generate`, {
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
  return OpenAIResponseSchema.parse(data)
}
