# Junjo Server: System Architecture for LLM Agents

This document provides a high-level overview of the Junjo Server architecture, designed to help LLM agents quickly understand the system's components and interaction flows.

## 1. System Overview

The Junjo Server is a multi-service system designed for ingesting, storing, and analyzing OpenTelemetry (OTel) data. It consists of two primary backend services and client-side telemetry collection via the Junjo Otel Exporter.

*   **`backend`**: The main application server. It handles user authentication, provides the API and web UI, and processes telemetry data for analysis.
*   **`ingestion-service`**: A dedicated, high-throughput service responsible for receiving OTel data from clients and persisting it to a Write-Ahead Log (WAL).
*   **Junjo Otel Exporter**: A specialized OpenTelemetry exporter that sends telemetry data to the ingestion-service.

## 2. Component Architecture

The services are designed to be decoupled, with specific responsibilities to ensure scalability and resilience.

```mermaid
flowchart TD
 subgraph s1["ingestion-service"]
        B{"BadgerDB WAL"}
        C("Internal gRPC API")
        A("gRPC Server")
        IN{"API Key<br>Auth + Cache"}
        J["Backend Internal Auth"]
  end
 subgraph backend["backend"]
        D("Ingestion Client")
        E("OTel Span Processor")
        F{"DuckDB"}
        H{"QDrant"}
        K("API Key Validator")
  end
 subgraph clients["clients"]
        G("Junjo Otel Exporter")
  end
    C -- Reads --> B
    A -- Writes --> B
    A -- Intercept --> IN
    IN -- Forwards Auth --> J
    D -- gRPC Internal --> C
    D -- Processes & Batches --> E
    E -- Indexes --> F
    E -- Vectorizes & Stores --> H
    J -- Validates API Key --> K
    G -- OTel Data + API Key --> A

    style C fill:#cce5ff,stroke:#004085
    style A fill:#d4edda,stroke:#155724
    style J fill:#cce5ff,stroke:#004085
    style D fill:#cce5ff,stroke:#004085
    style G fill:#fff3cd,stroke:#856404
```

### `backend` Service

*   **Responsibilities**:
    *   Serves the main web UI and REST API on port `1323`.
    *   Manages user accounts and API keys.
    *   Provides an internal gRPC endpoint for validating API keys.
    *   Reads data from the `ingestion-service` to index it into a queryable database (DuckDB) and vector store (QDrant).
*   **Internal Authentication Endpoint**:
    *   `J[Backend Internal Auth]`: Private gRPC endpoint for validating API keys.
*   **Key Files**:
    *   [`backend/main.go`](backend/main.go): Main application entry point.
    *   [`backend/api/internal_auth/grpc_api_key_auth.go`](backend/api/internal_auth/grpc_api_key_auth.go): Internal gRPC API key validation service.

### `ingestion-service`

*   **Responsibilities**:
    *   Exposes a public gRPC server on port `50051` that serves as the single point of contact for clients.
    *   **Enforces Authentication**: Protects its OTel endpoints using an API key interceptor that validates and caches keys using the backend's internal auth endpoint.
    *   Persists all incoming data to a highly-performant Write-Ahead Log (WAL) using BadgerDB.
*   **Key Files**:
    *   [`ingestion-service/main.go`](ingestion-service/main.go): Main application entry point.
    *   [`ingestion-service/server/server.go`](ingestion-service/server/server.go): gRPC server setup.
    *   [`ingestion-service/server/api_key_interceptor.go`](ingestion-service/server/api_key_interceptor.go): The API key authentication and caching logic.
    *   [`ingestion-service/backend_client/auth_client.go`](ingestion-service/backend_client/auth_client.go): The client for the backend's internal API key validation service.

## 3. Authentication Flow (API Key-based)

Authentication is handled via direct API key validation with caching, facilitated by the `ingestion-service` to provide secure and efficient access to the OTel ingestion endpoint.

```mermaid
sequenceDiagram
    participant Client as OTel SDK Client
    participant Ingestion as ingestion-service
    participant Backend as backend

    loop OTel Data Export
        Client->>Ingestion: 1. Send OTel data via gRPC (Metadata: `x-junjo-api-key: <api_key>`)
        activate Ingestion
        Ingestion->>Ingestion: 2. Interceptor validates and caches API Key (using Backend Internal API)
        alt API Key Invalid or Expired
            Ingestion-->>Client: 3. Reject with Unauthenticated error
        else API Key Valid
            Ingestion->>Ingestion: 4. Write data to WAL
            Ingestion-->>Client: 5. Success
        end
        deactivate Ingestion
    end
```

### Step-by-Step Process:

1.  **Direct API Key Transfer**: The client sends its API key in the metadata header `x-junjo-api-key` along with OTel data directly to the ingestion service's gRPC server.
2.  **Interceptor Processing**: The ingestion service's API key interceptor intercepts the request and first checks its local cache for the API key.
3.  **Cache Check**: If the API key is found in the cache and is not expired, the request proceeds to be written to the WAL.
4.  **Backend Validation**: If the API key is not in the cache or is expired, the ingestion service forwards the API key to the backend's internal authentication gRPC service for validation.
5.  **Validation Response**: The backend checks the API key against its database and returns the validation result.
6.  **Cache Update**: The ingestion service updates its cache with the validation result, including an expiration time.
7.  **Access Control**: If the API key is valid (either from cache or backend), the request is allowed to proceed and the OTel data is written to the WAL. Otherwise, it is rejected with an `Unauthenticated` error.

## 4. Data Flow: WAL and Indexing

The `ingestion-service` acts as a Write-Ahead Log (WAL) for the main `backend`. This decouples the high-throughput ingestion of OTel data from the more resource-intensive process of indexing that data for querying.

### Step-by-Step Process:

1.  **Write to WAL**: The `ingestion-service` receives OTel data via its public gRPC endpoint and immediately writes the raw, serialized data to a BadgerDB WAL. This is a fast, append-only operation.

2.  **Internal Read API**: The `ingestion-service` exposes a second, internal-only gRPC service (`WALReaderService`) that allows the `backend` to read data from the WAL in batches.

3.  **Client Polling**: The `backend`'s `ingestion_client` periodically polls the `WALReaderService`, requesting a batch of spans starting from the last key it successfully processed.

4.  **State Management**: The `backend` is responsible for persisting the key of the last span it indexed. This ensures that if the `backend` restarts, it can resume processing from where it left off without missing any data.

5.  **Processing and Indexing**: Once the `backend` receives a batch of spans, it uses its `otel_span_processor` to deserialize, process, and index the data into a DuckDB database and vector store (QDrant), making it available for querying via the main API.

This pull-based architecture makes the system resilient. The `ingestion-service` can continue to accept data even if the `backend` is temporarily down or slow to index.

## 5. OpenInference Semantic Conventions

Junjo Server uses **OpenInference** semantic conventions for LLM observability. OpenInference is a standardized format for capturing LLM-related telemetry data within OpenTelemetry spans.

### Key OpenInference Attributes

The system expects and utilizes the following OpenInference attributes on LLM spans:

*   **`llm.provider`**: The LLM provider (e.g., `"openai"`, `"anthropic"`, `"google"`)
    *   **Note**: OpenInference uses `"google"` for Gemini, but internally Junjo maps this to `"gemini"`
*   **`llm.model_name`**: The specific model identifier (e.g., `"gpt-4o"`, `"claude-3-5-sonnet-20241022"`, `"gemini-2.0-flash-exp"`)
*   **`llm.invocation_parameters`**: A JSON string containing all request parameters used during the LLM invocation
    *   Example: `'{"temperature": 0.7, "max_tokens": 2048, "reasoning_effort": "medium"}'`
    *   This is the **primary source** for generation settings in the Prompt Playground
*   **`input.mime_type`**: The MIME type of the input (e.g., `"application/json"` indicates JSON mode)
*   **`output.mime_type`**: The MIME type of the output
*   **`input.value`**: The actual prompt text sent to the LLM
*   **`output.value`**: The generated response from the LLM

### Provider Mapping

The system includes utilities for mapping between OpenInference provider names and internal representations:

*   **Frontend**: `frontend/src/features/prompt-playground/utils/provider-mapping.ts`
*   **Backend**: `backend/api/llm/provider/provider_mapping.go`

Current mappings:
*   `"google"` (OpenInference) → `"gemini"` (Junjo internal)
*   `"openai"` → `"openai"` (no mapping needed)
*   `"anthropic"` → `"anthropic"` (no mapping needed)

## 6. Prompt Playground Architecture

The Prompt Playground is a feature that allows users to replay, modify, and experiment with LLM requests captured from their applications.

### Core Concept

When a user clicks on an LLM span in the trace viewer, they can open the Prompt Playground to:
1. See the original prompt and model configuration
2. Modify the prompt and settings
3. Re-run the request against the same or different models/providers
4. Compare outputs

### Data Flow

```mermaid
sequenceDiagram
    participant User
    participant Frontend as Frontend (React)
    participant Backend as Backend API
    participant Provider as LLM Provider APIs

    User->>Frontend: Click LLM span
    Frontend->>Backend: GET /otel/trace/{traceId}/span/{spanId}
    Backend-->>Frontend: Span data with attributes
    Frontend->>Frontend: Extract llm.provider, llm.model_name, llm.invocation_parameters
    Frontend->>Frontend: Parse settings from llm.invocation_parameters
    User->>Frontend: Modify prompt/settings
    User->>Frontend: Click "Run"
    Frontend->>Backend: POST /llm/{provider}/generate
    Backend->>Provider: LLM API request
    Provider-->>Backend: Response
    Backend-->>Frontend: Generated output
    Frontend->>User: Display results
```

### Provider-Specific Endpoints

The backend provides separate REST endpoints for each LLM provider:

*   **OpenAI**: `POST /llm/openai/generate`
    *   Handler: `backend/api/llm/openai/handler.go`
    *   Schema: `backend/api/llm/openai/schemas.go`
*   **Anthropic**: `POST /llm/anthropic/generate`
    *   Handler: `backend/api/llm/anthropic/handler.go`
    *   Schema: `backend/api/llm/anthropic/schemas.go`
*   **Gemini**: `POST /llm/gemini/generate`
    *   Handler: `backend/api/llm/gemini/handler.go`
    *   Schema: `backend/api/llm/gemini/schemas.go`

Each handler:
1. Validates the request using Go structs
2. Retrieves the API key from environment variables
3. Forwards the request to the provider's API
4. Returns the response (or error) to the frontend

### Frontend Architecture

**Main Component**: `frontend/src/features/prompt-playground/PromptPlaygroundPage.tsx`

**Key State Management**:
*   Uses Redux Toolkit for state: `frontend/src/features/prompt-playground/store/slice.ts`
*   State includes:
    *   `selectedModel`: Currently selected model
    *   `selectedProvider`: Currently selected provider
    *   `jsonMode`: Whether JSON mode is enabled
    *   `generationSettings`: Provider-specific generation parameters

**Request Schemas**: `frontend/src/features/prompt-playground/schemas/`
*   `openai-request.ts`: Zod schemas for OpenAI requests/responses
*   `anthropic-request.ts`: Zod schemas for Anthropic requests/responses
*   `gemini-request.ts`: Zod schemas for Gemini requests/responses

### Generation Settings

The Prompt Playground supports provider-specific and model-specific generation settings:

#### Settings Import from OpenInference

When a span is loaded, settings are imported from `llm.invocation_parameters`:

```typescript
// Example llm.invocation_parameters value:
{
  "temperature": 0.7,
  "max_completion_tokens": 4096,
  "reasoning_effort": "medium"  // OpenAI reasoning models only
}
```

The frontend parses this JSON and populates the settings UI.

#### Provider-Specific Settings

**OpenAI**:
*   `reasoning_effort`: `"minimal" | "low" | "medium" | "high"` (reasoning models only: o1, o3, o4, gpt-5, but NOT o1-mini)
*   `max_completion_tokens`: Number (all models)
*   `temperature`: Number 0-2 (non-reasoning models only)

**Anthropic**:
*   `thinking_enabled`: Boolean (enables extended thinking)
*   `thinking_budget_tokens`: Number ≥1024 (requires thinking_enabled)
*   `temperature`: Number 0-1
*   `max_tokens`: Number (required by API)

**Gemini**:
*   `thinkingBudget`: Number (-1 = dynamic, 0 = disabled, 1-32768 = specific budget)
    *   **Important**: This must be nested in `generationConfig.thinkingConfig.thinkingBudget`
    *   Only supported on Gemini 2.5 series models (2.5-pro, 2.5-flash, 2.5-flash-lite)
*   `includeThoughts`: Boolean (includes thought summaries in response)
    *   **Important**: This must be nested in `generationConfig.thinkingConfig.includeThoughts`
*   `temperature`: Number 0-2
*   `maxOutputTokens`: Number

Example Gemini request structure with thinking:
```json
{
  "model": "gemini-2.5-flash",
  "contents": [...],
  "generationConfig": {
    "temperature": 0.7,
    "maxOutputTokens": 2048,
    "thinkingConfig": {
      "thinkingBudget": 1024,
      "includeThoughts": true
    }
  }
}
```

### Model Capability Detection

Different models support different parameters. The playground uses pattern matching to determine capabilities:

**OpenAI Reasoning Models**: `/^(o1-(?!mini)|o3-|o4-|gpt-5)/`
*   Supports: `reasoning_effort`, `max_completion_tokens`
*   Does NOT support: `temperature`
*   Note: `o1-mini` is excluded as it doesn't support `reasoning_effort`

**Gemini Thinking Models**: `/^gemini-2\.5-/`
*   Only 2.5 series supports thinking
*   2.0, 1.5, and earlier do NOT support thinking

**Anthropic Extended Thinking Models**: Claude Sonnet 4.x/3.7, Haiku 4.x, Opus 4.x
*   Older models like `claude-3-5-sonnet` do NOT support extended thinking

### Settings Reset Behavior

**Critical Implementation Detail**: The playground follows a "reset on change" pattern:

1. When the user changes the selected model or provider, ALL settings are reset to `{}`
2. Settings are then re-imported from `llm.invocation_parameters` if available
3. No attempt is made to preserve or migrate settings between model selections

This is implemented in `PromptPlaygroundPage.tsx` using a `useEffect` that watches `selectedModel` and `selectedProvider`.

### UI Components

**GenerationSettingsModal** (`frontend/src/features/prompt-playground/components/GenerationSettingsModal.tsx`):
*   Provider-specific sections with conditional rendering
*   Model detection to hide incompatible settings
*   Plain HTML form elements styled with Tailwind CSS

**ActiveSettingsDisplay** (`frontend/src/features/prompt-playground/components/ActiveSettingsDisplay.tsx`):
*   Shows active (non-default) settings as compact key-value pairs
*   Displayed between the prompt textarea and provider/model selectors

### Future Enhancement: OpenTelemetry Instrumentation

Currently, the playground endpoints (`/llm/{provider}/generate`) do NOT emit OpenTelemetry spans. Future work (documented in `PHASE4_OPENTELEMETRY_INSTRUMENTATION.md`) will add:
*   Span creation for each playground request
*   `llm.invocation_parameters` attribute with all request settings
*   Token usage metrics
*   Error tracking

This will allow playground usage to be captured and analyzed like production LLM calls.