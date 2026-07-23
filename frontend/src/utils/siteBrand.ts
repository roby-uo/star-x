/**
 * Preserve the private brand when an existing database still stores the
 * upstream project's former default name in its public settings.
 */
export const DEFAULT_SITE_NAME = 'star-X'

export function normalizeSiteName(value: unknown): string {
  const siteName = typeof value === 'string' ? value.trim() : ''
  return !siteName || siteName.toLowerCase() === 'sub2api'
    ? DEFAULT_SITE_NAME
    : siteName
}
