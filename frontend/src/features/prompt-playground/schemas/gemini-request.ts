import { z } from 'zod'

// Gemini Part Schema
export const GeminiPartSchema = z.object({
  text: z.string(),
})

export type GeminiPart = z.infer<typeof GeminiPartSchema>

// Gemini Content Schema
export const GeminiContentSchema = z.object({
  role: z.string().optional(),
  parts: z.array(GeminiPartSchema),
})

export type GeminiContent = z.infer<typeof GeminiContentSchema>

// Gemini Generation Config Schema
export const GeminiGenerationConfigSchema = z.object({
  temperature: z.number().optional(),
  topP: z.number().optional(),
  topK: z.number().optional(),
  maxOutputTokens: z.number().optional(),
  stopSequences: z.array(z.string()).optional(),
  responseMimeType: z.string().optional(),
})

export type GeminiGenerationConfig = z.infer<typeof GeminiGenerationConfigSchema>

// Gemini System Instruction Schema
export const GeminiSystemInstructionSchema = z.object({
  parts: z.array(GeminiPartSchema),
})

export type GeminiSystemInstruction = z.infer<typeof GeminiSystemInstructionSchema>

// Gemini Request Schema
export const GeminiRequestSchema = z.object({
  model: z.string().optional(),
  contents: z.array(GeminiContentSchema),
  generationConfig: GeminiGenerationConfigSchema.optional(),
  system_instruction: GeminiSystemInstructionSchema.optional(),
})

export type GeminiRequest = z.infer<typeof GeminiRequestSchema>

// Gemini Candidate Schema
export const GeminiCandidateSchema = z.object({
  content: GeminiContentSchema,
  finishReason: z.string().optional(),
  index: z.number(),
})

export type GeminiCandidate = z.infer<typeof GeminiCandidateSchema>

// Gemini Usage Metadata Schema
export const GeminiUsageMetadataSchema = z.object({
  promptTokenCount: z.number(),
  candidatesTokenCount: z.number(),
  totalTokenCount: z.number(),
})

export type GeminiUsageMetadata = z.infer<typeof GeminiUsageMetadataSchema>

// Gemini Response Schema
export const GeminiResponseSchema = z.object({
  candidates: z.array(GeminiCandidateSchema),
  usageMetadata: GeminiUsageMetadataSchema.optional(),
})

export type GeminiResponse = z.infer<typeof GeminiResponseSchema>
