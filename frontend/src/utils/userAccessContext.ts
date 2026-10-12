import type { CatalogModelType } from './modelCatalog'

export interface UserAccessContext {
  model: string
  platform: string
  kind: CatalogModelType
  tab: 'api' | 'test'
  groupId?: number
  keyId?: number
}

const platforms = new Set(['openai', 'anthropic', 'gemini', 'antigravity', 'grok'])
const kinds = new Set<CatalogModelType>(['Chat', 'Images', 'Videos'])

function singleString(value: unknown, maxLength: number): string | undefined {
  if (typeof value !== 'string' || value.length > maxLength || Array.from(value).some(character => character.charCodeAt(0) < 32 || character.charCodeAt(0) === 127)) return undefined
  const result = value.trim()
  return result || undefined
}

function positiveId(value: unknown): number | undefined {
  if (typeof value !== 'string' || !/^[1-9]\d{0,15}$/.test(value)) return undefined
  const id = Number(value)
  return Number.isSafeInteger(id) ? id : undefined
}

/** Carry only model identity and selection IDs between the two access workspace pages. */
export function parseUserAccessContext(query: Record<string, unknown>): UserAccessContext | null {
  const model = singleString(query.model, 256)
  const platform = singleString(query.platform, 32)
  const kind = singleString(query.kind, 16) as CatalogModelType | undefined
  // Model identity must be complete. A key, return URL or partial query is never a context.
  if (!model || /^(?:sk-|Bearer\s)/i.test(model) || !platform || !platforms.has(platform) || !kind || !kinds.has(kind)) return null
  const context: UserAccessContext = { model, platform, kind, tab: query.tab === 'test' ? 'test' : 'api' }
  const groupId = positiveId(query.group)
  const keyId = positiveId(query.key)
  if (groupId !== undefined) context.groupId = groupId
  if (keyId !== undefined) context.keyId = keyId
  return context
}

export function buildUserAccessQuery(
  context: UserAccessContext | null,
  options: { create?: boolean; keyId?: number } = {}
): Record<string, string> {
  if (!context) return {}
  // Revalidate even typed callers so no unknown field or actual API secret can reach a URL.
  const valid = parseUserAccessContext({
    model: context.model,
    platform: context.platform,
    kind: context.kind,
    tab: context.tab,
    group: context.groupId === undefined ? undefined : String(context.groupId),
    key: (options.keyId ?? context.keyId) === undefined ? undefined : String(options.keyId ?? context.keyId)
  })
  if (!valid) return {}
  const query: Record<string, string> = { model: valid.model, platform: valid.platform, kind: valid.kind, tab: valid.tab }
  if (valid.groupId !== undefined) query.group = String(valid.groupId)
  if (valid.keyId !== undefined) query.key = String(valid.keyId)
  if (options.create === true) query.create = '1'
  return query
}
