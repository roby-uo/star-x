import { describe, expect, it } from 'vitest'
import { buildStoreCardFilename, buildStoreCardPayload } from '../storeCardExport'

describe('store card export', () => {
  it('exports one trimmed code per CRLF-delimited line', () => {
    expect(buildStoreCardPayload([{ code: ' CODE-A ' }, { code: 'CODE-B' }])).toBe(
      'CODE-A\r\nCODE-B\r\n'
    )
  })

  it('uses the balance amount in the inventory filename', () => {
    expect(
      buildStoreCardFilename(
        [{ type: 'balance', value: 10 }],
        new Date('2026-08-14T00:00:00.000Z')
      )
    ).toBe('starx-store-cards-usd-10.00-2026-08-14.txt')
  })
})
