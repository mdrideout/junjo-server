# Native Provider SDK Migration Plan V2
## Direct HTTP Implementation (Maximum Transparency for OpenTelemetry)

---

## Executive Summary

**Migration Goal**: Replace Bellman-based unified LLM abstraction with provider-specific native endpoints for maximum transparency and full feature access, optimized for OpenTelemetry tracing in a playground/proxy use case.

**Strategy Change**: Originally planned to use official Go SDKs (openai-go, anthropic-sdk-go, genai), but encountered complex type system issues. **Pivoted to direct HTTP calls** because:
1. **Better for OpenTelemetry**: Standard HTTP instrumentation captures full request/response bodies
2. **Maximum transparency**: See exact JSON being sent to providers (critical for playground debugging)
3. **Simpler code**: No complex SDK type wrangling
4. **Already proven**: Current Gemini implementation uses HTTP successfully

---

## Current State (What's Done ✅)

### Backend Structure (KEEP - All Good)
```
backend/api/llm/
├── openai/
│   ├── schemas.go          ✅ Provider-specific types (KEEP)
│   └── handler.go          ⚠️  Uses SDK - needs HTTP rewrite
├── anthropic/
│   ├── schemas.go          ✅ Provider-specific types (KEEP)
│   └── handler.go          ⚠️  Uses SDK - needs HTTP rewrite
├── gemini/
│   ├── schemas.go          ✅ Provider-specific types (KEEP)
│   └── handler.go          ⚠️  Has unused import - minor fix
├── provider/
│   ├── types.go            ✅ ModelInfo, ProviderType (KEEP)
│   ├── models.go           ✅ Hardcoded model lists (KEEP)
│   ├── model_cache.go      ✅ Caching logic (KEEP)
│   └── provider_mapping.go ✅ OpenTel mappings (KEEP)
└── routes.go               ✅ Provider endpoints registered (KEEP)
```

**Removed** (cleanup successful):
- ❌ `controller.go` (old HandleUnifiedLLMRequest)
- ❌ `schemas.go` (old UnifiedLLMRequest/Response)
- ❌ `services.go` (old GeminiService)
- ❌ `provider/provider.go` (old LLMProvider interface)
- ❌ `provider/*_provider.go` (all Bellman-based providers)
- ❌ Bellman dependency from go.mod

### Frontend (COMPLETE - All Working ✅)
```
frontend/src/features/prompt-playground/
├── schemas/
│   ├── openai-request.ts      ✅ DONE
│   ├── anthropic-request.ts   ✅ DONE
│   └── gemini-request.ts      ✅ DONE
├── fetch/
│   ├── openai-request.ts      ✅ DONE
│   ├── anthropic-request.ts   ✅ DONE
│   └── gemini-request.ts      ✅ DONE
└── PromptPlaygroundPage.tsx   ✅ Provider-specific logic DONE
```

**Frontend is ready** - no changes needed after backend fix.

### Routes Registered (WORKING ✅)
```go
// In routes.go
e.POST("/llm/openai/generate", openai.HandleOpenAIGenerate)
e.POST("/llm/anthropic/generate", anthropic.HandleAnthropicGenerate)
e.POST("/llm/gemini/generate", gemini.HandleGeminiGenerate)

// Discovery endpoints still work
e.GET("/llm/providers", HandleGetProviders)
e.GET("/llm/models", HandleGetModels)
```

---

## Current Compilation Errors

### OpenAI Handler Errors:
```
api/llm/openai/handler.go:80:34: undefined: openai.ChatCompletionNewParamsResponseFormatJSONObject
api/llm/openai/handler.go:81:17: undefined: openai.ResponseFormatJSONObjectTypeJSONObject
```

### Anthropic Handler Errors:
```
api/llm/anthropic/handler.go:57:14: cannot use req.Model (variable of type string) as "github.com/anthropics/anthropic-sdk-go".Model value
api/llm/anthropic/handler.go:64:19: cannot use anthropic.NewTextBlock(...) as []TextBlockParam value
api/llm/anthropic/handler.go:83:17: cannot use "..." as param.Opt[string] value
api/llm/anthropic/handler.go:98:20: undefined: anthropic.ToolChoiceAnyTypeAny
api/llm/anthropic/handler.go:149:15: cannot use resp.Model (string type Model) as string value
```

### Gemini Handler:
```
api/llm/gemini/handler.go:5:2: "fmt" imported and not used
```
(Minor - just remove unused import)

**Root cause**: SDK type systems are complex, not worth the effort for our use case.

---

## Solution: Direct HTTP Implementation

### Reference Implementation (Already Working)

**Your old Gemini code** (before SDK attempt) in `backend/api/llm/services.go:23-53`:
```go
func (s *GeminiService) GenerateContent(requestBody GeminiRequest) ([]byte, error) {
    apiKey := os.Getenv("GEMINI_API_KEY")
    if apiKey == "" {
        return nil, fmt.Errorf("GEMINI_API_KEY environment variable is not set")
    }

    apiURL := fmt.Sprintf("%s%s:generateContent", geminiAPIBaseURL, requestBody.Model)

    jsonData, err := json.Marshal(requestBody)
    if err != nil {
        return nil, err
    }

    req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
    if err != nil {
        return nil, err
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("x-goog-api-key", apiKey)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    return io.ReadAll(resp.Body)
}
```

**This is the pattern to follow** for OpenAI and Anthropic!

---

## Implementation Guide

### Step 1: Rewrite OpenAI Handler (15 min)

**File**: `/backend/api/llm/openai/handler.go`

**Keep**: The schema types in `schemas.go` - they're perfect
**Replace**: The handler implementation

**New implementation**:
```go
package openai

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "strings"

    "github.com/labstack/echo/v4"
)

func HandleOpenAIGenerate(c echo.Context) error {
    var req OpenAIRequest
    if err := c.Bind(&req); err != nil {
        c.Logger().Error("Failed to bind request: ", err)
        return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
    }

    // Validate required fields
    if req.Model == "" {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "model is required"})
    }
    if len(req.Messages) == 0 {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "messages are required"})
    }

    // Get API key
    apiKey := os.Getenv("OPENAI_API_KEY")
    if apiKey == "" {
        return c.JSON(http.StatusServiceUnavailable, map[string]string{
            "error": "OPENAI_API_KEY environment variable is not set",
        })
    }

    // Build request body (matches OpenAI API exactly)
    requestBody := map[string]interface{}{
        "model":    req.Model,
        "messages": req.Messages,
    }

    // Add optional parameters
    if req.Temperature != nil {
        requestBody["temperature"] = *req.Temperature
    }
    if req.MaxTokens != nil {
        requestBody["max_tokens"] = *req.MaxTokens
    }
    if req.TopP != nil {
        requestBody["top_p"] = *req.TopP
    }
    if len(req.StopSequences) > 0 {
        requestBody["stop"] = req.StopSequences
    }

    // Add response format for JSON mode
    if req.ResponseFormat != nil && req.ResponseFormat.Type == "json_object" {
        requestBody["response_format"] = map[string]string{
            "type": "json_object",
        }
    }

    // Marshal to JSON
    jsonData, err := json.Marshal(requestBody)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
    }

    // Create HTTP request
    httpReq, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
    }

    // Set headers
    httpReq.Header.Set("Authorization", "Bearer "+apiKey)
    httpReq.Header.Set("Content-Type", "application/json")

    // Make request
    client := &http.Client{}
    resp, err := client.Do(httpReq)
    if err != nil {
        c.Logger().Error("Failed to generate content: ", err)
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
    }
    defer resp.Body.Close()

    // Read response
    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
    }

    // Check for errors
    if resp.StatusCode != http.StatusOK {
        c.Logger().Error("OpenAI API error: ", string(respBody))
        if strings.Contains(string(respBody), "api key") {
            return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": string(respBody)})
        }
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": string(respBody)})
    }

    // Parse response into our schema
    var openaiResp OpenAIResponse
    if err := json.Unmarshal(respBody, &openaiResp); err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
    }

    return c.JSON(http.StatusOK, openaiResp)
}
```

**Key points**:
- Uses standard `net/http` - no SDK
- Builds JSON request matching OpenAI API docs
- Returns response using our `OpenAIResponse` schema
- Error handling for API keys and HTTP errors

---

### Step 2: Rewrite Anthropic Handler (20 min)

**File**: `/backend/api/llm/anthropic/handler.go`

**Special consideration**: Anthropic's JSON mode uses **tool calling**, not a response_format parameter.

**New implementation**:
```go
package anthropic

import (
    "bytes"
    "context"
    "encoding/json"
    "io"
    "net/http"
    "os"
    "strings"

    "github.com/labstack/echo/v4"
)

func HandleAnthropicGenerate(c echo.Context) error {
    var req AnthropicRequest
    if err := c.Bind(&req); err != nil {
        c.Logger().Error("Failed to bind request: ", err)
        return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
    }

    // Validate required fields
    if req.Model == "" {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "model is required"})
    }
    if len(req.Messages) == 0 {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "messages are required"})
    }
    if req.MaxTokens == 0 {
        req.MaxTokens = 4096 // Default
    }

    // Get API key
    apiKey := os.Getenv("ANTHROPIC_API_KEY")
    if apiKey == "" {
        return c.JSON(http.StatusServiceUnavailable, map[string]string{
            "error": "ANTHROPIC_API_KEY environment variable is not set",
        })
    }

    // Build request body (matches Anthropic API exactly)
    requestBody := map[string]interface{}{
        "model":      req.Model,
        "messages":   req.Messages,
        "max_tokens": req.MaxTokens,
    }

    // Add system prompt if provided
    if req.System != "" {
        requestBody["system"] = req.System
    }

    // Add optional parameters
    if req.Temperature != nil {
        requestBody["temperature"] = *req.Temperature
    }
    if req.TopP != nil {
        requestBody["top_p"] = *req.TopP
    }
    if len(req.StopSequences) > 0 {
        requestBody["stop_sequences"] = req.StopSequences
    }

    // Handle JSON mode using tool calling (Anthropic's recommended approach)
    if req.JSONMode {
        requestBody["tools"] = []map[string]interface{}{
            {
                "name":        "structured_output",
                "description": "Return data in structured JSON format",
                "input_schema": map[string]interface{}{
                    "type": "object",
                    "properties": map[string]interface{}{
                        "output": map[string]interface{}{
                            "type":        "object",
                            "description": "The structured JSON response",
                        },
                    },
                    "required": []string{"output"},
                },
            },
        }
        requestBody["tool_choice"] = map[string]string{"type": "any"}
    }

    // Marshal to JSON
    jsonData, err := json.Marshal(requestBody)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
    }

    // Create HTTP request
    httpReq, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
    }

    // Set headers (Anthropic-specific)
    httpReq.Header.Set("x-api-key", apiKey)
    httpReq.Header.Set("anthropic-version", "2023-06-01")
    httpReq.Header.Set("content-type", "application/json")

    // Make request
    client := &http.Client{}
    resp, err := client.Do(httpReq)
    if err != nil {
        c.Logger().Error("Failed to generate content: ", err)
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
    }
    defer resp.Body.Close()

    // Read response
    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
    }

    // Check for errors
    if resp.StatusCode != http.StatusOK {
        c.Logger().Error("Anthropic API error: ", string(respBody))
        if strings.Contains(string(respBody), "api key") {
            return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": string(respBody)})
        }
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": string(respBody)})
    }

    // Parse response into our schema
    var anthropicResp AnthropicResponse
    if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
    }

    return c.JSON(http.StatusOK, anthropicResp)
}
```

**Key points**:
- Anthropic uses `x-api-key` header (not Bearer token)
- Requires `anthropic-version` header
- JSON mode implemented via tool calling (type: "any")
- Frontend will extract from `content[].input` for JSON responses

---

### Step 3: Fix Gemini Handler (5 min)

**File**: `/backend/api/llm/gemini/handler.go`

**Changes needed**:
1. Remove unused `fmt` import (line 5)
2. Simplify to single-turn generation (primary playground use case)

**Quick fix**:
```go
// At top, remove:
// import "fmt"

// Keep existing HTTP implementation, just simplify the contents handling
// Replace the complex multi-turn logic (lines 88-237) with simple single-turn:

func HandleGeminiGenerate(c echo.Context) error {
    var req GeminiRequest
    if err := c.Bind(&req); err != nil {
        c.Logger().Error("Failed to bind request: ", err)
        return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
    }

    if req.Model == "" {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "model is required"})
    }
    if len(req.Contents) == 0 {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "contents are required"})
    }

    apiKey := os.Getenv("GEMINI_API_KEY")
    if apiKey == "" {
        return c.JSON(http.StatusServiceUnavailable, map[string]string{
            "error": "GEMINI_API_KEY environment variable is not set",
        })
    }

    // Build request body
    requestBody := map[string]interface{}{
        "contents": req.Contents,
    }

    if req.GenerationConfig != nil {
        requestBody["generationConfig"] = req.GenerationConfig
    }
    if req.SystemInstruction != nil {
        requestBody["system_instruction"] = req.SystemInstruction
    }

    jsonData, err := json.Marshal(requestBody)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
    }

    // Make HTTP request
    url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
        req.Model, apiKey)

    httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
    }
    httpReq.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(httpReq)
    if err != nil {
        c.Logger().Error("Failed to generate content: ", err)
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
    }
    defer resp.Body.Close()

    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
    }

    if resp.StatusCode != http.StatusOK {
        c.Logger().Error("Gemini API error: ", string(respBody))
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": string(respBody)})
    }

    var geminiResp GeminiResponse
    if err := json.Unmarshal(respBody, &geminiResp); err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
    }

    return c.JSON(http.StatusOK, geminiResp)
}
```

**Add missing imports**:
```go
import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"

    "github.com/labstack/echo/v4"
)
```

---

### Step 4: Remove SDK Dependencies (5 min)

```bash
cd /Users/matt/repos/junjo-server/backend

# Remove SDK packages
go mod edit -droprequire github.com/openai/openai-go
go mod edit -droprequire github.com/anthropics/anthropic-sdk-go
go mod edit -droprequire github.com/google/generative-ai-go

# Clean up
go mod tidy
```

**Verify**:
```bash
grep -E "(openai-go|anthropic-sdk-go|generative-ai-go)" go.mod
# Should return nothing
```

---

### Step 5: Build & Verify (10 min)

```bash
cd /Users/matt/repos/junjo-server/backend

# Should compile cleanly
go build ./...

# If successful, run the server
go run main.go
```

**Expected**: No compilation errors, server starts successfully.

---

## Frontend Alignment (Already Done ✅)

The frontend in `PromptPlaygroundPage.tsx` already handles provider-specific responses correctly:

### OpenAI Response Extraction (line 207):
```typescript
const result = await openaiRequest({...})
outputText = result.choices[0]?.message?.content || ''
```

### Anthropic Response Extraction (lines 217-225):
```typescript
const result = await anthropicRequest({...})
if (jsonMode) {
    const toolUseBlock = result.content.find((block) => block.type === 'tool_use')
    if (toolUseBlock && toolUseBlock.input) {
        outputText = JSON.stringify(toolUseBlock.input, null, 2)
    }
} else {
    const textBlock = result.content.find((block) => block.type === 'text')
    outputText = textBlock?.text || ''
}
```

### Gemini Response Extraction (line 234):
```typescript
const result = await geminiRequest({...})
outputText = result.candidates[0]?.content?.parts[0]?.text || ''
```

**No frontend changes needed** - it already matches the backend response schemas!

---

## Testing Plan

### Manual Testing via Playground

1. **Start backend**:
   ```bash
   cd backend && go run main.go
   ```

2. **Start frontend**:
   ```bash
   cd frontend && npm run dev
   ```

3. **Test each provider**:
   - Navigate to a trace span in the playground
   - Test **OpenAI** with and without JSON mode
   - Test **Anthropic** with and without JSON mode
   - Test **Gemini** with and without JSON mode

4. **Verify**:
   - ✅ Responses appear in output field
   - ✅ JSON mode returns valid JSON (not markdown-wrapped)
   - ✅ Error messages are clear (e.g., "API key not set")
   - ✅ Model selection works
   - ✅ Provider switching works

### API Testing (curl)

**OpenAI**:
```bash
curl http://localhost:8080/llm/openai/generate \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

**Anthropic**:
```bash
curl http://localhost:8080/llm/anthropic/generate \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5",
    "messages": [{"role": "user", "content": "Hello"}],
    "max_tokens": 1024
  }'
```

**Gemini**:
```bash
curl http://localhost:8080/llm/gemini/generate \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-2.5-flash",
    "contents": [{"parts": [{"text": "Hello"}]}]
  }'
```

---

## Environment Variables Required

Make sure these are set in `.env`:
```bash
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
GEMINI_API_KEY=...
```

**Without these**, handlers will return 503 Service Unavailable.

---

## Common Issues & Solutions

### Issue 1: "OPENAI_API_KEY environment variable is not set"
**Solution**: Add API key to `.env` file

### Issue 2: Compilation errors about SDK types
**Solution**: Make sure you ran `go mod tidy` after removing SDK dependencies

### Issue 3: Frontend shows empty response
**Solution**: Check browser console for errors, verify API endpoint URLs match

### Issue 4: JSON mode doesn't work
**Solutions**:
- **OpenAI**: Verify `response_format.type = "json_object"` in request
- **Anthropic**: Verify response has `content[].type = "tool_use"`, extract from `.input`
- **Gemini**: Verify `generationConfig.responseMimeType = "application/json"`

---

## OpenTelemetry Instrumentation (Future - Phase 4)

Once handlers are working, add HTTP instrumentation:

```go
import "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

// In each handler, wrap the HTTP client:
client := &http.Client{
    Transport: otelhttp.NewTransport(http.DefaultTransport),
}

// This automatically creates spans with:
// - http.method
// - http.url
// - http.status_code
// - Full request/response bodies (configurable)
```

See `PHASE4_OPENTELEMETRY_INSTRUMENTATION.md` for complete guide.

---

## Success Criteria

✅ **Compilation**: `go build ./...` completes with no errors
✅ **Backend starts**: Server runs on port 8080
✅ **OpenAI works**: Can generate text and JSON responses
✅ **Anthropic works**: Can generate text and JSON (via tool calling)
✅ **Gemini works**: Can generate text and JSON responses
✅ **Frontend works**: Playground displays all responses correctly
✅ **Provider switching**: Can switch between providers without errors
✅ **Error handling**: Clear error messages for missing API keys

---

## Why Direct HTTP > SDKs for This Use Case

| Requirement | Direct HTTP | SDKs |
|-------------|-------------|------|
| **OpenTelemetry traces** | ✅ Full HTTP visibility | ❌ SDK internals hidden |
| **Request transparency** | ✅ See exact JSON | ❌ Abstracted by types |
| **Debugging** | ✅ Inspect JSON easily | ❌ Dive into SDK source |
| **Code complexity** | ✅ Simple maps/JSON | ❌ Complex type systems |
| **Dependencies** | ✅ Just net/http | ❌ 3 large SDKs |
| **Provider updates** | ✅ Immediate | ❌ Wait for SDK release |
| **Educational value** | ✅ See real APIs | ❌ SDK abstractions |
| **Playground use** | ✅ Perfect fit | ⚠️  Overkill |

---

## Estimated Completion Time

- **Rewrite OpenAI handler**: 15 minutes
- **Rewrite Anthropic handler**: 20 minutes
- **Fix Gemini handler**: 5 minutes
- **Remove SDK deps**: 5 minutes
- **Build & test**: 10 minutes

**Total**: ~60 minutes to working implementation

---

## Files to Modify (Summary)

| File | Action | Time |
|------|--------|------|
| `backend/api/llm/openai/handler.go` | Rewrite to HTTP | 15 min |
| `backend/api/llm/anthropic/handler.go` | Rewrite to HTTP | 20 min |
| `backend/api/llm/gemini/handler.go` | Simplify + fix imports | 5 min |
| `backend/go.mod` | Remove SDK deps | 5 min |
| **Frontend** | **No changes needed** | **0 min** |

---

## Next Steps (Start Here)

1. **Backup current work** (optional but recommended):
   ```bash
   git add -A
   git commit -m "WIP: SDK migration before HTTP pivot"
   ```

2. **Follow implementation steps above** in order:
   - Step 1: OpenAI handler
   - Step 2: Anthropic handler
   - Step 3: Gemini handler
   - Step 4: Remove dependencies
   - Step 5: Build & test

3. **Test via playground** - all three providers

4. **Mark todo complete**: Manual testing of all three providers

5. **Future**: Implement Phase 4 (OpenTelemetry instrumentation)

---

## Questions or Issues?

If you encounter problems:

1. **Check compilation errors** - look for SDK import references
2. **Verify API keys** - all three must be set
3. **Test curl first** - isolate backend vs. frontend issues
4. **Check response parsing** - verify JSON unmarshal matches schemas
5. **Review this doc** - all implementation details are above

**Remember**: Frontend is complete and correct. Focus on backend handlers only.
