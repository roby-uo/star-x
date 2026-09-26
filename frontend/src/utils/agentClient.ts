const knownClients: Array<[RegExp, string]> = [
  [/\bworkbuddy(?:[\s\/-]|$)/i, 'WorkBuddy'],
  [/\bclaude(?:-code|-cli)(?:[\s\/-]|$)/i, 'Claude Code'],
  [/\bcodex(?:-cli|-tui|_rust)?(?:[\s\/-]|$)/i, 'Codex'],
  [/\bopencode(?:[\s\/-]|$)/i, 'OpenCode'],
  [/\bgemini-cli(?:[\s\/-]|$)/i, 'Gemini CLI'],
  [/\btrae(?:-ide)?(?:[\s\/-]|$)/i, 'Trae'],
  [/\bhermes(?:[\s\/-]|$)/i, 'Hermes']
]

export function identifyAgentClient(userAgent: string | null | undefined): string | null {
  if (!userAgent) return null
  return knownClients.find(([pattern]) => pattern.test(userAgent))?.[1] ?? null
}
