import { expect, test } from '@playwright/test'

test('preceding completed package and exported fallback share the same FV endpoint', async ({
  page,
}, info) => {
  const read = () =>
    page
      .getByRole('img', { name: 'Character transition', exact: true })
      .evaluate((canvas: HTMLCanvasElement) =>
        Array.from(canvas.getContext('2d')!.getImageData(734, 998, 329, 221).data),
      )
  await page.goto('/?trial=viseme-a&oral=completed')
  await expect(
    page.getByRole('link', { name: 'Compare previous dentition', exact: true }),
  ).toBeVisible()
  await page.getByLabel('To pose', { exact: true }).selectOption('FV')
  await page.getByRole('button', { name: 'FV', exact: true }).click()
  await expect(page.getByTestId('pose')).toHaveText('FV')
  await page
    .getByRole('img', { name: 'Mouth detail', exact: true })
    .screenshot({ path: info.outputPath('live.png') })
  const live = await read()
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
  await page.reload()
  await expect(
    page.getByText('Mouth deformation is unavailable in this browser.', { exact: false }),
  ).toBeVisible()
  await page.getByLabel('To pose', { exact: true }).selectOption('FV')
  await page.getByRole('button', { name: 'FV', exact: true }).click()
  await expect(page.getByTestId('pose')).toHaveText('FV')
  await page
    .getByRole('img', { name: 'Mouth detail', exact: true })
    .screenshot({ path: info.outputPath('exported.png') })
  const exported = await read()
  expect(
    live.reduce((maximum, value, index) => Math.max(maximum, Math.abs(value - exported[index])), 0),
  ).toBeLessThanOrEqual(1)
})

test('fractional jaw translation filters tooth coverage without a black fringe', async ({
  page,
}) => {
  await page.goto('/?trial=viseme-a&oral=completed')
  await expect(page.getByRole('button', { name: 'Play', exact: true })).toBeEnabled()
  const result = await page.evaluate(async () => {
    const { posePackage: p, poseAssetUrl } = await import(
      '/src/experiments/viseme-a/usePosePackage.ts' as string
    )
    const { createCompletedMorph } = await import(
      '/src/experiments/viseme-a/completedMorph.ts' as string
    )
    const image = async (canvas: HTMLCanvasElement) => {
      const value = new Image()
      value.src = canvas.toDataURL()
      await value.decode()
      return value
    }
    const make = () => {
      const canvas = document.createElement('canvas')
      canvas.width = p.crop.width
      canvas.height = p.crop.height
      return canvas
    }
    const dental = make(),
      context = dental.getContext('2d')!
    context.fillStyle = 'rgb(180,150,120)'
    context.fillRect(120, 95, 80, 20)
    const transparent = await image(make()),
      lower = await image(dental)
    const black = make()
    black.getContext('2d')!.fillRect(0, 0, black.width, black.height)
    const tissue = await image(black)
    const mask = new Image()
    mask.src = poseAssetUrl(p.poses.EI.mask)
    await mask.decode()
    const pose = (id: string) => ({ data: p.poses[id], surface: tissue, cavity: tissue, mask })
    const renderer = createCompletedMorph(
      pose('EI'),
      pose('A'),
      p.pairs['EI-A'].mesh,
      transparent,
      lower,
      () => {},
      { ...p.oral, contact_shadow_strength: 0, depth_strength: 0 },
    )
    if (!renderer) throw Error('WebGL required')
    renderer.draw(0.35) // 0.7px lower-row translation: 30% edge coverage.
    const pixels = renderer.canvas.getContext('2d').getImageData(160, 95, 1, 2).data
    renderer.dispose()
    return Array.from(pixels as Uint8ClampedArray)
  })
  for (let c = 0; c < 3; c++) {
    expect(Math.abs(result[c] - [54, 45, 36][c])).toBeLessThanOrEqual(2)
    expect(Math.abs(result[c + 4] - [180, 150, 120][c])).toBeLessThanOrEqual(1)
  }
  expect(result[3]).toBe(255)
  expect(result[7]).toBe(255)
})

test('completed material: nine endpoints and the reported intermediate failures', async ({
  page,
}, info) => {
  const errors: string[] = [],
    posts: string[] = []
  page.on('pageerror', (error) => errors.push(error.message))
  page.on('request', (request) => {
    if (request.method() === 'POST') posts.push(request.url())
  })
  await page.goto('/?trial=viseme-a&oral=completed')
  const detail = page.getByRole('img', { name: 'Mouth detail', exact: true })
  await expect(page.getByRole('button', { name: 'Play', exact: true })).toBeEnabled()
  for (const id of ['Rest', 'A', 'EI', 'O', 'U', 'MBP', 'SZ', 'FV', 'CHSHR']) {
    await page.getByLabel('To pose', { exact: true }).selectOption(id)
    await page.getByRole('button', { name: id, exact: true }).last().click()
    await expect(detail).toHaveScreenshot(`completed-${id}.png`, {
      animations: 'disabled',
      maxDiffPixelRatio: 0.001,
    })
  }
  for (const [a, b] of [
    ['EI', 'FV'],
    ['FV', 'EI'],
    ['FV', 'A'],
    ['A', 'FV'],
    ['EI', 'SZ'],
    ['SZ', 'EI'],
    ['A', 'EI'],
    ['O', 'U'],
    ['MBP', 'A'],
    ['U', 'CHSHR'],
  ]) {
    await page.getByLabel('From pose', { exact: true }).selectOption(a)
    await page.getByLabel('To pose', { exact: true }).selectOption(b)
    const amount = a === 'FV' && b === 'A' ? 0.41 : 0.35
    await page
      .getByRole('slider', { name: 'Mouth blend' })
      .evaluate((input: HTMLInputElement, value) => {
        input.value = String(value)
        input.dispatchEvent(new Event('input', { bubbles: true }))
      }, amount)
    await expect(detail).toHaveScreenshot(`completed-${a}-${b}-${Math.round(amount * 100)}.png`, {
      animations: 'disabled',
      maxDiffPixelRatio: 0.001,
    })
    if (a === 'EI' && b === 'FV')
      await page.screenshot({ path: info.outputPath('ei-fv-35-browser.png'), fullPage: true })
    if (a === 'FV' && b === 'A')
      await page.screenshot({ path: info.outputPath('fv-a-41-browser.png'), fullPage: true })
  }
  await page.getByLabel('Transition', { exact: true }).selectOption('300')
  await page.getByRole('button', { name: 'Play', exact: true }).click()
  await expect.poll(() => page.getByTestId('pose').textContent(), { intervals: [30] }).toBe('CHSHR')
  await page.getByRole('button', { name: 'Pause', exact: true }).click()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  await page.screenshot({ path: info.outputPath('completed-browser.png'), fullPage: true })
  expect(errors).toEqual([])
  expect(posts).toEqual([])
})

test('all directed pairs keep exact endpoints, opaque shared enamel and outside-mask pixels', async ({
  page,
}) => {
  await page.goto('/?trial=viseme-a&oral=completed')
  await expect(page.getByRole('button', { name: 'Play', exact: true })).toBeEnabled()
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
    const [upper, lower] = await Promise.all([load(p.reference.upper), load(p.reference.lower)])
    const read = (image: CanvasImageSource) => {
      const c = document.createElement('canvas')
      c.width = p.crop.width
      c.height = p.crop.height
      c.getContext('2d')!.drawImage(image, 0, 0)
      return c.getContext('2d')!.getImageData(0, 0, c.width, c.height).data
    }
    const poses: Record<string, any> = {},
      endpoints: Record<string, Uint8ClampedArray> = {}
    for (const [id, data] of Object.entries(p.poses) as [string, any][])
      poses[id] = {
        data,
        surface: await load(data.surface),
        cavity: await load(data.cavity),
        mask: await load(data.mask),
      }
    const enamel = read(upper)
    let outside = 0,
      endpointDelta = 0,
      materialError = 0,
      materialSamples = 0,
      frames = 0
    for (const [key, pair] of Object.entries(p.pairs) as [string, any][]) {
      if (!pair.safe) throw Error(`Unsafe ${key}`)
      const [a, b] = key.split('-'),
        from = poses[a],
        to = poses[b]
      const renderer = createCompletedMorph(from, to, pair.mesh, upper, lower, () => {}, {
        ...p.oral,
        contact_shadow_strength: 0,
        depth_strength: 0,
      })
      if (!renderer) throw Error('WebGL required for this test')
      const maskA = read(from.mask),
        maskB = read(to.mask)
      for (const t of [0, 0.01, 0.25, 0.35, 0.5, 0.75, 0.99, 1]) {
        renderer.draw(t)
        const data = read(renderer.canvas)
        frames++
        for (let i = 0; i < data.length; i += 4)
          if (maskA[i] === 0 && maskB[i] === 0 && data[i + 3] !== 0) outside++
        if (t === 0 || t === 1) {
          const id = t === 0 ? a : b
          if (endpoints[id])
            for (let i = 0; i < data.length; i++)
              endpointDelta = Math.max(endpointDelta, Math.abs(data[i] - endpoints[id][i]))
          else endpoints[id] = new Uint8ClampedArray(data)
        }
        if (['EI-FV', 'EI-SZ', 'A-EI'].includes(key) && t === 0.35) {
          const inner = pair.mesh.rest
            .slice(20, 40)
            .map((v: number[], i: number) =>
              v.map((n, axis) => n * (1 - t) + pair.mesh.open[20 + i][axis] * t),
            )
          for (let y = 0; y < p.crop.height; y++)
            for (let x = 0; x < p.crop.width; x++) {
              const i = (y * p.crop.width + x) * 4,
                px = x + 0.5,
                py = y + 0.5
              let inside = false,
                distance = Infinity
              for (let j = 0, k = 19; j < 20; k = j++) {
                const u = inner[j],
                  v = inner[k],
                  dx = v[0] - u[0],
                  dy = v[1] - u[1]
                const f = Math.max(
                  0,
                  Math.min(1, ((px - u[0]) * dx + (py - u[1]) * dy) / (dx * dx + dy * dy)),
                )
                distance = Math.min(distance, Math.hypot(px - u[0] - f * dx, py - u[1] - f * dy))
                if (
                  u[1] > py !== v[1] > py &&
                  px < ((v[0] - u[0]) * (py - u[1])) / (v[1] - u[1]) + u[0]
                )
                  inside = !inside
              }
              if (
                inside &&
                distance > 2 &&
                enamel[i + 3] === 255 &&
                enamel[i - 1] === 255 &&
                enamel[i + 7] === 255 &&
                maskA[i] === 255 &&
                maskB[i] === 255
              ) {
                materialSamples++
                for (let c = 0; c < 3; c++)
                  materialError = Math.max(materialError, Math.abs(data[i + c] - enamel[i + c]))
              }
            }
        }
      }
      renderer.dispose()
    }
    const ei = poses.EI,
      mesh = p.pairs['EI-EI'].mesh
    const lit = createCompletedMorph(ei, ei, mesh, upper, lower, () => {}, p.oral)
    const unlit = createCompletedMorph(ei, ei, mesh, upper, lower, () => {}, {
      ...p.oral,
      contact_shadow_strength: 0,
      depth_strength: 0,
    })
    lit.draw(0)
    unlit.draw(0)
    const lighting = read(lit.canvas),
      flat = read(unlit.canvas)
    let shadowAlphaChanges = 0,
      shadedPixels = 0,
      lightingOvershoot = 0
    for (let i = 0; i < flat.length; i += 4) {
      if (lighting[i + 3] !== flat[i + 3]) shadowAlphaChanges++
      if (flat[i + 3] === 255 && lighting[i] < flat[i] - 1) shadedPixels++
      for (let c = 0; c < 3; c++)
        lightingOvershoot = Math.max(lightingOvershoot, lighting[i + c] - flat[i + c])
    }
    lit.dispose()
    unlit.dispose()
    return {
      outside,
      endpointDelta,
      materialError,
      materialSamples,
      frames,
      shadowAlphaChanges,
      shadedPixels,
      lightingOvershoot,
    }
  })
  expect(report.frames).toBe(648)
  expect(report.outside).toBe(0)
  expect(report.endpointDelta).toBe(0)
  expect(report.materialSamples).toBeGreaterThan(500)
  expect(report.materialError).toBeLessThanOrEqual(1)
  expect(report.shadowAlphaChanges).toBe(0)
  expect(report.shadedPixels).toBeGreaterThan(100)
  expect(report.lightingOvershoot).toBeLessThanOrEqual(1)
})
