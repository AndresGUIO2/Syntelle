// Bounded local review requested by the owner; no provider calls or full regression matrix.
import { chromium, webkit } from '@playwright/test'
import { mkdirSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'

const engine = process.argv.includes('--webkit') ? 'webkit' : 'chromium'
const root = resolve(
  process.env.SYNTELLE_NECK_REVIEW_OUTPUT ?? '../../.tools/neck-motion-19/browser-review',
  engine,
)
mkdirSync(root, { recursive: true })
const browser = await { chromium, webkit }[engine].launch()
const results = []
try {
  for (const character of engine === 'webkit' ? ['syntelle'] : ['syntelle', 'cafe', 'woodland']) {
    const page = await browser.newPage({ viewport: { width: 1440, height: 1100 } })
    const errors = [],
      posts = []
    page.on('pageerror', (error) => errors.push(error.message))
    page.on('request', (r) => {
      if (r.method() === 'POST') posts.push(r.url())
    })
    await page.goto(`http://127.0.0.1:5173/?trial=motion&character=${character}&head=neck-compare`)
    await page.getByRole('button', { name: 'Play mouth', exact: true }).waitFor()
    await page.getByRole('button', { name: 'Pause animation', exact: true }).click()
    await page.getByRole('button', { name: '3° yaw', exact: true }).click()
    await page.getByRole('button', { name: 'Dark Lounge', exact: true }).click()
    if (await page.getByLabel('Magnify neck', { exact: true }).count())
      await page.getByLabel('Magnify neck', { exact: true }).check()
    await page.locator('main').evaluate((e) => {
      e.scrollTop = 0
    })
    await page.screenshot({ path: resolve(root, `${character}-comparison.png`) })
    await page.getByLabel('From pose', { exact: true }).selectOption('FV')
    await page.getByLabel('To pose', { exact: true }).selectOption('A')
    await page.getByLabel('Mouth blend', { exact: true }).fill('0.41')
    await page.getByLabel('Eyelid closure', { exact: true }).fill('1')
    await page.locator('main').evaluate((e) => {
      e.scrollTop = 0
    })
    await page.screenshot({ path: resolve(root, `${character}-expression.png`) })
    const report = await page.evaluate(async (character) => {
      const { resolveMotionContext } = await import('/src/experiments/motion/motionCatalog.ts')
      const { createPerspectiveRenderer } =
        await import('/src/features/character/rendering/perspectiveRenderer.ts')
      const context = resolveMotionContext(character, true, 'neck'),
        data = context.definition
      const fg = new Image()
      fg.src = context.assetUrl(data.foreground)
      await fg.decode()
      const geometry = await fetch(context.assetUrl(data.projection.geometry.file)).then((r) =>
        r.arrayBuffer(),
      )
      const old = createPerspectiveRenderer(fg, data, geometry, () => {}, 1024, 'previous')
      const neck = createPerspectiveRenderer(fg, data, geometry, () => {}, 1024, 'neck')
      const full = createPerspectiveRenderer(fg, data, geometry, () => {}, 1024, 'full')
      const canvas = document.createElement('canvas')
      canvas.width = full.canvas.width
      canvas.height = full.canvas.height
      const ctx = canvas.getContext('2d')
      const read = (renderer) => {
        ctx.clearRect(0, 0, canvas.width, canvas.height)
        ctx.drawImage(renderer.canvas, 0, 0)
        return ctx.getImageData(0, 0, canvas.width, canvas.height).data
      }
      const files = []
      const save = (renderer, name) => {
        files.push({ name, data: renderer.canvas.toDataURL() })
      }
      old.draw(0, false)
      full.draw(0, false)
      const previous = read(old),
        neutral = read(full)
      let neutralDelta = 0
      for (let i = 0; i < neutral.length; i++)
        neutralDelta = Math.max(neutralDelta, Math.abs(neutral[i] - previous[i]))
      full.draw(1800, true, { yaw: 0, pitch: 0, roll: 0 })
      const breathing = read(full)
      let faceBreathDelta = 0,
        bodyBreathChanges = 0
      const chin =
        ((data.anatomy.chin_y - data.anatomy.face_height * 0.05) * canvas.height) /
        data.canvas.height
      for (let i = 0; i < neutral.length; i += 4) {
        const delta = Math.max(
          ...[0, 1, 2, 3].map((k) => Math.abs(neutral[i + k] - breathing[i + k])),
        )
        if (Math.floor(i / 4 / canvas.width) < chin)
          faceBreathDelta = Math.max(faceBreathDelta, delta)
        else if (delta) bodyBreathChanges++
      }
      for (const yaw of [-3, 0, 3]) {
        for (const [name, renderer] of [
          ['previous', old],
          ['neck', neck],
          ['full', full],
        ]) {
          renderer.draw(1800, true, { yaw, pitch: 1, roll: 0 })
          save(renderer, `${name}-${yaw}.png`)
        }
      }
      const a = read(old),
        b = read(neck)
      let neckChanges = 0
      for (let i = 0; i < a.length; i += 4)
        if (
          Math.floor(i / 4 / canvas.width) > chin &&
          a[i + 3] &&
          b[i + 3] &&
          [0, 1, 2].some((k) => a[i + k] !== b[i + k])
        )
          neckChanges++
      const stats = full.stats
      old.dispose()
      neck.dispose()
      full.dispose()
      return {
        neutralDelta,
        faceBreathDelta,
        bodyBreathChanges,
        neckChanges,
        stats,
        files,
        build_id: data.build_id,
      }
    }, character)
    const folder = resolve(root, character)
    mkdirSync(folder, { recursive: true })
    for (const file of report.files)
      writeFileSync(resolve(folder, file.name), Buffer.from(file.data.split(',')[1], 'base64'))
    delete report.files
    const row = { character, ...report, errors, new_provider_requests: posts.length }
    results.push(row)
    if (
      row.neutralDelta > 1 ||
      row.faceBreathDelta !== 0 ||
      !row.bodyBreathChanges ||
      !row.neckChanges ||
      errors.length ||
      posts.length
    )
      throw Error(`Neck review failed: ${JSON.stringify(row)}`)
    await page.close()
  }
} finally {
  writeFileSync(resolve(root, 'report.json'), JSON.stringify(results, null, 2))
  await browser.close()
}
console.log(JSON.stringify(results, null, 2))
