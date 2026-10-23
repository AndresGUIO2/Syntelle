import { execFile } from 'node:child_process'
import { mkdir, open, readFile, unlink, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { promisify } from 'node:util'

const root = fileURLToPath(new URL('../../', import.meta.url))
const execute = promisify(execFile)
const workerName = 'syntelle-render-native-local'
const nativeImage = 'syntelle-render-native:local'
const services = ['postgres', 'processor', 'api', 'web']
const compose = ['compose', '-f', resolve(root, 'compose.yaml'), '--profile', 'preparation']
const pause = (ms) => new Promise((done) => setTimeout(done, ms))

async function command(program, args, timeout = 120000) {
  try {
    return (
      await execute(program, args, {
        cwd: root,
        encoding: 'utf8',
        windowsHide: true,
        timeout,
        maxBuffer: 8 << 20,
      })
    ).stdout.trim()
  } catch (error) {
    // Docker errors may include private configuration. Expose only the operation/status.
    throw Error(
      `${program === 'docker' ? `Docker ${args[0]}` : 'Native launcher'} failed (${error.code ?? 'unavailable'}).`,
    )
  }
}
const docker = (args, timeout) => command('docker', args, timeout)
async function optionalInspect(kind, name) {
  try {
    // Never emit or persist Config.Env, which contains service credentials.
    return JSON.parse(
      await docker([
        kind,
        'inspect',
        name,
        '--format',
        kind === 'image'
          ? '{"id":{{json .Id}},"tags":{{json .RepoTags}},"labels":{{json .Config.Labels}}}'
          : '{"id":{{json .Id}},"name":{{json .Name}},"image":{{json .Image}},"imageName":{{json .Config.Image}},"running":{{json .State.Running}},"network":{{json .HostConfig.NetworkMode}},"labels":{{json .Config.Labels}}}',
      ]),
    )
  } catch {
    return undefined
  }
}
async function streamStatus() {
  try {
    const response = await fetch('http://127.0.0.1:8080/api/v1/streams/status', {
      signal: AbortSignal.timeout(1500),
    })
    return response.ok ? await response.json() : undefined
  } catch {
    return undefined
  }
}
async function waitFor(check, description, attempts = 40) {
  for (let i = 0; i < attempts; i++) {
    if (await check()) return
    await pause(500)
  }
  throw Error(`${description} is not ready. Run .\u005cdev status for diagnostics.`)
}
async function ensureDocker(autoStart = true) {
  let engine
  try {
    engine = await docker(['info', '--format', '{{.OSType}}'], 5000)
  } catch {
    if (!autoStart)
      throw Error('Docker Desktop is not running. Use dev start to start the local stack.')
    console.log('Starting Docker Desktop…')
    await docker(['desktop', 'start', '--detach'], 15000)
    await waitFor(
      async () => {
        try {
          return (await docker(['info', '--format', '{{.OSType}}'], 2000)) === 'linux'
        } catch {
          return false
        }
      },
      'Docker Linux engine',
      60,
    )
    engine = 'linux'
  }
  if (engine !== 'linux') throw Error('Select Linux containers in Docker Desktop.')
}

export function ownsWorker(worker) {
  return (
    worker?.name === `/${workerName}` &&
    (worker.labels?.['io.syntelle.project'] === 'syntelle' ||
      worker.imageName?.startsWith('syntelle-render-native:'))
  )
}

// A connected worker is reused only when attached to the current API namespace.
export async function ensureWorker({ inspect, apiID, status, stop, launch, wait }) {
  const existing = await inspect()
  if (existing && !ownsWorker(existing))
    throw Error('The native worker name belongs to another resource; it was left unchanged.')
  if (existing?.running && existing.network === `container:${apiID}`) {
    if ((await status())?.available) return 'reused'
    // Allow an existing worker to reconnect before considering it unhealthy.
    try {
      await wait(async () => (await status())?.available, 'Existing renderer', 10)
      return 'reused'
    } catch {
      /* replace below */
    }
  }
  if (existing) await stop(existing)
  if ((await status())?.available)
    throw Error('Another renderer is connected. Stop it explicitly before using dev start.')
  try {
    await launch()
  } catch (error) {
    // The launcher may time out or lose its process after Docker has created the container.
    const failed = await inspect()
    if (failed && ownsWorker(failed)) await stop(failed).catch(() => {})
    throw error
  }
  return 'started'
}

async function stopWorker() {
  const worker = await optionalInspect('container', workerName)
  if (!worker) return
  if (!ownsWorker(worker))
    throw Error('The native worker name belongs to another resource; it was left unchanged.')
  if (worker.running) await docker(['stop', '--time', '3', workerName])
  if (await optionalInspect('container', workerName)) await docker(['rm', workerName])
}

const oldContainers = new Set([
  'syntelle-native-efficiency-build-57',
  'syntelle-native-media-build',
  'syntelle-speech-trial-12',
])
const oldRepositories = new Set([
  'syntelle-native-media-build',
  'syntelle-api-check',
  'syntelle-api-checks',
  'syntelle-processor-checks',
])
export function cleanupPlan(containers, images, knownOwned = new Set()) {
  const disposable = containers.filter(
    (item) =>
      !item.running &&
      (oldContainers.has(item.name.replace(/^\//, '')) ||
        (item.labels?.['io.syntelle.project'] === 'syntelle' &&
          ['test', 'build'].includes(item.labels?.['io.syntelle.role']))),
  )
  const removed = new Set(disposable.map((item) => item.id))
  const retainedImages = new Set(
    containers.filter((item) => !removed.has(item.id)).map((item) => item.image),
  )
  const candidates = images.flatMap((image) => {
    if (retainedImages.has(image.id)) return []
    const tags = image.tags ?? []
    const legacy = tags.filter(
      (tag) => oldRepositories.has(tag.split(':')[0]) || /^syntelle-api:speech-\d+$/.test(tag),
    )
    if (legacy.length) return legacy
    return !tags.length &&
      (image.labels?.['io.syntelle.project'] === 'syntelle' || knownOwned.has(image.id))
      ? [image.id]
      : []
  })
  return {
    containers: disposable.map((item) => item.name.replace(/^\//, '')),
    images: [...new Set(candidates)],
  }
}
async function inventory() {
  const containerIDs = (await docker(['ps', '-aq', '--no-trunc'])).split(/\s+/).filter(Boolean)
  const imageIDs = [
    ...new Set((await docker(['image', 'ls', '-aq', '--no-trunc'])).split(/\s+/).filter(Boolean)),
  ]
  const containers = (
    await Promise.all(containerIDs.map((id) => optionalInspect('container', id)))
  ).filter(Boolean)
  const images = (await Promise.all(imageIDs.map((id) => optionalInspect('image', id)))).filter(
    Boolean,
  )
  return { containers, images }
}
async function knownImages() {
  try {
    return new Set(
      JSON.parse(await readFile(resolve(root, '.tools/dev/owned-image-ids.json'), 'utf8')),
    )
  } catch (error) {
    if (error.code === 'ENOENT') return new Set()
    throw error
  }
}
async function rememberImages() {
  const ids = await knownImages()
  for (const name of [
    'syntelle-processor:latest',
    'syntelle-api:latest',
    'syntelle-web:latest',
    nativeImage,
  ]) {
    const image = await optionalInspect('image', name)
    if (image) ids.add(image.id)
  }
  await writeFile(
    resolve(root, '.tools/dev/owned-image-ids.json'),
    JSON.stringify([...ids], null, 2) + '\n',
  )
}
async function clean({ cache = false, dryRun = false, quiet = false } = {}) {
  const { containers, images } = await inventory()
  const plan = cleanupPlan(containers, images, await knownImages())
  if (!quiet || plan.containers.length || plan.images.length)
    console.log(
      JSON.stringify(
        {
          ...plan,
          volumes: 'preserved',
          shared_build_cache_limit: cache ? '2GB' : 'unchanged',
          dry_run: dryRun,
        },
        null,
        2,
      ),
    )
  if (dryRun) return plan
  if (quiet && !cache && !plan.containers.length && !plan.images.length) return plan
  for (const name of plan.containers) {
    const current = await optionalInspect('container', name)
    if (current?.running) throw Error(`Container ${name} became active; cleanup stopped.`)
    if (current) await docker(['rm', name]) // No force and no volume removal.
  }
  for (const image of plan.images) await docker(['image', 'rm', image]) // Docker rejects references added meanwhile.
  if (cache) {
    console.log(
      'Trimming unused shared Docker build cache to 2GB; future builds may need to download/recompile again.',
    )
    await docker(['buildx', 'prune', '--force', '--max-used-space', '2GB'], 300000)
  }
  const report = {
    at: new Date().toISOString(),
    ...plan,
    volumes_removed: 0,
    cache_limit: cache ? '2GB' : null,
  }
  await writeFile(
    resolve(root, '.tools/dev/last-cleanup.json'),
    JSON.stringify(report, null, 2) + '\n',
  )
  return plan
}

async function build(onlyMissing = false) {
  await rememberImages()
  const selected = []
  for (const service of ['processor', 'api', 'web']) {
    if (!onlyMissing || !(await optionalInspect('image', `syntelle-${service}:latest`)))
      selected.push(service)
  }
  const native = !onlyMissing || !(await optionalInspect('image', nativeImage))
  if (!selected.length && !native) return
  console.log(`Building ${[...selected, ...(native ? ['native renderer'] : [])].join(', ')}…`)
  try {
    if (selected.length) await docker([...compose, 'build', ...selected], 1800000)
    if (native)
      await docker(
        [
          'build',
          '--platform',
          'linux/amd64',
          '-f',
          'services/render-worker/native.Dockerfile',
          '-t',
          nativeImage,
          '.',
        ],
        1800000,
      )
  } finally {
    await rememberImages()
    await clean({ cache: true })
  }
}
async function start(rebuild = false) {
  await build(!rebuild)
  console.log('Starting PostgreSQL, processor, API and web…')
  await docker(
    [...compose, 'up', '-d', '--no-build', '--wait', '--wait-timeout', '90', ...services],
    120000,
  )
  await waitFor(async () => (await streamStatus())?.configured, 'Streaming API')
  const apiID = await docker([...compose, 'ps', '-q', 'api'])
  const result = await ensureWorker({
    inspect: () => optionalInspect('container', workerName),
    apiID,
    status: streamStatus,
    stop: stopWorker,
    wait: waitFor,
    launch: () =>
      command(
        process.execPath,
        ['services/render-worker/scripts/start-local.mjs', '--detach'],
        90000,
      ),
  })
  await waitFor(async () => {
    try {
      return (await fetch('http://127.0.0.1:5173/', { signal: AbortSignal.timeout(1500) })).ok
    } catch {
      return false
    }
  }, 'Web player')
  await clean({ quiet: true })
  console.log(
    `Syntelle is ready: http://127.0.0.1:5173/ (renderer ${result}). You can close this terminal.`,
  )
}

async function locked(operation) {
  const directory = resolve(root, '.tools/dev')
  await mkdir(directory, { recursive: true })
  const path = resolve(directory, 'operation.lock')
  let handle
  for (let attempt = 0; attempt < 2; attempt++) {
    try {
      handle = await open(path, 'wx')
      break
    } catch (error) {
      if (error.code !== 'EEXIST') throw error
      const owner = Number(await readFile(path, 'utf8').catch(() => ''))
      if (!owner) throw Error('Another dev command is starting; try again shortly.')
      try {
        process.kill(owner, 0)
      } catch (error) {
        if (error.code === 'ESRCH') {
          await unlink(path)
          continue
        }
        throw error
      }
      throw Error('Another dev command is running; wait for it to finish.')
    }
  }
  if (!handle) throw Error('Could not acquire the development command lock.')
  try {
    await handle.writeFile(String(process.pid))
    await operation()
  } finally {
    await handle.close()
    await unlink(path)
  }
}

async function main() {
  const [action = 'help', ...flags] = process.argv.slice(2)
  const accepted = {
    start: ['--build'],
    stop: [],
    status: [],
    build: [],
    clean: ['--cache', '--dry-run'],
    help: [],
  }
  if (!Object.hasOwn(accepted, action) || flags.some((flag) => !accepted[action].includes(flag)))
    throw Error('Use dev start [--build], stop, status, build, or clean [--cache] [--dry-run].')
  if (action === 'help') {
    console.log(
      'dev start         Start the complete local stack in the background; reuse existing images.\ndev start --build Rebuild, then start everything.\ndev stop          Stop Syntelle; preserve databases and images.\ndev status        Show services and renderer readiness.\ndev build         Rebuild stable image tags; trim unused cache to 2GB.\ndev clean         Remove recognized old Syntelle build/test resources; never volumes.\ndev clean --cache Also trim unused shared Docker build cache to 2GB.\nAdd --dry-run to clean to inspect its plan.',
    )
    return
  }
  await locked(async () => {
    await ensureDocker(action === 'start' || action === 'build')
    if (action === 'start') await start(flags.includes('--build'))
    else if (action === 'stop') {
      await stopWorker()
      await docker([...compose, 'stop', '--timeout', '5', ...services])
      console.log('Syntelle stopped. Databases, prepared assets and current images are preserved.')
    } else if (action === 'status') {
      console.log(await docker([...compose, 'ps', '--format', 'table']))
      const status = await streamStatus()
      console.log(
        JSON.stringify({
          renderer: status?.available ? 'ready' : 'unavailable',
          active_streams: status?.active ?? null,
        }),
      )
    } else if (action === 'build') await build()
    else await clean({ cache: flags.includes('--cache'), dryRun: flags.includes('--dry-run') })
  })
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href)
  main().catch((error) => {
    console.error(error.message)
    process.exitCode = 1
  })
