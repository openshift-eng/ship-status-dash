import type { SLOEvaluation, SLOWorkspace } from '../../../../../types'

export interface TRTPayloadStream {
  release_controller: string
  release_controller_url?: string
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
  release_controller: string
  release_controller_url?: string
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

export interface ControllerGroup {
  controller: string
  url?: string
  streams: TRTPayloadStreamEval[]
}

export const groupStreamsByController = (streams: TRTPayloadStreamEval[]): ControllerGroup[] => {
  const groups: ControllerGroup[] = []
  for (const stream of streams) {
    const last = groups[groups.length - 1]
    if (last && last.controller === stream.release_controller) {
      last.streams.push(stream)
    } else {
      groups.push({
        controller: stream.release_controller,
        url: stream.release_controller_url,
        streams: [stream],
      })
    }
  }
  return groups
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

export const parseHours = (duration: string): number => {
  const match = duration.match(/^(\d+(?:\.\d+)?)h$/)
  return match ? parseFloat(match[1]) : 0
}

export const worstMiss = (result: TRTPayloadResult): TRTPayloadStreamEval | undefined => {
  const missed = result.streams.filter((stream) => !stream.met)
  if (missed.length === 0) {
    return undefined
  }
  const defaultWindowHours = parseHours(result.window)
  return missed.reduce((worst, stream) => {
    const worstWindow = parseHours(worst.window || result.window) || defaultWindowHours
    const streamWindow = parseHours(stream.window || result.window) || defaultWindowHours
    const worstElapsed = worst.last_accepted_at
      ? (Date.now() - new Date(worst.last_accepted_at).getTime()) / 36e5
      : Infinity
    const streamElapsed = stream.last_accepted_at
      ? (Date.now() - new Date(stream.last_accepted_at).getTime()) / 36e5
      : Infinity
    const worstExcess = worstElapsed - worstWindow
    const streamExcess = streamElapsed - streamWindow
    return streamExcess > worstExcess ? stream : worst
  })
}
