// This file provides a centralized configuration for the frontend application.

// During the Python backend migration, we support dual backends:
// - Go backend (port 1323): Legacy features
// - Python backend (port 1324): Auth and new features

declare global {
  interface Window {
    runtimeConfig?: {
      GO_API_HOST?: string
      PYTHON_API_HOST?: string
      // Legacy single host (for backward compatibility)
      API_HOST?: string
    }
  }
}

// Backend hosts
export const BACKEND_HOSTS = {
  go: window.runtimeConfig?.GO_API_HOST || 'http://localhost:1323',
  python: window.runtimeConfig?.PYTHON_API_HOST || 'http://localhost:1324',
}

// Auth endpoints use Python backend
const AUTH_ENDPOINTS = [
  '/users/db-has-users',
  '/users/create-first-user',
  '/users',
  '/sign-in',
  '/sign-out',
  '/auth-test',
]

/**
 * Get the appropriate backend host for a given endpoint.
 *
 * Auth endpoints → Python backend (port 1324)
 * All other endpoints → Go backend (port 1323)
 */
export function getApiHost(endpoint: string): string {
  // Check if this is an auth endpoint
  const isAuthEndpoint = AUTH_ENDPOINTS.some(authPath =>
    endpoint.startsWith(authPath) || endpoint.includes(authPath)
  )

  return isAuthEndpoint ? BACKEND_HOSTS.python : BACKEND_HOSTS.go
}

// Legacy API_HOST export (uses Go backend by default for backward compatibility)
// Components should migrate to use getApiHost() instead
export const API_HOST = window.runtimeConfig?.API_HOST || BACKEND_HOSTS.go

console.log('Junjo Frontend Backend Hosts:', {
  go: BACKEND_HOSTS.go,
  python: BACKEND_HOSTS.python,
})
