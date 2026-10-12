import { describe, expect, it } from 'vitest'
import { buildUserAccessQuery, parseUserAccessContext, type UserAccessContext } from '../userAccessContext'

const identity = { model: 'MiniMax-H3', platform: 'openai', kind: 'Videos' }

describe('user access workspace context', () => {
  it('round trips a model and selection IDs without carrying unrelated query fields', () => {
    const parsed = parseUserAccessContext({ ...identity, group: '7', key: '43', tab: 'test', create: '1', return: 'https://outside.example', secret: 'private-key' })
    expect(parsed).toEqual({ ...identity, groupId: 7, keyId: 43, tab: 'test' })
    expect(buildUserAccessQuery(parsed)).toEqual({ ...identity, group: '7', key: '43', tab: 'test' })
  })

  it('only adds explicit creation and a valid newly created numeric key ID', () => {
    const context = parseUserAccessContext({ ...identity, group: '7', key: '43' })
    expect(buildUserAccessQuery(context, { create: true, keyId: 64 })).toEqual({ ...identity, group: '7', key: '64', tab: 'api', create: '1' })
    expect(buildUserAccessQuery(context, { keyId: -1 })).toEqual({ ...identity, group: '7', tab: 'api' })
    expect(buildUserAccessQuery(null, { create: true })).toEqual({})
  })

  it.each([
    {}, { model: 'MiniMax-H3' }, { ...identity, model: '' },
    { ...identity, model: ['MiniMax-H3'] }, { ...identity, platform: ['openai'] },
    { ...identity, kind: ['Videos'] }, { ...identity, platform: 'unknown' },
    { ...identity, kind: 'Audio' }, { ...identity, model: 'a'.repeat(257) },
    { ...identity, model: 'invalid\nmodel' }, { ...identity, model: 'sk-api-private' },
    { ...identity, model: 'Bearer private-key' }
  ])('rejects incomplete, oversized, array or invalid identity: %j', query => {
    expect(parseUserAccessContext(query)).toBeNull()
  })

  it.each(['0', '-1', '1.5', '1e3', '01', 'Infinity', '9007199254740992', '12345678901234567', ['7'], 7, null])('drops an invalid selection ID: %j', value => {
    expect(parseUserAccessContext({ ...identity, group: value, key: value })).toEqual({ ...identity, tab: 'api' })
  })

  it('defaults unknown or array tab values to API without dropping valid identity', () => {
    expect(parseUserAccessContext({ ...identity, tab: ['test'] })?.tab).toBe('api')
    expect(parseUserAccessContext({ ...identity, tab: 'other' })?.tab).toBe('api')
  })

  it('revalidates builder inputs and never serializes secret or arbitrary redirect fields', () => {
    const context = { ...identity, kind: 'Videos', tab: 'api', groupId: 7, keyId: 43, secret: 'sk-private', return: '/admin' } as UserAccessContext
    expect(buildUserAccessQuery(context)).toEqual({ ...identity, tab: 'api', group: '7', key: '43' })
    expect(buildUserAccessQuery({ ...context, keyId: Number.NaN })).toEqual({ ...identity, tab: 'api', group: '7' })
    expect(buildUserAccessQuery({ ...context, model: 'sk-private' })).toEqual({})
  })
})
