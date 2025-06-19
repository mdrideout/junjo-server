import { useEffect, useState } from 'react'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts'

interface NodeExceptionData {
  exception_day: string
  exception_count: number
}

const NodeExceptionsGraph = () => {
  const [data, setData] = useState<NodeExceptionData[]>([])
  const [loading, setLoading] = useState<boolean>(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const fetchData = async () => {
      setLoading(true)
      setError(null)
      try {
        const response = await fetch('/api/otel/node-exceptions')
        if (!response.ok) {
          throw new Error(`HTTP error! status: ${response.status}`)
        }
        const result = (await response.json()) as NodeExceptionData[]
        // Format date for display if necessary, e.g., from "YYYY-MM-DDTHH:MM:SSZ" to "YYYY-MM-DD"
        const formattedData = result.map((item) => ({
          ...item,
          exception_day: item.exception_day.split('T')[0],
        }))
        setData(formattedData)
      } catch (e) {
        if (e instanceof Error) {
          setError(e.message)
        } else {
          setError('An unknown error occurred')
        }
        console.error('Failed to fetch node exceptions:', e)
      } finally {
        setLoading(false)
      }
    }

    fetchData()
  }, [])

  if (loading) {
    return <div>Loading graph data...</div>
  }

  if (error) {
    return <div className="text-red-500">Error loading graph data: {error}</div>
  }

  if (data.length === 0) {
    return <div>No node exception data available for the last 7 days.</div>
  }

  return (
    <div style={{ width: '100%', height: 300 }}>
      <h3 className="text-lg font-semibold mb-2">Node Exceptions (Last 7 Days)</h3>
      <ResponsiveContainer>
        <BarChart
          data={data}
          margin={{
            top: 5,
            right: 30,
            left: 20,
            bottom: 5,
          }}
        >
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis dataKey="exception_day" />
          <YAxis allowDecimals={false} />
          <Tooltip />
          <Legend />
          <Bar dataKey="exception_count" fill="#ef4444" name="Exceptions" />
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}

export default NodeExceptionsGraph
