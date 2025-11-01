/**
 * Utility functions for JSON Schema manipulation across LLM providers
 */

/**
 * Ensures a JSON schema is compatible with OpenAI's strict mode by adding
 * `additionalProperties: false` to all object types recursively.
 *
 * OpenAI's strict structured outputs require that every object in the schema
 * explicitly sets `additionalProperties: false`. Other providers (Gemini, Anthropic)
 * don't have this requirement, so schemas from those providers need transformation.
 *
 * @param schema - The original JSON schema (not modified)
 * @returns A new schema with `additionalProperties: false` added to all objects
 */
export function ensureOpenAISchemaCompatibility(schema: Record<string, any>): Record<string, any> {
  // Deep clone to avoid mutating the original
  const cloned = JSON.parse(JSON.stringify(schema))

  function addAdditionalPropertiesRecursive(obj: any): void {
    if (typeof obj !== 'object' || obj === null) {
      return
    }

    // If this is an object type definition, ensure additionalProperties is false
    if (obj.type === 'object') {
      obj.additionalProperties = false

      // Recursively process properties
      if (obj.properties && typeof obj.properties === 'object') {
        for (const key in obj.properties) {
          addAdditionalPropertiesRecursive(obj.properties[key])
        }
      }

      // Handle patternProperties if present
      if (obj.patternProperties && typeof obj.patternProperties === 'object') {
        for (const pattern in obj.patternProperties) {
          addAdditionalPropertiesRecursive(obj.patternProperties[pattern])
        }
      }

      // Handle additionalProperties if it's a schema (not just false/true)
      if (typeof obj.additionalProperties === 'object') {
        addAdditionalPropertiesRecursive(obj.additionalProperties)
      }
    }

    // Handle array items
    if (obj.type === 'array') {
      if (obj.items) {
        if (Array.isArray(obj.items)) {
          // Tuple validation
          obj.items.forEach((item: any) => addAdditionalPropertiesRecursive(item))
        } else {
          // Single schema for all items
          addAdditionalPropertiesRecursive(obj.items)
        }
      }

      // Handle prefixItems (JSON Schema 2020-12)
      if (obj.prefixItems && Array.isArray(obj.prefixItems)) {
        obj.prefixItems.forEach((item: any) => addAdditionalPropertiesRecursive(item))
      }
    }

    // Handle oneOf, anyOf, allOf
    ['oneOf', 'anyOf', 'allOf'].forEach((keyword) => {
      if (obj[keyword] && Array.isArray(obj[keyword])) {
        obj[keyword].forEach((subSchema: any) => addAdditionalPropertiesRecursive(subSchema))
      }
    })

    // Handle not
    if (obj.not) {
      addAdditionalPropertiesRecursive(obj.not)
    }

    // Handle definitions/defs (common places for reusable schemas)
    ['definitions', '$defs'].forEach((keyword) => {
      if (obj[keyword] && typeof obj[keyword] === 'object') {
        for (const key in obj[keyword]) {
          addAdditionalPropertiesRecursive(obj[keyword][key])
        }
      }
    })
  }

  addAdditionalPropertiesRecursive(cloned)
  return cloned
}
