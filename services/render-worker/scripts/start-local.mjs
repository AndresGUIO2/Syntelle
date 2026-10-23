import { execFile, spawn } from 'node:child_process'
import { readFile, access } from 'node:fs/promises'
import { resolve } from 'node:path'
import { parseArgs, parseEnv, promisify } from 'node:util'
import { fileURLToPath, pathToFileURL } from 'node:url'

const root = fileURLToPath(new URL('../../../', import.meta.url))
const execute = promisify(execFile)
const tokenName = 'SYNTELLE_RENDER_WORKER_TOKEN'
const defaultImage = 'syntelle-render-native:local'

export function localAPI(value) {
  const endpoint = new URL(value)
  if (!['http:', 'https:'].includes(endpoint.protocol) || endpoint.username || endpoint.password ||
      !['127.0.0.1', 'localhost', '[::1]'].includes(endpoint.hostname) ||
      endpoint.search || endpoint.hash || !['', '/'].includes(endpoint.pathname))
    throw Error('Configure a loopback API origin without credentials, a path or query.')
  return endpoint.origin
}

export function matchingCapacity(requested, actual) {
  if (!Number.isInteger(actual) || actual < 1 || actual > 64)
    throw Error('The API did not report a valid render capacity.')
  if (requested !== undefined && (!/^\d+$/.test(String(requested)) || Number(requested) !== actual))
    throw Error(`Render capacity must match the API (${actual}); the launcher does not change admission.`)
  return actual
}

export function nativeAssetMount({ assets, assetsVolume }) {
  if (assets !== undefined && assetsVolume !== undefined)
    throw Error('Choose --assets or --assets-volume; they cannot be combined.')
  if (assetsVolume !== undefined) {
    if (!/^[a-zA-Z0-9][a-zA-Z0-9_.-]*$/.test(assetsVolume))
      throw Error('Select an existing Docker volume by name, without a path or mount options.')
    return `type=volume,source=${assetsVolume},target=/opt/syntelle-assets,readonly,volume-nocopy`
  }
  if (!assets || assets.includes(','))
    throw Error('Select an asset directory without commas for the Docker bind mount.')
  return `type=bind,source=${assets},target=/opt/syntelle-assets,readonly`
}

export function localMediaPort(value = '8081') {
  if (!/^\d+$/.test(String(value)) || (Number(value) !== 0 && (Number(value) < 1024 || Number(value) > 65535)))
    throw Error('Local media port must be zero or between 1024 and 65535.')
  return Number(value)
}

export function videoEncoder(value = 'auto') {
  if (!['auto', 'nvenc', 'openh264'].includes(value))
    throw Error('Video encoder must be auto, nvenc or openh264.')
  return value
}

export function gpuArguments(enabled) {
  return enabled ? ['--gpus', 'all', '--env', 'NVIDIA_DRIVER_CAPABILITIES=compute,video,utility'] : []
}

export function rendererMode(value = 'auto') {
  if (!['auto', 'gpu', 'cpu'].includes(value)) throw Error('Renderer must be auto, gpu or cpu.')
  return value
}

export function gpuPipeline(value = 'device') {
  if (!['device','host'].includes(value)) throw Error('GPU pipeline must be device or host.')
  return value
}

// Docker runtime access and encoder initialization are separate checks. A host
// NVIDIA card alone does not establish container access (for example Hyper-V).
export async function gpuAvailable(image, { run, remove }) {
  const name = `syntelle-native-gpu-probe-${process.pid}`
  try {
    await run(['run', '--rm', '--name', name, '--network', 'none', '--read-only',
      '--cap-drop', 'ALL', '--security-opt', 'no-new-privileges:true',
      ...gpuArguments(true), '--entrypoint', 'syntelle-render-worker', image, '--help'])
    return true
  } catch { return false }
  finally { await remove(name).catch(() => {}) }
}

export function nativeArguments({ image, container, assets, assetsVolume, api, capacity, name, mediaPort, gpu = false, encoder = 'auto', renderer = 'auto', pipeline = 'device' }) {
  return ['run', '--detach', '--rm', '--name', name,
    '--label', 'io.syntelle.project=syntelle', '--label', 'io.syntelle.role=render-worker',
    '--log-driver', 'json-file', '--log-opt', 'max-size=5m', '--log-opt', 'max-file=2',
    '--network', `container:${container}`, '--cpus', '4', '--memory', '1g', '--pids-limit', '256',
    '--read-only', '--tmpfs', '/tmp:rw,noexec,nosuid,size=16m',
    '--cap-drop', 'ALL', '--security-opt', 'no-new-privileges:true',
    '--env', tokenName, '--env', `SYNTELLE_VIDEO_ENCODER=${videoEncoder(encoder)}`,
    '--env', `SYNTELLE_RENDERER=${rendererMode(renderer)}`,
    '--env', `SYNTELLE_GPU_PIPELINE=${gpuPipeline(pipeline)}`,
    ...gpuArguments(gpu), '--mount', nativeAssetMount({ assets, assetsVolume }),
    image, '--assets', '/opt/syntelle-assets', '--api', api, '--capacity', String(capacity),
    ...(mediaPort === undefined ? [] : ['--local-media-port', String(localMediaPort(mediaPort))])]
}

async function docker(args, options = {}) {
  try {
    return (await execute('docker', args, { cwd: root, encoding: 'utf8', maxBuffer: 1 << 20, ...options })).stdout.trim()
  } catch (error) {
    // Docker stderr may contain private environment data; keep this CLI boundary sanitized.
    // eslint-disable-next-line preserve-caught-error
    throw Error(`Docker ${args[0]} failed (exit ${error.code ?? 'unavailable'}).`)
  }
}

async function followNative(id) {
  let stopping = false
  const stop = async () => {
    if (stopping) return
    stopping = true
    await docker(['stop', '--time', '3', id]).catch(() => {})
  }
  process.once('SIGINT', stop)
  process.once('SIGTERM', stop)
  try {
    const child = spawn('docker', ['logs', '--follow', id], { cwd: root, stdio: 'inherit' })
    await new Promise((accept, reject) => {
      child.once('error', reject)
      child.once('exit', (code) => code === 0 || stopping ? accept() : reject(Error('Native worker log transport stopped.')))
    })
  } finally {
    await stop()
    process.removeListener('SIGINT', stop)
    process.removeListener('SIGTERM', stop)
  }
}

export async function waitForDetachedWorker(id, { inspect, status, stop, pause, attempts = 40 }) {
  try {
    for (let attempt = 0; attempt < attempts; attempt++) {
      if (await inspect(id) !== 'true') throw Error('Native renderer exited during startup.')
      if ((await status())?.available) return
      await pause(500)
    }
    throw Error('Native renderer did not connect within 20 seconds.')
  } catch (error) {
    await stop(id).catch(() => {})
    throw error
  }
}

async function run() {
  const { values } = parseArgs({ options: {
    engine: { type: 'string', default: 'native' }, assets: { type: 'string' }, 'assets-volume': { type: 'string' }, image: { type: 'string' },
    api: { type: 'string' }, 'api-container': { type: 'string' }, capacity: { type: 'string' },
    'local-media-port': { type: 'string' },
    'video-encoder': { type: 'string' },
    renderer: { type: 'string' },
    'gpu-pipeline': { type: 'string' },
    'dry-run': { type: 'boolean', default: false }, detach: { type: 'boolean', default: false },
    help: { type: 'boolean', default: false },
  } })
  if (values.help) {
    console.log('Usage: node services/render-worker/scripts/start-local.mjs [--dry-run] [--detach]')
    console.log('Native options: --assets <directory> | --assets-volume <existing-volume>; --image <local-image>; --api-container <existing-api>')
    console.log('Options: --api <loopback-origin> --capacity <must-match-api>; --engine native is accepted for compatibility')
    console.log('Video: --video-encoder auto|nvenc|openh264 (default auto: NVENC with automatic OpenH264 fallback)')
    console.log('Rendering: --renderer auto|gpu|cpu (default auto: native CUDA with CPU fallback)')
    console.log('Comparison: --gpu-pipeline device|host (default device; host preserves the preceding CPU patches and transfers)')
    return
  }
  if (values.engine !== 'native')
    throw Error('Only the native Go render worker is supported.')
  if (values['assets-volume'] !== undefined) {
    nativeAssetMount({ assets: values.assets, assetsVolume: values['assets-volume'] })
  }
  let fileEnvironment = {}
  try { fileEnvironment = parseEnv((await readFile(resolve(root, '.env'), 'utf8')).replace(/^\uFEFF/, '')) }
  catch (error) {
    if (error.code !== 'ENOENT') {
      // Keep private configuration diagnostics out of the user-facing CLI error.
      // eslint-disable-next-line preserve-caught-error
      throw Error('The private local environment file could not be read.')
    }
  }
  const configured = (key) => process.env[key] ?? fileEnvironment[key]
  const encoder = videoEncoder(values['video-encoder'] ?? configured('SYNTELLE_VIDEO_ENCODER') ?? 'auto')
  const renderer = rendererMode(values.renderer ?? configured('SYNTELLE_RENDERER') ?? 'auto')
  const pipeline = gpuPipeline(values['gpu-pipeline'] ?? configured('SYNTELLE_GPU_PIPELINE') ?? 'device')
  const token = configured(tokenName)
  if (!token || token.length < 32) throw Error(`Configure ${tokenName} privately before starting a worker.`)
  const api = localAPI(values.api ?? configured('SYNTELLE_RENDER_API_URL') ?? 'http://127.0.0.1:8080')
  let status
  try {
    const response = await fetch(`${api}/api/v1/streams/status`, { signal: AbortSignal.timeout(5000) })
    if (!response.ok) throw Error('HTTP status')
    status = await response.json()
  } catch { throw Error('The local API streaming status is unavailable; start the existing API service first.') }
  const capacity = matchingCapacity(values.capacity ?? configured('SYNTELLE_RENDER_CAPACITY'), status.capacity)
  if (!values['dry-run'] && status.available)
    throw Error('A private renderer is already connected. Stop that worker explicitly before starting a replacement.')
  const selected = { ...process.env, [tokenName]: token, SYNTELLE_RENDER_API_URL: api, SYNTELLE_RENDER_CAPACITY: String(capacity) }
  const assetsVolume = values['assets-volume']
  let assets
  if (assetsVolume !== undefined) {
    const found = await docker(['volume', 'inspect', assetsVolume, '--format', '{{.Name}}']).catch(() => {
      throw Error('The selected asset volume must already exist and contain the verified native package.')
    })
    if (found !== assetsVolume) throw Error('Docker did not identify the selected existing asset volume.')
  } else {
    assets = resolve(root, values.assets ?? configured('SYNTELLE_RENDER_ASSETS') ?? '.tools/native-render-56/package')
    nativeAssetMount({ assets })
    await access(resolve(assets, 'manifest.json')).catch(() => {
      throw Error('Prepare native assets first; see services/render-worker/README.md. No assets are generated by the launcher.')
    })
  }
  const image = values.image ?? defaultImage
  if (!image || image.startsWith('-') || /\s/.test(image)) throw Error('Configure one local native image name.')
  await docker(['image', 'inspect', image, '--format', '{{.Id}}']).catch(() => {
    throw Error(`Build ${image} first with npm run build:native in services/render-worker.`)
  })
  const container = values['api-container'] ?? await docker(['compose', '-f', resolve(root, 'compose.yaml'), 'ps', '-q', 'api'])
  if (!/^[a-zA-Z0-9][a-zA-Z0-9_.-]*$/.test(container ?? '')) throw Error('Select exactly one running local API container.')
  const running = await docker(['inspect', container, '--format', '{{.State.Running}}'])
  if (running !== 'true') throw Error('The selected API container is not running.')
  const name = 'syntelle-render-native-local'
  const existing = await docker(['container', 'ls', '--all', '--filter', `name=^/${name}$`, '--format', '{{.ID}}'])
  if (existing && !values['dry-run']) throw Error(`Container ${name} already exists; stop or remove it explicitly before starting another worker.`)
  const gpu = !values['dry-run'] && (encoder !== 'openh264' || renderer !== 'cpu') && await gpuAvailable(image, {
    run: (args) => docker(args, { timeout: 10000 }),
    remove: (name) => docker(['rm', '--force', name], { timeout: 5000 }),
  })
  if (!values['dry-run'] && encoder === 'nvenc' && !gpu)
    throw Error('Required NVENC cannot start: Docker has no usable NVIDIA runtime access. Use auto for OpenH264 fallback.')
  if (!values['dry-run'] && renderer === 'gpu' && !gpu)
    throw Error('Required GPU rendering cannot start: Docker has no usable NVIDIA runtime access.')
  if (!values['dry-run'] && encoder === 'auto' && !gpu)
    console.log('Docker GPU access is unavailable; the worker will use OpenH264.')
  const args = nativeArguments({ image, container, assets, assetsVolume, api, capacity, name,
    mediaPort: localMediaPort(values['local-media-port']), gpu, encoder, renderer, pipeline })
  if (values['dry-run']) {
    console.log(JSON.stringify({ engine: 'native', api, capacity, already_connected: !!status.available,
      container_environment_names: [tokenName, 'SYNTELLE_VIDEO_ENCODER', 'SYNTELLE_RENDERER', 'SYNTELLE_GPU_PIPELINE'], video_encoder: encoder, renderer, gpu_pipeline: pipeline,
      gpu_detection: 'checked at launch', docker_arguments: args }, null, 2))
    return
  }
  const id = await docker(args, { env: selected })
  if (!/^[a-f0-9]{64}$/.test(id)) throw Error('Docker did not return the new worker identity.')
  if (values.detach) {
    await waitForDetachedWorker(id, {
      inspect: (worker) => docker(['inspect', worker, '--format', '{{.State.Running}}']),
      status: async () => {
        try {
          const response = await fetch(`${api}/api/v1/streams/status`, { signal: AbortSignal.timeout(1500) })
          return response.ok ? await response.json() : undefined
        } catch { return undefined }
      },
      stop: (worker) => docker(['stop', '--time', '3', worker]),
      pause: (ms) => new Promise((done) => setTimeout(done, ms)),
    })
    console.log(`Syntelle native renderer is ready (${id.slice(0, 12)}). It continues in Docker after this terminal closes.`)
    return
  }
  console.log(`Syntelle native renderer started (${id.slice(0, 12)}). Press Ctrl+C to stop it.`)
  await followNative(id)
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  run().catch((error) => { console.error(error.message); process.exitCode = 1 })
}
