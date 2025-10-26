import { API_HOST } from '../../../config'
import { GeminiRequest, GeminiResponseSchema, type GeminiResponse } from '../schemas/gemini-request'

/**
 * Send a request to Gemini's GenerateContent API via our backend
 */
export const geminiRequest = async (payload: GeminiRequest): Promise<GeminiResponse> => {
  const response = await fetch(`${API_HOST}/llm/gemini/generate`, {
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
  return GeminiResponseSchema.parse(data)
}
