import { isSeedreamModel } from './modelTest'

export type CatalogModelType = 'Chat' | 'Images' | 'Videos'

export function isAudioModel(model: string): boolean {
  return /audio|tts|whisper/i.test(model)
}

export function catalogModelType(model: string, billingMode?: string | null): CatalogModelType {
  if (billingMode === 'image' || isSeedreamModel(model) || /^gpt-image/i.test(model)) return 'Images'
  if (billingMode === 'video' || /video|seedance|kling|sora|^minimax-h3(?:-max)?$/i.test(model)) return 'Videos'
  return 'Chat'
}

export function catalogModelEndpoint(type: CatalogModelType, platform?: string, model?: string): string {
  if (type === 'Images') return '/v1/images/generations'
  if (type === 'Videos') {
    if (platform === 'grok') return '/v1/videos/generations'
    return model === 'MiniMax-H3' ? '/v2/video_generation' : '暂未适配'
  }
  return '/v1/chat/completions'
}
