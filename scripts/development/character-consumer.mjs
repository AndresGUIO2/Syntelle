import { execFile, spawn } from 'node:child_process'
import { access, mkdir, readFile, unlink, writeFile } from 'node:fs/promises'
import { createHash } from 'node:crypto'
import { resolve } from 'node:path'
import { promisify, parseEnv } from 'node:util'

const execute = promisify(execFile)
const moduleName = 'app.workflows.characters.worker'
const pause = (ms) => new Promise((done) => setTimeout(done, ms))

export function consumerConfiguration(root, values) {
  const keys = ['SYNTELLE_CHARACTER_PACKAGES', 'SYNTELLE_CHARACTER_SCRATCH', 'SYNTELLE_CHARACTER_CHECKER']
  if (keys.every((key) => !values[key]) && !values.SYNTELLE_CHARACTER_APPROVAL) return null
  if (keys.some((key) => !values[key])) throw Error('Configure character packages, scratch and checker together before starting creation.')
  const [packages, scratch, checker] = keys.map((key) => resolve(root, values[key]))
  const approval = values.SYNTELLE_CHARACTER_APPROVAL ? resolve(root, values.SYNTELLE_CHARACTER_APPROVAL) : undefined
  const python = resolve(root, 'services/processor/.venv/Scripts/python.exe')
  const args = ['-u', '-m', moduleName, '--packages', packages, '--scratch', scratch, '--checker', checker,
    ...(approval ? ['--approval', approval] : [])]
  return { python, args, packages, scratch, checker, approval,
    hash: createHash('sha256').update(JSON.stringify({ python, args })).digest('hex') }
}

export function ownsConsumer(record, actual) {
  return record?.version === 'character-consumer/1' && Number.isSafeInteger(record.pid) && record.pid > 0 &&
    actual?.pid === record.pid && actual.birth === record.birth && actual.commandHash === record.commandHash
}

// Dependency seams exercise duplicate/PID reuse behavior without starting jobs.
export async function ensureConsumer(config, state, io) {
  const record = await io.read()
  if (record) {
    const actual = await io.inspect(record.pid)
    if (actual) {
      if (!ownsConsumer(record, actual)) throw Error('Character consumer ownership changed; the process was left untouched.')
      if (config && record.configurationHash === config.hash) return 'reused'
      await io.stop(record)
    }
    await io.remove()
  }
  if (!config) return 'disabled'
  if ((await io.find()).length) throw Error('An untracked character consumer is running; no duplicate was started.')
  const identity = await io.launch(config)
  await io.write({ version: 'character-consumer/1', ...identity, configurationHash: config.hash, state })
  return 'started'
}

async function settings(root) {
  let file = {}
  try { file = parseEnv((await readFile(resolve(root, '.env'), 'utf8')).replace(/^\uFEFF/, '')) }
  catch (error) { if (error.code !== 'ENOENT') throw Error('Private development configuration could not be read.') }
  return { ...file, ...process.env }
}

function processControl(root, python) {
  return async (action, record = {}) => {
    if (process.platform !== 'win32') throw Error('The configured local character consumer requires the Windows development host.')
    const args = ['-NoProfile', '-NonInteractive', '-File', resolve(root, 'scripts/development/character-process.ps1'),
      '-Action', action, '-Executable', python]
    if (record.pid) args.push('-ProcessId', String(record.pid))
    if (action === 'stop') args.push('-Birth', record.birth, '-CommandHash', record.commandHash)
    try {
      const result = await execute('pwsh.exe', args, { windowsHide: true, timeout: 10000, maxBuffer: 16384 })
      return JSON.parse(result.stdout)
    } catch { throw Error('Character consumer process ownership could not be verified.') }
  }
}

export async function characterConsumer(root, action) {
  const directory = resolve(root, '.tools/dev')
  const state = resolve(directory, 'character-consumer.json')
  const values = await settings(root)
  const config = action === 'stop' ? null : consumerConfiguration(root, values)
  const python = resolve(root, 'services/processor/.venv/Scripts/python.exe')
  const control = processControl(root, python)
  const io = {
    read: async () => {
      try { return JSON.parse(await readFile(state, 'utf8')) }
      catch (error) { if (error.code === 'ENOENT') return null; throw Error('Character consumer state is invalid; no process was changed.') }
    },
    inspect: (pid) => control('inspect', { pid }),
    stop: (record) => control('stop', record),
    find: () => control('find'),
    remove: async () => { await unlink(state).catch((error) => { if (error.code !== 'ENOENT') throw error }) },
    write: async (record) => { await mkdir(directory, { recursive: true }); await writeFile(state, JSON.stringify(record), { mode: 0o600 }) },
    launch: async (selected) => {
      for (const path of [selected.python, selected.checker, ...(selected.approval ? [selected.approval] : [])]) {
        await access(path).catch(() => { throw Error('Prepare the configured character checker, Python environment and optional approval file before starting creation.') })
      }
      await mkdir(selected.packages, { recursive: true })
      await mkdir(selected.scratch, { recursive: true })
      // Credentials remain in the child's environment/private .env. Never put
      // them in arguments, process metadata, logs or a browser response.
      const child = spawn(selected.python, selected.args, { cwd: resolve(root, 'services/processor'),
        env: { ...values, PYTHONUNBUFFERED: '1' }, detached: true, windowsHide: true, stdio: 'ignore' })
      await new Promise((accept, reject) => { child.once('spawn', accept); child.once('error', () => reject(Error('Character consumer could not start.'))) })
      child.unref()
      const identity = await io.inspect(child.pid)
      if (!identity || identity.foreign) throw Error('Character consumer exited before its identity was verified.')
      await io.write({ version: 'character-consumer/1', ...identity, configurationHash: selected.hash, state })
      await pause(750)
      if (!ownsConsumer({ version: 'character-consumer/1', ...identity }, await io.inspect(identity.pid))) throw Error('Character consumer did not remain running; its jobs were not reported ready.')
      return identity
    },
  }
  if (action === 'status') {
    const record = await io.read()
    if (!record) return config ? 'stopped' : 'disabled'
    const actual = await io.inspect(record.pid)
    return !actual ? 'stopped' : ownsConsumer(record, actual) ? 'running' : 'ownership-changed'
  }
  return ensureConsumer(action === 'stop' ? null : config, state, io)
}
