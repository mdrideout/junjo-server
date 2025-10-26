# Schema Detection Bug Fix & Test Implementation Plan

**Date**: 2025-10-26
**Status**: Planned (Not Yet Executed)

---

## Executive Summary

**Bug Discovered**: `PromptPlaygroundPage.tsx` uses `detectGeminiJsonSchema()` for UI display, which only detects Gemini schemas. This causes OpenAI and Anthropic schemas to be invisible in the UI (no banner, no modal, no indicators), even though they work correctly in requests.

**Solution**: Use the unified `detectJsonSchema()` detector for both UI display and request logic.

**Approach**: Test-Driven Development (TDD) - Write failing tests first, then fix the bug.

---

## Current Bug Analysis

### Problem Location
**File**: `frontend/src/features/prompt-playground/PromptPlaygroundPage.tsx`

**Line 257** (UI Display):
```typescript
const jsonSchemaInfo = detectGeminiJsonSchema(span)  // ❌ Gemini only
```

**Line 267** (Request Logic):
```typescript
const jsonSchema = span ? detectJsonSchema(span) : null  // ✅ All providers
```

**Line 582** (Active Settings):
```typescript
hasSchema={span ? detectJsonSchema(span) !== null : false}  // ✅ All providers
```

### Impact Assessment

| Component | OpenAI | Anthropic | Gemini | Status |
|-----------|--------|-----------|--------|--------|
| **Request Logic** | ✅ Works | ✅ Works | ✅ Works | Correct |
| **JsonSchemaBanner** | ❌ Hidden | ❌ Hidden | ✅ Shows | **BUG** |
| **JsonSchemaModal** | ❌ Broken | ❌ Broken | ✅ Works | **BUG** |
| **ActiveSettingsDisplay** | ✅ Works | ✅ Works | ✅ Works | Correct |

### Root Cause
- Used provider-specific detector (`detectGeminiJsonSchema`) for UI instead of unified detector (`detectJsonSchema`)
- Inconsistent usage across the component (3 different detection calls)
- No component/integration tests to catch this

---

## Test Strategy

### Testing Philosophy
**Approach**: Test-Driven Development (TDD)
1. Write tests that verify correct behavior (tests will fail initially)
2. Fix the bug
3. Verify all tests pass

### Test Levels

#### Level 1: Unit Tests (Existing) ✅
**File**: `frontend/src/features/prompt-playground/utils/provider-warnings.test.ts`
**Status**: Already passing (18 tests)
**Coverage**: Individual detector functions work correctly

#### Level 2: Integration Unit Tests (NEW)
**File**: `frontend/src/features/prompt-playground/utils/schema-integration.test.ts`
**Status**: To be created
**Coverage**: Logic flow without component rendering

#### Level 3: Component Tests (NEW)
**File**: `frontend/src/features/prompt-playground/PromptPlaygroundPage.test.tsx`
**Status**: To be created
**Coverage**: Full UI behavior with rendered components

---

## Implementation Plan

### Phase 1: Install Dependencies

**Required Packages**:
```bash
npm install --save-dev @testing-library/react @testing-library/jest-dom @testing-library/user-event
```

**Justification**:
- `@testing-library/react`: Component rendering and interaction testing
- `@testing-library/jest-dom`: Custom matchers for DOM testing (`.toBeInTheDocument()`, etc.)
- `@testing-library/user-event`: Simulate user interactions (clicks, typing)

---

### Phase 2: Create Integration Unit Tests (TDD)

**File**: `frontend/src/features/prompt-playground/utils/schema-integration.test.ts`

**Test Categories**:

#### 2.1: Schema Detection for UI Display (4 tests)
Tests that the unified detector works for all providers when determining UI display:

```typescript
describe('Schema Detection for UI Display', () => {
  it('should detect OpenAI schema for UI display', () => {
    const span = createOpenAISpanWithSchema()
    const jsonSchema = detectJsonSchema(span)
    const jsonSchemaInfo = jsonSchema ? { schema: jsonSchema } : null

    expect(jsonSchemaInfo).not.toBeNull()
    expect(jsonSchemaInfo?.schema).toHaveProperty('type', 'object')
  })

  it('should detect Anthropic schema for UI display', () => {
    const span = createAnthropicSpanWithSchema()
    const jsonSchema = detectJsonSchema(span)
    const jsonSchemaInfo = jsonSchema ? { schema: jsonSchema } : null

    expect(jsonSchemaInfo).not.toBeNull()
    expect(jsonSchemaInfo?.schema).toHaveProperty('type', 'object')
  })

  it('should detect Gemini schema for UI display', () => {
    const span = createGeminiSpanWithSchema()
    const jsonSchema = detectJsonSchema(span)
    const jsonSchemaInfo = jsonSchema ? { schema: jsonSchema } : null

    expect(jsonSchemaInfo).not.toBeNull()
    expect(jsonSchemaInfo?.schema).toHaveProperty('type', 'OBJECT')
  })

  it('should return null when no schema present', () => {
    const span = createSpanWithoutSchema('openai')
    const jsonSchema = detectJsonSchema(span)
    const jsonSchemaInfo = jsonSchema ? { schema: jsonSchema } : null

    expect(jsonSchemaInfo).toBeNull()
  })
})
```

**Expected**: Tests 1-2 will **conceptually pass** (the logic is correct), but demonstrate the bug when used in component.

#### 2.2: Toggle Logic Scenarios (12 tests)
Test all 4 scenarios from the logic table × 3 providers:

| Scenario | Toggle | Schema | Expected Behavior |
|----------|--------|--------|-------------------|
| 1 | ON | Yes | Structured output with schema |
| 2 | ON | No | Schema-less JSON mode |
| 3 | OFF | Yes | Normal text (schema ignored) |
| 4 | OFF | No | Normal text |

**Sample Test Structure**:
```typescript
describe('Structured Output Toggle Logic', () => {
  describe('Scenario 1: Toggle ON + Schema Detected', () => {
    it('OpenAI: should use structured output with schema', () => {
      const span = createOpenAISpanWithSchema()
      const jsonMode = true
      const jsonSchema = detectJsonSchema(span)

      expect(jsonSchema).not.toBeNull()
      // Verify request would include response_format.json_schema
    })

    it('Anthropic: should use tool-based structured output', () => {
      const span = createAnthropicSpanWithSchema()
      const jsonMode = true
      const jsonSchema = detectJsonSchema(span)

      expect(jsonSchema).not.toBeNull()
      // Verify request would include tools with input_schema
    })

    it('Gemini: should use response_json_schema', () => {
      const span = createGeminiSpanWithSchema()
      const jsonMode = true
      const jsonSchema = detectJsonSchema(span)

      expect(jsonSchema).not.toBeNull()
      // Verify request would include response_json_schema
    })
  })

  describe('Scenario 2: Toggle ON + No Schema', () => {
    it('OpenAI: should use schema-less JSON mode (json_object)', () => {
      const span = createSpanWithoutSchema('openai')
      const jsonMode = true
      const jsonSchema = detectJsonSchema(span)

      expect(jsonSchema).toBeNull()
      // Verify request would include response_format: { type: 'json_object' }
    })
    // ... similar for Anthropic, Gemini
  })

  describe('Scenario 3: Toggle OFF + Schema Detected', () => {
    it('should NOT include schema in request when toggle disabled', () => {
      const span = createOpenAISpanWithSchema()
      const jsonMode = false
      const jsonSchema = detectJsonSchema(span)

      expect(jsonSchema).not.toBeNull()  // Schema exists
      // But verify it's NOT included in request (normal text mode)
    })
  })

  describe('Scenario 4: Toggle OFF + No Schema', () => {
    it('should use normal text mode', () => {
      const span = createSpanWithoutSchema('openai')
      const jsonMode = false
      const jsonSchema = detectJsonSchema(span)

      expect(jsonSchema).toBeNull()
      // Verify normal text request
    })
  })
})
```

**Total**: ~16 integration unit tests

---

### Phase 3: Create Component Tests

**File**: `frontend/src/features/prompt-playground/PromptPlaygroundPage.test.tsx`

**Setup**:
```typescript
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { Provider } from 'react-redux'
import { BrowserRouter } from 'react-router'
import { configureStore } from '@reduxjs/toolkit'
import PromptPlaygroundPage from './PromptPlaygroundPage'
import { createOpenAISpanWithSchema, createAnthropicSpanWithSchema, createGeminiSpanWithSchema } from './utils/provider-warnings.test'

// Mock store setup
const createMockStore = (initialState = {}) => {
  return configureStore({
    reducer: {
      promptPlaygroundState: promptPlaygroundReducer,
      // ... other reducers
    },
    preloadedState: initialState
  })
}
```

#### 3.1: Banner Display Tests (4 tests)

```typescript
describe('JsonSchemaBanner Display', () => {
  it('should show banner for OpenAI span with schema', () => {
    const span = createOpenAISpanWithSchema()
    const store = createMockStore({ /* state with span */ })

    render(
      <Provider store={store}>
        <BrowserRouter>
          <PromptPlaygroundPage />
        </BrowserRouter>
      </Provider>
    )

    expect(screen.getByText(/JSON Schema detected and active/i)).toBeInTheDocument()
  })

  it('should show banner for Anthropic span with schema', () => {
    const span = createAnthropicSpanWithSchema()
    // ... similar test
    expect(screen.getByText(/JSON Schema detected and active/i)).toBeInTheDocument()
  })

  it('should show banner for Gemini span with schema', () => {
    const span = createGeminiSpanWithSchema()
    // ... similar test
    expect(screen.getByText(/JSON Schema detected and active/i)).toBeInTheDocument()
  })

  it('should NOT show banner when no schema detected', () => {
    const span = createSpanWithoutSchema('openai')
    // ... similar test
    expect(screen.queryByText(/JSON Schema detected and active/i)).not.toBeInTheDocument()
  })
})
```

**Expected Before Fix**: Tests 1-2 will **FAIL** (OpenAI/Anthropic banners don't show)

#### 3.2: Schema Modal Tests (4 tests)

```typescript
describe('JsonSchemaModal', () => {
  it('should open modal and display OpenAI schema when banner clicked', async () => {
    const span = createOpenAISpanWithSchema()
    const store = createMockStore({ /* state with span */ })

    render(
      <Provider store={store}>
        <BrowserRouter>
          <PromptPlaygroundPage />
        </BrowserRouter>
      </Provider>
    )

    const banner = screen.getByText(/JSON Schema detected and active/i)
    fireEvent.click(banner)

    await waitFor(() => {
      expect(screen.getByText(/Response JSON Schema Used/i)).toBeInTheDocument()
      expect(screen.getByText(/"name"/i)).toBeInTheDocument() // Schema content
    })
  })

  // Similar tests for Anthropic, Gemini, and null case
})
```

**Expected Before Fix**: Tests for OpenAI/Anthropic will **FAIL**

#### 3.3: Active Settings Display Tests (4 tests)

```typescript
describe('ActiveSettingsDisplay - json_schema indicator', () => {
  it('should show "json_schema: active" when toggle ON and OpenAI schema detected', () => {
    const span = createOpenAISpanWithSchema()
    const store = createMockStore({
      promptPlaygroundState: { jsonMode: true, /* other state */ }
    })

    render(
      <Provider store={store}>
        <BrowserRouter>
          <PromptPlaygroundPage />
        </BrowserRouter>
      </Provider>
    )

    expect(screen.getByText(/json_schema: active/i)).toBeInTheDocument()
  })

  // Similar for Anthropic, Gemini

  it('should NOT show indicator when toggle OFF', () => {
    const span = createOpenAISpanWithSchema()
    const store = createMockStore({
      promptPlaygroundState: { jsonMode: false }
    })

    render(/* ... */)

    expect(screen.queryByText(/json_schema: active/i)).not.toBeInTheDocument()
  })
})
```

**Expected Before Fix**: This already works (uses unified detector on line 582)

#### 3.4: Schema-less Warning Tests (2 tests)

```typescript
describe('Schema-less JSON Mode Warning', () => {
  it('should show warning when toggle ON but no schema detected', () => {
    const span = createSpanWithoutSchema('openai')
    const store = createMockStore({
      promptPlaygroundState: { jsonMode: true }
    })

    render(/* ... */)

    expect(screen.getByText(/No JSON schema detected.*schema-less JSON mode/i)).toBeInTheDocument()
  })

  it('should NOT show warning when schema is detected', () => {
    const span = createOpenAISpanWithSchema()
    const store = createMockStore({
      promptPlaygroundState: { jsonMode: true }
    })

    render(/* ... */)

    expect(screen.queryByText(/schema-less JSON mode/i)).not.toBeInTheDocument()
  })
})
```

**Total**: ~15 component tests

---

### Phase 4: Fix the Bug

**File**: `frontend/src/features/prompt-playground/PromptPlaygroundPage.tsx`

**Changes Required**:

#### Change 1: Line 257 (Fix UI detection)
```typescript
// BEFORE (WRONG - Gemini only)
const providerWarning = detectProviderWarnings(span)
const jsonSchemaInfo = detectGeminiJsonSchema(span)

// AFTER (FIXED - All providers)
const providerWarning = detectProviderWarnings(span)
const jsonSchema = span ? detectJsonSchema(span) : null
const jsonSchemaInfo = jsonSchema ? { schema: jsonSchema } : null
```

#### Change 2: Line 267 (Remove duplicate detection)
```typescript
// BEFORE (Duplicate detection call)
const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
  event.preventDefault()
  setTestStartTime(new Date().toISOString())
  setTestEndTime(null)
  const formData = new FormData(event.currentTarget)
  const prompt = formData.get('prompt') as string

  // Extract JSON schema from span if available (on-demand)
  const jsonSchema = span ? detectJsonSchema(span) : null
  // ... rest of function

// AFTER (Use variable from line 257)
const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
  event.preventDefault()
  setTestStartTime(new Date().toISOString())
  setTestEndTime(null)
  const formData = new FormData(event.currentTarget)
  const prompt = formData.get('prompt') as string

  // jsonSchema already extracted at component level (line 257)
  // ... rest of function
```

#### Change 3: Line 582 (Use cached detection)
```typescript
// BEFORE (Third detection call)
<ActiveSettingsDisplay
  settings={generationSettings}
  provider={selectedProvider}
  jsonMode={jsonMode}
  hasSchema={span ? detectJsonSchema(span) !== null : false}
/>

// AFTER (Use variable from line 257)
<ActiveSettingsDisplay
  settings={generationSettings}
  provider={selectedProvider}
  jsonMode={jsonMode}
  hasSchema={jsonSchema !== null}
/>
```

**Benefits of Changes**:
1. ✅ Fixes bug - UI indicators work for all 3 providers
2. ✅ Performance - One detection call instead of three
3. ✅ Maintainability - Single source of truth for schema detection
4. ✅ Consistency - Same detector used everywhere

---

### Phase 5: Verify Tests Pass

**Execution Order**:

1. **Install dependencies**:
   ```bash
   npm install --save-dev @testing-library/react @testing-library/jest-dom @testing-library/user-event
   ```

2. **Create integration unit tests**:
   - File: `frontend/src/features/prompt-playground/utils/schema-integration.test.ts`
   - Run: `npm run test:run utils/schema-integration.test.ts`
   - **Expected**: Some tests may fail (demonstrating the bug context)

3. **Create component tests**:
   - File: `frontend/src/features/prompt-playground/PromptPlaygroundPage.test.tsx`
   - Run: `npm run test:run PromptPlaygroundPage.test.tsx`
   - **Expected**: ~8 tests will FAIL (OpenAI/Anthropic UI display)

4. **Fix the bug**:
   - Edit: `frontend/src/features/prompt-playground/PromptPlaygroundPage.tsx`
   - Apply all 3 changes above

5. **Run all tests**:
   ```bash
   npm run test:run
   ```
   - **Expected**: All tests pass ✅

6. **Verify coverage**:
   - Existing utility tests: 18 tests ✅
   - New integration unit tests: ~16 tests ✅
   - New component tests: ~15 tests ✅
   - **Total**: ~49 tests passing

---

## Test Coverage Matrix

| Scenario | Provider | Toggle | Schema | Unit Test | Component Test | Expected Behavior |
|----------|----------|--------|--------|-----------|----------------|-------------------|
| 1 | OpenAI | ON | Yes | ✅ | ✅ | Banner shows, request uses schema |
| 2 | OpenAI | ON | No | ✅ | ✅ | Warning shows, uses json_object |
| 3 | OpenAI | OFF | Yes | ✅ | ✅ | Banner hidden, no schema in request |
| 4 | OpenAI | OFF | No | ✅ | ✅ | Normal text mode |
| 5 | Anthropic | ON | Yes | ✅ | ✅ | Banner shows, request uses tools |
| 6 | Anthropic | ON | No | ✅ | ✅ | Warning shows, uses jsonMode |
| 7 | Anthropic | OFF | Yes | ✅ | ✅ | Banner hidden, no tools |
| 8 | Anthropic | OFF | No | ✅ | ✅ | Normal text mode |
| 9 | Gemini | ON | Yes | ✅ | ✅ | Banner shows, uses response_json_schema |
| 10 | Gemini | ON | No | ✅ | ✅ | Warning shows, uses responseMimeType |
| 11 | Gemini | OFF | Yes | ✅ | ✅ | Banner hidden, no schema |
| 12 | Gemini | OFF | No | ✅ | ✅ | Normal text mode |

**Total Coverage**: 12 scenarios × 2 test types = 24 test cases minimum

---

## Expected Outcomes

### Before Fix

**Test Results**:
- Existing utility tests: **18/18 passing** ✅
- Integration unit tests: **~16/16 passing** ✅ (logic is sound)
- Component tests: **~7/15 passing** ❌ (UI broken for OpenAI/Anthropic)
- **Total**: ~41/49 (84% pass rate)

**User Experience**:
- OpenAI schemas: ❌ No UI indicators, ✅ Requests work
- Anthropic schemas: ❌ No UI indicators, ✅ Requests work
- Gemini schemas: ✅ Full functionality

### After Fix

**Test Results**:
- Existing utility tests: **18/18 passing** ✅
- Integration unit tests: **16/16 passing** ✅
- Component tests: **15/15 passing** ✅
- **Total**: **49/49 passing** (100% pass rate)

**User Experience**:
- OpenAI schemas: ✅ Full UI indicators, ✅ Requests work
- Anthropic schemas: ✅ Full UI indicators, ✅ Requests work
- Gemini schemas: ✅ Full functionality

**Performance**:
- Before: 3 schema detection calls per render
- After: 1 schema detection call per render
- **Improvement**: 66% reduction in detection overhead

---

## Risk Assessment

### Low Risk
- ✅ Changes are localized to one component
- ✅ Existing tests ensure utility functions still work
- ✅ Backend logic unchanged
- ✅ No API changes

### Medium Risk
- ⚠️ Component tests may require Redux/Router setup complexity
- ⚠️ Need to mock API calls in component tests

### Mitigation Strategies
1. Use helper functions for common test setup (mock store, routing)
2. Mock API endpoints at the component test level
3. Test incrementally (utilities → integration → component)
4. Run existing tests frequently to catch regressions

---

## Success Criteria

### Must Have ✅
1. Bug fixed - unified detector used for UI display
2. All existing tests still pass
3. Component tests verify UI behavior for all 3 providers
4. All 4 toggle/schema scenarios tested

### Should Have ✅
1. Integration unit tests for logic flow
2. Performance improvement (single detection call)
3. Test coverage documentation

### Nice to Have
1. Visual regression tests (screenshots)
2. E2E tests with real backend
3. Test coverage report generation

---

## Future Enhancements

### Additional Test Coverage
1. **Error handling**: What happens if span data is corrupted?
2. **State transitions**: Toggle changes while request in flight
3. **Multiple schemas**: Span with multiple tool definitions
4. **Cross-provider**: User changes provider with schema loaded

### Performance Testing
1. Measure render time with/without schemas
2. Profile schema detection overhead
3. Benchmark large schema parsing

### Accessibility Testing
1. Screen reader support for banner
2. Keyboard navigation for modal
3. Focus management

---

## Appendix: File Structure

```
junjo-server/
├── schema-detection-fix-plan.md           # This document
├── frontend/
│   ├── package.json                       # Update with new dependencies
│   ├── vitest.config.ts                   # Existing Vitest config
│   └── src/
│       └── features/
│           └── prompt-playground/
│               ├── PromptPlaygroundPage.tsx              # FIX BUG HERE
│               ├── PromptPlaygroundPage.test.tsx         # NEW - Component tests
│               ├── utils/
│               │   ├── provider-warnings.ts              # Existing
│               │   ├── provider-warnings.test.ts         # Existing (18 tests)
│               │   └── schema-integration.test.ts        # NEW - Integration unit tests
│               └── components/
│                   ├── JsonSchemaBanner.tsx              # Affected by bug
│                   ├── JsonSchemaModal.tsx               # Affected by bug
│                   └── ActiveSettingsDisplay.tsx         # Already works
└── backend/
    └── api/llm/                            # No changes needed
```

---

## Timeline Estimate

| Phase | Task | Estimated Time |
|-------|------|----------------|
| 1 | Install dependencies | 5 minutes |
| 2 | Create integration unit tests | 45 minutes |
| 3 | Create component tests | 90 minutes |
| 4 | Fix bug (3 changes) | 15 minutes |
| 5 | Verify all tests pass | 15 minutes |
| | **Total** | **2.5 hours** |

---

## Conclusion

This plan takes a comprehensive, test-driven approach to fixing a subtle but impactful bug in the Prompt Playground's schema detection. By writing tests first, we ensure:

1. **Documentation**: Tests serve as living specification
2. **Confidence**: All scenarios are covered
3. **Regression Prevention**: Future changes won't reintroduce the bug
4. **Maintainability**: Clear understanding of expected behavior

The fix itself is simple (3 line changes), but the test infrastructure provides long-term value for the codebase.
