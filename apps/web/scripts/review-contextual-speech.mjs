// Replays archived audio through the real UI. Every speech route is intercepted.
import { chromium } from '@playwright/test'
import { readFileSync, mkdirSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'

const vitality = process.argv.includes('--vitality-candidate')
const projection = process.argv.includes('--rgbd-candidate')
const root = resolve(
  process.env.SYNTELLE_REVIEW_OUTPUT ??
    `../../.tools/${projection ? 'rgbd-motion-18' : vitality ? 'vitality-motion-17' : 'contextual-motion-15'}`,
)
const browser = await chromium.launch({ headless: true, args: ['--mute-audio'] })
const results = []
for (const character of ['syntelle', 'cafe', 'woodland']) {
  for (const language of ['en', 'es']) {
    const source = resolve(
      `../../.tools/head-motion-12/speech/${character === 'syntelle' ? 'ava' : 'andrew'}-${language}`,
    )
    const request = JSON.parse(readFileSync(resolve(source, 'request.json'), 'utf8'))
    const response = JSON.parse(readFileSync(resolve(source, 'response.json'), 'utf8'))
    const directory = resolve(root, character, `speech-${language}`)
    mkdirSync(directory, { recursive: true })
    const page = await browser.newPage({ viewport: { width: 1440, height: 1100 } })
    let intercepted = 0
    const errors = []
    page.on('pageerror', (error) => errors.push(error.message))
    await page.route('**/api/v1/speech/**', async (route) => {
      if (route.request().url().endsWith('/status'))
        return route.fulfill({ json: { configured: true } })
      if (!route.request().url().endsWith('/synthesize')) return route.abort()
      const body = route.request().postDataJSON()
      if (
        body.text !== request.text ||
        body.voice !== request.voice ||
        body.language !== request.language
      ) {
        errors.push('Playback request differs from its archived audio.')
        return route.abort()
      }
      intercepted++
      return route.fulfill({ json: { ...response, cached: true } })
    })
    await page.addInitScript(() => {
      const prototype = WebGLRenderingContext.prototype,
        locations = new WeakMap()
      const location = prototype.getUniformLocation,
        uniform = prototype.uniform3f
      window.__headReview = []
      prototype.getUniformLocation = function (program, name) {
        const result = location.call(this, program, name)
        if (result) locations.set(result, name)
        return result
      }
      prototype.uniform3f = function (target, yaw, pitch, roll) {
        if (target && locations.get(target) === 'angles')
          window.__headReview.push({ time: performance.now(), yaw, pitch, roll })
        return uniform.call(this, target, yaw, pitch, roll)
      }
    })
    await page.goto(
      `http://127.0.0.1:5173/?trial=motion&character=${character}&head=${projection ? 'rgbd-candidate' : vitality ? 'vitality-candidate' : 'contextual-candidate'}`,
    )
    await page.getByLabel('Text to speak', { exact: true }).fill(request.text)
    await page.getByLabel('Speech language').selectOption(request.language)
    await page.getByLabel('Dragon HD voice').selectOption(request.voice)
    const plan = await page.evaluate(
      async ({ response, request, character, projection, vitality }) => {
        const { decodeSpeechWav } = await import('/src/features/character/speech/speechTimeline.ts')
        const { planSpeechBehavior } =
          await import('/src/features/character/animation/speechBehavior.ts')
        const { resolveMotionContext } = await import('/src/experiments/motion/motionCatalog.ts')
        const pcm = decodeSpeechWav(
          Uint8Array.from(atob(response.audio_base64), (c) => c.charCodeAt(0)),
        )
        const seed = [...(character + request.text)].reduce(
          (n, c) => (Math.imul(n, 31) + c.charCodeAt(0)) >>> 0,
          17,
        )
        return planSpeechBehavior(
          pcm.samples,
          pcm.sampleRate,
          seed,
          resolveMotionContext(
            character,
            true,
            projection ? 'rgbd' : vitality ? 'vitality' : 'contextual',
          ).definition.behavior,
        )
      },
      { response, request, character, projection, vitality },
    )
    writeFileSync(resolve(directory, 'plan.json'), JSON.stringify(plan, null, 2))
    writeFileSync(resolve(directory, 'source-request.json'), JSON.stringify(request, null, 2))
    await page.getByRole('button', { name: 'Speak', exact: true }).click()
    await page.getByRole('button', { name: 'Pause speech', exact: true }).waitFor()
    for (let i = 0; i < Math.ceil(response.duration_ms / 250); i++) {
      const frame = await page
        .getByRole('img', { name: 'Animated character', exact: true })
        .evaluate((canvas) => canvas.toDataURL())
      writeFileSync(
        resolve(directory, `frame-${String(i).padStart(3, '0')}.png`),
        Buffer.from(frame.split(',')[1], 'base64'),
      )
      await page.waitForTimeout(250)
    }
    await page.screenshot({ path: resolve(directory, 'browser.png'), fullPage: true })
    const trace = await page.evaluate(() => window.__headReview)
    writeFileSync(resolve(directory, 'head-uniforms.json'), JSON.stringify(trace))
    if (errors.length || intercepted !== 1)
      throw Error(JSON.stringify({ character, language, errors, intercepted }))
    results.push({
      character,
      language,
      gestures: plan.gestures.length,
      cues: plan.cues.length,
      intercepted_requests: intercepted,
      new_provider_requests: 0,
      errors,
    })
    await page.close()
  }
}
await browser.close()
writeFileSync(resolve(root, 'cached-speech-review.json'), JSON.stringify(results, null, 2))
console.log(JSON.stringify(results, null, 2))
