import { expect, test } from '@playwright/test'
import { readFile, writeFile } from 'node:fs/promises'

test('manual expressions work on Syntelle and return exactly to neutral', async ({
  page,
}, info) => {
  const errors: string[] = []
  page.on('pageerror', (e) => errors.push(e.message))
  await page.route('**/api/v1/speech/status', (route) =>
    route.fulfill({ json: { configured: true } }),
  )
  await page.goto('/?expressions=preview')
  await page.getByRole('button', { name: 'Read aloud', exact: true }).click()
  const results = []
  for (const name of ['Syntelle']) {
    await expect(page.getByRole('region', { name: `${name} character preview` })).toHaveAttribute(
      'aria-busy',
      'false',
    )
    await page.getByRole('button', { name: 'Customize', exact: true }).click()
    await page.getByLabel('Blinking', { exact: true }).uncheck()
    await page.getByLabel('Natural movement', { exact: true }).uncheck()
    await page.getByLabel('Automatic expressions during speech', { exact: true }).uncheck()
    const frame = page.getByRole('img', { name: 'Animated character', exact: true })
    await page.waitForTimeout(80)
    const neutral = await frame.evaluate((c: HTMLCanvasElement) => c.toDataURL())
    const change = async (label: string) => {
      await page.getByLabel(label, { exact: true }).fill('1')
      await page.waitForTimeout(350)
      const image = await frame.evaluate((c: HTMLCanvasElement) => c.toDataURL())
      expect(image).not.toBe(neutral)
      await writeFile(
        info.outputPath(`${name}-${label.replaceAll(' ', '-')}.png`),
        Buffer.from(image.split(',')[1]!, 'base64'),
      )
      await page.getByRole('button', { name: 'Reset expression', exact: true }).click()
      await page.waitForTimeout(350)
      expect(await frame.evaluate((c: HTMLCanvasElement) => c.toDataURL())).toBe(neutral)
      return image.length
    }
    if (name !== 'Syntelle') await expect(page.getByLabel('Smile', { exact: true })).toHaveCount(0)
    results.push({
      name,
      smile: name === 'Syntelle' ? await change('Smile') : null,
      brows: await change('Raise eyebrows'),
    })
    await page.getByRole('button', { name: 'Close customization' }).click()
  }
  expect(errors).toEqual([])
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  await expect(
    page.getByLabel('Automatic expressions during speech', { exact: true }),
  ).toBeDisabled()
  await expect(page.getByLabel('Raise eyebrows', { exact: true })).toBeEnabled()
  await page.getByLabel('Raise eyebrows', { exact: true }).fill('0.5')
  await page.getByRole('button', { name: 'Close customization' }).click()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  await info.attach('manual-expressions', {
    body: JSON.stringify(results),
    contentType: 'application/json',
  })
  await page.screenshot({ path: info.outputPath('expressions-workspace.png'), fullPage: true })
})

test('speech expressions are deterministic, sparse, bounded and release on interruption', async ({
  page,
}) => {
  await page.goto('/?view=validation')
  const result = await page.evaluate(async () => {
    const { createExpressionDirector, planExpressions } = await import(
      '/src/features/character/animation/expressions.ts' as string
    )
    const plan = {
      version: 'speech-context/1',
      duration: 10,
      gestures: [],
      cues: [
        { time: 0, kind: 'onset', strength: 0.5 },
        { time: 1.8, kind: 'emphasis', strength: 0.9 },
        { time: 3.8, kind: 'pause', strength: 0.7 },
        { time: 6, kind: 'emphasis', strength: 0.8 },
        { time: 9.9, kind: 'ending', strength: 0.8 },
      ],
    }
    const director = createExpressionDirector(42, true),
      other = createExpressionDirector(42, true)
    let peakSmile = 0,
      peakBrows = 0,
      speed = 0
    let previous = { smile: 0, brows: 0 },
      continuity = 0
    for (let ms = 0; ms < 12000; ms += 20) {
      const speech = { active: ms < 5000, position: ms / 1000, session: 1, plan }
      const value = director.sample(ms, { smile: 0, brows: 0 }, true, false, speech)
      if (
        JSON.stringify(value) !==
        JSON.stringify(other.sample(ms, { smile: 0, brows: 0 }, true, false, speech))
      )
        throw Error('Nondeterministic expressions')
      peakSmile = Math.max(peakSmile, value.smile)
      peakBrows = Math.max(peakBrows, value.brows)
      speed = Math.max(
        speed,
        Math.abs(value.smile - previous.smile) / 0.02,
        Math.abs(value.brows - previous.brows) / 0.02,
      )
      if (ms === 5000)
        continuity = Math.max(
          Math.abs(value.smile - previous.smile),
          Math.abs(value.brows - previous.brows),
        )
      previous = value
    }
    const speech = { active: true, position: 1, session: 2, plan }
    const reduced = director.sample(12020, { smile: 0.8, brows: 0.4 }, true, true, speech)
    const complete = createExpressionDirector(42, true)
    complete.sample(0, { smile: 0, brows: 0 }, true, false, { ...speech, position: 9.9 })
    complete.sample(100, { smile: 0, brows: 0 }, true, false, {
      ...speech,
      active: false,
      position: 10,
    })
    const completedSmile = complete.sample(450, { smile: 0, brows: 0 }, true, false, {
      ...speech,
      active: false,
      position: 10,
    }).smile
    const unprepared = createExpressionDirector(42).sample(0, { smile: 1, brows: 0 }, false, true, {
      ...speech,
      active: false,
    }).smile
    return {
      peakSmile,
      peakBrows,
      speed,
      continuity,
      final: previous,
      reduced,
      events: planExpressions(plan, 42).length,
      completedSmile,
      unprepared,
    }
  })
  expect(result.peakSmile).toBe(0)
  expect(result.completedSmile).toBeGreaterThan(0.3)
  expect(result.unprepared).toBe(0)
  expect(result.peakBrows).toBeGreaterThan(0.15)
  expect(result.speed).toBeLessThan(3)
  expect(result.continuity).toBeLessThan(0.03)
  expect(result.final).toEqual({ smile: 0, brows: 0 })
  expect(result.reduced).toEqual({ smile: 0.8, brows: 0.4 })
  expect(result.events).toBeLessThanOrEqual(4)
})

test('cached Dragon playback activates expressions and keeps manual controls available', async ({
  page,
}, info) => {
  test.skip(info.project.name !== 'chromium-desktop', 'One cached recording replay is sufficient.')
  const cached = JSON.parse(
    await readFile(
      new URL('../../../../.tools/head-motion-12/speech/ava-en/response.json', import.meta.url),
      'utf8',
    ),
  )
  let intercepted = 0
  await page.route('**/api/v1/speech/status', (route) =>
    route.fulfill({ json: { configured: true } }),
  )
  await page.route('**/api/v1/speech/synthesize', (route) => {
    intercepted++
    return route.fulfill({ json: cached })
  })
  await page.goto('/?expressions=preview')
  await page.getByRole('button', { name: 'Read aloud', exact: true }).click()
  await expect(page.getByRole('region', { name: 'Syntelle character preview' })).toHaveAttribute(
    'aria-busy',
    'false',
  )
  await page
    .getByLabel('Text to speak', { exact: true })
    .fill('Offline replay of the archived Dragon recording.')
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  await page.getByLabel('Natural movement', { exact: true }).uncheck()
  await page.getByLabel('Blinking', { exact: true }).uncheck()
  await page.getByRole('button', { name: 'Close customization' }).click()
  await page.getByRole('button', { name: 'Speak', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Pause', exact: true })).toBeVisible()
  await page.waitForTimeout(1500)
  const active = await page
    .getByRole('img', { name: 'Animated character', exact: true })
    .evaluate((canvas: HTMLCanvasElement) => {
      const gl = canvas.getContext('webgl')!
      const program = gl.getParameter(gl.CURRENT_PROGRAM) as WebGLProgram
      const location = gl.getUniformLocation(program, 'expressionAmount')
      if (!location) throw Error('Expression shader is unavailable')
      return Array.from(gl.getUniform(program, location) as Float32Array)
    })
  expect(active[0]).toBe(0)
  expect(active[1]).toBeGreaterThan(0.05)
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  await expect(page.getByLabel('Smile', { exact: true })).toBeEnabled()
  await page.getByLabel('Smile', { exact: true }).fill('0.5')
  await page.getByRole('button', { name: 'Close customization' }).click()
  await page.getByRole('button', { name: 'Stop', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Pause', exact: true })).toHaveCount(0)
  expect(intercepted).toBe(1)
  await page.screenshot({ path: info.outputPath('cached-speech-expression.png'), fullPage: true })
})
