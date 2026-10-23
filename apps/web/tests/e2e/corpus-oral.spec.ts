import { expect, test } from '@playwright/test'

for (const character of ['syntelle', 'cafe', 'woodland']) {
  test(`${character}: shared material, all directed transitions and live controls`, async ({
    page,
  }, info) => {
    test.setTimeout(60_000)
    await page.goto(`/?trial=viseme-a&oral=semantic&character=${character}`)
    await expect(page.getByRole('button', { name: 'Play', exact: true })).toBeEnabled()
    await expect(
      page.getByText(`Development character: ${character}`, { exact: false }),
    ).toBeVisible()
    const measurements = await page.evaluate(async () => {
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
      const pixels = (image: CanvasImageSource) => {
        const canvas = document.createElement('canvas')
        canvas.width = p.crop.width
        canvas.height = p.crop.height
        const ctx = canvas.getContext('2d')!
        ctx.drawImage(image, 0, 0)
        return ctx.getImageData(0, 0, canvas.width, canvas.height).data
      }
      const [upper, lower] = await Promise.all([load(p.reference.upper), load(p.reference.lower)])
      const enamel = pixels(upper)
      const poses: Record<string, any> = {}
      for (const [id, data] of Object.entries(p.poses) as [string, any][]) {
        poses[id] = {
          data,
          surface: await load(data.surface),
          cavity: await load(data.cavity),
          mask: await load(data.mask),
        }
      }
      const endpoints: Record<string, Uint8ClampedArray> = {}
      let frames = 0,
        endpointDelta = 0,
        outsideAlpha = 0,
        textureDelta = 0,
        enamelSamples = 0
      for (const [key, pair] of Object.entries(p.pairs) as [string, any][]) {
        if (!pair.safe) throw Error(`Invalid geometry: ${key}`)
        const [a, b] = key.split('-'),
          from = poses[a],
          to = poses[b]
        const renderer = createCompletedMorph(from, to, pair.mesh, upper, lower, () => {}, {
          ...p.oral,
          contact_shadow_strength: 0,
          depth_strength: 0,
        })
        if (!renderer) throw Error('WebGL unavailable')
        const maskA = pixels(from.mask),
          maskB = pixels(to.mask)
        for (const amount of [0, 0.01, 0.25, 0.35, 0.41, 0.5, 0.75, 0.99, 1]) {
          renderer.draw(amount)
          const actual = pixels(renderer.canvas)
          frames++
          for (let i = 0; i < actual.length; i += 4) {
            if (maskA[i] === 0 && maskB[i] === 0)
              outsideAlpha = Math.max(outsideAlpha, actual[i + 3])
            // Stable exposed enamel at both poses cannot disappear in between.
            if (
              maskA[i] === 255 &&
              maskB[i] === 255 &&
              maskA[i + 1] >= 152 &&
              maskB[i + 1] >= 152 &&
              enamel[i + 3] === 255
            ) {
              enamelSamples++
              for (let c = 0; c < 3; c++)
                textureDelta = Math.max(textureDelta, Math.abs(actual[i + c] - enamel[i + c]))
            }
          }
          if (amount === 0 || amount === 1) {
            const id = amount === 0 ? a : b
            if (endpoints[id])
              for (let i = 0; i < actual.length; i++)
                endpointDelta = Math.max(endpointDelta, Math.abs(actual[i] - endpoints[id][i]))
            else endpoints[id] = new Uint8ClampedArray(actual)
          }
        }
        renderer.dispose()
      }
      return {
        frames,
        endpointDelta,
        outsideAlpha,
        textureDelta,
        enamelSamples,
        fv: p.poses.FV.preparation,
      }
    })
    await info.attach('technical-measurements-not-visual-approval', {
      body: JSON.stringify(measurements),
      contentType: 'application/json',
    })
    expect(measurements.frames).toBe(729)
    expect(measurements.endpointDelta).toBe(0)
    expect(measurements.outsideAlpha).toBe(0)
    expect(measurements.enamelSamples).toBeGreaterThan(0)
    expect(measurements.textureDelta).toBeLessThanOrEqual(1)
    expect(measurements.fv.ready).toBe(true)
    await page.getByLabel('To pose', { exact: true }).selectOption('FV')
    await page.getByRole('button', { name: 'FV', exact: true }).click()
    await expect(page.getByTestId('pose')).toHaveText('FV')
    await page.getByText('Compare preparation · FV', { exact: true }).click()
    await expect(
      page.getByRole('img', { name: 'Original generated mouth', exact: true }),
    ).toBeVisible()
    await expect(
      page.getByRole('img', { name: 'Current target mouth endpoint', exact: true }),
    ).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
    await page.screenshot({ path: info.outputPath(`${character}-fv.png`), fullPage: true })
  })
}
