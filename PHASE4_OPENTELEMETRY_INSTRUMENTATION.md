# Phase 4: OpenTelemetry Instrumentation for Prompt Playground

## Overview
Add OpenTelemetry tracing and metrics to the new provider-specific LLM endpoints to enable observability and monitoring of playground usage.

---

## Goals

1. **Trace LLM requests**: Capture spans for each LLM generation request
2. **Provider-specific attributes**: Tag spans with provider, model, and configuration details
3. **Token usage metrics**: Record input/output tokens for cost tracking
4. **Error tracking**: Capture failures and API errors with context
5. **Performance monitoring**: Track latency per provider and model

---

## Implementation Plan

### 1. Backend Instrumentation

#### 1.1 Add OpenTelemetry to Provider Handlers

For each provider handler (`openai/handler.go`, `anthropic/handler.go`, `gemini/handler.go`):

**Before request:**
```go
ctx, span := otel.Tracer("junjo-server").Start(c.Request().Context(), "llm.generate")
defer span.End()

// Set standard LLM semantic conventions
span.SetAttributes(
    attribute.String("gen_ai.system", "openai"), // or "anthropic", "gemini"
    attribute.String("gen_ai.request.model", req.Model),
)
```

**After response:**
```go
// Record token usage
span.SetAttributes(
    attribute.Int("gen_ai.usage.input_tokens", resp.Usage.PromptTokens),
    attribute.Int("gen_ai.usage.output_tokens", resp.Usage.CompletionTokens),
)

// Record response metadata
span.SetAttributes(
    attribute.String("gen_ai.response.id", resp.ID),
    attribute.String("gen_ai.response.finish_reason", resp.Choices[0].FinishReason),
)
```

**On error:**
```go
span.RecordError(err)
span.SetStatus(codes.Error, err.Error())
```

#### 1.2 Semantic Conventions

Follow the [OpenInference GenAI semantic conventions](https://github.com/Arize-ai/openinference/blob/main/spec/semantic_conventions.md):

**Common attributes:**
- `gen_ai.system` - Provider name (`openai`, `anthropic`, `gemini`)
- `gen_ai.request.model` - Model identifier
- `gen_ai.request.temperature` - Temperature parameter
- `gen_ai.request.max_tokens` - Max output tokens
- `gen_ai.usage.input_tokens` - Input token count
- `gen_ai.usage.output_tokens` - Output token count
- `gen_ai.response.finish_reason` - Why generation stopped

**Provider-specific:**
- OpenAI: `gen_ai.openai.response_format.type` (`json_object` or `text`)
- Anthropic: `gen_ai.anthropic.tool_use` (boolean for tool calling)
- Gemini: `gen_ai.gemini.response_mime_type` (`application/json` for JSON mode)

#### 1.3 Example: OpenAI Handler with Telemetry

```go
func HandleOpenAIGenerate(c echo.Context) error {
    // Start span
    ctx, span := otel.Tracer("junjo-server").Start(c.Request().Context(), "llm.generate.openai")
    defer span.End()

    var req OpenAIRequest
    if err := c.Bind(&req); err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "failed to bind request")
        return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
    }

    // Set request attributes
    span.SetAttributes(
        attribute.String("gen_ai.system", "openai"),
        attribute.String("gen_ai.request.model", req.Model),
    )
    if req.Temperature != nil {
        span.SetAttributes(attribute.Float64("gen_ai.request.temperature", *req.Temperature))
    }
    if req.ResponseFormat != nil {
        span.SetAttributes(attribute.String("gen_ai.openai.response_format.type", req.ResponseFormat.Type))
    }

    // ... existing request logic ...

    // Set response attributes
    span.SetAttributes(
        attribute.Int("gen_ai.usage.input_tokens", int(resp.Usage.PromptTokens)),
        attribute.Int("gen_ai.usage.output_tokens", int(resp.Usage.CompletionTokens)),
        attribute.String("gen_ai.response.id", resp.ID),
        attribute.String("gen_ai.response.finish_reason", string(resp.Choices[0].FinishReason)),
    )
    span.SetStatus(codes.Ok, "generation successful")

    return c.JSON(http.StatusOK, response)
}
```

---

### 2. Metrics Collection

#### 2.1 Add Metrics for Token Usage

```go
var (
    llmRequestCounter = otelmetric.Int64Counter(
        "gen_ai.requests.total",
        metric.WithDescription("Total number of LLM requests"),
    )

    llmTokenUsage = otelmetric.Int64Histogram(
        "gen_ai.usage.tokens",
        metric.WithDescription("Token usage per request"),
    )

    llmLatency = otelmetric.Float64Histogram(
        "gen_ai.latency.duration",
        metric.WithDescription("LLM request latency in milliseconds"),
        metric.WithUnit("ms"),
    )
)

// In handler:
start := time.Now()

// ... make request ...

duration := time.Since(start).Milliseconds()
llmRequestCounter.Add(ctx, 1, metric.WithAttributes(
    attribute.String("provider", "openai"),
    attribute.String("model", req.Model),
))
llmTokenUsage.Record(ctx, int64(resp.Usage.TotalTokens), metric.WithAttributes(
    attribute.String("provider", "openai"),
    attribute.String("type", "total"),
))
llmLatency.Record(ctx, float64(duration), metric.WithAttributes(
    attribute.String("provider", "openai"),
    attribute.String("model", req.Model),
))
```

---

### 3. Frontend Instrumentation (Optional)

#### 3.1 Add Browser Tracing

Use OpenTelemetry Web SDK to trace frontend requests:

```typescript
import { WebTracerProvider } from '@opentelemetry/sdk-trace-web'
import { ZoneContextManager } from '@opentelemetry/context-zone'
import { FetchInstrumentation } from '@opentelemetry/instrumentation-fetch'

const provider = new WebTracerProvider()
provider.register({
  contextManager: new ZoneContextManager(),
})

// Auto-instrument fetch calls
const fetchInstrumentation = new FetchInstrumentation({
  propagateTraceHeaderCorsUrls: [/localhost:8080/],
})
```

---

## Testing Plan

1. **Trace Verification**: Confirm spans appear in trace viewer with correct attributes
2. **Metrics Dashboard**: Build Grafana dashboard showing:
   - Requests per provider/model
   - Token usage over time
   - Latency percentiles (p50, p95, p99)
   - Error rates
3. **Cost Tracking**: Use token metrics to estimate API costs per provider

---

## Future Enhancements

- **Sampling**: Configure trace sampling for high-volume production use
- **Baggage**: Propagate user context through distributed traces
- **Exemplars**: Link traces to metrics for drill-down analysis
- **Alerts**: Set up alerts for high latency or error rates

---

## References

- [OpenInference GenAI Semantic Conventions](https://github.com/Arize-ai/openinference/blob/main/spec/semantic_conventions.md)
- [OpenTelemetry Go SDK](https://opentelemetry.io/docs/languages/go/)
- [OpenTelemetry Semantic Conventions for GenAI](https://opentelemetry.io/docs/specs/semconv/gen-ai/)
