export const MINIMAX_H3_MODELS = ['MiniMax-H3', 'MiniMax-H3-Max'] as const

export interface MiniMaxVideoSpec {
  resolutions: string[]
  minDuration: number
  maxDuration: number
  modes: string[]
  prices: Record<string, number>
}

/** Only expose the generation modes supported by our relay adapter. */
export function miniMaxVideoSpec(model?: string): MiniMaxVideoSpec | undefined {
  if (model === 'MiniMax-H3') return {
    resolutions: ['768P', '2K'], minDuration: 4, maxDuration: 15,
    modes: ['text', 'first_frame', 'first_last_frame'], prices: { '768P': 0.08, '2K': 0.13 }
  }
  if (model === 'MiniMax-H3-Max') return {
    resolutions: ['480P', '768P'], minDuration: 5, maxDuration: 15,
    modes: ['text', 'first_frame', 'first_last_frame'], prices: { '480P': 0.05, '768P': 0.08 }
  }
  return undefined
}
