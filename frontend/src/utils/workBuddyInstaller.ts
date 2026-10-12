import { catalogAudioModelPattern, catalogImageModelPattern, catalogVideoModelPattern } from './modelCatalog'

interface WorkBuddyInstallerOptions {
  apiKey: string
  baseUrl: string
  windows: boolean
}

/** Import primary text models only; capability flags belong to verified client configuration. */
export function buildWorkBuddyInstaller({ apiKey, baseUrl, windows }: WorkBuddyInstallerOptions): string {
  const root = baseUrl.replace(/\/+$/, '').replace(/\/v1\/?$/, '')
  const modelsEndpoint = `${root}/v1/models?purpose=agent`
  const chatEndpoint = `${root}/v1/chat/completions`
  const nonChatPattern = [catalogImageModelPattern, catalogVideoModelPattern, catalogAudioModelPattern].map(pattern => pattern.source).join('|')
  if (windows) {
    return `$path = Join-Path $env:USERPROFILE ".workbuddy\\models.json"
New-Item -ItemType Directory -Force -Path (Split-Path $path) | Out-Null
if (Test-Path $path) { Copy-Item $path "$path.starx-backup-$(Get-Date -Format 'yyyyMMdd-HHmmss')" }
$current = if (Test-Path $path) { Get-Content $path -Raw | ConvertFrom-Json } else { [PSCustomObject]@{ models = @() } }
$headers = @{ Authorization = "Bearer ${apiKey}" }
$nonChatPattern = '${nonChatPattern}'
function Test-StarXChatModel($item) {
  if ($item.id -isnot [string] -or -not $item.id.Trim()) { return $false }
  if ($item.model_type -and $item.model_type -notin @('chat', 'unknown')) { return $false }
  if ($item.supported_endpoints -and '/v1/chat/completions' -notin @($item.supported_endpoints)) { return $false }
  if ($item.model_type -ne 'chat' -and $item.id -match $nonChatPattern) { return $false }
  return $true
}
$remote = @()
$loaded = $false
try {
  $response = Invoke-RestMethod -Uri "${modelsEndpoint}" -Headers $headers -Method Get -TimeoutSec 20
  $remote = @($response.data)
  $loaded = $true
} catch {
  Write-Warning "Could not load Star-X primary model list; existing configuration is preserved. Retry before choosing a model."
}
$ids = @($remote | Where-Object { Test-StarXChatModel $_ } | ForEach-Object { $_.id } | Select-Object -Unique)
$models = @($current.models | Where-Object {
  -not $loaded -or $_.vendor -ne 'star-X' -or $_.id -in $ids
})
foreach ($id in $ids) {
  $existing = @($models | Where-Object { $_.id -eq $id }) | Select-Object -First 1
  if ($null -eq $existing) {
    $models += [PSCustomObject]@{
      id = $id
      name = "Star-X $id"
      vendor = "star-X"
      apiKey = "${apiKey}"
      url = "${chatEndpoint}"
    }
  } elseif ($existing.vendor -eq "star-X") {
    $existing | Add-Member -NotePropertyName name -NotePropertyValue "Star-X $id" -Force
    $existing | Add-Member -NotePropertyName apiKey -NotePropertyValue "${apiKey}" -Force
    $existing | Add-Member -NotePropertyName url -NotePropertyValue "${chatEndpoint}" -Force
  }
}
$removedIds = @($current.models | Where-Object { $_.id -notin @($models.id) } | ForEach-Object { $_.id })
$availableModels = @(@($current.availableModels) + @($ids) | Where-Object { $_ -notin $removedIds } | Select-Object -Unique)
$current | Add-Member -NotePropertyName models -NotePropertyValue $models -Force
$current | Add-Member -NotePropertyName availableModels -NotePropertyValue $availableModels -Force
$json = $current | ConvertTo-Json -Depth 100
[System.IO.File]::WriteAllText($path, $json, (New-Object System.Text.UTF8Encoding($false)))
if ($loaded -and $ids.Count -eq 0) { Write-Warning "No compatible text models are available for this key. Image/video models require separate tools or APIs." }
Write-Host "Star-X WorkBuddy configuration completed. Loaded $($ids.Count) text model(s). Please restart WorkBuddy and test tool calling." -ForegroundColor Green`
  }
  return `set -e
path="$HOME/.workbuddy/models.json"
mkdir -p "$(dirname "$path")"
[ ! -f "$path" ] || cp "$path" "$path.starx-backup-$(date +%Y%m%d-%H%M%S)"
python3 - "$path" <<'STARX_PY'
import json, re, sys, urllib.request
path = sys.argv[1]
api_key = ${JSON.stringify(apiKey)}
models_url = ${JSON.stringify(modelsEndpoint)}
chat_url = ${JSON.stringify(chatEndpoint)}
non_chat = re.compile(${JSON.stringify(nonChatPattern)}, re.I)
def is_chat(item):
    if not isinstance(item, dict) or not isinstance(item.get('id'), str) or not item['id'].strip(): return False
    kind = item.get('model_type')
    if kind and kind not in ('chat', 'unknown'): return False
    endpoints = item.get('supported_endpoints')
    if endpoints and '/v1/chat/completions' not in endpoints: return False
    return kind == 'chat' or not non_chat.search(item['id'])
try:
    with open(path, 'r', encoding='utf-8-sig') as handle: current = json.load(handle)
except FileNotFoundError: current = {}
remote, loaded = [], False
try:
    request = urllib.request.Request(models_url, headers={'Authorization': f'Bearer {api_key}'})
    with urllib.request.urlopen(request, timeout=20) as response:
        payload = json.load(response)
    remote, loaded = payload.get('data', []), True
except Exception:
    print('Warning: could not load Star-X primary model list; existing configuration is preserved. Retry before choosing a model.', file=sys.stderr)
ids = list(dict.fromkeys(item['id'] for item in remote if is_chat(item)))
original = current.get('models') or []
models = [item for item in original if isinstance(item, dict) and (not loaded or item.get('vendor') != 'star-X' or item.get('id') in ids)]
for model_id in ids:
    model = next((item for item in models if item.get('id') == model_id), None)
    if model is None:
        models.append({'id': model_id, 'name': f'Star-X {model_id}', 'vendor': 'star-X', 'apiKey': api_key, 'url': chat_url})
    elif model.get('vendor') == 'star-X':
        model.update({'name': f'Star-X {model_id}', 'apiKey': api_key, 'url': chat_url})
current['models'] = models
remaining_ids = {item.get('id') for item in models}
removed_ids = {item.get('id') for item in original if isinstance(item, dict) and item.get('id') not in remaining_ids}
available = current.get('availableModels') or []
current['availableModels'] = list(dict.fromkeys(model_id for model_id in available + ids if model_id not in removed_ids))
with open(path, 'w', encoding='utf-8') as handle:
    json.dump(current, handle, ensure_ascii=False, indent=2); handle.write('\\n')
if loaded and not ids:
    print('Warning: no compatible text models are available for this key. Image/video models require separate tools or APIs.', file=sys.stderr)
STARX_PY
printf '%s\\n' 'Star-X WorkBuddy configuration completed. Text model list refreshed. Please restart WorkBuddy and test tool calling.'`
}
