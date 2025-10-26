import { Link, useParams } from 'react-router'
import { useEffect, useState, useRef } from 'react'
import { OtelSpan } from '../traces/schemas/schemas'
import { API_HOST } from '../../config'
import { useAppDispatch, useAppSelector } from '../../root-store/hooks'
import { openaiRequest } from './fetch/openai-request'
import { anthropicRequest } from './fetch/anthropic-request'
import { geminiRequest } from './fetch/gemini-request'
import { PromptPlaygroundActions } from './store/slice'
import ModelSelector from './components/ModelSelector'
import ProviderSelector from './components/ProviderSelector'
import { Switch } from '../../components/catalyst/switch'
import { getSpanDurationString } from '../../util/duration-utils'
import CircularProgress from '../../components/CircularProgress'
import { detectProviderWarnings, detectGeminiJsonSchema } from './utils/provider-warnings'
import { mapOtelProviderToInternal } from './utils/provider-mapping'
import ProviderWarningBanner from './components/ProviderWarningBanner'
import ProviderWarningModal from './components/ProviderWarningModal'
import JsonSchemaBanner from './components/JsonSchemaBanner'
import JsonSchemaModal from './components/JsonSchemaModal'
import GenerationSettingsModal from './components/GenerationSettingsModal'
import ActiveSettingsDisplay from './components/ActiveSettingsDisplay'

export default function PromptPlaygroundPage() {
  const { serviceName, traceId, spanId } = useParams<{
    serviceName: string
    traceId: string
    spanId: string
  }>()
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)
  const [span, setSpan] = useState<OtelSpan | null>(null)
  const [originalModel, setOriginalModel] = useState<string | null>(null)
  const [originalProvider, setOriginalProvider] = useState<string | null>(null)
  const [testStartTime, setTestStartTime] = useState<string | null>(null)
  const [testEndTime, setTestEndTime] = useState<string | null>(null)
  const [warningModalOpen, setWarningModalOpen] = useState(false)
  const [schemaModalOpen, setSchemaModalOpen] = useState(false)
  const [settingsModalOpen, setSettingsModalOpen] = useState(false)
  const dispatch = useAppDispatch()
  const {
    output,
    loading: outputLoading,
    error: outputError,
  } = useAppSelector((state) => state.promptPlaygroundState)
  const selectedModel = useAppSelector((state) => state.promptPlaygroundState.selectedModel)
  const selectedProvider = useAppSelector((state) => state.promptPlaygroundState.selectedProvider)
  const jsonMode = useAppSelector((state) => state.promptPlaygroundState.jsonMode)
  const generationSettings = useAppSelector((state) => state.promptPlaygroundState.generationSettings)
  const prevModelRef = useRef<string | null>(null)
  const prevProviderRef = useRef<string | null>(null)

  useEffect(() => {
    if (span) {
      // Extract and set model
      const modelName = span.attributes_json['llm.model_name'] || 'Unknown'
      dispatch(PromptPlaygroundActions.setSelectedModel(modelName))
      setOriginalModel(modelName)

      // Extract and set provider using OpenInference mapping
      // OpenInference uses "google" for Gemini, we use "gemini" internally
      const otelProviderName = span.attributes_json['llm.provider']
      const internalProvider = otelProviderName ? mapOtelProviderToInternal(otelProviderName) : 'gemini'
      dispatch(PromptPlaygroundActions.setSelectedProvider(internalProvider))
      setOriginalProvider(internalProvider)

      // Set JSON mode if MIME type is application/json
      const mimeType = span.attributes_json['input.mime_type']
      if (mimeType === 'application/json') {
        dispatch(PromptPlaygroundActions.setJsonMode(true))
      }

      // Import generation settings from OpenInference llm.invocation_parameters
      const invocationParams = span.attributes_json['llm.invocation_parameters']
      if (invocationParams) {
        try {
          // Parse the JSON string containing invocation parameters
          const params = typeof invocationParams === 'string' ? JSON.parse(invocationParams) : invocationParams
          const importedSettings: Record<string, unknown> = {}

          // Common settings across all providers
          if (params.temperature !== undefined && params.temperature !== null) {
            importedSettings.temperature = Number(params.temperature)
          }

          // Provider-specific settings
          if (internalProvider === 'openai') {
            if (params.reasoning_effort) {
              importedSettings.reasoning_effort = params.reasoning_effort
            }
            if (params.max_completion_tokens !== undefined && params.max_completion_tokens !== null) {
              importedSettings.max_completion_tokens = Number(params.max_completion_tokens)
            }
            if (params.max_tokens !== undefined && params.max_tokens !== null) {
              importedSettings.max_tokens = Number(params.max_tokens)
            }
          } else if (internalProvider === 'anthropic') {
            if (params.max_tokens !== undefined && params.max_tokens !== null) {
              importedSettings.max_tokens = Number(params.max_tokens)
            }
            if (params.thinking) {
              if (params.thinking.type === 'enabled') {
                importedSettings.thinking_enabled = true
              }
              if (params.thinking.budget_tokens !== undefined && params.thinking.budget_tokens !== null) {
                importedSettings.thinking_budget_tokens = Number(params.thinking.budget_tokens)
              }
            }
          } else if (internalProvider === 'gemini') {
            if (params.thinkingBudget !== undefined && params.thinkingBudget !== null) {
              importedSettings.thinkingBudget = Number(params.thinkingBudget)
            }
            if (params.includeThoughts !== undefined && params.includeThoughts !== null) {
              importedSettings.includeThoughts = Boolean(params.includeThoughts)
            }
            if (params.maxOutputTokens !== undefined && params.maxOutputTokens !== null) {
              importedSettings.maxOutputTokens = Number(params.maxOutputTokens)
            }
          }

          // Only dispatch if we found any settings to import
          if (Object.keys(importedSettings).length > 0) {
            dispatch(PromptPlaygroundActions.setGenerationSettings(importedSettings))
          }
        } catch (error) {
          console.error('Failed to parse llm.invocation_parameters:', error)
        }
      }
    }
  }, [span, dispatch])

  // Reset model selection when provider changes (unless it's the initial load)
  useEffect(() => {
    if (selectedProvider && originalProvider && selectedProvider !== originalProvider) {
      dispatch(PromptPlaygroundActions.setSelectedModel(null))
    }
  }, [selectedProvider, originalProvider, dispatch])

  // Reset all settings when model or provider changes
  useEffect(() => {
    // Skip on initial mount
    if (prevModelRef.current === null && prevProviderRef.current === null) {
      prevModelRef.current = selectedModel
      prevProviderRef.current = selectedProvider
      return
    }

    // Only run if model or provider actually changed
    if (prevModelRef.current === selectedModel && prevProviderRef.current === selectedProvider) {
      return
    }

    // Reset all settings to defaults when model/provider changes
    dispatch(PromptPlaygroundActions.resetGenerationSettings())

    prevModelRef.current = selectedModel
    prevProviderRef.current = selectedProvider
  }, [selectedModel, selectedProvider, dispatch])

  useEffect(() => {
    const fetchSpan = async () => {
      try {
        setLoading(true)
        setError(false)
        const response = await fetch(`${API_HOST}/otel/trace/${traceId}/span/${spanId}`, {
          credentials: 'include',
        })
        if (!response.ok) {
          throw new Error('Failed to fetch span')
        }
        const data = await response.json()
        setSpan(data)
      } catch (error) {
        setError(true)
      } finally {
        setLoading(false)
      }
    }

    fetchSpan()
  }, [traceId, spanId, dispatch])

  if (loading) {
    return <div>Loading...</div>
  }

  if (error) {
    return <div>Error loading span.</div>
  }

  if (!span) {
    return <div>Span not found.</div>
  }

  const getInputValue = (value: any) => {
    if (typeof value === 'string') {
      try {
        const parsed = JSON.parse(value)
        if (parsed.contents) {
          return parsed.contents
        }
        return value
      } catch (e) {
        return value
      }
    }
    return ''
  }

  const getOutputValue = (attributes: any) => {
    // Try to get the output value from various possible attribute paths
    let outputValue =
      attributes['llm.output_messages.0.message.content'] ||
      attributes['output.value'] ||
      attributes['llm.output']

    if (outputValue) {
      // If it's a string, try to parse it as JSON
      if (typeof outputValue === 'string') {
        try {
          const parsed = JSON.parse(outputValue)

          // Check if it's a Gemini response with candidates structure
          if (parsed.candidates && Array.isArray(parsed.candidates)) {
            // Extract just the text content if available
            if (parsed.candidates[0]?.content?.parts?.[0]?.text) {
              return parsed.candidates[0].content.parts[0].text
            }
          }

          // For other JSON structures, format with indentation for readability
          return JSON.stringify(parsed, null, 2)
        } catch (e) {
          // If it's not valid JSON, return as-is
          return outputValue
        }
      }

      // If it's already an object, format it as JSON string with indentation
      if (typeof outputValue === 'object') {
        return JSON.stringify(outputValue, null, 2)
      }
    }

    return ''
  }

  const inputValue = getInputValue(span.attributes_json['input.value'])
  const outputValue = getOutputValue(span.attributes_json)
  const modelName = span.attributes_json['llm.model_name'] || 'Unknown'
  const provider = span.attributes_json['llm.provider'] || 'Unknown'
  const mimeType = span.attributes_json['input.mime_type'] || ''

  // Compute provider warning and JSON schema info directly from span (no state needed)
  const providerWarning = detectProviderWarnings(span)
  const jsonSchemaInfo = detectGeminiJsonSchema(span)

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setTestStartTime(new Date().toISOString())
    setTestEndTime(null)
    const formData = new FormData(event.currentTarget)
    const prompt = formData.get('prompt') as string

    // Validate required fields
    if (!selectedModel) {
      console.error('No model selected')
      dispatch(PromptPlaygroundActions.setError('Please select a model'))
      return
    }
    if (!selectedProvider) {
      console.error('No provider selected')
      dispatch(PromptPlaygroundActions.setError('Please select a provider'))
      return
    }

    try {
      dispatch(PromptPlaygroundActions.setOutput(null))
      dispatch(PromptPlaygroundActions.setLoading(true))
      dispatch(PromptPlaygroundActions.setError(null))

      let outputText = ''

      // Build provider-specific requests
      if (selectedProvider === 'openai') {
        // Check if it's a reasoning model (o1, o3, o4, gpt-5 series)
        const isReasoningModel = selectedModel ? /^(o1-|o3-|o4-|gpt-5)/.test(selectedModel) : false

        const result = await openaiRequest({
          model: selectedModel,
          messages: [{ role: 'user', content: prompt }],
          ...(jsonMode && { response_format: { type: 'json_object' } }),
          // Only send reasoning_effort for reasoning models
          ...(isReasoningModel &&
            generationSettings.reasoning_effort && { reasoning_effort: generationSettings.reasoning_effort }),
          ...(isReasoningModel &&
            generationSettings.max_completion_tokens && {
              max_completion_tokens: generationSettings.max_completion_tokens,
            }),
          // Only send temperature for non-reasoning models
          ...(!isReasoningModel &&
            generationSettings.temperature !== undefined && { temperature: generationSettings.temperature }),
          // max_tokens for non-reasoning models
          ...(!isReasoningModel &&
            generationSettings.max_completion_tokens && {
              max_tokens: generationSettings.max_completion_tokens,
            }),
        })
        outputText = result.choices[0]?.message?.content || ''
      } else if (selectedProvider === 'anthropic') {
        const result = await anthropicRequest({
          model: selectedModel,
          messages: [{ role: 'user', content: prompt }],
          max_tokens: generationSettings.max_tokens || 4096,
          jsonMode,
          ...(generationSettings.thinking_enabled &&
            generationSettings.thinking_budget_tokens && {
              thinking: {
                type: 'enabled' as const,
                budget_tokens: generationSettings.thinking_budget_tokens,
              },
            }),
          ...(generationSettings.temperature !== undefined && {
            temperature: generationSettings.temperature,
          }),
        })
        // Extract output from response
        // If JSON mode, extract from tool use, otherwise from text
        if (jsonMode && result.content.length > 0) {
          const toolUseBlock = result.content.find((block) => block.type === 'tool_use')
          if (toolUseBlock && toolUseBlock.input) {
            outputText = JSON.stringify(toolUseBlock.input, null, 2)
          }
        } else {
          const textBlock = result.content.find((block) => block.type === 'text')
          outputText = textBlock?.text || ''
        }
      } else if (selectedProvider === 'gemini') {
        const result = await geminiRequest({
          model: selectedModel,
          contents: [{ parts: [{ text: prompt }] }],
          generationConfig: {
            ...(jsonMode && { responseMimeType: 'application/json' }),
            ...(generationSettings.temperature !== undefined && {
              temperature: generationSettings.temperature,
            }),
            ...(generationSettings.maxOutputTokens && {
              maxOutputTokens: generationSettings.maxOutputTokens,
            }),
            ...((generationSettings.thinkingBudget !== undefined ||
              generationSettings.includeThoughts !== undefined) && {
              thinkingConfig: {
                ...(generationSettings.thinkingBudget !== undefined && {
                  thinkingBudget: generationSettings.thinkingBudget,
                }),
                ...(generationSettings.includeThoughts !== undefined && {
                  includeThoughts: generationSettings.includeThoughts,
                }),
              },
            }),
          },
        })
        outputText = result.candidates[0]?.content?.parts[0]?.text || ''
      } else {
        throw new Error(`Unknown provider: ${selectedProvider}`)
      }

      dispatch(PromptPlaygroundActions.setOutput(outputText))
      setTestEndTime(new Date().toISOString())
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Error generating content'
      dispatch(PromptPlaygroundActions.setError(errorMessage))
    } finally {
      dispatch(PromptPlaygroundActions.setLoading(false))
    }
  }

  return (
    <div className={'px-2 py-3 flex flex-col h-dvh overflow-hidden'}>
      <div className={'px-2 pb-4'}>
        <div className={'mb-1 flex gap-x-3 font-bold'}>
          <Link to={'/logs'} className={'hover:underline'}>
            Logs
          </Link>
          <div>&rarr;</div>
          <Link to={`/logs/${serviceName}`} className={'hover:underline'}>
            {serviceName}
          </Link>
          <div>&rarr;</div>
          <Link to={`/traces/${serviceName}`} className={'hover:underline'}>
            Traces
          </Link>
          <div>&rarr;</div>
          <Link to={`/traces/${serviceName}/${traceId}`} className={'hover:underline'}>
            {traceId}
          </Link>
          <div>&rarr;</div>
          <Link to={`/traces/${serviceName}/${traceId}/${spanId}`} className={'hover:underline'}>
            {spanId}
          </Link>
          <div>&rarr;</div>
          <div>Prompt Playground</div>
        </div>
        <div className={'text-zinc-400 text-xs'}>readableDate</div>
      </div>
      <hr />
      <div className={'overflow-scroll pt-4 pb-10'}>
        <div className="flex gap-8">
          {/* Left Column: Original */}
          <div className="w-1/2">
            <div className="flex justify-between items-center mb-4">
              <div className="text-2xl font-bold mb-2 dark:text-white">Original</div>
              <div className="text-sm text-zinc-500 dark:text-zinc-400 mb-2">
                {provider} / {modelName}
                {mimeType && (
                  <span className="ml-2 px-2 py-1 text-xs font-semibold rounded-full bg-zinc-200 text-zinc-800 dark:bg-zinc-700 dark:text-zinc-200">
                    {mimeType}
                  </span>
                )}
              </div>
            </div>

            <div className="mb-4">
              <label
                htmlFor="original-prompt"
                className="block text-sm font-medium text-zinc-700 dark:text-zinc-300"
              >
                Prompt
              </label>
              <div className="mt-1">
                <textarea
                  id="original-prompt"
                  rows={15}
                  className="p-2 shadow-sm focus:ring-indigo-500 focus:border-indigo-500 block w-full sm:text-sm border border-zinc-300 rounded-md resize-y bg-zinc-50 dark:bg-zinc-800 dark:border-zinc-700 dark:text-zinc-300"
                  value={inputValue}
                  readOnly
                />
              </div>
            </div>

            <div>
              <div className="flex justify-between items-center">
                <label
                  htmlFor="original-output"
                  className="block text-sm font-medium text-zinc-700 dark:text-zinc-300"
                >
                  Output
                </label>
                <div className="text-sm text-zinc-500 dark:text-zinc-400">
                  {getSpanDurationString(span.start_time, span.end_time)}
                </div>
              </div>
              <div className="mt-1">
                <textarea
                  id="original-output"
                  rows={15}
                  className="p-2 shadow-sm focus:ring-indigo-500 focus:border-indigo-500 block w-full sm:text-sm border border-zinc-300 rounded-md resize-y bg-zinc-50 dark:bg-zinc-800 dark:border-zinc-700 dark:text-zinc-300"
                  value={outputValue}
                  readOnly
                />
              </div>
            </div>
          </div>

          {/* Right Column: Playground */}
          <div className="w-1/2">
            <div className="flex justify-between items-center mb-4">
              <div className="text-sm text-zinc-500 dark:text-zinc-400">
                <div className="flex items-center gap-2">
                  <ProviderSelector originalProvider={originalProvider} />
                  <ModelSelector
                    originalModel={originalModel}
                    originalProvider={originalProvider}
                    provider={selectedProvider}
                  />
                  <div className="flex items-center gap-2">
                    <Switch
                      checked={jsonMode}
                      onChange={(checked) => dispatch(PromptPlaygroundActions.setJsonMode(checked))}
                      className="group"
                    />
                    <span className="text-sm text-zinc-500 dark:text-zinc-400">JSON Mode</span>
                  </div>
                  <button
                    type="button"
                    onClick={() => setSettingsModalOpen(true)}
                    className="p-1.5 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 transition-colors"
                    title="Generation Settings"
                  >
                    <svg
                      className="w-5 h-5 text-zinc-500 dark:text-zinc-400"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"
                      />
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                      />
                    </svg>
                  </button>
                </div>
              </div>
            </div>
            <form onSubmit={handleSubmit}>
              <ActiveSettingsDisplay settings={generationSettings} provider={selectedProvider} />
              <div className="mb-4">
                <label
                  htmlFor="prompt"
                  className="block text-sm font-medium text-zinc-700 dark:text-zinc-300"
                >
                  Prompt
                </label>
                <div className="mt-1">
                  <textarea
                    id="prompt"
                    name="prompt"
                    rows={15}
                    className="p-2 shadow-sm focus:ring-indigo-500 focus:border-indigo-500 block w-full sm:text-sm border border-zinc-300 rounded-md resize-y dark:bg-zinc-900 dark:border-zinc-700 dark:text-zinc-300"
                    defaultValue={inputValue}
                  />
                </div>
              </div>
              <div className="mb-4">
                <div className="flex justify-between items-center">
                  <label
                    htmlFor="output"
                    className="block text-sm font-medium text-zinc-700 dark:text-zinc-300"
                  >
                    Output
                  </label>
                  <div className="text-sm text-zinc-500 dark:text-zinc-400">
                    {testStartTime && testEndTime ? getSpanDurationString(testStartTime, testEndTime) : ''}
                  </div>
                </div>
                <div className="mt-1">
                  <textarea
                    id="output"
                    name="output"
                    rows={15}
                    className="p-2 shadow-sm focus:ring-indigo-500 focus:border-indigo-500 block w-full sm:text-sm border border-zinc-300 rounded-md resize-y dark:bg-zinc-900 dark:border-zinc-700 dark:text-zinc-300"
                    value={output || ''}
                    readOnly
                  />
                </div>
              </div>
              {providerWarning && <ProviderWarningBanner onClick={() => setWarningModalOpen(true)} />}

              {!providerWarning && jsonSchemaInfo && (
                <JsonSchemaBanner onClick={() => setSchemaModalOpen(true)} />
              )}

              {jsonMode && !providerWarning && !jsonSchemaInfo && (
                <div className={'text-xs text-zinc-500 mb-4'}>
                  JSON Note: IF your original LLM request provided a typed schema / model with the request,
                  these playground outputs will be missing that context, and results may be different.
                </div>
              )}

              <button
                type="submit"
                className="px-3 py-1.5 text-sm font-semibold rounded-md bg-zinc-900 text-white hover:bg-zinc-800 dark:bg-indigo-600 dark:hover:bg-indigo-500"
                disabled={outputLoading}
              >
                {outputLoading ? (
                  <div className="flex items-center gap-2">
                    <CircularProgress />
                    <span>Loading...</span>
                  </div>
                ) : (
                  'Generate'
                )}
              </button>
              {outputError && <div className="text-red-500 mt-2">{outputError}</div>}
            </form>
          </div>
        </div>
      </div>

      {providerWarning && (
        <ProviderWarningModal
          isOpen={warningModalOpen}
          onClose={() => setWarningModalOpen(false)}
          warning={providerWarning}
        />
      )}

      {jsonSchemaInfo && (
        <JsonSchemaModal
          isOpen={schemaModalOpen}
          onClose={() => setSchemaModalOpen(false)}
          schemaInfo={jsonSchemaInfo}
        />
      )}

      <GenerationSettingsModal
        isOpen={settingsModalOpen}
        onClose={() => setSettingsModalOpen(false)}
        provider={selectedProvider}
        model={selectedModel}
        settings={generationSettings}
        onSave={(settings) => dispatch(PromptPlaygroundActions.setGenerationSettings(settings))}
      />
    </div>
  )
}
