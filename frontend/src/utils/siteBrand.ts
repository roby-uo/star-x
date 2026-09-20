/**
 * Preserve the private brand when an existing database still stores the
 * upstream project's former default name in its public settings.
 */
export const DEFAULT_SITE_NAME = 'star-X API算力中转站'
export const DEFAULT_SITE_SUBTITLE = 'star-X API算力中转站'

const LEGACY_DEFAULT_SITE_NAMES = new Set(['sub2api', 'star-x'])
const LEGACY_DEFAULT_SITE_SUBTITLES = new Set([
  '你的私人 AI API 网关',
  'Your Private AI API Gateway',
])

export function normalizeSiteName(value: unknown): string {
  const siteName = typeof value === 'string' ? value.trim() : ''
  return !siteName || LEGACY_DEFAULT_SITE_NAMES.has(siteName.toLowerCase())
    ? DEFAULT_SITE_NAME
    : siteName
}

export function normalizeSiteSubtitle(value: unknown): string {
  const siteSubtitle = typeof value === 'string' ? value.trim() : ''
  return !siteSubtitle || LEGACY_DEFAULT_SITE_SUBTITLES.has(siteSubtitle)
    ? DEFAULT_SITE_SUBTITLE
    : siteSubtitle
}
