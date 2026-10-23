import { createHash, randomBytes } from 'node:crypto'
import { mkdir, readFile, readdir, writeFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import { resolve, relative, join } from 'node:path'

const root = fileURLToPath(new URL('../../../', import.meta.url))
const sourceFolders = [
  'apps/web/src/experiments/viseme-a/characters/syntelle',
  'apps/web/src/experiments/motion/assets/syntelle',
  'apps/web/src/experiments/motion/rgbd/syntelle',
  'apps/web/src/experiments/motion/gaze/syntelle',
  'apps/web/src/experiments/motion/expressions/syntelle',
]
async function filesAt(folder) {
  const files = []
  for (const entry of await readdir(folder, { withFileTypes: true })) {
    const path = join(folder, entry.name)
    if (entry.isDirectory()) files.push(...await filesAt(path))
    else if (entry.isFile()) files.push(path)
    else throw Error('Prepared resources must be ordinary files.')
  }
  return files.sort()
}
const files = []
for (const source of sourceFolders) {
  for (const path of await filesAt(resolve(root, source))) {
    const bytes = await readFile(path)
    files.push({ bytes: bytes.length, path: relative(root, path).replaceAll('\\', '/'), sha256: createHash('sha256').update(bytes).digest('hex') })
  }
}
const packageHash = createHash('sha256').update(JSON.stringify(files)).digest('hex')
const evidence = resolve(root, '.tools/accounts-streaming-49')
await mkdir(evidence, { recursive: true })
await writeFile(join(evidence, 'package-manifest.json'), JSON.stringify({ version: 1, character: 'syntelle', package_hash: packageHash, files }, null, 2))
const envPath = resolve(root, '.env')
let environment = await readFile(envPath, 'utf8').catch(() => '')
environment = environment.replace(/^\uFEFF/, '')
const values = {
  SYNTELLE_ACCOUNTS: '1',
  SYNTELLE_STREAMING: '1',
  SYNTELLE_APP_ORIGIN: 'http://127.0.0.1:5173',
  SYNTELLE_AUTH_SECRET: randomBytes(48).toString('base64url'),
  SYNTELLE_RENDER_WORKER_TOKEN: randomBytes(48).toString('base64url'),
  SYNTELLE_PACKAGE_HASH: packageHash,
}
const configured = []
for (const [name, value] of Object.entries(values)) {
  const pattern = new RegExp(`^${name}=(.*)$`, 'm')
  const existing = environment.match(pattern)
  if (name !== 'SYNTELLE_PACKAGE_HASH' && existing?.[1]?.trim()) continue
  environment = existing ? environment.replace(pattern, `${name}=${value}`) : `${environment.trimEnd()}\n${name}=${value}\n`
  configured.push(name)
}
await writeFile(envPath, environment, { mode: 0o600 })
process.stdout.write(`${JSON.stringify({ package_hash: packageHash, package_files: files.length, configured_names: configured })}\n`)
