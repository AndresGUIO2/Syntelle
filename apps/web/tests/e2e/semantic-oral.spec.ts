import { expect, test } from '@playwright/test'

test('corrected FV supports playback and retains automatic comparison', async ({ page }, info) => {
  await page.goto('/?trial=viseme-a')
  await expect(page.getByRole('button', { name: 'Play', exact: true })).toBeEnabled({
    timeout: 15000,
  })
  await page.getByLabel('To pose', { exact: true }).selectOption('CHSHR')
  await page.getByRole('button', { name: 'CHSHR', exact: true }).click()
  await page.getByText('Compare preparation · CHSHR', { exact: true }).click()
  await expect(
    page.getByRole('img', { name: 'Automatic semantic boundaries', exact: true }),
  ).toBeVisible()
  await expect(
    page.getByRole('img', { name: 'Current target mouth endpoint', exact: true }),
  ).toBeVisible()
  await page.screenshot({ path: info.outputPath('chshr-comparison.png'), fullPage: true })
  await page.getByLabel('To pose', { exact: true }).selectOption('FV')
  await page.getByRole('button', { name: 'FV', exact: true }).click()
  await expect(page.getByRole('alert')).toHaveCount(0)
  const preparation = await page.evaluate(async () => {
    const { posePackage: p } = await import('/src/experiments/viseme-a/usePosePackage.ts' as string)
    return {
      ready: p.poses.FV.preparation.ready,
      lower: p.poses.FV.lower_visible,
      registration: p.poses.FV.oral_registration,
    }
  })
  expect(preparation.ready).toBe(true)
  expect(preparation.lower).toBe(false)
  expect(preparation.registration.aperture_shape_unchanged).toBe(true)
  await page.screenshot({ path: info.outputPath('fv-corrected.png'), fullPage: true })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
})

test('semantic endpoints preserve observed lips and transitions keep opaque dental material', async ({
  page,
}, info) => {
  await page.goto('/?trial=viseme-a&oral=semantic')
  await expect(page.getByRole('button', { name: 'Play', exact: true })).toBeEnabled({
    timeout: 15000,
  })
  const report = await page.evaluate(async () => {
    const { posePackage: p, poseAssetUrl } = await import(
      '/src/experiments/viseme-a/usePosePackage.ts' as string
    )
    const { createCompletedMorph } = await import(
      '/src/experiments/viseme-a/completedMorph.ts' as string
    )
    const load = async (name: string) => {
      const image = new Image()
      image.src = poseAssetUrl(name)
      await image.decode()
      return image
    }
    const read = (image: CanvasImageSource) => {
      const c = document.createElement('canvas')
      c.width = p.crop.width
      c.height = p.crop.height
      c.getContext('2d')!.drawImage(image, 0, 0)
      return c.getContext('2d')!.getImageData(0, 0, c.width, c.height).data
    }
    const [upper, lower] = await Promise.all([load(p.reference.upper), load(p.reference.lower)])
    const base = await load('base.webp')
    const readOverBase = (image: CanvasImageSource) => {
      const c = document.createElement('canvas')
      c.width = p.crop.width
      c.height = p.crop.height
      const ctx = c.getContext('2d')!
      ctx.drawImage(base, -p.crop.x, -p.crop.y)
      ctx.drawImage(image, 0, 0)
      return ctx.getImageData(0, 0, c.width, c.height).data
    }
    const enamel = read(upper),
      lowerPixels = read(lower)
    const poses: Record<string, any> = {},
      originals: Record<string, Uint8ClampedArray> = {},
      endpoints: Record<string, Uint8ClampedArray> = {}
    let lipDelta = 0,
      lipSamples = 0,
      fallbackDelta = 0,
      toothDelta = 0,
      toothSamples = 0,
      endpointDelta = 0,
      outside = 0,
      frames = 0
    const fallbackDifferences: any[] = []
    for (const [id, data] of Object.entries(p.poses) as [string, any][]) {
      poses[id] = {
        data,
        surface: await load(data.surface),
        cavity: await load(data.cavity),
        mask: await load(data.mask),
      }
      originals[id] = read(await load(data.comparison.original))
    }
    for (const [key, pair] of Object.entries(p.pairs) as [string, any][]) {
      if (!pair.safe) throw Error(`Inverting geometry: ${key}`)
      const [a, b] = key.split('-'),
        from = poses[a],
        to = poses[b]
      const renderer = createCompletedMorph(from, to, pair.mesh, upper, lower, () => {}, {
        ...p.oral,
        contact_shadow_strength: 0,
        depth_strength: 0,
      })
      if (!renderer) throw Error('WebGL required')
      const maskA = read(from.mask),
        maskB = read(to.mask)
      for (const t of [0, 0.01, 0.25, 0.35, 0.41, 0.5, 0.75, 0.99, 1]) {
        renderer.draw(t)
        const pixels = read(renderer.canvas)
        frames++
        for (let i = 0; i < pixels.length; i += 4)
          if (maskA[i] === 0 && maskB[i] === 0 && pixels[i + 3] !== 0) outside++
        if (t === 0 || t === 1) {
          const id = t === 0 ? a : b,
            mask = t === 0 ? maskA : maskB
          if (endpoints[id])
            for (let i = 0; i < pixels.length; i++)
              endpointDelta = Math.max(endpointDelta, Math.abs(pixels[i] - endpoints[id][i]))
          else endpoints[id] = new Uint8ClampedArray(pixels)
          // Well outside the true aperture but inside the opaque edit patch,
          // the original photographic lips must remain unchanged.
          for (let i = 0; i < pixels.length; i += 4)
            if (mask[i] === 255 && mask[i + 1] <= 100) {
              lipSamples++
              for (let c = 0; c < 3; c++)
                lipDelta = Math.max(lipDelta, Math.abs(pixels[i + c] - originals[id][i + c]))
            }
        }
        if (a === b && t === 0) {
          // Measure opaque upper material only inside the observed opening.
          for (let i = 0; i < pixels.length; i += 4)
            if (enamel[i + 3] === 255 && maskA[i] === 255 && maskA[i + 1] >= 144) {
              toothSamples++
              for (let c = 0; c < 3; c++)
                toothDelta = Math.max(toothDelta, Math.abs(pixels[i + c] - enamel[i + c]))
            }
        }
      }
      renderer.dispose()
    }
    for (const [id, from] of Object.entries(poses) as [string, any][]) {
      const renderer = createCompletedMorph(
        from,
        from,
        p.pairs[`${id}-${id}`].mesh,
        upper,
        lower,
        () => {},
        p.oral,
      )!
      renderer.draw(0)
      // Compare the actual opaque portrait, not unstable straight RGB in
      // nearly transparent texels (the main stage uses this same operation).
      const live = readOverBase(renderer.canvas),
        fallback = readOverBase(await load(from.data.endpoint))
      const mask = read(from.mask)
      for (let i = 0; i < live.length; i++) {
        const difference = Math.abs(live[i] - fallback[i])
        fallbackDelta = Math.max(fallbackDelta, difference)
        if (difference > 1)
          fallbackDifferences.push({
            id,
            x: Math.floor(i / 4) % p.crop.width,
            y: Math.floor(i / 4 / p.crop.width),
            channel: i % 4,
            difference,
            alpha: mask[i - (i % 4)],
            live: live[i],
            fallback: fallback[i],
          })
      }
      renderer.dispose()
    }
    return {
      frames,
      outside,
      endpointDelta,
      lipDelta,
      lipSamples,
      toothDelta,
      toothSamples,
      fallbackDelta,
      fallbackDifferences,
      lowerHasOpaquePixels: lowerPixels.some((v, i) => i % 4 === 3 && v === 255),
    }
  })
  await info.attach('semantic-measurements', {
    body: JSON.stringify(report),
    contentType: 'application/json',
  })
  expect(report.frames).toBe(729)
  expect(report.outside).toBe(0)
  expect(report.endpointDelta).toBe(0)
  expect(report.lipSamples).toBeGreaterThan(10000)
  expect(report.lipDelta).toBeLessThanOrEqual(1)
  expect(report.toothSamples).toBeGreaterThan(100)
  expect(report.toothDelta).toBeLessThanOrEqual(1)
  // Chromium exports and WebKit's premultiplied texture sampling differ by
  // two 8-bit levels in a few shaded edge channels. Exact per-engine endpoint
  // equality and unshaded opaque enamel remain zero/one above; cap BOTH the
  // cross-engine error and its extent instead of accepting a broad mismatch.
  expect(report.fallbackDelta).toBeLessThanOrEqual(2)
  expect(report.fallbackDifferences.length).toBeLessThanOrEqual(32)
  expect(report.lowerHasOpaquePixels).toBe(true)
})

test('dim lower enamel stays opaque during semantic interpolation', async ({ page }) => {
  await page.goto('/?trial=viseme-a&oral=semantic')
  await expect(page.getByRole('button', { name: 'Play', exact: true })).toBeEnabled({
    timeout: 15000,
  })
  const pixels = await page.evaluate(async () => {
    const { posePackage: p } = await import('/src/experiments/viseme-a/usePosePackage.ts' as string)
    const { createCompletedMorph } = await import(
      '/src/experiments/viseme-a/completedMorph.ts' as string
    )
    const image = async (fill?: string, tooth = false) => {
      const c = document.createElement('canvas')
      c.width = p.crop.width
      c.height = p.crop.height
      const ctx = c.getContext('2d')!
      if (fill) {
        ctx.fillStyle = fill
        tooth ? ctx.fillRect(120, 95, 80, 20) : ctx.fillRect(0, 0, c.width, c.height)
      }
      const result = new Image()
      result.src = c.toDataURL()
      await result.decode()
      return result
    }
    const [transparent, dim, surface, mask] = await Promise.all([
      image(),
      image('rgb(30,20,15)', true),
      image('black'),
      image('rgb(255,255,0)'),
    ])
    const pose = (id: string) => ({
      data: { ...p.poses[id], lower_translation: [0, 0] },
      surface,
      cavity: surface,
      mask,
    })
    const renderer = createCompletedMorph(
      pose('A'),
      pose('EI'),
      p.pairs['A-EI'].mesh,
      transparent,
      dim,
      () => {},
      { ...p.oral, contact_shadow_strength: 0, depth_strength: 0 },
    )!
    const samples: number[][] = []
    for (const amount of [0.1, 0.35, 0.65, 0.9]) {
      renderer.draw(amount)
      samples.push(
        Array.from(
          renderer.canvas.getContext('2d')!.getImageData(160, 100, 1, 1).data as Uint8ClampedArray,
        ),
      )
    }
    renderer.dispose()
    return samples
  })
  for (const pixel of pixels) {
    expect(pixel[3]).toBe(255)
    for (let channel = 0; channel < 3; channel++)
      expect(Math.abs(pixel[channel] - [30, 20, 15][channel])).toBeLessThanOrEqual(1)
  }
})
