import { chromium, webkit } from '@playwright/test'
import { mkdirSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
const engine = process.argv.includes('--webkit') ? 'webkit' : 'chromium'
const natural = process.argv.includes('--natural')
const projection = process.argv.includes('--rgbd-candidate')
const matchedLegacy = process.argv.includes('--matched-legacy')
const browser = await { chromium, webkit }[engine].launch({ headless: true }),
  results = []
for (const mobile of [false, true])
  for (const character of ['syntelle', 'cafe', 'woodland']) {
    const context = await browser.newContext({
      viewport: mobile ? { width: 390, height: 844 } : { width: 1440, height: 1000 },
      isMobile: mobile,
      hasTouch: mobile,
    })
    const page = await context.newPage()
    await page.goto(
      `http://127.0.0.1:5173/?trial=motion&character=${character}${projection ? `&head=rgbd-candidate${matchedLegacy ? '&render=legacy' : ''}` : natural ? '&head=natural' : ''}`,
    )
    await page
      .getByRole('button', { name: 'Play mouth', exact: true })
      .waitFor({ state: 'visible' })
    await page.getByLabel('From pose', { exact: true }).selectOption('EI')
    await page.getByLabel('To pose', { exact: true }).selectOption('FV')
    await page.getByLabel('Background blur', { exact: true }).fill('20')
    await page.getByRole('button', { name: 'Play mouth', exact: true }).click()
    if (projection)
      await page.getByRole('button', { name: 'Replay comparison', exact: true }).click()
    const result = await page.evaluate(
      () =>
        new Promise((resolve) => {
          const frames = []
          let last = null
          const started = performance.now()
          function frame(now) {
            if (last !== null) frames.push(now - last)
            last = now
            if (now - started < 4500) {
              requestAnimationFrame(frame)
              return
            }
            const sorted = [...frames].sort((a, b) => a - b)
            resolve({
              frames: frames.length,
              elapsed_ms: now - started,
              average_fps: 1000 / (frames.reduce((a, b) => a + b, 0) / frames.length),
              median_frame_ms: sorted[Math.floor(sorted.length * 0.5)],
              p95_frame_ms: sorted[Math.floor(sorted.length * 0.95)],
            })
          }
          requestAnimationFrame(frame)
        }),
    )
    results.push({
      engine,
      renderer: matchedLegacy
        ? 'legacy-matched-turns'
        : projection
          ? 'rgbd-perspective/1'
          : natural
            ? 'legacy-natural'
            : 'legacy',
      character,
      viewport: mobile ? 'mobile emulation' : 'desktop',
      ...result,
      local_target_met: result.average_fps >= (mobile ? 29.5 : 59),
      note: 'Local desktop hardware; mobile viewport does not reproduce mobile GPU/CPU or a real iPhone.',
    })
    await context.close()
  }
await browser.close()
const directory = resolve(process.env.SYNTELLE_REVIEW_OUTPUT ?? '../../.tools/motion-09')
mkdirSync(directory, { recursive: true })
writeFileSync(resolve(directory, `performance-${engine}.json`), JSON.stringify(results, null, 2))
console.log(JSON.stringify(results, null, 2))
