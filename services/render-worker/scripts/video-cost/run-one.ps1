param(
  [ValidateSet(960,720)][int]$Height = 720,
  [ValidateSet('animated','frozen','color-only')][string]$Mask = 'animated',
  [ValidateSet('fp64','fp32')][string]$Precision = 'fp64',
  [ValidateSet(1,2,4,8)][int]$Sessions = 1,
  [ValidateRange(3500,60000)][int]$SpeechMs = 5000,
  [ValidateRange(1,32)][int]$SpeechParts = 1,
  [string]$OutputDirectory = '',
  [string]$Image = 'syntelle-render-build:video-cost',
  [string]$AssetsVolume = ''
)
$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '../../../..')).Path
$name = 'syntelle-cost-probe-96'
$assets = Join-Path $repo '.tools/disgust-readability-93/package-v6'
$assetsMount = if ($AssetsVolume) { "${AssetsVolume}:/assets:ro" } else { "${assets}:/assets:ro" }
$probe = Join-Path $repo 'services/render-worker/cmd/render-probe'
$speech = Join-Path $repo '.tools/gesture-tempo-95/inputs/cached-speech.json'
$suffix = if ($SpeechParts -gt 1) { "-speech$SpeechParts" } else { '' }
if (-not $OutputDirectory) { $OutputDirectory = Join-Path $repo '.tools/video-cost-96' }
New-Item -ItemType Directory -Force $OutputDirectory | Out-Null
$report = Join-Path $OutputDirectory "$Height-$Mask-$Precision-$Sessions$suffix.json"
$health = 'http://127.0.0.1:3000/health'
try {
  $existing = docker ps -q --filter "name=^/$name$"
  if ($existing) { throw "Diagnostic container $name is already running." }
  docker run --detach --rm --name $name --gpus all --cpus 4 --memory 2g `
    -p 127.0.0.1:3000:3000/tcp -p 127.0.0.1:3001:3001/udp `
    -e NVIDIA_DRIVER_CAPABILITIES=compute,video,utility `
    -e SYNTELLE_RENDERER=gpu -e SYNTELLE_VIDEO_ENCODER=nvenc `
    -e LD_LIBRARY_PATH=/opt/syntelle-nvenc/lib `
    -v $assetsMount `
    -v "${probe}:/src/services/render-worker/cmd/render-probe:ro" `
    -w /src/services/render-worker --entrypoint go $Image `
    run -tags nativecodecs,libyuv,nvenc,nativegpu ./cmd/render-probe `
    --assets /assets --foreground --sessions $Sessions --video-height $Height `
    --mask-mode $Mask --precision $Precision `
    --loopback-docker --local-media-port 3001 --listen 0.0.0.0:3000 | Out-Null
  $ready = $false
  for ($i=0; $i -lt 1200; $i++) {
    try { $ready = (Invoke-RestMethod -Uri $health -TimeoutSec 2).ready; if ($ready) { break } }
    catch { Start-Sleep -Milliseconds 500 }
  }
  if (-not $ready) { throw 'Offline renderer did not become ready.' }
  node (Join-Path $repo 'services/render-worker/scripts/review-cpu.mjs') `
    --host http://127.0.0.1:3000 --sessions $Sessions `
    --speech-file $speech --speech-ms $SpeechMs --speech-parts $SpeechParts --run-id cost2026 `
    --output $report
  if ($LASTEXITCODE -ne 0) { throw "Receiver exited with code $LASTEXITCODE; inspect $report." }
} finally {
  $running = docker ps -q --filter "name=^/$name$"
  if ($running) { docker stop --timeout 3 $name | Out-Null }
}
