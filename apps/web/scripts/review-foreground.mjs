// Preview staged foreground bytes without installing them or invoking providers.
import { chromium } from '@playwright/test'
import { mkdirSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'

const root = resolve(
  process.env.SYNTELLE_FOREGROUND_OUTPUT ?? '../../.tools/foreground-11/prepared',
)
const browser = await chromium.launch({ headless: true })
try {
  for (const character of ['syntelle', 'cafe', 'woodland']) {
    const output = resolve(root, character, 'browser-review')
    mkdirSync(output, { recursive: true })
    const page = await browser.newPage({ viewport: { width: 1440, height: 1100 } })
    await page.route('**/src/experiments/motion/assets/**/foreground.webp*', (route) =>
      route.request().resourceType() === 'image'
        ? route.fulfill({
            path: resolve(root, character, 'foreground.webp'),
            contentType: 'image/webp',
          })
        : route.continue(),
    )
    await page.goto(`http://127.0.0.1:5173/?trial=motion&character=${character}`)
    await page
      .getByRole('button', { name: 'Play mouth', exact: true })
      .waitFor({ state: 'visible' })
    await page.getByRole('button', { name: 'Pause animation', exact: true }).click()
    for (const name of ['Light Studio', 'Dark Lounge', 'Natural Space']) {
      await page.getByRole('button', { name, exact: true }).click()
      for (const blur of [0, 9]) {
        await page.getByLabel('Background blur', { exact: true }).fill(String(blur))
        await page
          .getByTestId('motion-composition')
          .screenshot({ path: resolve(output, `${name}-${blur}.png`) })
      }
    }
    // Deterministic maximum-motion sample through the actual shared compositor.
    const frames = await page.evaluate(async (character) => {
      const { motionDefinition, motionAssetUrl } =
        await import('/src/experiments/motion/motionCatalog.ts')
      const { createCharacterRenderer } =
        await import('/src/features/character/rendering/characterRenderer.ts')
      const m = motionDefinition(character)
      const load = async (name) => {
        const image = new Image()
        image.src = motionAssetUrl(character, name)
        await image.decode()
        return image
      }
      const body = createCharacterRenderer(await load(m.foreground), await load(m.depth), m, () => {
        throw Error('Context lost')
      })
      const images = {}
      for (const ms of [0, 2100, 5800, 9100]) {
        body.draw(ms, true)
        images[ms] = body.canvas.toDataURL()
      }
      body.dispose()
      return images
    }, character)
    for (const [ms, data] of Object.entries(frames))
      writeFileSync(resolve(output, `motion-${ms}.png`), Buffer.from(data.split(',')[1], 'base64'))
    await page.close()
  }
} finally {
  await browser.close()
}
