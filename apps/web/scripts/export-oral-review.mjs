// Offline browser rendering of staged material. Never invokes generation APIs.
import { chromium } from '@playwright/test'
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { extname, relative, resolve } from 'node:path'
import { createHash } from 'node:crypto'
const previous = process.argv.includes('--previous')
const oralIndex = process.argv.indexOf('--oral')
const oral = oralIndex >= 0 ? process.argv[oralIndex + 1] : previous ? 'previous' : 'completed'
const packageIndex = process.argv.indexOf('--package')
const packageDirectory = packageIndex >= 0 ? resolve(process.argv[packageIndex + 1]) : null
let external = null
if (packageDirectory) {
  const p = JSON.parse(readFileSync(resolve(packageDirectory, 'package.json'), 'utf8'))
  const names = new Set(['base.webp', p.reference.upper, p.reference.lower])
  for (const pose of Object.values(p.poses))
    for (const key of ['surface', 'mask', 'cavity']) if (pose[key]) names.add(pose[key])
  const images = {}
  for (const name of names) {
    const path = resolve(packageDirectory, name),
      local = relative(packageDirectory, path)
    if (local.startsWith('..') || !['.webp', '.png'].includes(extname(path)))
      throw Error('Invalid staged image path')
    images[name] =
      `data:image/${extname(path) === '.png' ? 'png' : 'webp'};base64,${readFileSync(path).toString('base64')}`
  }
  // Pass only the declared image assets. Do not expand Vite filesystem access
  // to the project root, where secret configuration also lives.
  external = { package: p, images }
}
const directory = process.argv.indexOf('--output')
const output = resolve(
  directory >= 0
    ? process.argv[directory + 1]
    : `../../.tools/oral-artifacts-05/browser-review${previous ? '-previous' : ''}`,
)
mkdirSync(output, { recursive: true })
const browser = await chromium.launch({ headless: true })
const page = await browser.newPage({ viewport: { width: 1440, height: 1100 } })
await page.goto(`http://localhost:5173/?trial=viseme-a&oral=${oral}`)
await page.getByRole('button', { name: 'Play', exact: true }).waitFor()
const report = await page.evaluate(
  async ({ previous, external }) => {
    const installed = external ? null : await import('/src/experiments/viseme-a/usePosePackage.ts')
    const p = external ? external.package : installed.posePackage
    const poseAssetUrl = external ? (name) => external.images[name] : installed.poseAssetUrl
    const { createCompletedMorph } = await import('/src/experiments/viseme-a/completedMorph.ts')
    const { createPoseMorph } = await import('/src/experiments/viseme-a/poseMorph.ts')
    const load = async (name) => {
      const image = new Image()
      image.src = poseAssetUrl(name)
      await image.decode()
      return image
    }
    const [upper, lower] = await Promise.all([load(p.reference.upper), load(p.reference.lower)])
    const poses = {}
    for (const [id, data] of Object.entries(p.poses))
      poses[id] = {
        data,
        surface: await load(data.surface),
        cavity: data.cavity ? await load(data.cavity) : undefined,
        mask: await load(data.mask),
      }
    const images = {},
      checks = [],
      endpointPixels = {},
      endpointDeltas = {},
      endpointRGBA = {}
    let maximumEndpointDelta = 0,
      maximumOutsideAlpha = 0
    const pixels = (image) => {
      const c = document.createElement('canvas')
      c.width = p.crop.width
      c.height = p.crop.height
      c.getContext('2d').drawImage(image, 0, 0)
      return c.getContext('2d').getImageData(0, 0, c.width, c.height).data
    }
    for (const [key, pair] of Object.entries(p.pairs)) {
      if (!pair.safe) {
        if (previous) continue
        throw Error(`Unsafe geometry: ${key}`)
      }
      const [a, b] = key.split('-'),
        from = poses[a],
        to = poses[b]
      const renderer = (previous ? createPoseMorph : createCompletedMorph)(
        from,
        to,
        pair.mesh,
        upper,
        lower,
        () => {
          throw Error('Context lost')
        },
        p.oral,
      )
      if (!renderer) throw Error('Renderer initialization failed')
      const maskFrom = pixels(from.mask),
        maskTo = pixels(to.mask)
      for (const t of [0, 0.01, 0.25, 0.35, 0.41, 0.5, 0.75, 0.99, 1]) {
        renderer.draw(t)
        const png = renderer.canvas.toDataURL('image/png')
        if (a === b && t === 0) images[`endpoint-${a}`] = png
        if (a !== b && [0.25, 0.35, 0.41, 0.5, 0.75].includes(t)) images[`${key}-${t}`] = png
        const data = renderer.canvas
          .getContext('2d')
          .getImageData(0, 0, p.crop.width, p.crop.height).data
        if (a === b && t === 0) endpointRGBA[a] = Array.from(data)
        for (let i = 0; i < data.length; i += 4)
          if (maskFrom[i] === 0 && maskTo[i] === 0)
            maximumOutsideAlpha = Math.max(maximumOutsideAlpha, data[i + 3])
        if (t === 0 || t === 1) {
          const id = t === 0 ? a : b
          if (endpointPixels[id])
            for (let i = 0; i < data.length; i++) {
              const delta = Math.abs(data[i] - endpointPixels[id][i])
              maximumEndpointDelta = Math.max(maximumEndpointDelta, delta)
              endpointDeltas[id] = Math.max(endpointDeltas[id] ?? 0, delta)
            }
          else endpointPixels[id] = new Uint8ClampedArray(data)
        }
        checks.push({ pair: key, t, nonempty: data.some((v) => v > 0) })
      }
      renderer.dispose()
    }
    return {
      images,
      checks,
      build_id: p.build_id,
      maximumEndpointDelta,
      maximumOutsideAlpha,
      endpointDeltas,
      endpointRGBA,
    }
  },
  { previous, external },
)
for (const [name, data] of Object.entries(report.images))
  writeFileSync(resolve(output, `${name}.png`), Buffer.from(data.split(',')[1], 'base64'))
delete report.images
report.raw_endpoint_sha256 = {}
for (const [pose, data] of Object.entries(report.endpointRGBA)) {
  const bytes = Buffer.from(data)
  writeFileSync(resolve(output, `endpoint-${pose}.rgba`), bytes)
  report.raw_endpoint_sha256[pose] = createHash('sha256').update(bytes).digest('hex')
}
delete report.endpointRGBA
writeFileSync(resolve(output, 'report.json'), JSON.stringify(report, null, 2))
if (!external) {
  await page.getByLabel('To pose', { exact: true }).selectOption('CHSHR')
  await page.getByRole('button', { name: 'CHSHR', exact: true }).last().click()
  await page.screenshot({ path: resolve(output, 'candidate-browser.png'), fullPage: true })
}
await browser.close()
console.log(`Exported ${report.checks.length} shared-compositor frames.`)
