import { formatReasoningEffort } from './format'

const chineseLabels: Record<string, string> = {
  none: '无',
  minimal: '极低',
  low: '低',
  medium: '中',
  high: '高',
  xhigh: '极高',
  extrahigh: '极高',
  max: '最高'
}

export function formatUsageReasoning(effort: string | null | undefined, locale: string): string {
  const normalized = (effort ?? '').toLowerCase().replace(/[-_\s]/g, '')
  if (locale.startsWith('zh') && chineseLabels[normalized]) return chineseLabels[normalized]
  return formatReasoningEffort(effort)
}
