import { z } from 'zod'

// Unified LLM Request Schema
export const UnifiedLLMRequestSchema = z.object({
  provider: z.string(),
  model: z.string(),
  prompt: z.string(),
  systemPrompt: z.string().optional(),
  jsonMode: z.boolean().optional(),
  temperature: z.number().optional(),
  maxTokens: z.number().optional(),
  topP: z.number().optional(),
  topK: z.number().optional(),
  stopSequences: z.array(z.string()).optional(),
})

export type UnifiedLLMRequest = z.infer<typeof UnifiedLLMRequestSchema>

// Unified LLM Response Schema
export const UnifiedLLMResponseSchema = z.object({
  text: z.string(),
  provider: z.string(),
  model: z.string(),
})

export type UnifiedLLMResponse = z.infer<typeof UnifiedLLMResponseSchema>

// Provider Info Schema
export const ProviderInfoSchema = z.object({
  name: z.string(),
  available: z.boolean(),
})

export type ProviderInfo = z.infer<typeof ProviderInfoSchema>

// Model Info Schema
export const ModelInfoSchema = z.object({
  id: z.string(),
  name: z.string(),
  provider: z.string(),
  description: z.string().optional(),
  contextWindow: z.number().optional(),
  maxOutputTokens: z.number().optional(),
  createdAt: z.string().optional(),
  capabilities: z.array(z.string()).optional(),
  metadata: z.record(z.string()).optional(),
})

export type ModelInfo = z.infer<typeof ModelInfoSchema>
