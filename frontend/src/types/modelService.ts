import { miniMaxVideoSpec } from '@/utils/minimaxVideo'

export interface ModelServiceRule {
  enabled: boolean
  resolutions: string[]
  max_duration: number
  modes: string[]
}
export interface ModelServicePolicy {
  platform: string
  model: string
  image_sizes?: string[]
  max_images?: number
  kind: 'chat' | 'image' | 'video'
  state: 'draft' | 'published' | 'paused'
  resolutions?: string[]
  min_duration?: number
  max_duration?: number
  modes?: string[]
  prices?: Record<string, number>
  groups?: Record<string, ModelServiceRule>
}
export interface VideoQuote {
  currency: string
  unit_price: number
  multiplier: number
  total: number
  version: string
}
export const videoModes = [{ value: 'text', label: '文生视频' }, { value: 'first_frame', label: '首帧图生视频' }, { value: 'first_last_frame', label: '首尾帧视频' }]
export function defaultModelPolicy(model: string, platform: string, kind: ModelServicePolicy['kind']): ModelServicePolicy {
  const spec = kind === 'video' && platform === 'openai' ? miniMaxVideoSpec(model) : undefined
  return { model, platform, kind, state: 'draft', ...(spec ? { resolutions: spec.resolutions, min_duration: spec.minDuration, max_duration: spec.maxDuration, modes: spec.modes, prices: spec.prices } : {}), groups: {} }
}
