import { spawnSync } from 'node:child_process'
import { describe, expect, it } from 'vitest'
import { buildWorkBuddyInstaller } from '../workBuddyInstaller'

const options = { apiKey: 'sk-unit-test', baseUrl: 'https://example.com/v1', windows: false }

// Execute the generated Python with in-memory files and a mocked HTTP response.
// This checks the configuration users run, without touching a profile or making a request.
function runInstaller(data: unknown[], initial: Record<string, unknown> = {}, failure = false) {
  const command = buildWorkBuddyInstaller(options)
  const script = command.split("<<'STARX_PY'\n")[1].split('\nSTARX_PY')[0]
  const harness = `import builtins, io, json, sys, urllib.request
initial = ${JSON.stringify(JSON.stringify(initial))}
payload = ${JSON.stringify(JSON.stringify({ data }))}
output = None
class Output(io.StringIO):
    def __exit__(self, *args):
        global output
        output = self.getvalue()
def fake_open(path, mode, **kwargs):
    assert path == 'memory-config'
    return Output() if mode == 'w' else io.StringIO(initial)
def fake_urlopen(request, **kwargs):
    assert request.full_url == 'https://example.com/v1/models?purpose=agent'
    ${failure ? "raise OSError('offline')" : 'return io.StringIO(payload)'}
builtins.open = fake_open
urllib.request.urlopen = fake_urlopen
sys.argv = ['installer', 'memory-config']
exec(${JSON.stringify(script)})
print(output)
`
  const result = spawnSync(process.platform === 'win32' ? 'python' : 'python3', ['-c', harness], { encoding: 'utf8' })
  expect(result.error).toBeUndefined()
  expect(result.status, result.stderr).toBe(0)
  return { config: JSON.parse(result.stdout), warnings: result.stderr }
}

describe('WorkBuddy primary model import', () => {
  it('filters media using metadata and legacy model families without inventing capabilities', () => {
    const { config } = runInstaller([
      { id: 'gpt-5.5', model_type: 'chat', supported_endpoints: ['/v1/chat/completions'] },
      { id: 'MiniMax-M2.7' },
      { id: 'video-understanding-vl', model_type: 'unknown' },
      { id: 'custom-picture', model_type: 'image' },
      { id: 'custom-movie', model_type: 'video' },
      { id: 'MiniMax-H3' }, { id: 'MiniMax-H3-Max' },
      { id: 'doubao-seedream-5-0-pro-260628' }, { id: 'gpt-image-1' },
      { id: 'sora-2' }, { id: 'kling-v2' }, { id: 'veo-3' },
      { id: 'claude-native', model_type: 'chat', supported_endpoints: ['/v1/messages'] },
      { id: 'voice-model', model_type: 'audio' },
      { id: 'embed-model', model_type: 'embedding' }
    ], {
      models: [
        { id: 'gpt-5.5', vendor: 'star-X', supportsToolCall: true, supportsImages: true, maxInputTokens: 1000 },
        { id: 'MiniMax-H3', vendor: 'star-X' },
        { id: 'sora-2', vendor: 'another-provider', customOption: 'keep' }
      ],
      availableModels: ['gpt-5.5', 'MiniMax-H3', 'sora-2'], theme: 'existing'
    })
    expect(config.models.map((model: { id: string }) => model.id)).toEqual(['gpt-5.5', 'sora-2', 'MiniMax-M2.7', 'video-understanding-vl'])
    expect(config.models[0]).toMatchObject({ supportsToolCall: true, supportsImages: true, maxInputTokens: 1000, apiKey: 'sk-unit-test' })
    expect(config.models[1]).toEqual({ id: 'sora-2', vendor: 'another-provider', customOption: 'keep' })
    expect(config.models[2]).toEqual({ id: 'MiniMax-M2.7', name: 'Star-X MiniMax-M2.7', vendor: 'star-X', apiKey: 'sk-unit-test', url: 'https://example.com/v1/chat/completions' })
    expect(config.availableModels).not.toContain('MiniMax-H3')
    expect(config.theme).toBe('existing')
  })

  it('trusts explicit chat metadata over a model-family-looking alias', () => {
    const { config } = runInstaller([{ id: 'kling-reasoner', model_type: 'chat', supported_endpoints: ['/v1/chat/completions'] }], {
      models: [{ id: 'kling-reasoner', vendor: 'star-X', supportsToolCall: true }], availableModels: ['kling-reasoner']
    })
    expect(config.models[0]).toMatchObject({ id: 'kling-reasoner', supportsToolCall: true })
    expect(config.availableModels).toEqual(['kling-reasoner'])
  })

  it('does not add an assumed fallback when the key only permits media', () => {
    const { config, warnings } = runInstaller([{ id: 'MiniMax-H3', model_type: 'video' }])
    expect(config.models).toEqual([])
    expect(config.availableModels).toEqual([])
    expect(warnings).toContain('no compatible text models')
  })

  it('removes an old custom media alias that is absent from the filtered discovery response', () => {
    const { config } = runInstaller([{ id: 'custom-chat', model_type: 'chat' }], {
      models: [{ id: 'image-alias', vendor: 'star-X', supportsToolCall: true }, { id: 'other-alias', vendor: 'another-provider' }],
      availableModels: ['image-alias', 'other-alias']
    })
    expect(config.models.map((model: { id: string }) => model.id)).toEqual(['other-alias', 'custom-chat'])
    expect(config.availableModels).toEqual(['other-alias', 'custom-chat'])
  })

  it('preserves existing configuration when discovery fails and reports no assumed availability', () => {
    const initial = { models: [{ id: 'custom-text', vendor: 'star-X', supportsToolCall: false }], availableModels: ['custom-text'] }
    const { config, warnings } = runInstaller([], initial, true)
    expect(config).toEqual(initial)
    expect(warnings).toContain('existing configuration is preserved')
  })

  it('keeps repeated IDs owned by other providers', () => {
    const initial = { models: [{ id: 'custom-text', vendor: 'first' }, { id: 'custom-text', vendor: 'second' }], availableModels: ['custom-text'] }
    expect(runInstaller([{ id: 'custom-text' }], initial).config).toEqual(initial)
  })

  it('renders equivalent Windows filtering and avoids capability guesses', () => {
    const script = buildWorkBuddyInstaller({ ...options, windows: true })
    expect(script).toContain('/models?purpose=agent')
    expect(script).toContain("$item.model_type -notin @('chat', 'unknown')")
    expect(script).toContain("'/v1/chat/completions' -notin @($item.supported_endpoints)")
    expect(script).toContain('No compatible text models')
    expect(script).not.toContain('supportsToolCall = $true')
    expect(script).not.toContain('maxInputTokens = 400000')
    expect(script).not.toContain('$fallback')
  })

  it.runIf(process.platform === 'win32')('executes the Windows import without touching a real profile', () => {
    const initial = { models: [{ id: 'old-alias', vendor: 'star-X' }, { id: 'custom-text', vendor: 'star-X', supportsToolCall: false }, { id: 'other-model', vendor: 'other' }], availableModels: ['old-alias', 'custom-text', 'other-model'] }
    const remote = { data: [{ id: 'custom-text', model_type: 'chat', supported_endpoints: ['/v1/chat/completions'] }, { id: 'MiniMax-H3', model_type: 'video' }, { id: 'new-text', model_type: 'unknown', supported_endpoints: [] }] }
    const script = buildWorkBuddyInstaller({ ...options, windows: true })
      .replace('[System.IO.File]::WriteAllText($path, $json, (New-Object System.Text.UTF8Encoding($false)))', '$global:writtenJson = $json')
    const harness = `$initialJson = @'
${JSON.stringify(initial)}
'@
$remoteJson = @'
${JSON.stringify(remote)}
'@
function New-Item { param($ItemType, [switch]$Force, $Path) }
function Test-Path { param($Path) return $true }
function Copy-Item { param($Path, $Destination) }
function Get-Content { param($Path, [switch]$Raw) return $initialJson }
function Invoke-RestMethod {
  param($Uri, $Headers, $Method, $TimeoutSec)
  if ($Uri -ne 'https://example.com/v1/models?purpose=agent') { throw 'wrong discovery URL' }
  return ($remoteJson | ConvertFrom-Json)
}
${script}
Write-Output '__RESULT__'
Write-Output $global:writtenJson
`
    const result = spawnSync('powershell.exe', ['-NoProfile', '-NonInteractive', '-EncodedCommand', Buffer.from(harness, 'utf16le').toString('base64')], { encoding: 'utf8' })
    expect(result.error).toBeUndefined()
    expect(result.status, result.stderr).toBe(0)
    const config = JSON.parse(result.stdout.split('__RESULT__')[1])
    expect(config.models.map((model: { id: string }) => model.id)).toEqual(['custom-text', 'other-model', 'new-text'])
    expect(config.models[0]).toMatchObject({ supportsToolCall: false, apiKey: 'sk-unit-test' })
    expect(config.models[1]).toEqual(initial.models[2])
    expect(config.models[2]).not.toHaveProperty('supportsToolCall')
    expect(config.availableModels).toEqual(['custom-text', 'other-model', 'new-text'])
  })
})
