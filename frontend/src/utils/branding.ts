import { sanitizeUrl } from '@/utils/url'

const defaultLogoUrl = '/starx-logo-transparent.png'

export function updateFavicon(logoUrl: string): void {
  const sanitizedLogoUrl = sanitizeUrl(logoUrl, {
    allowRelative: true,
    allowDataUrl: true,
  })
  const resolvedLogoUrl = sanitizedLogoUrl || defaultLogoUrl

  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'icon'
    document.head.appendChild(link)
  }

  link.type = resolvedLogoUrl.endsWith('.svg') ? 'image/svg+xml' : 'image/png'
  link.href = resolvedLogoUrl
}
