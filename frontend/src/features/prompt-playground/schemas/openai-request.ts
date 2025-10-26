import { z } from 'zod'

// OpenAI Message Schema
export const OpenAIMessageSchema = z.object({
  role: z.enum(['system', 'user', 'assistant']),
  content: z.string(),
})

export type OpenAIMessage = z.infer<typeof OpenAIMessageSchema>

// OpenAI Response Format Schema
export const OpenAIResponseFormatSchema = z.object({
  type: z.enum(['json_object', 'text']),
})

export type OpenAIResponseFormat = z.infer<typeof OpenAIResponseFormatSchema>

// OpenAI Reasoning Effort Schema
export const OpenAIReasoningEffortSchema = z.enum(['minimal', 'low', 'medium', 'high'])

export type OpenAIReasoningEffort = z.infer<typeof OpenAIReasoningEffortSchema>

// OpenAI Request Schema
export const OpenAIRequestSchema = z.object({
  model: z.string(),
  messages: z.array(OpenAIMessageSchema),
  temperature: z.number().optional(),
  max_tokens: z.number().optional(),
  max_completion_tokens: z.number().optional(),
  reasoning_effort: OpenAIReasoningEffortSchema.optional(),
  top_p: z.number().optional(),
  stop: z.array(z.string()).optional(),
  response_format: OpenAIResponseFormatSchema.optional(),
})

export type OpenAIRequest = z.infer<typeof OpenAIRequestSchema>

// OpenAI Choice Schema
export const OpenAIChoiceSchema = z.object({
  index: z.number(),
  message: OpenAIMessageSchema,
  finish_reason: z.string(),
})

export type OpenAIChoice = z.infer<typeof OpenAIChoiceSchema>

// OpenAI Usage Schema
export const OpenAIUsageSchema = z.object({
  prompt_tokens: z.number(),
  completion_tokens: z.number(),
  total_tokens: z.number(),
})

export type OpenAIUsage = z.infer<typeof OpenAIUsageSchema>

// OpenAI Response Schema
export const OpenAIResponseSchema = z.object({
  id: z.string(),
  object: z.string(),
  created: z.number(),
  model: z.string(),
  choices: z.array(OpenAIChoiceSchema),
  usage: OpenAIUsageSchema,
})

export type OpenAIResponse = z.infer<typeof OpenAIResponseSchema>
