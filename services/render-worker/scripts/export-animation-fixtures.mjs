// Offline oracle: bundle and execute the real TS animation modules with a fixed clock.
import { createRequire } from 'node:module'
import { readFile, writeFile, mkdir } from 'node:fs/promises'
import { resolve, dirname } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { createHash } from 'node:crypto'
const root = resolve(dirname(fileURLToPath(import.meta.url)), '../../..')
const requireWeb = createRequire(resolve(root, 'apps/web/package.json'))
const { rolldown } = await import(
  pathToFileURL(requireWeb.resolve('rolldown')).href
)
const args = process.argv.slice(2)
const option = (name, fallback) => {
  const i = args.indexOf(name)
  return i < 0 ? fallback : args[i + 1]
}
const readJSON = async (name) =>
  JSON.parse(await readFile(resolve(root, name), 'utf8'))
const paths = {
  motion: 'apps/web/src/experiments/motion/rgbd/syntelle/package.json',
  gaze: 'apps/web/src/experiments/motion/gaze/syntelle/gaze.json',
  expressions:
    'apps/web/src/experiments/motion/expressions/syntelle/expressions.json',
  oral: 'apps/web/src/experiments/viseme-a/characters/syntelle/package.json',
}
const input = {}
for (const [key, path] of Object.entries(paths))
  input[key] = await readJSON(path)
input.prepared = await readJSON(
  option('--speech', '.tools/express-cpu-54/linux/cached-speech.json'),
)
const build = await rolldown({
  input: resolve(
    root,
    'services/render-worker/scripts/export-animation-fixtures.ts',
  ),
})
const { output } = await build.generate({ format: 'esm' })
const code = output.find((item) => item.type === 'chunk').code
await build.close()
const { exportFixtures } = await import(
  'data:text/javascript;base64,' + Buffer.from(code).toString('base64')
)
const result = exportFixtures(input)
result.reference_sources = {}
for (const path of [
  ...Object.values(paths),
  'apps/web/src/features/character/animation/attention.ts',
  'apps/web/src/features/character/animation/speakingAttention.ts',
  'apps/web/src/features/character/animation/breathing.ts',
  'apps/web/src/features/character/animation/expressions.ts',
  'apps/web/src/features/character/animation/speechBehavior.ts',
  'apps/web/src/features/character/animation/timing.ts',
  'apps/web/src/features/character/animation/performance.ts',
  'apps/web/src/features/character/speech/speechTimeline.ts',
])
  result.reference_sources[path] = createHash('sha256')
    .update(await readFile(resolve(root, path)))
    .digest('hex')
const destination = resolve(
  root,
  option(
    '--output',
    'services/render-worker/internal/nativeanimation/testdata/animation-fixtures.json',
  ),
)
await mkdir(dirname(destination), { recursive: true })
await writeFile(destination, JSON.stringify(result) + '\n')
process.stdout.write(
  JSON.stringify({
    output: destination,
    cases: result.cases.map((c) => ({ name: c.name, frames: c.rows.length })),
    paid_provider_calls: 0,
  }) + '\n',
)
