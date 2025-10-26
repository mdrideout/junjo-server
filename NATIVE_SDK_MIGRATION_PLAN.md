# Native Provider SDK Migration Plan
## Complete rewrite from unified abstraction to provider-specific native SDKs

---

## Overview
Migrate from Bellman-based unified LLM interface to native provider SDKs with provider-specific endpoints and schemas. This eliminates abstraction layers for maximum transparency and full feature access.

## Architecture Changes

### Current → New
- ❌ `/llm/v2/generate` unified endpoint → ✅ `/llm/openai/generate`, `/llm/anthropic/generate`, `/llm/gemini/generate`
- ❌ `UnifiedRequest`/`UnifiedResponse` → ✅ Provider-specific request/response schemas
- ❌ Bellman library → ✅ Native SDKs: `openai-go`, `anthropic-sdk-go`, `google.golang.org/genai`
- ❌ Instruction-based JSON mode → ✅ Native structured output (OpenAI `response_format`, Anthropic tool calling)

---

## Phase 1: Backend - Native SDK Integration (2 days)

### Day 1.1: Install Dependencies & OpenAI Provider

**1. Add Go dependencies:**
```bash
go get github.com/openai/openai-go
go get github.com/anthropics/anthropic-sdk-go
go get google.golang.org/genai
```

**2. Create OpenAI provider with native SDK:**
- **File**: `/backend/api/llm/openai/schemas.go`
  - Define `OpenAIRequest` matching OpenAI's chat completions API
  - Support messages array, tools, response_format (JSON Schema)
  - Define `OpenAIResponse` matching OpenAI's response format

- **File**: `/backend/api/llm/openai/handler.go`
  - Create `HandleOpenAIGenerate(c echo.Context)` handler
  - Use `github.com/openai/openai-go` client directly
  - For JSON mode: Use native `response_format: { type: "json_object" }`
  - Map request → `openai.ChatCompletionNewParams`
  - Return native OpenAI response format

**3. Create OpenAI endpoint:**
- **File**: `/backend/api/llm/routes.go`
  - Add route: `POST /llm/openai/generate`

---

### Day 1.2: Anthropic Provider with Tool Calling

**4. Create Anthropic provider with native SDK:**
- **File**: `/backend/api/llm/anthropic/schemas.go`
  - Define `AnthropicRequest` matching Anthropic's Messages API
  - Support messages array, tools, system prompt
  - Add `structuredOutput` field for JSON schema
  - Define `AnthropicResponse` matching Anthropic's response format

- **File**: `/backend/api/llm/anthropic/handler.go`
  - Create `HandleAnthropicGenerate(c echo.Context)` handler
  - Use `github.com/anthropics/anthropic-sdk-go` client directly
  - **For structured output:** Use Anthropic's recommended approach:
    ```go
    // Define tool with JSON schema
    tool := anthropic.ToolParam{
        Name: "structured_output",
        Description: "Return structured data",
        InputSchema: req.StructuredOutputSchema,
    }

    // Force tool use
    resp := client.Messages.New(ctx, anthropic.MessageNewParams{
        Model: req.Model,
        Messages: req.Messages,
        Tools: []anthropic.ToolParam{tool},
        ToolChoice: anthropic.ToolChoiceRequiredParam{
            Type: anthropic.ToolChoiceRequiredTypeRequired,
        },
    })

    // Extract structured output from tool call
    for _, block := range resp.Content {
        if toolUse := block.AsToolUse(); toolUse != nil {
            return toolUse.Input // This is the structured JSON
        }
    }
    ```
  - Return native Anthropic response format

**5. Create Anthropic endpoint:**
- **File**: `/backend/api/llm/routes.go`
  - Add route: `POST /llm/anthropic/generate`

---

### Day 2.1: Gemini Provider with Native SDK

**6. Update Gemini to use native SDK:**
- **File**: `/backend/api/llm/gemini/schemas.go`
  - Define `GeminiRequest` matching Gemini's GenerateContent API
  - Support contents array, tools, generationConfig
  - Define `GeminiResponse` matching Gemini's response format

- **File**: `/backend/api/llm/gemini/handler.go`
  - Replace HTTP-based implementation with `google.golang.org/genai`
  - Use native client for generation
  - For JSON mode: Use `responseMimeType: "application/json"`
  - Return native Gemini response format

**7. Create Gemini endpoint:**
- **File**: `/backend/api/llm/routes.go`
  - Add route: `POST /llm/gemini/generate`

---

### Day 2.2: Cleanup & Remove Old Code

**8. Remove Bellman dependency:**
- Update `go.mod`: Remove `github.com/modfin/bellman`
- Run `go mod tidy`

**9. Remove old unified code:**
- Delete `/backend/api/llm/provider/provider.go` (old LLMProvider interface)
- Delete `/backend/api/llm/provider/openai_provider.go` (Bellman-based)
- Delete `/backend/api/llm/provider/anthropic_provider.go` (Bellman-based)
- Delete `/backend/api/llm/provider/gemini_provider.go` (HTTP-based)
- Delete `/backend/api/llm/controller.go` (old HandleUnifiedLLMRequest)
- Delete `/backend/api/llm/schemas.go` (UnifiedLLMRequest/Response)
- Keep `/backend/api/llm/provider/provider_mapping.go` (OpenTel mappings still useful)
- Keep `/backend/api/llm/provider/models.go` (hardcoded model lists)
- Keep `/backend/api/llm/provider/model_cache.go` (caching logic)

**10. Update routes:**
- **File**: `/backend/api/llm/routes.go`
  - Remove: `POST /llm/v2/generate` → `HandleUnifiedLLMRequest`
  - Keep: `GET /llm/providers`, `GET /llm/models` (these can still work)
  - Add new routes registered in previous steps

**11. Update model fetching (optional):**
- Create provider-specific model fetch endpoints if needed:
  - `GET /llm/openai/models`
  - `GET /llm/anthropic/models`
  - `GET /llm/gemini/models`

---

## Phase 2: Frontend - Provider-Specific Requests (1.5 days)

### Day 3.1: Frontend Schemas

**12. Create OpenAI schemas:**
- **File**: `/frontend/src/features/prompt-playground/schemas/openai-request.ts`
  ```typescript
  export const OpenAIMessageSchema = z.object({
    role: z.enum(['system', 'user', 'assistant']),
    content: z.string(),
  })

  export const OpenAIRequestSchema = z.object({
    model: z.string(),
    messages: z.array(OpenAIMessageSchema),
    temperature: z.number().optional(),
    max_tokens: z.number().optional(),
    response_format: z.object({
      type: z.literal('json_object')
    }).optional(),
  })

  export const OpenAIResponseSchema = z.object({
    id: z.string(),
    choices: z.array(z.object({
      message: z.object({
        content: z.string(),
      }),
    })),
  })
  ```

**13. Create Anthropic schemas:**
- **File**: `/frontend/src/features/prompt-playground/schemas/anthropic-request.ts`
  ```typescript
  export const AnthropicMessageSchema = z.object({
    role: z.enum(['user', 'assistant']),
    content: z.string(),
  })

  export const AnthropicRequestSchema = z.object({
    model: z.string(),
    messages: z.array(AnthropicMessageSchema),
    system: z.string().optional(),
    max_tokens: z.number(),
    temperature: z.number().optional(),
    // For structured output
    tools: z.array(z.any()).optional(),
    tool_choice: z.any().optional(),
  })

  export const AnthropicResponseSchema = z.object({
    id: z.string(),
    content: z.array(z.object({
      type: z.string(),
      text: z.string().optional(),
      // For tool use
      name: z.string().optional(),
      input: z.any().optional(),
    })),
  })
  ```

**14. Create Gemini schemas:**
- **File**: `/frontend/src/features/prompt-playground/schemas/gemini-request.ts`
  ```typescript
  export const GeminiRequestSchema = z.object({
    model: z.string(),
    contents: z.array(z.object({
      parts: z.array(z.object({
        text: z.string(),
      })),
    })),
    generationConfig: z.object({
      temperature: z.number().optional(),
      maxOutputTokens: z.number().optional(),
      responseMimeType: z.string().optional(),
    }).optional(),
  })

  export const GeminiResponseSchema = z.object({
    candidates: z.array(z.object({
      content: z.object({
        parts: z.array(z.object({
          text: z.string(),
        })),
      }),
    })),
  })
  ```

---

### Day 3.2: Frontend Request Functions

**15. Create provider-specific request functions:**
- **File**: `/frontend/src/features/prompt-playground/fetch/openai-request.ts`
  ```typescript
  export const openaiRequest = async (payload: OpenAIRequest) => {
    const response = await fetch(`${API_HOST}/llm/openai/generate`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
      credentials: 'include',
    })
    // Handle response...
    return OpenAIResponseSchema.parse(await response.json())
  }
  ```

- **File**: `/frontend/src/features/prompt-playground/fetch/anthropic-request.ts`
  - Similar structure for Anthropic

- **File**: `/frontend/src/features/prompt-playground/fetch/gemini-request.ts`
  - Similar structure for Gemini

---

### Day 4.1: Frontend Integration

**16. Update playground component:**
- **File**: `/frontend/src/features/prompt-playground/PromptPlaygroundPage.tsx`
  - Update `handleSubmit` to build provider-specific requests:
    ```typescript
    const handleSubmit = async (event: React.FormEvent) => {
      event.preventDefault()
      const formData = new FormData(event.currentTarget)
      const prompt = formData.get('prompt') as string

      // Build provider-specific request
      if (selectedProvider === 'openai') {
        const request: OpenAIRequest = {
          model: selectedModel!,
          messages: [{ role: 'user', content: prompt }],
          ...(jsonMode && { response_format: { type: 'json_object' } }),
        }
        const result = await openaiRequest(request)
        dispatch(PromptPlaygroundActions.setOutput(result.choices[0].message.content))
      } else if (selectedProvider === 'anthropic') {
        const request: AnthropicRequest = {
          model: selectedModel!,
          messages: [{ role: 'user', content: prompt }],
          max_tokens: 4096,
          ...(jsonMode && {
            tools: [/* structured output tool */],
            tool_choice: { type: 'required' },
          }),
        }
        const result = await anthropicRequest(request)
        // Extract from tool call if JSON mode, otherwise from text
        dispatch(PromptPlaygroundActions.setOutput(/* ... */))
      } else if (selectedProvider === 'gemini') {
        const request: GeminiRequest = {
          model: selectedModel!,
          contents: [{ parts: [{ text: prompt }] }],
          ...(jsonMode && {
            generationConfig: { responseMimeType: 'application/json' },
          }),
        }
        const result = await geminiRequest(request)
        dispatch(PromptPlaygroundActions.setOutput(result.candidates[0].content.parts[0].text))
      }
    }
    ```

**17. Remove old unified code:**
- Delete `/frontend/src/features/prompt-playground/schemas/unified-request.ts`
- Delete `/frontend/src/features/prompt-playground/fetch/unified-llm-request.ts`
- Update imports throughout frontend

---

## Phase 3: Testing & Verification (0.5 day)

### Day 4.2: Integration Testing

**18. Test each provider:**
- OpenAI: Regular prompt + JSON mode (verify `response_format` works)
- Anthropic: Regular prompt + JSON mode (verify tool calling works, no markdown wrapping)
- Gemini: Regular prompt + JSON mode (verify `responseMimeType` works)

**19. Verify playground features:**
- Model selection works for each provider
- Provider switching works
- JSON mode toggle works correctly for each provider
- Error handling works (missing API keys, etc.)
- Response parsing works for each provider format

**20. Update documentation:**
- Document new API endpoints
- Document provider-specific schemas
- Document structured output approaches for each provider

---

## Files Changed Summary

### Backend (New Files)
- `/backend/api/llm/openai/schemas.go`
- `/backend/api/llm/openai/handler.go`
- `/backend/api/llm/anthropic/schemas.go`
- `/backend/api/llm/anthropic/handler.go`
- `/backend/api/llm/gemini/schemas.go`
- `/backend/api/llm/gemini/handler.go`

### Backend (Modified)
- `/backend/api/llm/routes.go` - New provider-specific routes
- `/backend/go.mod` - Add native SDKs, remove Bellman

### Backend (Deleted)
- `/backend/api/llm/provider/provider.go`
- `/backend/api/llm/provider/openai_provider.go`
- `/backend/api/llm/provider/anthropic_provider.go`
- `/backend/api/llm/provider/gemini_provider.go`
- `/backend/api/llm/controller.go`
- `/backend/api/llm/schemas.go`

### Frontend (New Files)
- `/frontend/src/features/prompt-playground/schemas/openai-request.ts`
- `/frontend/src/features/prompt-playground/schemas/anthropic-request.ts`
- `/frontend/src/features/prompt-playground/schemas/gemini-request.ts`
- `/frontend/src/features/prompt-playground/fetch/openai-request.ts`
- `/frontend/src/features/prompt-playground/fetch/anthropic-request.ts`
- `/frontend/src/features/prompt-playground/fetch/gemini-request.ts`

### Frontend (Modified)
- `/frontend/src/features/prompt-playground/PromptPlaygroundPage.tsx` - Provider-specific request handling

### Frontend (Deleted)
- `/frontend/src/features/prompt-playground/schemas/unified-request.ts`
- `/frontend/src/features/prompt-playground/fetch/unified-llm-request.ts`

---

## Benefits of New Architecture

1. ✅ **Transparency**: No abstraction - see exactly what each provider receives
2. ✅ **Native JSON mode**: Anthropic uses tool calling (no markdown wrapping)
3. ✅ **Full feature access**: All provider-specific capabilities available
4. ✅ **Production-aligned**: Matches real-world usage of each SDK
5. ✅ **Future-proof**: Provider updates immediately available via SDK updates
6. ✅ **Educational**: Users learn actual provider APIs

---

## Timeline: 4 Days Total

- **Day 1**: OpenAI + Anthropic backend (native SDKs, structured output)
- **Day 2**: Gemini backend + cleanup (remove Bellman, remove unified code)
- **Day 3**: Frontend schemas + request functions
- **Day 4**: Frontend integration (0.5 day) + Testing (0.5 day)

---

## Key Implementation Details

### Anthropic Structured Output (Best Practice)

Anthropic does NOT have a `response_format` parameter. The recommended approach is using tool calling:

```go
// 1. Define a tool with your desired JSON schema
tool := anthropic.ToolParam{
    Name:        "structured_output",
    Description: anthropic.String("Return data in structured format"),
    InputSchema: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "field1": map[string]string{"type": "string"},
            "field2": map[string]string{"type": "number"},
        },
        "required": []string{"field1", "field2"},
    },
}

// 2. Force the model to use this specific tool
resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
    Model:    anthropic.String(req.Model),
    MaxTokens: anthropic.Int(4096),
    Messages:  req.Messages,
    Tools:     []anthropic.ToolParam{tool},
    ToolChoice: anthropic.ToolChoiceRequiredParam{
        Type: anthropic.ToolChoiceRequiredTypeRequired,
    },
})

// 3. Extract the structured output from tool use
for _, block := range resp.Content {
    if toolUse := block.AsToolUse(); toolUse != nil {
        // toolUse.Input contains the structured JSON
        return toolUse.Input
    }
}
```

This approach:
- ✅ Returns pure JSON (no markdown wrapping)
- ✅ 100% reliable (enforced by API)
- ✅ Supports JSON schema validation
- ✅ Anthropic's official recommended method

### OpenAI Structured Output

OpenAI provides native `response_format` parameter:

```go
resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
    Model:    openai.String(req.Model),
    Messages: req.Messages,
    ResponseFormat: openai.ChatCompletionNewParamsResponseFormatJSONObject{
        Type: openai.ChatCompletionNewParamsResponseFormatTypeJSONObject,
    },
})
```

### Gemini Structured Output

Gemini provides `responseMimeType`:

```go
resp, err := client.GenerateContent(ctx, &genai.GenerateContentRequest{
    Model: req.Model,
    Contents: req.Contents,
    GenerationConfig: &genai.GenerationConfig{
        ResponseMimeType: "application/json",
    },
})
```

---

Ready to begin implementation!
