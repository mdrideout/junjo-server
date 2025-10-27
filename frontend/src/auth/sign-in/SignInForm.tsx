import { useContext, useEffect, useState } from 'react'
import { useNavigate } from 'react-router'
import { AuthContext } from '../auth-context'
import { getApiHost } from '../../config'

export default function SignInForm() {
  const [error, setError] = useState<string | null>(null)
  const { isAuthenticated, login } = useContext(AuthContext)
  const navigate = useNavigate()

  useEffect(() => {
    if (isAuthenticated) {
      navigate('/')
    }
  }, [isAuthenticated, navigate])

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()

    setError(null)

    const formData = new FormData(event.currentTarget)
    const email = formData.get('email') as string
    const password = formData.get('password') as string

    // Perform sign in
    try {
      const endpoint = '/sign-in'
      const response = await fetch(`${getApiHost(endpoint)}${endpoint}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password }),
        credentials: 'include',
      })

      if (!response.ok) {
        const data = await response.json()

        if (data.detail) {
          throw new Error(data.detail)
        }
        if (data.message) {
          throw new Error(data.message)
        }
        throw new Error('Sign-in failed')
      }

      // Python backend uses SameSite cookies for CSRF protection
      // No separate CSRF token needed

      login('') // Token not used with session-based auth

      navigate('/')
    } catch (err: any) {
      setError(err.message)
    }
  }

  return (
    <>
      <h1>SIGN IN</h1>
      <form onSubmit={handleSubmit} className="mb-6 text-black w-xs">
        <div className="flex flex-col gap-y-2">
          <input type="hidden" name="actionType" value="signIn" />
          <input
            type="email"
            name="email"
            placeholder="Email address"
            required
            className="bg-slate-300 text-black py-1 px-2 rounded-sm"
          />
          <input
            type="password"
            name="password"
            placeholder="Password"
            autoComplete="current-password"
            required
            className="bg-slate-300 text-black py-1 px-2 rounded-sm"
          />
          <button
            type="submit"
            className="py-1 px-2 bg-zinc-200 hover:bg-zinc-300 cursor-pointer rounded-md font-bold"
          >
            Sign In
          </button>
          {error && <p className="text-red-500">{error}</p>}
        </div>
      </form>
    </>
  )
}
