import { describe, expect, it } from 'vitest'
import { identifyAgentClient } from '../agentClient'

describe('identifyAgentClient', () => {
  it('recognizes known client identifiers only', () => {
    expect(identifyAgentClient('WorkBuddy/1.2')).toBe('WorkBuddy')
    expect(identifyAgentClient('codex-cli/1.0')).toBe('Codex')
    expect(identifyAgentClient('Mozilla/5.0 Chrome/130')).toBeNull()
    expect(identifyAgentClient(null)).toBeNull()
  })
})
