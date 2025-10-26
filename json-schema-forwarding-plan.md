# JSON Schema Forwarding in Prompt Playground

## Overview

This document outlines the plan to implement proper JSON schema detection and forwarding in the Prompt Playground when JSON Mode is enabled.

## Current State

### What Works
- **Gemini Detection**: We can detect `response_json_schema` from Gemini's `genai` library telemetry
- **User Visibility**: A banner and modal display the detected schema to users
- **Basic JSON Mode**: All three providers support basic unstructured JSON output
  - OpenAI: `response_format: { type: 'json_object' }`
  - Anthropic: Tool-based JSON via `jsonMode` flag
  - Gemini: `responseMimeType: 'application/json'`

### What's Broken
- **Schema Not Forwarded**: Detected schemas are displayed but NOT used in new playground requests
- **Incomplete Detection**: Only Gemini schemas are detected; OpenAI and Anthropic schemas are not detected from telemetry
- **Lost Structure**: Users see that a schema was used in the original request, but playground regenerations lose that structure

## Why This Matters

### User Experience Problem
1. User makes an LLM request with a typed schema (e.g., Pydantic model, TypeScript type)
2. Request is traced and appears in the UI
3. User clicks "Open in Playground" to experiment with the prompt
4. **Problem**: Playground shows the schema was used, but doesn't actually use it
5. **Result**: Playground outputs are unstructured and don't match the original request's behavior

### Observability Value
- Structured outputs are critical for production LLM applications
- Telemetry that captures schemas enables:
  - Reproducing exact request conditions in the playground
  - Understanding schema evolution over time
  - Debugging schema-related issues
  - Comparing outputs across different schema versions

## Technical Goals

### 1. Schema Detection from OpenTelemetry
Detect JSON schemas from all three providers using OpenInference semantic conventions.

**Reference**: [OpenInference Semantic Conventions](https://github.com/Arize-ai/openinference/blob/main/spec/semantic_conventions.md)

**Research Questions**:
- How does OpenAI structured outputs appear in OpenTelemetry traces?
  - What attribute keys are used for the schema?
  - Is it in `llm.invocation_parameters`?
  - What's the JSON structure?
- How does Anthropic tool-based JSON appear in traces?
  - Where is the `input_schema` stored?
  - Is it part of the tools array in invocation parameters?
  - Can we reconstruct the schema from tool definitions?
- Are there standardized OpenInference attributes for JSON schemas across providers?
- Should we extend our telemetry to ensure schemas are captured properly?

### 2. Schema Forwarding to Providers
When a schema is detected, forward it appropriately to each provider.

#### OpenAI - Structured Outputs
**Reference**: [OpenAI Structured Outputs Guide](https://platform.openai.com/docs/guides/structured-outputs)

**Implementation Requirements**:
- Add `json_schema` to `OpenAIResponseFormatSchema`
- Support `response_format: { type: 'json_schema', json_schema: { ... } }`
- Schema format: JSON Schema (draft 7 or later)
- Detect from telemetry: `llm.invocation_parameters.response_format.json_schema`

**Schema Structure**:
```json
{
  "type": "json_schema",
  "json_schema": {
    "name": "schema_name",
    "strict": true,
    "schema": {
      "type": "object",
      "properties": { ... },
      "required": [ ... ],
      "additionalProperties": false
    }
  }
}
```

#### Anthropic - Tool-Based JSON
**Reference**: [Anthropic Messages API](https://docs.claude.com/en/api/messages)

**Implementation Requirements**:
- Schema is provided via tool definitions
- Use the `input_schema` field in tool definitions
- Schema format: JSON Schema
- Tool-based approach:
  ```json
  {
    "tools": [{
      "name": "json_response",
      "description": "Return structured data",
      "input_schema": {
        "type": "object",
        "properties": { ... },
        "required": [ ... ]
      }
    }],
    "tool_choice": { "type": "tool", "name": "json_response" }
  }
  ```

**Detection Questions**:
- Where are tool definitions stored in OpenTelemetry traces?
- Is it `llm.invocation_parameters.tools`?
- Can we identify JSON-mode tool usage vs. general tool usage?

#### Gemini - JSON Schema Support
**Reference**: [Gemini Structured Output](https://ai.google.dev/gemini-api/docs/structured-output)

**Implementation Requirements**:
- Add `responseSchema` to `GeminiGenerationConfigSchema`
- Support both `responseMimeType` and `responseSchema` together
- Schema format: OpenAPI 3.0 schema (subset of JSON Schema)
- Already detecting `response_json_schema` from telemetry ✅

**Schema Structure**:
```json
{
  "generationConfig": {
    "responseMimeType": "application/json",
    "responseSchema": {
      "type": "OBJECT",
      "properties": {
        "field": { "type": "STRING" }
      },
      "required": ["field"]
    }
  }
}
```

**Note**: Gemini uses OpenAPI schema types (`STRING`, `INTEGER`, `OBJECT`) vs JSON Schema types (`string`, `integer`, `object`)

### 3. Schema Translation
Some schemas may need translation between formats:
- OpenAI/Anthropic use JSON Schema
- Gemini uses OpenAPI 3.0 schema (similar but different type names)
- Need schema conversion utilities for cross-provider compatibility

**Research Questions**:
- Can we use a detected Gemini schema for OpenAI requests?
- Can we use a detected OpenAI schema for Gemini requests?
- What are the compatibility constraints?
- Do we need a schema normalization layer?

## Implementation Phases

### Phase 1: Research & Telemetry Audit
1. Investigate OpenTelemetry traces from all three providers
2. Document where schemas appear in `llm.invocation_parameters`
3. Identify gaps in current telemetry capture
4. Create test traces with known schemas for each provider
5. Update `provider-warnings.ts` detection functions

### Phase 2: Schema Detection
1. Implement `detectOpenAIJsonSchema()` function
2. Implement `detectAnthropicJsonSchema()` function
3. Extend `detectGeminiJsonSchema()` if needed
4. Update UI to show schema info for all providers (not just Gemini)
5. Add schema preview in the playground UI

### Phase 3: Schema Storage
1. Add schema to Redux state (`promptPlaygroundState`)
2. Store detected schema when span loads
3. Preserve schema when switching providers (with translation)
4. Add UI controls to enable/disable schema usage

### Phase 4: Schema Forwarding
1. Update `OpenAIRequestSchema` to support `json_schema` response format
2. Update `AnthropicRequestSchema` to support tool-based schemas
3. Update `GeminiGenerationConfigSchema` to support `responseSchema`
4. Update request functions to include schemas when available
5. Handle schema format conversion between providers

### Phase 5: Schema Translation (Optional)
1. Build schema conversion utilities
2. Support using detected schemas across different providers
3. Handle edge cases and incompatibilities
4. Add warnings for unsupported schema features

### Phase 6: UI/UX Enhancements
1. Add schema editor for manual modifications
2. Show schema validation errors clearly
3. Compare outputs with/without schema
4. Add schema examples and templates

## Open Questions

### Telemetry Standards
- Does OpenInference define standard attributes for JSON schemas?
- Should we contribute back to OpenInference if we discover gaps?
- How do other observability tools handle schema capture?

### Schema Portability
- Can we reliably convert schemas between provider formats?
- What features are provider-specific and non-portable?
- Should we warn users when switching providers with incompatible schemas?

### User Control
- Should schema usage be automatic or opt-in?
- Should users be able to edit schemas in the playground?
- How do we handle invalid or unsupported schemas?

### Backend vs Frontend
- Should schema conversion happen in the backend or frontend?
- Do we need backend validation of schemas before forwarding?
- Should we store schemas separately for versioning/reuse?

## Success Criteria

1. ✅ Detect JSON schemas from all three providers' OpenTelemetry traces
2. ✅ Display detected schemas to users in the playground
3. ✅ Forward schemas correctly to each provider in playground requests
4. ✅ Playground outputs match original request structure when using schemas
5. ✅ Clear UX when schemas are detected and used
6. ✅ Graceful degradation when schemas can't be applied

## References

- [Anthropic Messages API](https://docs.claude.com/en/api/messages) - See `input_schema` for tool definitions
- [Gemini Structured Output](https://ai.google.dev/gemini-api/docs/structured-output) - OpenAPI 3.0 schema format
- [OpenAI Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs) - JSON Schema format
- [OpenInference Semantic Conventions](https://github.com/Arize-ai/openinference/blob/main/spec/semantic_conventions.md) - Telemetry standards

## Next Steps

1. Create test cases with known schemas for each provider
2. Capture and analyze OpenTelemetry traces
3. Document schema attribute locations in telemetry
4. Begin Phase 1 implementation
