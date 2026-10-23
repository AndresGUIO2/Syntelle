import { expect, test } from '@playwright/test'

test('shared idle profile moves promptly, holds briefly and remains deterministic and bounded', async ({
  page,
}, info) => {
  await page.goto('/?view=validation')
  const results = await page.evaluate(async () => {
    const { loadDemoCharacter } = await import('/src/features/demo/catalog.ts' as string)
    const { idlePostureAt, visibleIdleProfile } = await import(
      '/src/features/character/animation/idlePosture.ts' as string
    )
    const { quietPosture } = await import(
      '/src/features/character/animation/speechBehavior.ts' as string
    )
    const rows = []
    for (const id of ['syntelle', 'cafe', 'woodland']) {
      const context = await loadDemoCharacter(id),
        definition = context.motion.definition
      const seed = definition.animation_seed,
        bounds = definition.projection.proposed_idle_degrees
      let previous = idlePostureAt(0, seed, bounds, context.motion.idle)
      let previousOld = quietPosture(0, seed, bounds)
      let moving = 0,
        oldMoving = 0,
        maximumVelocity = 0,
        maximumStill = 0,
        still = 0,
        initialPeak = 0
      let maximumYaw = 0,
        deterministic = true
      for (let ms = 50; ms <= 120000; ms += 50) {
        const current = idlePostureAt(ms, seed, bounds, context.motion.idle)
        const old = quietPosture(ms, seed, bounds)
        deterministic &&=
          JSON.stringify(current) ===
          JSON.stringify(idlePostureAt(ms, seed, bounds, context.motion.idle))
        const velocity = Math.abs(current.yaw - previous.yaw) / 0.05
        maximumVelocity = Math.max(maximumVelocity, velocity)
        maximumYaw = Math.max(maximumYaw, Math.abs(current.yaw))
        if (ms <= 7000) initialPeak = Math.max(initialPeak, Math.abs(current.yaw))
        if (velocity > 0.05) {
          moving++
          still = 0
        } else {
          still += 0.05
          maximumStill = Math.max(maximumStill, still)
        }
        if (Math.abs(old.yaw - previousOld.yaw) / 0.05 > 0.05) oldMoving++
        previous = current
        previousOld = old
      }
      rows.push({
        id,
        profile: context.motion.idle,
        deterministic,
        initialPeak,
        maximumYaw,
        maximumVelocity,
        maximumStill,
        movingFraction: moving / 2400,
        previousMovingFraction: oldMoving / 2400,
        limit: definition.projection.bounds.yaw,
      })
    }
    return { profile: visibleIdleProfile, rows }
  })
  for (const row of results.rows) {
    expect(row.deterministic).toBe(true)
    expect(row.profile.version).toBe('idle-posture/1')
    expect(row.initialPeak).toBeGreaterThan(1.9)
    expect(row.maximumYaw).toBeLessThanOrEqual(row.limit)
    expect(row.maximumVelocity).toBeLessThan(2)
    expect(row.maximumStill).toBeLessThan(2.6)
    expect(row.movingFraction).toBeGreaterThan(0.6)
    expect(row.movingFraction).toBeGreaterThan(row.previousMovingFraction * 2)
  }
  await info.attach('idle-trajectories', {
    body: JSON.stringify(results, null, 2),
    contentType: 'application/json',
  })
})

test('demo shows idle face movement and its movement toggle still freezes the character', async ({
  page,
}, info) => {
  const errors: string[] = [],
    posts: string[] = []
  page.on('pageerror', (error) => errors.push(error.message))
  page.on('request', (request) => {
    if (request.method() === 'POST') posts.push(request.url())
  })
  await page.route('**/api/v1/speech/status', (route) =>
    route.fulfill({ json: { configured: true } }),
  )
  await page.goto('/')
  await expect(page.getByRole('region', { name: 'Syntelle character preview' })).toHaveAttribute(
    'aria-busy',
    'false',
  )
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  await page.getByLabel('Blinking', { exact: true }).uncheck()
  await page.getByRole('button', { name: 'Close customization' }).click()
  const frame = page.getByRole('img', { name: 'Animated character', exact: true })
  const before = await frame.evaluate(async (canvas: HTMLCanvasElement) => {
    const { loadDemoCharacter } = await import('/src/features/demo/catalog.ts' as string)
    const d = (await loadDemoCharacter('syntelle')).motion.definition
    const a = d.anatomy
    const crop = document.createElement('canvas')
    crop.width = crop.height = 128
    const ctx = crop.getContext('2d')!
    ctx.drawImage(
      canvas,
      a.head_pivot[0] - a.face_width / 2,
      a.chin_y - a.face_height,
      a.face_width,
      a.face_height,
      0,
      0,
      128,
      128,
    )
    return Array.from(ctx.getImageData(0, 0, 128, 128).data)
  })
  await page.waitForTimeout(4000)
  const change = await frame.evaluate(async (canvas: HTMLCanvasElement, baseline) => {
    const { loadDemoCharacter } = await import('/src/features/demo/catalog.ts' as string)
    const a = (await loadDemoCharacter('syntelle')).motion.definition.anatomy
    const crop = document.createElement('canvas')
    crop.width = crop.height = 128
    const ctx = crop.getContext('2d')!
    ctx.drawImage(
      canvas,
      a.head_pivot[0] - a.face_width / 2,
      a.chin_y - a.face_height,
      a.face_width,
      a.face_height,
      0,
      0,
      128,
      128,
    )
    const current = ctx.getImageData(0, 0, 128, 128).data
    let difference = 0
    for (let i = 0; i < current.length; i++)
      if (i % 4 !== 3) difference += Math.abs(current[i]! - baseline[i]!)
    return difference / (128 * 128 * 3)
  }, before)
  expect(change).toBeGreaterThan(0.15)
  await page.screenshot({ path: info.outputPath('idle-active.png'), fullPage: true })
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  await page.getByLabel('Natural movement', { exact: true }).uncheck()
  await page.getByRole('button', { name: 'Close customization' }).click()
  await page.waitForTimeout(100)
  const frozen = await frame.evaluate((canvas: HTMLCanvasElement) => canvas.toDataURL())
  await page.waitForTimeout(400)
  expect(await frame.evaluate((canvas: HTMLCanvasElement) => canvas.toDataURL())).toBe(frozen)
  expect(errors).toEqual([])
  expect(posts).toEqual([])
  await info.attach('idle-face-difference', {
    body: JSON.stringify({ mean_rgb_difference: change }),
    contentType: 'application/json',
  })
})
