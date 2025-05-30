import { JSX } from 'react'
import { JunjoSpanType, OtelSpan } from '../../otel/schemas/schemas'
import {
  CubeIcon,
  Square3Stack3DIcon,
  Squares2X2Icon,
  CircleStackIcon,
  QuestionMarkCircleIcon,
  SparklesIcon,
} from '@heroicons/react/24/solid'
import { OpenInferenceSpanKind } from '../../otel/schemas/attribute-schemas-openinference'

/**
 * Span Icon Constructor
 * Returns the icon for the span based on attribute information
 * @param span
 */
export function SpanIconConstructor(props: {
  span: OtelSpan | undefined
  active: boolean
  size?: string
}): JSX.Element {
  const { span, active, size = 'size-5' } = props
  const iconColor = active ? 'text-amber-500' : 'text-zinc-600 dark:text-zinc-400'

  // Undefined
  if (span === undefined) {
    return <QuestionMarkCircleIcon className={`${size} ${iconColor}`} />
  }

  const attributes = span.attributes_json

  // Junjo Workflow Span
  if (span.junjo_span_type === JunjoSpanType.SUBFLOW) {
    return <Square3Stack3DIcon className={`${size} ${iconColor}`} />
  }

  // Junjo Node Span
  if (span.junjo_span_type === JunjoSpanType.NODE) {
    return <CubeIcon className={`${size} ${iconColor}`} />
  }

  // Junjo RunConcurrent Span
  if (span.junjo_span_type === JunjoSpanType.RUN_CONCURRENT) {
    return <Squares2X2Icon className={`${size} ${iconColor}`} />
  }

  // Database Span
  // If attributes contains "db.system" key
  if (attributes['db.system']) {
    return <CircleStackIcon className={`${size} ${iconColor}`} />
  }

  // OpenTelemetry Genai Attributes: gen_ai.system
  if (attributes['gen_ai.system']) {
    return <SparklesIcon className={`${size} ${iconColor}`} />
  }

  // OpenInference LLM Attributes
  if (attributes['openinference.span.kind'] === OpenInferenceSpanKind.LLM) {
    return <SparklesIcon className={`${size} ${iconColor}`} />
  }

  // Default
  return <div></div>
}
