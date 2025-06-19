import NodeExceptionsGraph from './NodeExceptionsGraph'

export default function DashboardPage() {
  // Render the users list
  return (
    <div className={'px-3 py-4 flex flex-col h-dvh overflow-hidden'}>
      <div className={'flex gap-x-3 px-2 items-center'}>
        <div className={'flex gap-x-3 font-bold'}>Dashboard</div>
      </div>
      <hr className={'my-4'} />
      <div className={'px-2 flex-grow overflow-y-auto'}>
        <p className="mb-4">Welcome to Junjo Server. This dashboard is on our roadmap.</p>
        <NodeExceptionsGraph />
      </div>
    </div>
  )
}
