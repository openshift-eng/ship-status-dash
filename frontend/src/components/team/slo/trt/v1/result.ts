import type { SLOEvaluation, SLOWorkspace } from '../../../../../types'

export interface TRTPayloadStream {
  release_controller: string
  name: string
  window?: string
  min_accepted?: number
}

export interface TRTPayloadSettings {
  window: string
  min_accepted: number
  recent_payloads: number
  streams: TRTPayloadStream[]
}

export interface TRTPayloadStreamEval {
  key: string
  accepted: number
  met: boolean
  last_accepted_at?: string
  window?: string
  target?: { min_accepted: number }
}

export interface TRTPayloadResult {
  window: string
  target: { min_accepted: number }
  streams: TRTPayloadStreamEval[]
}

export const trtPayloadSettings = (workspace: SLOWorkspace): TRTPayloadSettings | undefined => {
  if (workspace.kind !== 'payload_streams' || workspace.schema_version !== 1 || !workspace.spec) {
    return undefined
  }
  return workspace.spec as TRTPayloadSettings
}

export const trtPayloadResult = (evaluation?: SLOEvaluation): TRTPayloadResult | undefined => {
  if (!evaluation || evaluation.source !== 'payload_acceptance' || !evaluation.result) {
    return undefined
  }
  return evaluation.result as TRTPayloadResult
}

export const worstMiss = (result: TRTPayloadResult): TRTPayloadStreamEval | undefined => {
  const missed = result.streams.filter((stream) => !stream.met)
  if (missed.length === 0) {
    return undefined
  }
  return missed.reduce((worst, stream) => {
    if (!worst.last_accepted_at) {
      return worst
    }
    if (!stream.last_accepted_at) {
      return stream
    }
    return new Date(stream.last_accepted_at) < new Date(worst.last_accepted_at) ? stream : worst
  })
}
