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
 * Handles both Gemini (with complex grouping) and other providers (simple list)
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

  // For non-Gemini providers, create a simple single group structure
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
