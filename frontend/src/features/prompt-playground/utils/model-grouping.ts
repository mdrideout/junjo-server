import type { ModelInfo } from '../schemas/unified-request'

export interface ParsedModelInfo {
  original: ModelInfo
  productFamily: string // "gemini", "gemma", etc. - PRIMARY grouping
  version: string // "2.5", "2.0", "1.5", etc. - SECONDARY grouping
  variant: string // "flash", "pro", "flash-8b", etc. - the specific model variant
  releaseType: 'stable' | 'preview' | 'exp' // for filtering only
  displayName: string
  sortKey: string // for sorting within groups
}

/**
 * Parse an Anthropic model name into its components
 * Examples:
 * - claude-3-opus-20240229 → { productFamily: "opus", version: "3", variant: "opus", releaseType: "stable" }
 * - claude-3-5-sonnet-20240620 → { productFamily: "sonnet", version: "3.5", variant: "sonnet", releaseType: "stable" }
 * - claude-3-haiku-20240307 → { productFamily: "haiku", version: "3", variant: "haiku", releaseType: "stable" }
 * - claude-sonnet-4-5-20250929 → { productFamily: "sonnet", version: "4.5", variant: "sonnet", releaseType: "stable" }
 */
export function parseAnthropicModel(model: ModelInfo): ParsedModelInfo {
  const id = model.id.toLowerCase()

  // All Anthropic models are considered stable
  const releaseType: 'stable' | 'preview' | 'exp' = 'stable'

  // Extract product family (haiku, sonnet, opus)
  let productFamily = 'unknown'
  let variant = 'unknown'

  if (id.includes('haiku')) {
    productFamily = 'haiku'
    variant = 'haiku'
  } else if (id.includes('sonnet')) {
    productFamily = 'sonnet'
    variant = 'sonnet'
  } else if (id.includes('opus')) {
    productFamily = 'opus'
    variant = 'opus'
  }

  // Extract version - handle both patterns:
  // claude-3-opus-... (version 3)
  // claude-3-5-sonnet-... (version 3.5)
  // claude-sonnet-4-5-... (version 4.5)
  let version = '0.0'

  // Try pattern: claude-X-Y-family or claude-family-X-Y
  const versionMatch1 = id.match(/claude-(\d+)-(\d+)/)
  const versionMatch2 = id.match(/claude-(?:haiku|sonnet|opus)-(\d+)-(\d+)/)

  if (versionMatch1) {
    const [, major, minor] = versionMatch1
    version = minor === '0' ? major : `${major}.${minor}`
  } else if (versionMatch2) {
    const [, major, minor] = versionMatch2
    version = minor === '0' ? major : `${major}.${minor}`
  } else {
    // Try single version number: claude-X-family
    const singleVersionMatch = id.match(/claude-(\d+)-/)
    if (singleVersionMatch) {
      version = singleVersionMatch[1]
    }
  }

  // Create display name - capitalize the product family
  const displayName = productFamily.charAt(0).toUpperCase() + productFamily.slice(1)

  // Create sort key: version (descending), then product family
  // Product family order: opus (highest tier) > sonnet > haiku
  const familyOrder = productFamily === 'opus' ? 0 : productFamily === 'sonnet' ? 1 : productFamily === 'haiku' ? 2 : 3
  const sortKey = `${version}-${familyOrder}`

  return {
    original: model,
    productFamily,
    version,
    variant,
    releaseType,
    displayName,
    sortKey,
  }
}

/**
 * Parse a Gemini/Gemma model name into its components
 * Examples:
 * - gemini-2.5-flash-exp → { productFamily: "gemini", version: "2.5", variant: "flash", releaseType: "exp" }
 * - gemini-2.0-flash-preview-06-17 → { productFamily: "gemini", version: "2.0", variant: "flash", releaseType: "preview" }
 * - gemini-1.5-pro → { productFamily: "gemini", version: "1.5", variant: "pro", releaseType: "stable" }
 * - gemma-2-9b-it → { productFamily: "gemma", version: "2", variant: "9b-it", releaseType: "stable" }
 */
export function parseGeminiModel(model: ModelInfo): ParsedModelInfo {
  const id = model.id.toLowerCase()

  // Extract release type
  let releaseType: 'stable' | 'preview' | 'exp' = 'stable'
  if (id.includes('-exp')) {
    releaseType = 'exp'
  } else if (id.includes('-preview')) {
    releaseType = 'preview'
  }

  // Extract product family (gemini, gemma, etc.) - first segment
  const parts = id.split('-')
  const productFamily = parts[0] // "gemini" or "gemma"

  // Extract version (e.g., "2.5", "2.0", "1.5")
  const versionMatch = id.match(/^(?:gemini|gemma)-(\d+(?:\.\d+)?)/)
  const version = versionMatch ? versionMatch[1] : '0.0'

  // Extract variant (everything between version and release type suffixes)
  let variant = 'unknown'
  const withoutFamily = id.replace(`${productFamily}-`, '')
  const withoutVersion = withoutFamily.replace(`${version}-`, '')

  // Remove release type suffixes and date suffixes
  let cleanVariant = withoutVersion
    .replace(/-exp$/i, '')
    .replace(/-preview.*$/i, '')

  // The remaining is the variant
  variant = cleanVariant || 'base'

  // Create display name (just the variant name, capitalized)
  const displayName = variant.split('-').map(part =>
    part.charAt(0).toUpperCase() + part.slice(1)
  ).join(' ')

  // Create sort key: variant name, then release type (stable first)
  const releaseTypeOrder = releaseType === 'stable' ? 0 : releaseType === 'preview' ? 1 : 2
  const sortKey = `${variant}-${releaseTypeOrder}`

  return {
    original: model,
    productFamily,
    version,
    variant,
    releaseType,
    displayName,
    sortKey,
  }
}

/**
 * Group models hierarchically: Product Family > Version > Release Type > Variants
 */
export interface ReleaseTypeGroup {
  releaseType: 'stable' | 'preview' | 'exp'
  displayName: string
  models: ParsedModelInfo[]
}

export interface VersionGroup {
  version: string
  releaseTypeGroups: ReleaseTypeGroup[]
}

export interface ProductFamilyGroup {
  productFamily: string
  displayName: string
  versionGroups: VersionGroup[]
}

export function groupGeminiModels(models: ModelInfo[]): ProductFamilyGroup[] {
  // Parse all models
  const parsed = models.map(parseGeminiModel)

  // Group by product family first
  const familyMap = new Map<string, ParsedModelInfo[]>()
  for (const model of parsed) {
    if (!familyMap.has(model.productFamily)) {
      familyMap.set(model.productFamily, [])
    }
    familyMap.get(model.productFamily)!.push(model)
  }

  // Build hierarchical structure
  const result: ProductFamilyGroup[] = []
  for (const [productFamily, familyModels] of familyMap) {
    // Group by version within this family
    const versionMap = new Map<string, ParsedModelInfo[]>()
    for (const model of familyModels) {
      if (!versionMap.has(model.version)) {
        versionMap.set(model.version, [])
      }
      versionMap.get(model.version)!.push(model)
    }

    // Build version groups with release type sub-groups
    const versionGroups: VersionGroup[] = []
    for (const [version, versionModels] of versionMap) {
      // Group by release type within this version
      const releaseTypeMap = new Map<'stable' | 'preview' | 'exp', ParsedModelInfo[]>()
      for (const model of versionModels) {
        if (!releaseTypeMap.has(model.releaseType)) {
          releaseTypeMap.set(model.releaseType, [])
        }
        releaseTypeMap.get(model.releaseType)!.push(model)
      }

      // Sort models within each release type by sortKey
      const releaseTypeGroups: ReleaseTypeGroup[] = []
      for (const [releaseType, releaseTypeModels] of releaseTypeMap) {
        releaseTypeModels.sort((a, b) => a.sortKey.localeCompare(b.sortKey))

        const displayName = releaseType === 'stable' ? 'Stable' :
                           releaseType === 'preview' ? 'Preview' :
                           'Experimental'

        releaseTypeGroups.push({
          releaseType,
          displayName,
          models: releaseTypeModels
        })
      }

      // Sort release type groups: stable first, then preview, then exp
      releaseTypeGroups.sort((a, b) => {
        const order = { stable: 0, preview: 1, exp: 2 }
        return order[a.releaseType] - order[b.releaseType]
      })

      versionGroups.push({ version, releaseTypeGroups })
    }

    // Sort version groups by version number (descending - newest first)
    versionGroups.sort((a, b) => {
      const aNum = parseFloat(a.version)
      const bNum = parseFloat(b.version)
      return bNum - aNum // Descending order
    })

    result.push({
      productFamily,
      displayName: productFamily.charAt(0).toUpperCase() + productFamily.slice(1),
      versionGroups,
    })
  }

  // Sort product families alphabetically
  result.sort((a, b) => a.productFamily.localeCompare(b.productFamily))

  return result
}

/**
 * Group Anthropic models by Product Family (Haiku/Sonnet/Opus) with simple flat structure
 */
export function groupAnthropicModels(models: ModelInfo[]): ProductFamilyGroup[] {
  // Parse all models
  const parsed = models.map(parseAnthropicModel)

  // Group by product family (haiku, sonnet, opus)
  const familyMap = new Map<string, ParsedModelInfo[]>()
  for (const model of parsed) {
    if (!familyMap.has(model.productFamily)) {
      familyMap.set(model.productFamily, [])
    }
    familyMap.get(model.productFamily)!.push(model)
  }

  // Build simple structure: Product Family > Models (no version subgroups)
  const result: ProductFamilyGroup[] = []
  for (const [productFamily, familyModels] of familyMap) {
    // Sort models by version descending (newest first), then by model ID
    familyModels.sort((a, b) => {
      const versionCompare = parseFloat(b.version) - parseFloat(a.version)
      if (versionCompare !== 0) return versionCompare
      return a.original.id.localeCompare(b.original.id)
    })

    // Create a single version group with all models
    const versionGroups: VersionGroup[] = [
      {
        version: '', // No version header needed
        releaseTypeGroups: [
          {
            releaseType: 'stable',
            displayName: 'Stable',
            models: familyModels
          }
        ]
      }
    ]

    result.push({
      productFamily,
      displayName: productFamily.charAt(0).toUpperCase() + productFamily.slice(1),
      versionGroups,
    })
  }

  // Sort product families by tier: Opus > Sonnet > Haiku
  result.sort((a, b) => {
    const order = { opus: 0, sonnet: 1, haiku: 2 }
    const aOrder = order[a.productFamily as keyof typeof order] ?? 99
    const bOrder = order[b.productFamily as keyof typeof order] ?? 99
    return aOrder - bOrder
  })

  return result
}

/**
 * Filter models by release type
 */
export function filterModelsByReleaseType(
  groups: ProductFamilyGroup[],
  options: { includeStable: boolean; includePreview: boolean; includeExp: boolean }
): ProductFamilyGroup[] {
  return groups
    .map((familyGroup) => ({
      ...familyGroup,
      versionGroups: familyGroup.versionGroups
        .map((versionGroup) => ({
          ...versionGroup,
          releaseTypeGroups: versionGroup.releaseTypeGroups.filter((releaseTypeGroup) => {
            if (releaseTypeGroup.releaseType === 'stable' && options.includeStable) return true
            if (releaseTypeGroup.releaseType === 'preview' && options.includePreview) return true
            if (releaseTypeGroup.releaseType === 'exp' && options.includeExp) return true
            return false
          }),
        }))
        .filter((versionGroup) => versionGroup.releaseTypeGroups.length > 0), // Remove empty version groups
    }))
    .filter((familyGroup) => familyGroup.versionGroups.length > 0) // Remove empty family groups
}

/**
 * Organize models into hierarchical groups for display
 * Handles Gemini, Anthropic (with complex grouping) and other providers (simple list)
 */
export function organizeModels(
  models: ModelInfo[],
  provider: string | null,
  filters: { includeStable: boolean; includePreview: boolean; includeExp: boolean }
): ProductFamilyGroup[] {
  if (provider === 'gemini') {
    const groups = groupGeminiModels(models)
    return filterModelsByReleaseType(groups, filters)
  }

  if (provider === 'anthropic') {
    const groups = groupAnthropicModels(models)
    // Anthropic models are all stable, but we still apply the filter for consistency
    return filterModelsByReleaseType(groups, filters)
  }

  // For other providers, create a simple single group structure
  return [
    {
      productFamily: provider || 'models',
      displayName: provider ? provider.charAt(0).toUpperCase() + provider.slice(1) : 'Models',
      versionGroups: [
        {
          version: '',
          releaseTypeGroups: [
            {
              releaseType: 'stable' as const,
              displayName: 'Stable',
              models: models.map((model) => ({
                original: model,
                productFamily: provider || 'unknown',
                version: '',
                variant: model.name || model.id,
                releaseType: 'stable' as const,
                displayName: model.name || model.id,
                sortKey: model.id,
              })),
            },
          ],
        },
      ],
    },
  ]
}
