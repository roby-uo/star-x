import { describe, expect, it } from 'vitest'
import { formatUsageReasoning } from '../usageReasoning'

describe('formatUsageReasoning', () => {
  it('localizes recorded effort without changing the underlying value', () => {
    expect(formatUsageReasoning('low', 'zh')).toBe('低')
    expect(formatUsageReasoning('xhigh', 'zh')).toBe('极高')
    expect(formatUsageReasoning('max', 'zh')).toBe('最高')
    expect(formatUsageReasoning('high', 'en')).toBe('High')
    expect(formatUsageReasoning(null, 'zh')).toBe('-')
  })
})
