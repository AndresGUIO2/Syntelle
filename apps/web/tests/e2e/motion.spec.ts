import { expect, test } from '@playwright/test'

for (const character of ['syntelle', 'cafe', 'woodland']) {
  test(`${character}: backgrounds and blur leave character pixels unchanged`, async ({
    page,
  }, info) => {
    const errors: string[] = [],
      posts: string[] = []
    page.on('pageerror', (e) => errors.push(e.message))
    page.on('request', (r) => {
      if (r.method() === 'POST') posts.push(r.url())
    })
    await page.goto(`/?trial=motion&character=${character}`)
    const frame = page.getByRole('img', { name: 'Animated character', exact: true })
    await expect(page.getByRole('button', { name: 'Play mouth', exact: true })).toBeEnabled()
    await page.getByRole('button', { name: 'Pause animation', exact: true }).click()
    const before = await frame.evaluate((canvas: HTMLCanvasElement) => canvas.toDataURL())
    for (const name of ['Dark Lounge', 'Natural Space', 'Light Studio']) {
      await page.getByRole('button', { name, exact: true }).click()
      await expect(page.getByRole('button', { name, exact: true })).toHaveAttribute(
        'aria-pressed',
        'true',
      )
      await page.getByLabel('Background blur', { exact: true }).press('End')
      await expect(page.getByTestId('background-layer')).toHaveCSS('filter', 'blur(20px)')
      expect(await frame.evaluate((canvas: HTMLCanvasElement) => canvas.toDataURL())).toBe(before)
      await page.screenshot({
        path: info.outputPath(`${character}-${name}-blur.png`),
        fullPage: true,
      })
    }
    await page.reload()
    await expect(page.getByLabel('Background blur', { exact: true })).toHaveValue('20')
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
    expect(errors).toEqual([])
    expect(posts).toEqual([])
  })

  test(`${character}: eye geometry keeps iris fixed, closed source exact and mouth stable`, async ({
    page,
  }, info) => {
    test.setTimeout(60_000)
    await page.goto(`/?trial=motion&character=${character}`)
    await expect(page.getByRole('button', { name: 'Play mouth', exact: true })).toBeEnabled()
    await page.getByLabel('Automatic blinking', { exact: true }).uncheck()
    await page.getByLabel('Subtle movement', { exact: true }).uncheck()
    const result = await page.evaluate(async (character) => {
      const { motionDefinition, motionAssetUrl } = await import(
        '/src/experiments/motion/useMotionAssets.ts' as string
      )
      const { createEyeMorph } = await import('/src/experiments/motion/eyeMorph.ts' as string)
      const { createCharacterRenderer } = await import(
        '/src/experiments/motion/characterRenderer.ts' as string
      )
      const { blinkAt, randomSequence } = await import(
        '/src/experiments/motion/useMotionPlayback.ts' as string
      )
      const m = motionDefinition(character),
        load = async (file: string) => {
          const im = new Image()
          im.src = motionAssetUrl(character, file)
          await im.decode()
          return im
        }
      const [opened, closed, mask, fg, depth] = await Promise.all(
        [m.eyes.open, m.eyes.closed, m.eyes.mask, m.foreground, m.depth].map(load),
      )
      const read = (image: CanvasImageSource, w: number, h: number) => {
        const c = document.createElement('canvas')
        c.width = w
        c.height = h
        c.getContext('2d')!.drawImage(image, 0, 0, w, h)
        return c.getContext('2d')!.getImageData(0, 0, w, h).data
      }
      const w = opened.width,
        h = opened.height,
        source = read(opened, w, h),
        target = read(closed, w, h),
        alpha = read(mask, w, h)
      // Compare original pixels at native resolution: GPU linear display
      // downsampling and Canvas2D's area filter intentionally differ.
      const eye = createEyeMorph(opened, closed, mask, m.eyes.curves, () => {}),
        body = createCharacterRenderer(
          fg,
          depth,
          m,
          () => {},
          Math.max(m.canvas.width, m.canvas.height),
        )
      let outside = 0,
        iris = 0,
        closure = 0,
        neutral = 0,
        irisSamples = 0
      for (const amount of [0, 0.01, 0.25, 0.5, 0.75, 0.99, 1]) {
        eye.draw(amount)
        const data = read(eye.canvas, w, h)
        for (let y = 0; y < h; y++)
          for (let x = 0; x < w; x++) {
            const i = (y * w + x) * 4,
              c = m.eyes.curves[x],
              a = (1 - amount) * c[0] + amount * c[2],
              b = (1 - amount) * c[1] + amount * c[2]
            for (let k = 0; k < 3; k++) {
              if (alpha[i] === 0) outside = Math.max(outside, Math.abs(data[i + k] - source[i + k]))
              if (c[3] && y > a + 1 && y < b - 1) {
                iris = Math.max(iris, Math.abs(data[i + k] - source[i + k]))
                irisSamples++
              }
              if (amount === 1 && alpha[i] === 255)
                closure = Math.max(closure, Math.abs(data[i + k] - target[i + k]))
            }
          }
      }
      body.draw(0, false)
      const actual = read(body.canvas, body.canvas.width, body.canvas.height),
        reference = read(fg, body.canvas.width, body.canvas.height)
      for (let i = 0; i < actual.length; i += 4)
        if (reference[i + 3] === 255)
          for (let k = 0; k < 4; k++)
            neutral = Math.max(neutral, Math.abs(actual[i + k] - reference[i + k]))
      const first = randomSequence(117),
        second = randomSequence(117)
      const deterministic = Array.from({ length: 100 }, () => first() === second()).every(Boolean)
      const timing = [0, 40, 80, 120, 190, 260].map((time) => blinkAt(time, m.blink))
      body.dispose()
      eye.dispose()
      return {
        outside,
        iris,
        closure,
        neutral,
        irisSamples,
        deterministic,
        timing,
        geometry: m.geometry_validation,
      }
    }, character)
    await info.attach('pixel-and-timing-invariants', {
      body: JSON.stringify(result),
      contentType: 'application/json',
    })
    expect(result.outside).toBe(0)
    expect(result.iris).toBe(0)
    expect(result.irisSamples).toBeGreaterThan(100)
    expect(result.closure).toBe(0)
    expect(result.neutral).toBeLessThanOrEqual(1)
    expect(result.deterministic).toBe(true)
    expect(result.timing).toEqual([0, 0.5, 1, 1, 0.5, 0])
    expect(result.geometry.minimum_signed_double_area).toBeGreaterThan(0)
    const detail = page.getByRole('img', { name: 'Eyelid detail', exact: true })
    const eyed = () => detail.evaluate((canvas: HTMLCanvasElement) => canvas.toDataURL())
    await page.getByLabel('Eyelid closure', { exact: true }).press('End')
    await page.evaluate(
      () => new Promise<void>((resolve) => requestAnimationFrame(() => resolve())),
    )
    const eyeBefore = await eyed()
    for (const pose of ['Rest', 'A', 'EI', 'O', 'U', 'MBP', 'SZ', 'FV', 'CHSHR']) {
      await page.getByLabel('To pose', { exact: true }).selectOption(pose)
      await page.getByRole('button', { name: pose, exact: true }).last().click()
      expect(await eyed()).toBe(eyeBefore)
    }
    for (const [a, b, value] of [
      ['EI', 'FV', 0.35],
      ['FV', 'A', 0.41],
      ['EI', 'SZ', 0.5],
      ['O', 'U', 0.5],
      ['MBP', 'A', 0.5],
    ] as const) {
      await page.getByLabel('From pose', { exact: true }).selectOption(a)
      await page.getByLabel('To pose', { exact: true }).selectOption(b)
      await page.getByLabel('Mouth blend', { exact: true }).fill(String(value))
      expect(await eyed()).toBe(eyeBefore)
    }
    await page.getByLabel('Eyelid closure', { exact: true }).press('Home')
    await page.getByRole('button', { name: 'Blink once', exact: true }).click()
    await expect
      .poll(() => page.getByLabel('Eyelid closure', { exact: true }).inputValue(), {
        intervals: [16],
      })
      .not.toBe('0')
    await expect(page.getByLabel('Eyelid closure', { exact: true })).toHaveValue('0', {
      timeout: 2000,
    })
    await page.getByRole('button', { name: 'Play mouth', exact: true }).click()
    await page.getByRole('button', { name: 'Pause animation', exact: true }).click()
    await expect(page.getByRole('button', { name: 'Play mouth', exact: true })).toBeDisabled()
  })
}

test('reduced motion allows manual inspection and disables autonomous playback', async ({
  page,
}) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.goto('/?trial=motion&character=syntelle')
  await expect(page.getByRole('img', { name: 'Animated character', exact: true })).toBeVisible()
  await expect(page.getByLabel('Automatic blinking', { exact: true })).not.toBeChecked()
  await expect(page.getByLabel('Subtle movement', { exact: true })).not.toBeChecked()
  await expect(page.getByRole('button', { name: 'Blink once', exact: true })).toBeDisabled()
  await page.getByLabel('Eyelid closure', { exact: true }).fill('0.5')
  await expect(page.getByLabel('Eyelid closure', { exact: true })).toHaveValue('0.5')
})

test('WebGL failure is explicit and preserves the link to the mouth trial', async ({ page }) => {
  await page.addInitScript(() => {
    const original = HTMLCanvasElement.prototype.getContext
    HTMLCanvasElement.prototype.getContext = function (
      this: HTMLCanvasElement,
      kind: string,
      ...args: any[]
    ) {
      if (kind === 'webgl') return null
      return original.call(this, kind as any, ...args)
    } as typeof original
  })
  await page.goto('/?trial=motion&character=syntelle')
  await expect(page.getByRole('alert')).toContainText('Animation is unavailable')
  await expect(
    page.getByRole('link', { name: 'Back to mouth trial', exact: true }),
  ).toHaveAttribute('href', '?trial=viseme-a&oral=semantic&character=syntelle')
})

test('all pose switches reuse three graphics contexts and preserve identical endpoints', async ({
  page,
}, info) => {
  test.setTimeout(60_000)
  await page.addInitScript(() => {
    const getContext = HTMLCanvasElement.prototype.getContext,
      canvases = new WeakSet<HTMLCanvasElement>()
    ;(window as any).motionContextCount = 0
    HTMLCanvasElement.prototype.getContext = function (
      this: HTMLCanvasElement,
      kind: string,
      ...args: any[]
    ) {
      if (kind === 'webgl' && !canvases.has(this)) {
        canvases.add(this)
        ;(window as any).motionContextCount++
      }
      return getContext.call(this, kind as any, ...args)
    } as typeof getContext
  })
  await page.goto('/?trial=motion&character=cafe')
  await expect(page.getByRole('button', { name: 'Play mouth', exact: true })).toBeEnabled()
  await page.getByLabel('Automatic blinking', { exact: true }).uncheck()
  await page.getByLabel('Subtle movement', { exact: true }).uncheck()
  await page.getByLabel('Eyelid closure', { exact: true }).fill('0')
  const count = () => page.evaluate(() => (window as any).motionContextCount)
  expect(await count()).toBe(3)
  const frame = page.getByRole('img', { name: 'Animated character', exact: true }),
    endpoints: Record<string, string> = {}
  const ids = ['Rest', 'A', 'EI', 'O', 'U', 'MBP', 'SZ', 'FV', 'CHSHR']
  for (const a of ids)
    for (const b of ids) {
      await page.getByLabel('From pose', { exact: true }).selectOption(a)
      await page.getByLabel('To pose', { exact: true }).selectOption(b)
      await page.getByRole('button', { name: b, exact: true }).last().click()
      await page.evaluate(
        () => new Promise<void>((resolve) => requestAnimationFrame(() => resolve())),
      )
      const hash = await frame.evaluate((canvas: HTMLCanvasElement) => {
        const text = canvas.toDataURL()
        let hash = 2166136261
        for (let i = 0; i < text.length; i++) hash = Math.imul(hash ^ text.charCodeAt(i), 16777619)
        return String(hash >>> 0)
      })
      if (endpoints[b]) expect(hash).toBe(endpoints[b])
      else endpoints[b] = hash
    }
  expect(await count()).toBe(3)
  await expect(page.getByRole('alert')).toHaveCount(0)
  await info.attach('reused-context-endpoints', {
    body: JSON.stringify(endpoints),
    contentType: 'application/json',
  })
})

test('visibility suspension freezes rendering and resumes without a time jump', async ({
  page,
}) => {
  await page.goto('/?trial=motion&character=cafe')
  await expect(page.getByRole('button', { name: 'Play mouth', exact: true })).toBeEnabled()
  await page.getByRole('button', { name: 'Play mouth', exact: true }).click()
  // Dispatch the browser visibility signal deterministically. A real hidden tab
  // can suspend the test runner's own timers and cannot be timed reliably here.
  await page.evaluate(() => {
    Object.defineProperty(document, 'hidden', { configurable: true, get: () => true })
    document.dispatchEvent(new Event('visibilitychange'))
  })
  const frame = page.getByRole('img', { name: 'Animated character', exact: true }),
    read = () => frame.evaluate((canvas: HTMLCanvasElement) => canvas.toDataURL())
  const frozen = await read()
  await page.waitForTimeout(350)
  expect(await read()).toBe(frozen)
  await page.evaluate(() => {
    Object.defineProperty(document, 'hidden', { configurable: true, get: () => false })
    document.dispatchEvent(new Event('visibilitychange'))
  })
  await expect.poll(read).not.toBe(frozen)
  await expect(page.getByRole('button', { name: 'Play mouth', exact: true })).toHaveAttribute(
    'aria-pressed',
    'false',
  )
})
