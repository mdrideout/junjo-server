import { z } from 'zod'

// Anthropic Message Schema
export const AnthropicMessageSchema = z.object({
  role: z.enum(['user', 'assistant']),
  content: z.string(),
})

export type AnthropicMessage = z.infer<typeof AnthropicMessageSchema>

// Anthropic Tool Choice Schema
export const AnthropicToolChoiceSchema = z.object({
  type: z.enum(['auto', 'any', 'tool']),
  name: z.string().optional(),
})

export type AnthropicToolChoice = z.infer<typeof AnthropicToolChoiceSchema>

// Anthropic Tool Schema
export const AnthropicToolSchema = z.object({
  name: z.string(),
  description: z.string().optional(),
  input_schema: z.record(z.any()),
})

export type AnthropicTool = z.infer<typeof AnthropicToolSchema>

// Anthropic Request Schema
export const AnthropicRequestSchema = z.object({
  model: z.string(),
  messages: z.array(AnthropicMessageSchema),
  system: z.string().optional(),
  max_tokens: z.number(),
  temperature: z.number().optional(),
  top_p: z.number().optional(),
  stop_sequences: z.array(z.string()).optional(),
  tools: z.array(AnthropicToolSchema).optional(),
  tool_choice: AnthropicToolChoiceSchema.optional(),
  jsonMode: z.boolean().optional(), // Extension: auto-configures tool calling for JSON
})

export type AnthropicRequest = z.infer<typeof AnthropicRequestSchema>

// Anthropic Content Block Schema
export const AnthropicContentBlockSchema = z.object({
  type: z.string(),
  text: z.string().optional(),
  id: z.string().optional(),
  name: z.string().optional(),
  input: z.record(z.any()).optional(),
})

export type AnthropicContentBlock = z.infer<typeof AnthropicContentBlockSchema>

// Anthropic Usage Schema
export const AnthropicUsageSchema = z.object({
  input_tokens: z.number(),
  output_tokens: z.number(),
})

export type AnthropicUsage = z.infer<typeof AnthropicUsageSchema>

// Anthropic Response Schema
export const AnthropicResponseSchema = z.object({
  id: z.string(),
  type: z.string(),
  role: z.string(),
  content: z.array(AnthropicContentBlockSchema),
  model: z.string(),
  stop_reason: z.string(),
  stop_sequence: z.string().optional(),
  usage: AnthropicUsageSchema,
})

export type AnthropicResponse = z.infer<typeof AnthropicResponseSchema>
