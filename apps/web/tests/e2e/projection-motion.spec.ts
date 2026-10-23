import { test, expect } from '@playwright/test'

test('comparison replay is deterministic and all three views freeze with the shared clock', async ({
  page,
}) => {
  await page.clock.install()
  await page.goto('/?trial=motion&character=syntelle&head=rgbd-compare')
  await expect(page.getByRole('button', { name: 'Play mouth', exact: true })).toBeEnabled()
  await page.clock.pauseAt(new Date((await page.evaluate(() => Date.now())) + 100))
  await page.getByLabel('Automatic blinking', { exact: true }).uncheck()
  const poses = await page.evaluate(async () => {
    const { projectionReviewPose } = await import(
      '/src/features/character/animation/projectionReview.ts' as string
    )
    return [0, 2500, 6500, 10000, 13500, 16000].map((t) => projectionReviewPose(t))
  })
  expect(poses[0]).toEqual(poses.at(-1))
  expect(poses[1].yaw).toBe(3)
  expect(poses[2].yaw).toBe(-3)
  await page.getByRole('button', { name: 'Replay comparison', exact: true }).click()
  await page.clock.runFor(1200)
  await page.getByRole('button', { name: 'Pause animation', exact: true }).click()
  await page.clock.runFor(32)
  const read = () =>
    page
      .getByRole('img', { name: 'Animated character', exact: true })
      .evaluateAll((canvases: HTMLCanvasElement[]) => canvases.map((c) => c.toDataURL()))
  const before = await read()
  await page.clock.runFor(2000)
  expect(await read()).toEqual(before)
  await page.evaluate(() => {
    Object.defineProperty(document, 'hidden', { configurable: true, value: true })
    document.dispatchEvent(new Event('visibilitychange'))
  })
  await page.getByRole('button', { name: 'Resume animation', exact: true }).click()
  await page.clock.runFor(2000)
  expect(await read()).toEqual(before)
  await page.evaluate(() => {
    Object.defineProperty(document, 'hidden', { configurable: true, value: false })
    document.dispatchEvent(new Event('visibilitychange'))
  })
  await page.clock.runFor(1000)
  expect(await read()).not.toEqual(before)
})

test('neutral projective sampling preserves the existing image and separates facial depth during rotation', async ({
  page,
}, info) => {
  await page.goto('/?trial=motion&character=syntelle&head=rgbd-candidate')
  const results = await page.evaluate(async () => {
    const { resolveMotionContext } = await import(
      '/src/experiments/motion/motionCatalog.ts' as string
    )
    const { createCharacterRenderer } = await import(
      '/src/features/character/rendering/characterRenderer.ts' as string
    )
    const { createPerspectiveRenderer } = await import(
      '/src/features/character/rendering/perspectiveRenderer.ts' as string
    )
    const results = []
    for (const name of ['syntelle', 'cafe', 'woodland']) {
      const c = resolveMotionContext(name, true, 'rgbd'),
        m = c.definition
      const load = async (name: string) => {
        const im = new Image()
        im.src = c.assetUrl(name)
        await im.decode()
        return im
      }
      const [fg, depth, geometry] = await Promise.all([
        load(m.foreground),
        load(m.depth),
        fetch(c.assetUrl(m.projection.geometry.file)).then((r) => r.arrayBuffer()),
      ])
      const old = createCharacterRenderer(fg, depth, m, () => {}),
        next = createPerspectiveRenderer(fg, m, geometry, () => {})
      // Independent source-image oracle: a single flat quad has no internal
      // tessellation/snap error and does not use the candidate's projection.
      const quad = createCharacterRenderer(
        fg,
        depth,
        {
          ...m,
          rig: {
            ...m.rig,
            grid: 1,
            vertices: [
              [0, 0, 0, 0, 0],
              [1, 0, 0, 0, 0],
              [0, 1, 0, 0, 0],
              [1, 1, 0, 0, 0],
            ],
          },
        },
        () => {},
      )
      const read = (canvas: HTMLCanvasElement) => {
        const out = document.createElement('canvas')
        out.width = canvas.width
        out.height = canvas.height
        out.getContext('2d')!.drawImage(canvas, 0, 0)
        return out.getContext('2d')!.getImageData(0, 0, out.width, out.height).data
      }
      old.draw(0, false)
      next.draw(0, false)
      quad.draw(0, false)
      const a = read(quad.canvas),
        b = read(next.canvas)
      next.draw(1800, true, { yaw: 0, pitch: 0, roll: 0 })
      const breathing = read(next.canvas)
      let breathingFaceDelta = 0,
        breathingBodyDelta = 0
      const renderScale = next.canvas.width / m.canvas.width
      for (let i = 0; i < b.length; i += 4) {
        const y = Math.floor(i / 4 / next.canvas.width) / renderScale
        for (let k = 0; k < 4; k++) {
          const delta = Math.abs(b[i + k] - breathing[i + k])
          if (y < m.anatomy.chin_y) breathingFaceDelta = Math.max(breathingFaceDelta, delta)
          else breathingBodyDelta += delta
        }
      }
      let maximumOpaque = 0,
        maximumAlpha = 0,
        changed = 0
      for (let i = 0; i < a.length; i += 4) {
        maximumAlpha = Math.max(maximumAlpha, Math.abs(a[i + 3] - b[i + 3]))
        if (a[i + 3] === 255 && b[i + 3] === 255)
          for (let k = 0; k < 3; k++)
            maximumOpaque = Math.max(maximumOpaque, Math.abs(a[i + k] - b[i + k]))
        if (
          a[i] !== b[i] ||
          a[i + 1] !== b[i + 1] ||
          a[i + 2] !== b[i + 2] ||
          a[i + 3] !== b[i + 3]
        )
          changed++
      }
      next.draw(0, true, { yaw: 3, pitch: 0, roll: 0 })
      const turned = read(next.canvas)
      let faceChanges = 0,
        torsoChanges = 0
      const scale = next.canvas.width / m.canvas.width
      for (let i = 0; i < b.length; i += 4) {
        if (
          b[i] === turned[i] &&
          b[i + 1] === turned[i + 1] &&
          b[i + 2] === turned[i + 2] &&
          b[i + 3] === turned[i + 3]
        )
          continue
        const y = Math.floor(i / 4 / next.canvas.width) / scale
        if (y < m.anatomy.chin_y) faceChanges++
        if (y > m.anatomy.chin_y + m.anatomy.face_height * 0.6) torsoChanges++
      }
      results.push({
        name,
        maximumOpaque,
        maximumAlpha,
        changed,
        faceChanges,
        torsoChanges,
        stats: next.stats,
        breathingFaceDelta,
        breathingBodyDelta,
      })
      old.dispose()
      next.dispose()
      quad.dispose()
    }
    return results
  })
  await info.attach('neutral-and-local-motion', {
    body: JSON.stringify(results),
    contentType: 'application/json',
  })
  for (const r of results) {
    expect(r.maximumOpaque, r.name).toBeLessThanOrEqual(1)
    expect(r.maximumAlpha, r.name).toBeLessThanOrEqual(1)
    expect(r.faceChanges).toBeGreaterThan(500)
    expect(r.torsoChanges).toBe(0)
    expect(r.stats.fullUploads).toBe(1)
    expect(r.stats.vertexUploads).toBe(1)
    expect(r.breathingFaceDelta, r.name).toBe(0)
    expect(r.breathingBodyDelta, r.name).toBeGreaterThan(500)
  }
})

test('depth testing selects the near surface independently of draw order, including transparent foreground', async ({
  page,
}) => {
  await page.goto('/?trial=motion&character=syntelle&head=rgbd-candidate')
  const result = await page.evaluate(async () => {
    const { resolveMotionContext } = await import(
      '/src/experiments/motion/motionCatalog.ts' as string
    )
    const { createPerspectiveRenderer } = await import(
      '/src/features/character/rendering/perspectiveRenderer.ts' as string
    )
    const original = resolveMotionContext('syntelle', true, 'rgbd').definition
    const run = async (reverse: boolean, transparent: boolean) => {
      const canvas = document.createElement('canvas')
      canvas.width = canvas.height = 64
      const ctx = canvas.getContext('2d')!
      ctx.fillStyle = transparent ? 'rgba(255,0,0,.5)' : 'red'
      ctx.fillRect(18, 15, 9, 34)
      ctx.fillStyle = 'blue'
      ctx.fillRect(38, 15, 9, 34)
      const image = new Image()
      image.src = canvas.toDataURL()
      await image.decode()
      const vertices = new Float32Array(
        [
          [0.31, 0.35, 24, 1, 0],
          [0.39, 0.35, 24, 1, 0],
          [0.31, 0.65, 24, 1, 0],
          [0.39, 0.65, 24, 1, 0],
          [0.61, 0.35, 0, 1, 0],
          [0.69, 0.35, 0, 1, 0],
          [0.61, 0.65, 0, 1, 0],
          [0.69, 0.65, 0, 1, 0],
        ].flat(),
      )
      const near = [0, 1, 2, 2, 1, 3],
        far = [4, 5, 6, 6, 5, 7]
      const indices = new Uint16Array(reverse ? [...far, ...near] : [...near, ...far]),
        fringe = new Uint16Array(transparent ? near : [])
      const buffer = new Uint8Array(vertices.byteLength + indices.byteLength + fringe.byteLength)
      buffer.set(new Uint8Array(vertices.buffer))
      buffer.set(new Uint8Array(indices.buffer), vertices.byteLength)
      buffer.set(new Uint8Array(fringe.buffer), vertices.byteLength + indices.byteLength)
      const data = {
        ...original,
        canvas: { width: 64, height: 64 },
        projection: {
          ...original.projection,
          source_sha256: original.original_sha256,
          geometry: { vertices: 8, triangles: 4, fringe_triangles: transparent ? 2 : 0 },
          validation: { valid: true },
          bounds: { yaw: 35, pitch: 1, roll: 1 },
          pivot: [0, 0, 0],
          camera: {
            size: [64, 64],
            center: [32, 32],
            pivot_px: [32, 32],
            focal: 96,
            distance: 96,
            near: 32,
            far: 160,
          },
        },
      }
      const renderer = createPerspectiveRenderer(image, data, buffer.buffer, () => {})
      renderer.draw(0, true, { yaw: 30, pitch: 0, roll: 0 })
      ctx.clearRect(0, 0, 64, 64)
      ctx.drawImage(renderer.canvas, 0, 0)
      const pixel = Array.from(ctx.getImageData(40, 32, 1, 1).data)
      renderer.dispose()
      return pixel
    }
    return {
      first: await run(false, false),
      reversed: await run(true, false),
      transparent: await run(true, true),
    }
  })
  expect(result.first).toEqual([255, 0, 0, 255])
  expect(result.reversed).toEqual(result.first)
  expect(result.transparent[0]).toBeGreaterThanOrEqual(126)
  expect(result.transparent[2]).toBeGreaterThanOrEqual(126)
  expect(result.transparent[3]).toBe(255)
})

test('a corrupted geometry download fails explicitly before creating a renderer', async ({
  page,
}) => {
  await page.route('**/rgbd/syntelle/projection.bin*', (r) =>
    new URL(r.request().url()).searchParams.has('url')
      ? r.continue()
      : r.fulfill({ body: Buffer.from('invalid'), contentType: 'application/octet-stream' }),
  )
  await page.goto('/?trial=motion&character=syntelle&head=rgbd-candidate')
  await expect(page.getByRole('alert')).toContainText('checksum differs')
})

for (const name of ['syntelle', 'cafe', 'woodland']) {
  test(`${name}: projected silhouette, expressions and backgrounds remain independent`, async ({
    page,
  }, info) => {
    test.setTimeout(60000)
    await page.goto(`/?trial=motion&character=${name}&head=rgbd-candidate`)
    await expect(page.getByRole('button', { name: 'Play mouth', exact: true })).toBeEnabled()
    await page.getByRole('button', { name: 'Pause animation', exact: true }).click()
    await page.getByRole('button', { name: '3° yaw', exact: true }).click()
    await page.getByLabel('Eyelid closure', { exact: true }).fill('0.5')
    await page.getByLabel('From pose', { exact: true }).selectOption('EI')
    await page.getByLabel('To pose', { exact: true }).selectOption('FV')
    await page.getByLabel('Mouth blend', { exact: true }).fill('0.35')
    const frame = page.getByRole('img', { name: 'Animated character', exact: true })
    const read = () => frame.evaluate((canvas: HTMLCanvasElement) => canvas.toDataURL())
    // Wait for the shared expression patch and body draw, not just reactive labels.
    await page.evaluate(
      () =>
        new Promise<void>((resolve) =>
          requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
        ),
    )
    const before = await read()
    for (const background of ['Light Studio', 'Dark Lounge', 'Natural Space'])
      for (const blur of ['0', '20']) {
        await page.getByRole('button', { name: background, exact: true }).click()
        await page.getByLabel('Background blur', { exact: true }).fill(blur)
        expect(await read()).toBe(before)
        await page.locator('main').evaluate((element) => {
          element.scrollTop = 0
        })
        await page.screenshot({ path: info.outputPath(`${name}-${background}-${blur}.png`) })
      }
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await expect(
      page.getByRole('button', { name: 'Replay comparison', exact: true }),
    ).toBeDisabled()
    await expect(page.getByLabel('Automatic blinking', { exact: true })).not.toBeChecked()
  })
  test(`${name}: projection preview loads, shares expression updates and exposes matched comparison`, async ({
    page,
  }, info) => {
    const errors: string[] = [],
      posts: string[] = []
    page.on('pageerror', (e) => errors.push(e.message))
    page.on('request', (r) => {
      if (r.method() === 'POST') posts.push(r.url())
    })
    await page.goto(`/?trial=motion&character=${name}&head=rgbd-compare`)
    await expect(page.getByRole('button', { name: 'Play mouth', exact: true })).toBeEnabled()
    await expect(page.getByRole('img', { name: 'Animated character', exact: true })).toHaveCount(3)
    await page.getByRole('button', { name: 'Pause animation', exact: true }).click()
    await page.getByLabel('Eyelid closure', { exact: true }).fill('0')
    for (const value of [-3, 0, 3]) {
      await page.getByRole('button', { name: `${value}° yaw`, exact: true }).click()
      await page.locator('main').evaluate((element) => {
        element.scrollTop = 0
      })
      await page.screenshot({ path: info.outputPath(`${name}-yaw-${value}.png`), fullPage: true })
    }
    await page.getByLabel('Eyelid closure', { exact: true }).fill('1')
    await page.getByLabel('To pose', { exact: true }).selectOption('FV')
    await page.getByLabel('Mouth blend', { exact: true }).fill('0.35')
    await page.screenshot({ path: info.outputPath(`${name}-fv-eyes.png`), fullPage: true })
    await page.getByLabel('Compare all three versions', { exact: true }).uncheck()
    await expect(page.getByRole('img', { name: 'Animated character', exact: true })).toHaveCount(1)
    expect(errors).toEqual([])
    expect(posts).toEqual([])
  })
}
