import { useState, useMemo } from 'react'
import { Button } from '../../../components/catalyst/button'
import {
  Dialog,
  DialogActions,
  DialogBody,
  DialogDescription,
  DialogTitle,
} from '../../../components/catalyst/dialog'
import type { ModelInfo } from '../schemas/unified-request'
import {
  organizeModels,
  type ProductFamilyGroup,
  type ParsedModelInfo,
} from '../utils/model-grouping'
import clsx from 'clsx'
import RefreshButton from './RefreshButton'

interface ModelSelectorModalProps {
  isOpen: boolean
  onClose: () => void
  models: ModelInfo[]
  provider: string | null
  selectedModel: string | null
  originalModelId: string | null
  onSelectModel: (modelId: string) => void
  onRefresh: () => void
  isRefreshing: boolean
  error: string | null
}

export default function ModelSelectorModal({
  isOpen,
  onClose,
  models,
  provider,
  selectedModel,
  originalModelId,
  onSelectModel,
  onRefresh,
  isRefreshing,
  error,
}: ModelSelectorModalProps) {
  // Filter state (preview and exp default to off)
  const [includeStable, setIncludeStable] = useState(true)
  const [includePreview, setIncludePreview] = useState(false)
  const [includeExp, setIncludeExp] = useState(false)

  // Find the original model info
  const originalModel =
    originalModelId && originalModelId !== 'Unknown' ? models.find((m) => m.id === originalModelId) : null

  // Group and filter models
  const productFamilyGroups = useMemo(
    () => organizeModels(models, provider, { includeStable, includePreview, includeExp }),
    [models, provider, includeStable, includePreview, includeExp]
  )

  const handleSelectModel = (modelId: string) => {
    onSelectModel(modelId)
    onClose()
  }

  return (
    <Dialog open={isOpen} onClose={onClose} size="3xl">
      <DialogTitle>Select Model</DialogTitle>
      <DialogDescription>
        {provider === 'gemini'
          ? 'Choose a model from the available Gemini models. Filter by release type to narrow your selection.'
          : `Choose a model from the available ${provider} models.`}
      </DialogDescription>
      <DialogBody>
        {/* Original Model Card */}
        {originalModel && (
          <div className="-mt-3 mb-4 pb-4 border-b border-zinc-200 dark:border-zinc-700">
            <OriginalModelCard model={originalModel} onSelect={() => handleSelectModel(originalModel.id)} />
          </div>
        )}

        {/* Filter Controls and Refresh (only for Gemini) */}
        {provider === 'gemini' && (
          <div className="mb-4 pb-4 border-b border-zinc-200 dark:border-zinc-700">
            <div className="flex items-center justify-between gap-4">
              <div className="flex gap-4">
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={includeStable}
                    onChange={(e) => setIncludeStable(e.target.checked)}
                    className="w-4 h-4 text-indigo-600 border-zinc-300 rounded focus:ring-indigo-500"
                  />
                  <span className="text-sm text-zinc-700 dark:text-zinc-300">Stable</span>
                </label>
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={includePreview}
                    onChange={(e) => setIncludePreview(e.target.checked)}
                    className="w-4 h-4 text-indigo-600 border-zinc-300 rounded focus:ring-indigo-500"
                  />
                  <span className="text-sm text-zinc-700 dark:text-zinc-300">Preview</span>
                </label>
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={includeExp}
                    onChange={(e) => setIncludeExp(e.target.checked)}
                    className="w-4 h-4 text-indigo-600 border-zinc-300 rounded focus:ring-indigo-500"
                  />
                  <span className="text-sm text-zinc-700 dark:text-zinc-300">Experimental</span>
                </label>
              </div>
              <RefreshButton onClick={onRefresh} disabled={isRefreshing} isRefreshing={isRefreshing} />
            </div>
          </div>
        )}

        {/* Refresh Button (for non-Gemini providers) */}
        {provider && provider !== 'gemini' && (
          <div className="mb-4 pb-4 border-b border-zinc-200 dark:border-zinc-700">
            <div className="flex justify-end">
              <RefreshButton onClick={onRefresh} disabled={isRefreshing} isRefreshing={isRefreshing} />
            </div>
          </div>
        )}

        {/* Error Display */}
        {error && (
          <div className="mb-4 p-4 rounded-lg bg-red-50 dark:bg-red-950/20 border border-red-200 dark:border-red-800">
            <div className="flex items-start gap-3">
              <svg
                className="w-5 h-5 text-red-600 dark:text-red-400 flex-shrink-0 mt-0.5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
              <div className="flex-1">
                <h4 className="text-sm font-semibold text-red-800 dark:text-red-200 mb-1">
                  Failed to Load Models
                </h4>
                <p className="text-sm text-red-700 dark:text-red-300">{error}</p>
                {(error.includes('Failed to fetch models') || error.includes('Failed to refresh models')) && (
                  <p className="text-xs text-red-600 dark:text-red-400 mt-2">
                    This may be due to a missing or invalid API key. Check your .env file and ensure the API key
                    for {provider} is configured.
                  </p>
                )}
              </div>
            </div>
          </div>
        )}

        {/* Model Groups */}
        <div className="space-y-6">
          {productFamilyGroups.length === 0 ? (
            <div className="text-center py-8 text-zinc-500 dark:text-zinc-400">
              No models match the selected filters.
            </div>
          ) : (
            productFamilyGroups.map((familyGroup) => (
              <ProductFamilySection
                key={familyGroup.productFamily}
                familyGroup={familyGroup}
                selectedModel={selectedModel}
                onSelectModel={handleSelectModel}
                provider={provider}
              />
            ))
          )}
        </div>

        <DialogActions>
          <Button plain onClick={onClose}>
            Cancel
          </Button>
        </DialogActions>
      </DialogBody>
    </Dialog>
  )
}

interface ProductFamilySectionProps {
  familyGroup: ProductFamilyGroup
  selectedModel: string | null
  onSelectModel: (modelId: string) => void
  provider: string | null
}

function ProductFamilySection({
  familyGroup,
  selectedModel,
  onSelectModel,
  provider,
}: ProductFamilySectionProps) {
  return (
    <div>
      {/* Product Family Header (Gemini/Gemma for gemini provider, Opus/Sonnet/Haiku for anthropic) */}
      {(provider === 'gemini' || provider === 'anthropic') && (
        <h3 className="text-base font-bold text-zinc-900 dark:text-zinc-100 mb-3">
          {familyGroup.displayName}
        </h3>
      )}

      {/* Version Groups */}
      {familyGroup.versionGroups.map((versionGroup) => (
        <div key={versionGroup.version} className="mb-4 last:mb-0">
          {/* Version Header */}
          {provider === 'gemini' && versionGroup.version && (
            <p className="text-sm font-bold text-zinc-700 dark:text-zinc-300 mb-2">
              {versionGroup.version === '0.0' ? 'Latest' : `Version ${versionGroup.version}`}
            </p>
          )}

          {/* Release Type Groups */}
          {versionGroup.releaseTypeGroups.map((releaseTypeGroup) => (
            <div key={releaseTypeGroup.releaseType} className="mb-3 last:mb-0">
              {/* Release Type Header */}
              {provider === 'gemini' && (
                <p className="text-xs font-semibold text-zinc-600 dark:text-zinc-400 mb-2 pl-2">
                  {releaseTypeGroup.displayName}
                </p>
              )}

              {/* Model Cards */}
              <div className="grid grid-cols-1 gap-2">
                {releaseTypeGroup.models.map((model) => (
                  <ModelCard
                    key={model.original.id}
                    model={model}
                    isSelected={model.original.id === selectedModel}
                    onSelect={() => onSelectModel(model.original.id)}
                  />
                ))}
              </div>
            </div>
          ))}
        </div>
      ))}
    </div>
  )
}

interface ModelCardProps {
  model: ParsedModelInfo
  isSelected: boolean
  onSelect: () => void
}

function ModelCard({ model, isSelected, onSelect }: ModelCardProps) {
  const { original, displayName } = model

  return (
    <button
      type="button"
      onClick={onSelect}
      className={clsx(
        'text-left p-3 rounded-lg border transition-colors cursor-pointer',
        isSelected
          ? 'border-indigo-500 bg-indigo-50 dark:bg-indigo-950/30'
          : 'border-zinc-200 dark:border-zinc-700 hover:border-zinc-300 dark:hover:border-zinc-600 hover:bg-zinc-50 dark:hover:bg-zinc-800/50',
      )}
    >
      <div className="flex items-start justify-between">
        <div className="flex-1">
          <div className="flex items-baseline gap-1">
            <span className="font-medium text-sm text-zinc-900 dark:text-zinc-100">{displayName}</span>{' '}
            <span className="text-xs text-zinc-500 dark:text-zinc-400">{original.id}</span>
          </div>

          {/* Generally the description is redundant. */}
          {/* {original.description && (
            <div className="text-xs text-zinc-600 dark:text-zinc-400 mt-1.5">{original.description}</div>
          )} */}
          <div className="flex gap-3 mt-2 text-xs text-zinc-500 dark:text-zinc-400">
            {original.contextWindow && (
              <span>
                <span className="font-medium">Context:</span> {original.contextWindow.toLocaleString()} tokens
              </span>
            )}
            {original.maxOutputTokens && (
              <span>
                <span className="font-medium">Max output:</span> {original.maxOutputTokens.toLocaleString()}{' '}
                tokens
              </span>
            )}
          </div>
        </div>
        {isSelected && (
          <div className="ml-3 flex-shrink-0">
            <div className="w-5 h-5 rounded-full bg-indigo-600 flex items-center justify-center">
              <svg className="w-3 h-3 text-white" fill="currentColor" viewBox="0 0 12 12">
                <path d="M10 3L4.5 8.5L2 6" stroke="currentColor" strokeWidth="2" fill="none" />
              </svg>
            </div>
          </div>
        )}
      </div>
    </button>
  )
}

interface OriginalModelCardProps {
  model: ModelInfo
  onSelect: () => void
}

function OriginalModelCard({ model, onSelect }: OriginalModelCardProps) {
  return (
    <button
      type="button"
      onClick={onSelect}
      className="w-full text-left p-3 rounded-lg border border-blue-300 bg-blue-50 dark:border-blue-700 dark:bg-blue-950/20 hover:bg-blue-100 dark:hover:bg-blue-950/30 transition-colors cursor-pointer"
    >
      <div className="flex items-start justify-between">
        <div className="flex-1">
          <div className="flex items-baseline gap-1">
            <span className="font-medium text-sm text-zinc-900 dark:text-zinc-100">
              {model.name || model.id}
            </span>
          </div>
          <div className="text-xs text-zinc-500 dark:text-zinc-400 mt-0.5">{model.id}</div>
          <div className="flex gap-3 mt-2 text-xs text-zinc-500 dark:text-zinc-400">
            {model.contextWindow && (
              <span>
                <span className="font-medium">Context:</span> {model.contextWindow.toLocaleString()} tokens
              </span>
            )}
            {model.maxOutputTokens && (
              <span>
                <span className="font-medium">Max output:</span> {model.maxOutputTokens.toLocaleString()}{' '}
                tokens
              </span>
            )}
          </div>
        </div>
        <div className="ml-3 flex-shrink-0">
          <span className="text-xs px-2 py-1 rounded bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200 font-medium">
            Use Original
          </span>
        </div>
      </div>
    </button>
  )
}
