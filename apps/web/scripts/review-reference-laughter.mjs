import { createServer } from 'node:http'
import { createRequire } from 'node:module'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
import assert from 'node:assert/strict'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../..')
const argumentsByName = new Map()
for (let index = 2; index < process.argv.length; index += 2) {
  const key = process.argv[index],
    value = process.argv[index + 1]
  if (!['--bundle', '--output'].includes(key) || !value)
    throw Error('Use --bundle <character-resource-bundle.json> --output <new-review-directory>')
  argumentsByName.set(key, value)
}
if (!argumentsByName.has('--bundle') || !argumentsByName.has('--output'))
  throw Error('Both --bundle and --output are required')
const bundlePath = path.resolve(root, argumentsByName.get('--bundle'))
const directory = path.resolve(root, argumentsByName.get('--output'))
// Every run is retained; a correction must use a new directory.
await mkdir(directory, { recursive: false })
const require = createRequire(path.join(root, 'apps/web/package.json'))
const ts = require('typescript'),
  { chromium, webkit } = require('playwright')
const responses = new Map()
for (const name of ['completedMorph', 'referenceLaughterPass', 'eyeMorph', 'gl']) {
  const source = await readFile(
    path.join(root, 'apps/web/src/features/character/rendering', name + '.ts'),
    'utf8',
  )
  responses.set('/' + name + '.js', {
    type: 'text/javascript',
    body: ts
      .transpileModule(source, {
        compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
      })
      .outputText.replaceAll("'./referenceLaughterPass'", "'./referenceLaughterPass.js'")
      .replaceAll("'./gl'", "'./gl.js'"),
  })
}
const bundle = JSON.parse(await readFile(bundlePath, 'utf8'))
assert.equal(bundle.version, 'character-resource-bundle/1')
const folder = (name) => {
  const value = path.resolve(root, bundle[name])
  return value.endsWith('.json') ? path.dirname(value) : value
}
const oral = JSON.parse(await readFile(path.join(folder('oral'), 'package.json'), 'utf8'))
const expression = JSON.parse(
  await readFile(path.join(folder('expressions'), 'expressions.json'), 'utf8'),
)
const motion = JSON.parse(await readFile(path.join(folder('motion'), 'package.json'), 'utf8'))
const gaze = JSON.parse(await readFile(path.join(folder('gaze'), 'gaze.json'), 'utf8'))
assert.equal(expression.source_sha256, bundle.source_sha256)
assert.equal(motion.original_sha256, bundle.source_sha256)
assert.equal(oral.source_sha256, bundle.source_sha256)
assert.equal(expression.reference_laughter?.version, 'reference-laughter/1')
assert.equal(expression.reference_laughter.validation.valid, true)
assert.equal(expression.reference_laughter.dental_ready, true)
assert.equal(expression.reference_laughter.motion_build_id, motion.build_id)
assert.equal(expression.reference_laughter.oral_build_id, oral.build_id)
const assets = {}
async function asset(key, location) {
  const url = '/assets/' + Object.keys(assets).length
  assets[key] = url
  responses.set(url, {
    type: location.endsWith('.webp') ? 'image/webp' : 'image/png',
    body: await readFile(location),
  })
}
await asset('upper', path.join(folder('oral'), oral.reference.upper))
await asset('lower', path.join(folder('oral'), oral.reference.lower))
for (const id of Object.keys(oral.poses))
  for (const key of ['surface', 'mask', 'cavity'])
    await asset(id + '-' + key, path.join(folder('oral'), oral.poses[id][key]))
for (const name of ['foreground', 'depth'])
  await asset(name, path.join(folder('motion_assets'), motion[name]))
for (const name of ['open', 'closed', 'mask'])
  await asset('eye-' + name, path.join(folder('motion_assets'), motion.eyes[name]))
for (const name of ['sclera.png', 'ownership.png'])
  await asset(name, path.join(folder('gaze'), name))
for (const [index, frame] of expression.reference_laughter.frames.entries())
  await asset('face' + index, path.join(folder('expressions'), frame.file))
await asset(
  'reference-cavity',
  path.join(folder('expressions'), expression.reference_laughter.oral.cavity.file),
)
for (const row of ['upper', 'lower']) {
  const observed = expression.reference_laughter.oral['observed_' + row]
  if (observed) await asset('reference-' + row, path.join(folder('expressions'), observed.file))
}
responses.set('/data.json', {
  type: 'application/json',
  body: JSON.stringify({ oral, expression, motion, gaze, assets }),
})
const server = createServer((request, response) => {
  const value = responses.get(request.url)
  response.setHeader('content-type', value?.type ?? 'text/html')
  response.end(
    value?.body ?? '<!doctype html><html><body style="margin:0;background:#29272d"></body></html>',
  )
})
await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve))
const origin = `http://127.0.0.1:${server.address().port}`
const results = []
try {
  for (const [name, browserType] of [
    ['chromium', chromium],
    ['webkit', webkit],
  ]) {
    const browser = await browserType.launch()
    try {
      const page = await browser.newPage({ viewport: { width: 720, height: 960 } })
      const errors = []
      page.on('pageerror', (error) => errors.push(error.message))
      await page.goto(origin)
      const report = await page.evaluate(async () => {
        const { createCompletedMorph } = await import('/completedMorph.js'),
          { createEyeMorph } = await import('/eyeMorph.js')
        const { oral, expression, motion, gaze, assets } = await (await fetch('/data.json')).json()
        const images = {}
        await Promise.all(
          Object.entries(assets).map(async ([name, url]) => {
            const image = new Image()
            image.src = url
            await image.decode()
            images[name] = image
          }),
        )
        const failures = [],
          unavailable = () => failures.push('context-lost')
        const pose = (id) => ({
          data: oral.poses[id],
          surface: images[id + '-surface'],
          mask: images[id + '-mask'],
          cavity: images[id + '-cavity'],
          endpoint: images[id + '-surface'],
          frame: document.createElement('canvas'),
        })
        const reference = expression.reference_laughter
        const mouth = createCompletedMorph(
          pose('Rest'),
          pose('A'),
          oral.pairs['Rest-A'].mesh,
          images.upper,
          images.lower,
          unavailable,
          oral.oral,
          {
            package: reference,
            images: {
              frames: reference.frames.map((_, index) => images['face' + index]),
              cavity: images['reference-cavity'],
              upper: images['reference-upper'],
              lower: images['reference-lower'],
            },
            oralCrop: oral.crop,
            chinY: motion.anatomy.chin_y,
          },
        )
        if (!mouth) throw Error('Actual reference initialization failed')
        const eyes = createEyeMorph(
          images['eye-open'],
          images['eye-closed'],
          images['eye-mask'],
          motion.eyes.curves,
          unavailable,
          {
            package: gaze,
            images: { sclera: images['sclera.png'], ownership: images['ownership.png'] },
          },
          reference,
          reference.frames.map((_, index) => images['face' + index]),
        )
        const full = document.createElement('canvas')
        full.width = motion.canvas.width
        full.height = motion.canvas.height
        const canvas = document.createElement('canvas')
        canvas.width = 720
        canvas.height = 960
        document.body.appendChild(canvas)
        const draw = (amount, energy, closure = 0) => {
          const ctx = full.getContext('2d')
          ctx.clearRect(0, 0, full.width, full.height)
          ctx.drawImage(images.foreground, 0, 0)
          mouth.drawReference(amount, energy)
          eyes.drawReference(amount, closure, [
            [0, 0],
            [0, 0],
          ])
          ctx.drawImage(mouth.gpuCanvas, reference.crop.x, reference.crop.y)
          ctx.drawImage(eyes.canvas, reference.eyes.crop.x, reference.eyes.crop.y)
          const display = canvas.getContext('2d')
          display.clearRect(0, 0, 720, 960)
          display.drawImage(full, 0, 0, 720, 960)
          return canvas.toDataURL()
        }
        const frames = []
        for (const level of [0, 0.25, 0.5, 0.75, 1]) frames.push({ level, image: draw(level, 0) })
        frames.push({ level: 'jaw', image: draw(1, 0.8) })
        frames.push({ level: 'blink', image: draw(1, 0.4, 0.6) })
        const errors = [
          mouth.gpuCanvas.getContext('webgl').getError(),
          eyes.canvas.getContext('webgl').getError(),
        ]
        mouth.draw(0)
        errors.push(mouth.gpuCanvas.getContext('webgl').getError())
        mouth.dispose()
        eyes.dispose()
        // Exercise the production speech shader separately from the reference
        // pass: narrow openings previously retained open-reference lighting.
        for (const id of ['A', 'EI', 'FV', 'MBP', 'O', 'SZ', 'U', 'CHSHR']) {
          const speech = createCompletedMorph(
            pose('Rest'),
            pose(id),
            oral.pairs['Rest-' + id].mesh,
            images.upper,
            images.lower,
            unavailable,
            oral.oral,
          )
          if (!speech) throw Error('Speech shader initialization failed: ' + id)
          for (const amount of [0.25, 0.6, 1]) {
            speech.draw(amount)
            frames.push({
              level: 'speech-' + id + '-' + amount,
              image: speech.gpuCanvas.toDataURL(),
            })
          }
          if (speech.gpuCanvas.getContext('webgl').getError() !== 0)
            throw Error('Speech shader error: ' + id)
          speech.dispose()
        }
        return { frames, failures, errors }
      })
      assert.deepEqual(errors, [])
      assert.deepEqual(report.failures, [])
      assert.deepEqual(report.errors, [0, 0, 0])
      for (const frame of report.frames)
        await writeFile(
          path.join(directory, `browser-${name}-${frame.level}.png`),
          Buffer.from(frame.image.split(',')[1], 'base64'),
        )
      results.push({
        browser: name,
        failures: report.failures,
        gl_errors: report.errors,
        captured_frames: report.frames.map(({ level }) => level),
        passed: true,
      })
    } finally {
      await browser.close()
    }
  }
  await writeFile(
    path.join(directory, 'browser-actual.json'),
    JSON.stringify(
      {
        results,
        source_sha256: bundle.source_sha256,
        reference_sha256: expression.reference_laughter.reference_sha256,
        private_harness: true,
        rgbd_live_playback: false,
        owner_acceptance: false,
        paid_provider_calls: 0,
      },
      null,
      2,
    ) + '\n',
  )
  console.log(JSON.stringify(results))
} finally {
  await new Promise((resolve) => server.close(resolve))
}
