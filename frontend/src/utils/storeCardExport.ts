import type { RedeemCode } from '@/types'

export function buildStoreCardPayload(codes: Pick<RedeemCode, 'code'>[]): string {
  const values = codes.map((item) => item.code.trim()).filter(Boolean)
  return values.length > 0 ? `${values.join('\r\n')}\r\n` : ''
}

export function buildStoreCardFilename(
  codes: Pick<RedeemCode, 'type' | 'value'>[],
  date = new Date()
): string {
  const first = codes[0]
  const amount = first?.type === 'balance' ? `-usd-${first.value.toFixed(2)}` : ''
  return `starx-store-cards${amount}-${date.toISOString().slice(0, 10)}.txt`
}
