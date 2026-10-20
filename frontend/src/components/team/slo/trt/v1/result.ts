import type { SLOEvaluation, SLOWorkspace } from '../../../../../types'
import { formatAge } from '../../format'

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

interface Controllered {
  release_controller: string
  release_controller_url?: string
}

export interface ControllerGroup<T extends Controllered = TRTPayloadStreamEval> {
  controller: string
  url?: string
  streams: T[]
}

export const groupStreamsByController = <T extends Controllered>(
  streams: T[],
): ControllerGroup<T>[] => {
  const map = new Map<string, ControllerGroup<T>>()
  const order: string[] = []
  for (const stream of streams) {
    const key = stream.release_controller
    let group = map.get(key)
    if (!group) {
      group = { controller: key, url: stream.release_controller_url, streams: [] }
      map.set(key, group)
      order.push(key)
    }
    group.streams.push(stream)
  }
  return order.map((key) => map.get(key)!)
}

export const streamTooltip = (
  stream: { last_accepted_at?: string; window?: string },
  defaultWindow: string,
): string => {
  const windowHours = parseHours(stream.window || defaultWindow) || parseHours(defaultWindow)
  if (!stream.last_accepted_at) {
    return `Never accepted; expected every ${windowHours}h`
  }
  return `Last accepted ${formatAge(stream.last_accepted_at)} ago; expected every ${windowHours}h`
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
  const units: Record<string, number> = { h: 1, m: 1 / 60, s: 1 / 3600, ms: 1 / 3_600_000 }
  if (!/^(\d+(?:\.\d+)?(?:h|ms|m|s))+$/.test(duration)) {
    return 0
  }
  let hours = 0
  for (const [, value, unit] of duration.matchAll(/(\d+(?:\.\d+)?)(h|ms|m|s)/g)) {
    hours += parseFloat(value) * units[unit]
  }
  return hours
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
