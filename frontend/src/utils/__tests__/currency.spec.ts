import { describe, expect, it } from 'vitest'
import { cnyToLedger, formatCny, formatCnyFromLedger, formatMoney, ledgerToCny } from '../currency'

describe('fixed CNY display over the legacy ledger', () => {
  it('preserves the purchasing power of existing balances', () => {
    expect(formatCnyFromLedger(100)).toBe('¥620.00')
  })

  it('round trips CNY deposits without rounding the ledger to cents', () => {
    expect(cnyToLedger(100)).toBeCloseTo(16.129032258, 9)
    expect(ledgerToCny(cnyToLedger(100))).toBeCloseTo(100, 12)
    expect(cnyToLedger(0.01)).toBeGreaterThan(0)
    expect(formatCnyFromLedger(cnyToLedger(0.01))).toBe('¥0.01')
  })

  it('distinguishes cash already in CNY from USD ledger amounts', () => {
    expect(formatMoney(100, 'CNY')).toBe('¥100.00')
    expect(formatMoney(100, 'USD')).toBe('¥620.00')
    expect(formatCny(0.0001)).toBe('¥0.000100')
  })
})
