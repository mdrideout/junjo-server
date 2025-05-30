import { ListUsersResponse, ListUsersResponseSchema } from '../schema'

export const fetchUsers = async (): Promise<ListUsersResponse> => {
  const response = await fetch(`http://localhost:1323/users`, {
    method: 'GET',
    credentials: 'include',
    headers: {
      Accept: 'application/json',
    },
  })

  if (!response.ok) {
    throw new Error(`fetchUsers failed: ${response.statusText}`)
  }

  const data = await response.json()

  try {
    // Validate the data
    return ListUsersResponseSchema.parse(data)
  } catch (error) {
    console.error('Data validation error:', error)
    throw new Error('Invalid data received from server')
  }
}
