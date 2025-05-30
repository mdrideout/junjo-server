import { useAppSelector } from '../../../root-store/hooks'
import { RootState } from '../../../root-store/store'
import WorkflowStateEventNavButtons from './WorkflowStateDiffNavButtons'
import { formatMicrosecondsSinceEpochToTime } from '../../../util/duration-utils'

interface WorkflowDetailStateNavProps {
  serviceName: string
  workflowSpanID: string
}

export default function WorkflowDetailStateNav(props: WorkflowDetailStateNavProps) {
  const { serviceName, workflowSpanID } = props

  const activeSetStateEvent = useAppSelector(
    (state: RootState) => state.workflowDetailState.activeSetStateEvent,
  )

  if (!activeSetStateEvent) {
    return null
  }

  const atts = activeSetStateEvent.attributes
  const statePatchTime = activeSetStateEvent?.timeUnixNano
  const start_micro = statePatchTime
    ? formatMicrosecondsSinceEpochToTime(activeSetStateEvent?.timeUnixNano / 1000)
    : null

  return (
    <div className={'flex items-start justify-between gap-x-2 text-xs text-zinc-500'}>
      <div>
        {atts['junjo.store.name']} &rarr; {atts['junjo.store.action']} &rarr; {atts.id}
      </div>
      <div className={'font-mono flex items-center gap-x-2'}>
        {start_micro}
        <WorkflowStateEventNavButtons serviceName={serviceName} workflowSpanID={workflowSpanID} />
      </div>
    </div>
  )
}
