import { describe, expect, it } from 'vitest'
import {
  DEFAULT_SITE_NAME,
  DEFAULT_SITE_SUBTITLE,
  normalizeSiteName,
  normalizeSiteSubtitle,
} from '@/utils/siteBrand'

describe('site brand defaults', () => {
  it.each(['', 'star-X', 'sub2api'])('maps legacy site name %j to the current brand', (value) => {
    expect(normalizeSiteName(value)).toBe(DEFAULT_SITE_NAME)
  })

  it.each(['', '你的私人 AI API 网关', 'Your Private AI API Gateway'])(
    'maps legacy subtitle %j to the current brand',
    (value) => {
      expect(normalizeSiteSubtitle(value)).toBe(DEFAULT_SITE_SUBTITLE)
    },
  )

  it('preserves custom branding', () => {
    expect(normalizeSiteName('Example Gateway')).toBe('Example Gateway')
    expect(normalizeSiteSubtitle('Custom subtitle')).toBe('Custom subtitle')
  })
})
