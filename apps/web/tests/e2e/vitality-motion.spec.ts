import { test, expect } from '@playwright/test'

test('breathing has shorter inhales, varied cycles, smooth bounds and repeatable seeking', async ({
  page,
}) => {
  await page.goto('/?trial=motion&character=syntelle&head=vitality-candidate')
  const result = await page.evaluate(async () => {
    const { createBreathingTrack } = await import(
      '/src/features/character/animation/breathing.ts' as string
    )
    const { resolveMotionContext } = await import(
      '/src/experiments/motion/motionCatalog.ts' as string
    )
    const settings = resolveMotionContext('syntelle', true, 'vitality').definition.breathing
    const track = createBreathingTrack(42, settings)
    const values = Array.from({ length: 18001 }, (_, i) => track.sample(i * 10))
    const troughs: number[] = [],
      peaks: number[] = [0]
    for (let i = 1; i < values.length - 1; i++) {
      if (values[i] < values[i - 1] && values[i] <= values[i + 1]) troughs.push(i)
      if (values[i] > values[i - 1] && values[i] >= values[i + 1]) peaks.push(i)
    }
    const cycles = peaks.slice(1).map((end, i) => {
      const start = peaks[i],
        trough = troughs.find((t) => t > start && t < end)!
      return {
        duration: (end - start) * 10,
        inhale: (trough - start) / (end - start),
        amplitude: -values[trough],
      }
    })
    const seek = [0, 1234, 30000, 400, 27000].map((ms) => track.sample(ms))
    const fresh = [0, 1234, 30000, 400, 27000].map((ms) =>
      createBreathingTrack(42, settings).sample(ms),
    )
    let rejects = false
    try {
      createBreathingTrack(42, { ...settings, amplitude: [1, 2] })
    } catch {
      rejects = true
    }
    return {
      cycles,
      seek,
      fresh,
      rejects,
      min: Math.min(...values),
      max: Math.max(...values),
      maxStep: Math.max(...values.slice(1).map((v, i) => Math.abs(v - values[i]))),
    }
  })
  expect(result.cycles.length).toBeGreaterThan(25)
  expect(
    result.cycles.every(
      (c) =>
        c.duration >= 4580 &&
        c.duration <= 6020 &&
        c.inhale > 0.31 &&
        c.inhale < 0.39 &&
        c.amplitude >= 0.819 &&
        c.amplitude <= 1,
    ),
  ).toBe(true)
  expect(new Set(result.cycles.map((c) => Math.round(c.duration / 100))).size).toBeGreaterThan(5)
  expect(
    Math.max(...result.cycles.map((c) => c.amplitude)) -
      Math.min(...result.cycles.map((c) => c.amplitude)),
  ).toBeGreaterThan(0.12)
  expect(result.min).toBeGreaterThanOrEqual(-1)
  expect(result.max).toBe(0)
  expect(result.maxStep).toBeLessThan(0.014)
  expect(result.seek).toEqual(result.fresh)
  expect(result.rejects).toBe(true)
})

test('both blink closures spend the shared allowance; reservations cancel and never create triples', async ({
  page,
}, info) => {
  await page.goto('/?trial=motion&character=syntelle&head=vitality-candidate')
  const result = await page.evaluate(async () => {
    const { createContextualBlinkPlanner } = await import(
      '/src/features/character/animation/speechBehavior.ts' as string
    )
    const { resolveMotionContext } = await import(
      '/src/experiments/motion/motionCatalog.ts' as string
    )
    const settings = resolveMotionContext('syntelle', true, 'vitality').definition.behavior.blink
    const idle = { active: false, position: 0, session: 0, plan: null }
    function run() {
      const planner = createContextualBlinkPlanner(42, settings),
        starts: number[] = [],
        groups: number[] = []
      let second: number | null = null,
        busyUntil = 0
      for (let ms = 0; ms < 3600000; ms += 16) {
        if (second !== null && ms >= second) {
          if (!planner.startReserved(ms)) throw Error('Lost reservation')
          starts.push(ms)
          busyUntil = ms + 260
          second = null
        }
        if (planner.sample(ms, idle, ms >= busyUntil && second === null)) {
          starts.push(ms)
          busyUntil = ms + 260
          if (planner.reserveDouble(ms)) {
            groups.push(ms)
            second = ms + 380
          }
        }
        if (starts.length > 2 + (ms * settings.rate_per_minute) / 60000 + 1e-6)
          throw Error('Frequency budget exceeded')
      }
      return { starts, groups }
    }
    const a = run(),
      b = run(),
      manual = createContextualBlinkPlanner(1, settings)
    const before = manual.canStartDouble(),
      first = manual.beginManualDouble(0),
      duplicate = manual.beginManualDouble(1)
    manual.cancelDouble()
    const noSecond = manual.startReserved(400)
    // The first closure remains spent; only the unshown second is refunded.
    const afterCancel = manual.canStartDouble()
    manual.record(500)
    const noFreePair = manual.beginManualDouble(501)
    return {
      ...a,
      deterministic: JSON.stringify(a) === JSON.stringify(b),
      before,
      first,
      duplicate,
      noSecond,
      afterCancel,
      noFreePair,
    }
  })
  expect(result.deterministic).toBe(true)
  expect(result.starts.length).toBeLessThanOrEqual(962)
  expect(result.starts.length).toBeGreaterThan(500)
  expect(result.groups.length).toBeGreaterThan(8)
  expect(result.groups.length).toBeLessThan(result.starts.length * 0.2)
  for (let i = 2; i < result.starts.length; i++)
    expect(result.starts[i] - result.starts[i - 2]).toBeGreaterThanOrEqual(800)
  expect(result.before && result.first).toBe(true)
  expect(result.duplicate || result.noSecond || result.afterCancel || result.noFreePair).toBe(false)
  await info.attach('one-hour-blink-simulation', {
    body: JSON.stringify({
      closures: result.starts.length,
      double_groups: result.groups.length,
      rate_limit: '16 closures/minute plus two initial credits; both closures charged',
      deterministic: result.deterministic,
    }),
    contentType: 'application/json',
  })
})

test('breathing moves the torso without changing face pixels or rebuilding textures', async ({
  page,
}) => {
  test.setTimeout(60000)
  await page.goto('/?trial=motion&character=syntelle&head=vitality-candidate')
  const results = await page.evaluate(async () => {
    const { createCharacterRenderer } = await import(
      '/src/features/character/rendering/characterRenderer.ts' as string
    )
    const { resolveMotionContext } = await import(
      '/src/experiments/motion/motionCatalog.ts' as string
    )
    const { resolvePoseContext } = await import(
      '/src/experiments/viseme-a/poseCatalog.ts' as string
    )
    const results = []
    for (const name of ['syntelle', 'cafe', 'woodland']) {
      const context = resolveMotionContext(name, true, 'vitality'),
        m = context.definition
      const load = async (name: string) => {
        const image = new Image()
        image.src = context.assetUrl(name)
        await image.decode()
        return image
      }
      const [foreground, depth] = await Promise.all([load(m.foreground), load(m.depth)])
      const body = createCharacterRenderer(foreground, depth, m, () => {
        throw Error('Renderer failed')
      })
      const copy = () => {
        const c = document.createElement('canvas')
        c.width = body.canvas.width
        c.height = body.canvas.height
        c.getContext('2d')!.drawImage(body.canvas, 0, 0)
        return c.getContext('2d')!.getImageData(0, 0, c.width, c.height).data
      }
      body.draw(0, true, { yaw: 0, pitch: 0, roll: 0 })
      const neutral = copy()
      body.draw(1800, true, { yaw: 0, pitch: 0, roll: 0 })
      const inhale = copy()
      const scale = body.canvas.width / m.canvas.width
      let faceDelta = 0,
        bodyDelta = 0
      for (let i = 0; i < neutral.length; i += 4) {
        const x = (i / 4) % body.canvas.width,
          y = Math.floor(i / 4 / body.canvas.width)
        const inFace = [
          m.eyes.crop,
          resolvePoseContext({ character: name, trial: 'motion' }).package.crop,
        ].some(
          (rect) =>
            x > rect.x * scale + 2 &&
            x < (rect.x + rect.width) * scale - 2 &&
            y > rect.y * scale + 2 &&
            y < (rect.y + rect.height) * scale - 2,
        )
        for (let c = 0; c < 4; c++) {
          const delta = Math.abs(neutral[i + c] - inhale[i + c])
          if (inFace) faceDelta = Math.max(faceDelta, delta)
          if (y > m.anatomy.chin_y * scale) bodyDelta += delta
        }
      }
      body.draw(1800, false)
      const off = copy()
      results.push({
        name,
        faceDelta,
        bodyDelta,
        neutralRestored: off.every((v, i) => v === neutral[i]),
        fullUploads: body.stats.fullUploads,
      })
      body.dispose()
    }
    return results
  })
  for (const value of results) {
    expect(value.faceDelta, value.name).toBe(0)
    expect(value.bodyDelta, value.name).toBeGreaterThan(500)
    expect(value.neutralRestored, value.name).toBe(true)
    expect(value.fullUploads, value.name).toBe(1)
  }
})

for (const character of ['syntelle', 'cafe', 'woodland']) {
  test(`${character}: real double blink opens between closures and freezes while paused`, async ({
    page,
  }, info) => {
    const posts: string[] = [],
      errors: string[] = []
    page.on('request', (r) => {
      if (r.method() === 'POST') posts.push(r.url())
    })
    page.on('pageerror', (e) => errors.push(e.message))
    await page.clock.install()
    await page.goto(`/?trial=motion&character=${character}&head=vitality-candidate`)
    await expect(page.getByRole('button', { name: 'Play mouth', exact: true })).toBeEnabled()
    await page.clock.pauseAt(new Date((await page.evaluate(() => Date.now())) + 100))
    await page.getByLabel('Automatic blinking', { exact: true }).uncheck()
    await page.getByLabel('Subtle movement', { exact: true }).uncheck()
    const twice = page.getByRole('button', { name: 'Blink twice', exact: true })
    await expect(twice).toBeEnabled()
    await twice.click()
    const slider = page.getByLabel('Eyelid closure', { exact: true })
    const closure = async () => Number(await slider.inputValue())
    await page.clock.runFor(96)
    expect(await closure()).toBeGreaterThan(0.99)
    await page.screenshot({ path: info.outputPath('first-closure.png') })
    await page.getByRole('button', { name: 'Pause animation', exact: true }).click()
    const paused = await closure()
    await page.clock.runFor(1000)
    expect(await closure()).toBe(paused)
    await page.getByRole('button', { name: 'Resume animation', exact: true }).click()
    await page.clock.runFor(224)
    expect(await closure()).toBe(0)
    await page.screenshot({ path: info.outputPath('between-closures.png') })
    let secondPeak = 0,
      captured = false
    for (let i = 0; i < 24; i++) {
      await page.clock.runFor(16)
      secondPeak = Math.max(secondPeak, await closure())
      if (!captured && (await closure()) === 1) {
        await page.screenshot({ path: info.outputPath('second-closure.png') })
        captured = true
      }
    }
    expect(secondPeak).toBeGreaterThan(0.99)
    expect(captured).toBe(true)
    await page.clock.runFor(400)
    expect(await closure()).toBe(0)
    await expect(twice).toBeDisabled()
    await expect(
      page.getByRole('link', { name: 'Compare previous breathing and blinking' }),
    ).toHaveAttribute('href', `?trial=motion&character=${character}&head=contextual-candidate`)
    expect(errors).toEqual([])
    expect(posts).toEqual([])
  })
}

test('manual eyelids and reduced motion cancel a pending second closure', async ({ page }) => {
  await page.clock.install()
  await page.goto('/?trial=motion&character=syntelle&head=vitality-candidate')
  await expect(page.getByRole('button', { name: 'Blink twice', exact: true })).toBeEnabled()
  await page.getByLabel('Automatic blinking', { exact: true }).uncheck()
  await page.getByRole('button', { name: 'Blink twice', exact: true }).click()
  await page.clock.runFor(280)
  const slider = page.getByLabel('Eyelid closure', { exact: true })
  await slider.fill('0.25')
  await page.clock.runFor(800)
  await expect(slider).toHaveValue('0.25')
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await expect(page.getByRole('button', { name: 'Blink twice', exact: true })).toBeDisabled()
  await expect(page.getByLabel('Subtle movement', { exact: true })).not.toBeChecked()
  await expect(slider).toHaveValue('0')
})

test('hidden tabs freeze the open gap without dropping or duplicating the reserved blink', async ({
  page,
}) => {
  await page.clock.install()
  await page.goto('/?trial=motion&character=syntelle&head=vitality-candidate')
  await expect(page.getByRole('button', { name: 'Blink twice', exact: true })).toBeEnabled()
  await page.clock.pauseAt(new Date((await page.evaluate(() => Date.now())) + 100))
  await page.getByLabel('Automatic blinking', { exact: true }).uncheck()
  await page.getByRole('button', { name: 'Blink twice', exact: true }).click()
  await page.clock.runFor(304)
  const slider = page.getByLabel('Eyelid closure', { exact: true })
  await expect(slider).toHaveValue('0')
  await page.evaluate(() => {
    Object.defineProperty(document, 'hidden', { configurable: true, value: true })
    document.dispatchEvent(new Event('visibilitychange'))
  })
  await page.clock.runFor(2000)
  await expect(slider).toHaveValue('0')
  await page.evaluate(() => {
    Object.defineProperty(document, 'hidden', { configurable: true, value: false })
    document.dispatchEvent(new Event('visibilitychange'))
  })
  let closures = 0,
    previouslyClosed = false
  for (let i = 0; i < 40; i++) {
    await page.clock.runFor(16)
    const closed = Number(await slider.inputValue()) === 1
    if (closed && !previouslyClosed) closures++
    previouslyClosed = closed
  }
  expect(closures).toBe(1)
  await expect(slider).toHaveValue('0')
})
