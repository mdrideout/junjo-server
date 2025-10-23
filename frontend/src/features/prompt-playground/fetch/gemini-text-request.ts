import { API_HOST } from '../../../config'
import { GeminiTextRequest } from '../schemas/gemini-text-request'
import { GeminiTextResponseSchema } from '../schemas/gemini-text-response'

export const geminiTextRequest = async (payload: GeminiTextRequest) => {
  const response = await fetch(`${API_HOST}/llm/generate`, {
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
  return GeminiTextResponseSchema.parse(data)
}
