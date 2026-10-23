import { expect, test } from '@playwright/test'
import { readFile, writeFile } from 'node:fs/promises'

test('gaze composition clears vacated iris pixels without borrowing skin colors', async ({
  page,
}) => {
  await page.goto('/?view=validation')
  const result = await page.evaluate(async () => {
    const { createEyeMorph } = await import(
      '/src/features/character/rendering/eyeMorph.ts' as string
    )
    const width = 100,
      height = 40,
      source = new Uint8ClampedArray(width * height * 4),
      ownership = new Uint8ClampedArray(source.length),
      plate = new Uint8ClampedArray(source.length)
    for (let y = 0; y < height; y++)
      for (let x = 0; x < width; x++) {
        const i = (y * width + x) * 4,
          inside = x >= 3 && x < 47 && y >= 5 && y < 35,
          iris = Math.max(0, Math.min(1, 8.5 - Math.hypot(x - 25, y - 25)))
        source.set(
          inside
            ? [
                200 * (1 - iris) + 10 * iris,
                200 * (1 - iris) + 20 * iris,
                200 * (1 - iris) + 30 * iris,
                255,
              ]
            : [250, 20, 20, 255],
          i,
        )
        ownership.set([inside ? 255 : 0, Math.round(iris * 255), inside ? 255 : 0, 255], i)
        plate.set(inside ? [200, 200, 200, 255] : [250, 20, 20, 255], i)
      }
    const image = (bytes: Uint8ClampedArray<ArrayBuffer>): Promise<HTMLImageElement> =>
      new Promise((resolve) => {
        const c = document.createElement('canvas')
        c.width = width
        c.height = height
        c.getContext('2d')!.putImageData(new ImageData(bytes, width, height), 0, 0)
        const image = new Image()
        image.onload = () => resolve(image)
        image.src = c.toDataURL()
      })
    const [opened, mask, sclera, own] = await Promise.all([
      image(source),
      image(ownership),
      image(plate),
      image(ownership),
    ])
    const eye = createEyeMorph(
      opened,
      opened,
      mask,
      Array.from({ length: width }, () => [0, 0, 0, 0]),
      () => {
        throw Error('Lost synthetic eye renderer')
      },
      {
        package: { eyes: [{ center: [25, 25] }, { center: [75, 25] }] },
        images: { sclera, ownership: own },
      },
    )
    const read = () => {
      const c = document.createElement('canvas')
      c.width = width
      c.height = height
      const ctx = c.getContext('2d')!
      ctx.drawImage(eye.canvas, 0, 0)
      return ctx.getImageData(0, 0, width, height).data
    }
    eye.draw(0, [
      [0, 0],
      [0, 0],
    ])
    const original = read()
    eye.draw(0, [
      [0.0001, 0],
      [0, 0],
    ])
    const almostNeutral = read()
    const neutralEdgeDifference = Math.max(
      ...original.map((value, i) => Math.abs(value - almostNeutral[i]!)),
    )
    eye.draw(0, [
      [0, -4],
      [0, 0],
    ])
    const frame = read(),
      p = (32 * width + 25) * 4
    let previous: Uint8ClampedArray | null = null,
      maxStep = 0,
      skinLeak = 0
    for (let shift = 0.1; shift < 0.9; shift += 0.05) {
      eye.draw(0, [
        [0, -shift],
        [0, 0],
      ])
      const pixels = read()
      for (let y = 5; y < 35; y++)
        for (let x = 3; x < 47; x++) {
          const i = (y * width + x) * 4
          if (pixels[i]! > 220 && pixels[i + 1]! < 80) skinLeak++
          if (previous)
            for (let channel = 0; channel < 3; channel++)
              maxStep = Math.max(maxStep, Math.abs(pixels[i + channel]! - previous[i + channel]!))
        }
      previous = pixels
    }
    eye.dispose()
    return { vacated: Array.from(frame.slice(p, p + 3)), maxStep, skinLeak, neutralEdgeDifference }
  })
  expect(result.vacated).toEqual([200, 200, 200])
  expect(result.skinLeak).toBe(0)
  expect(result.maxStep).toBeLessThan(20)
  expect(result.neutralEdgeDifference).toBeLessThanOrEqual(1)
})

test('gaze boundary review covers diagonal offsets and partial blinks', async ({ page }, info) => {
  await page.goto('/?view=validation')
  const sheets = await page.evaluate(async () => {
    const { loadDemoCharacter } = await import('/src/features/demo/catalog.ts' as string)
    const { createEyeMorph } = await import(
      '/src/features/character/rendering/eyeMorph.ts' as string
    )
    const load = (url: string): Promise<HTMLImageElement> =>
      new Promise((resolve, reject) => {
        const image = new Image()
        image.onload = () => resolve(image)
        image.onerror = reject
        image.src = url
      })
    const output = []
    for (const id of ['syntelle', 'cafe', 'woodland']) {
      const context = await loadDemoCharacter(id, 'attention'),
        pkg = context.motion.definition!,
        gaze = context.motion.attention!
      const [opened, closed, mask, sclera, ownership] = await Promise.all(
        [pkg.eyes.open, pkg.eyes.closed, pkg.eyes.mask, 'sclera.png', 'ownership.png'].map((name) =>
          load(context.motion.assetUrl(name)),
        ),
      )
      const eye = createEyeMorph(
        opened!,
        closed!,
        mask!,
        pkg.eyes.curves,
        () => {
          throw Error('Lost eye renderer')
        },
        { package: gaze, images: { sclera: sclera!, ownership: ownership! } },
      )
      const width = 600,
        height = Math.round((opened!.height * width) / opened!.width)
      const sheet = document.createElement('canvas')
      sheet.width = width * 2
      sheet.height = (height + 24) * 4
      const ctx = sheet.getContext('2d')!
      ctx.fillStyle = '#242229'
      ctx.fillRect(0, 0, sheet.width, sheet.height)
      const frames = [
        ['Original', 0, 0, 0],
        ['Left', -1, 0, 0],
        ['Right', 1, 0, 0],
        ['Up', 0, -1, 0],
        ['Down', 0, 1, 0],
        ['Diagonal', -0.707, 0.707, 0],
        ['Diagonal + partial blink', 0.707, -0.707, 0.45],
        ['Closed', 1, 1, 1],
      ] as const
      frames.forEach(([name, dx, dy, blink], i) => {
        const offsets = gaze.eyes.map(
          (e: { bounds_px: number[][] }) =>
            [dx * e.bounds_px[0]![dx < 0 ? 0 : 1]!, dy * e.bounds_px[1]![dy < 0 ? 0 : 1]!] as const,
        )
        eye.draw(blink, offsets)
        const x = (i % 2) * width,
          y = Math.floor(i / 2) * (height + 24)
        ctx.fillStyle = 'white'
        ctx.fillText(id + ' · ' + name, x + 8, y + 16)
        ctx.drawImage(eye.canvas, x, y + 24, width, height)
      })
      eye.dispose()
      output.push({ id, image: sheet.toDataURL() })
    }
    return output
  })
  for (const sheet of sheets) {
    const path = info.outputPath(`${sheet.id}-gaze-boundaries.png`)
    await writeFile(path, Buffer.from(sheet.image.split(',')[1]!, 'base64'))
    await info.attach(sheet.id, { path, contentType: 'image/png' })
  }
})

test('attention follows diagonal, deterministic, bounded trajectories and reserves nod space', async ({
  page,
}, info) => {
  await page.goto('/?view=validation')
  const results = await page.evaluate(async () => {
    const { attentionAt, createAttentionDirector, gazeOffsets } = await import(
      '/src/features/character/animation/attention.ts' as string
    )
    const { loadDemoCharacter } = await import('/src/features/demo/catalog.ts' as string)
    const rows = []
    for (const id of ['syntelle', 'cafe', 'woodland']) {
      const context = await loadDemoCharacter(id, 'attention'),
        d = context.motion.definition
      const seed = d.animation_seed,
        bounds = d.projection.bounds,
        gaze = context.motion.attention
      const director = createAttentionDirector(seed, bounds)
      let previous = attentionAt(0, seed).head,
        maxVelocity = 0,
        diagonal = false,
        glances = 0
      let offCamera = false,
        maxGlance = 0,
        maxPitch = 0
      for (let ms = 0; ms < 120000; ms += 20) {
        const a = attentionAt(ms, seed)
        if (JSON.stringify(a) !== JSON.stringify(attentionAt(ms, seed)))
          throw Error('Non deterministic')
        if (ms < 6000 && Math.abs(a.head.yaw) > 1 && Math.abs(a.head.pitch) > 0.35) diagonal = true
        const lookingAway = Math.abs(a.glance[0]) > 0.01
        if (lookingAway && !offCamera) glances++
        offCamera = lookingAway
        maxGlance = Math.max(maxGlance, Math.abs(a.glance[0]))
        const speech = {
          active: ms >= 15000 && ms < 20000,
          position: (ms - 15000) / 1000,
          session: 1,
          plan: {
            version: 'speech-context/1',
            duration: 5,
            cues: [],
            gestures: [{ time: 1, duration: 1.4, pitch: -1.1, yaw: 0.2, kind: 'ending' }],
          },
        }
        const frame = director.sample(ms, speech)
        for (const axis of ['yaw', 'pitch', 'roll']) {
          if (Math.abs(frame.head[axis]) > bounds[axis] + 1e-8) throw Error('Envelope exceeded')
          maxVelocity = Math.max(maxVelocity, Math.abs(frame.head[axis] - previous[axis]) / 0.02)
        }
        maxPitch = Math.max(maxPitch, Math.abs(frame.head.pitch))
        const offsets = gazeOffsets(gaze, frame.head, frame.glance)
        for (let i = 0; i < 2; i++)
          for (let axis = 0; axis < 2; axis++) {
            const v = offsets[i][axis],
              limit = gaze.eyes[i].bounds_px[axis][v < 0 ? 0 : 1]
            if (Math.abs(v) > limit + 1e-8) throw Error('Gaze outside source support')
          }
        if (offsets[0][0] * offsets[1][0] < 0 || offsets[0][1] * offsets[1][1] < 0)
          throw Error('Crossed eyes')
        previous = frame.head
      }
      rows.push({ id, diagonal, glances, maxGlance, maxVelocity, maxPitch })
    }
    return rows
  })
  for (const row of results) {
    expect(row.diagonal).toBe(true)
    expect(row.glances).toBeGreaterThan(7)
    expect(row.glances).toBeLessThan(16)
    expect(row.maxVelocity).toBeLessThan(5)
    expect(row.maxPitch).toBeGreaterThan(0.6)
  }
  await info.attach('attention-trajectories', {
    body: JSON.stringify(results, null, 2),
    contentType: 'application/json',
  })
})

test('gaze shader preserves neutral, fully closed, pupil texture and the eye boundary', async ({
  page,
}, info) => {
  await page.goto('/?view=validation')
  const result = await page.evaluate(async () => {
    const { loadDemoCharacter } = await import('/src/features/demo/catalog.ts' as string)
    const { createEyeMorph } = await import(
      '/src/features/character/rendering/eyeMorph.ts' as string
    )
    const load = (url: string): Promise<HTMLImageElement> =>
      new Promise((resolve, reject) => {
        const image = new Image()
        image.onload = () => resolve(image)
        image.onerror = reject
        image.src = url
      })
    const rows = [],
      sheet = document.createElement('canvas')
    sheet.width = 600
    sheet.height = 1500
    const ctx = sheet.getContext('2d')!
    ctx.fillStyle = '#242229'
    ctx.fillRect(0, 0, 600, 1500)
    let y = 0
    for (const id of ['syntelle', 'cafe', 'woodland']) {
      const context = await loadDemoCharacter(id, 'attention'),
        d = context.motion.definition,
        gaze = context.motion.attention
      const [opened, closed, mask, sclera, ownership] = await Promise.all(
        [d.eyes.open, d.eyes.closed, d.eyes.mask, 'sclera.png', 'ownership.png'].map((name) =>
          load(context.motion.assetUrl(name)),
        ),
      )
      const callback = () => {
        throw Error('WebGL context lost')
      }
      const eye = createEyeMorph(opened, closed, mask, d.eyes.curves, callback, {
        package: gaze,
        images: { sclera, ownership },
      })
      const old = createEyeMorph(opened, closed, mask, d.eyes.curves, callback)
      const read = (canvas: HTMLCanvasElement | HTMLImageElement) => {
        const copy = document.createElement('canvas')
        copy.width = canvas.width
        copy.height = canvas.height
        const c = copy.getContext('2d')!
        c.drawImage(canvas, 0, 0)
        return c.getImageData(0, 0, copy.width, copy.height).data
      }
      const offsets = gaze.eyes.map((e: { width_px: number }) => [e.width_px * 0.025, 0])
      eye.draw(0)
      old.draw(0)
      const neutral = read(eye.canvas),
        original = read(old.canvas)
      const exactNeutral = neutral.every((v, i) => v === original[i])
      eye.draw(1, offsets)
      old.draw(1)
      const closedData = read(eye.canvas),
        closedOriginal = read(old.canvas)
      const exactClosed = closedData.every((v, i) => v === closedOriginal[i])
      const own = read(ownership)
      const pupil = gaze.eyes[0].center
      const p = (Math.round(pupil[1]) * opened.width + Math.round(pupil[0])) * 4
      let changed = 0,
        outside = 0
      eye.draw(0, offsets)
      const shifted = read(eye.canvas)
      for (let i = 0; i < neutral.length; i += 4) {
        const difference = Math.max(
          ...[0, 1, 2].map((c) => Math.abs(shifted[i + c] - neutral[i + c])),
        )
        if (difference > 0) changed++
        if (own[i] === 0 && difference > 0) outside++
      }
      // The pupil is translated from the same source sample, not scaled or dissolved.
      const x = Math.round(pupil[0] + offsets[0][0]),
        yy = Math.round(pupil[1])
      const q = (yy * opened.width + x) * 4
      const pupilDifference = Math.max(
        ...[0, 1, 2].map((c) => Math.abs(shifted[q + c] - original[p + c])),
      )
      ctx.fillStyle = 'white'
      ctx.fillText(id + ': original / shifted / partial blink', 8, y + 14)
      const height = Math.round((600 * opened.height) / opened.width)
      old.draw(0)
      ctx.drawImage(old.canvas, 0, y + 18, 600, height)
      ctx.drawImage(eye.canvas, 0, y + 20 + height, 600, height)
      eye.draw(0.45, offsets)
      ctx.drawImage(eye.canvas, 0, y + 22 + height * 2, 600, height)
      rows.push({ id, exactNeutral, exactClosed, changed, outside, pupilDifference })
      eye.dispose()
      old.dispose()
      y += 26 + height * 3
    }
    const cropped = document.createElement('canvas')
    cropped.width = sheet.width
    cropped.height = y
    cropped.getContext('2d')!.drawImage(sheet, 0, 0)
    return { rows, sheet: cropped.toDataURL() }
  })
  for (const row of result.rows) {
    expect(row.exactNeutral).toBe(true)
    expect(row.exactClosed).toBe(true)
    expect(row.outside).toBe(0)
    expect(row.changed).toBeGreaterThan(20)
    expect(row.pupilDifference).toBeLessThan(25)
  }
  await info.attach('gaze-contact-sheet', {
    path: await (async () => {
      const path = info.outputPath('gaze-contact-sheet.png')
      await writeFile(path, Buffer.from(result.sheet.split(',')[1]!, 'base64'))
      return path
    })(),
    contentType: 'image/png',
  })
  await info.attach('gaze-invariants', {
    body: JSON.stringify(result.rows, null, 2),
    contentType: 'application/json',
  })
})

test('cached Dragon speech keeps articulation, gaze and cancellation working', async ({
  page,
}, info) => {
  test.skip(
    info.project.name !== 'chromium-desktop',
    'One actual cached-audio replay; Windows WebKit has no Web Audio.',
  )
  const response = JSON.parse(
    await readFile(
      new URL('../../../../.tools/head-motion-12/speech/ava-en/response.json', import.meta.url),
      'utf8',
    ),
  )
  let requests = 0
  const errors: string[] = []
  page.on('pageerror', (e) => errors.push(e.message))
  await page.route('**/api/v1/speech/status', (route) =>
    route.fulfill({ json: { configured: true } }),
  )
  await page.route('**/api/v1/speech/synthesize', (route) => {
    requests++
    return route.fulfill({ json: response })
  })
  await page.goto('/?idle=attention')
  await page.getByRole('button', { name: 'Read aloud', exact: true }).click()
  await expect(page.getByRole('region', { name: 'Syntelle character preview' })).toHaveAttribute(
    'aria-busy',
    'false',
  )
  await page
    .getByLabel('Text to speak', { exact: true })
    .fill('Offline replay of the archived Ava recording.')
  await page.getByRole('button', { name: 'Speak', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Pause', exact: true })).toBeVisible()
  await page.waitForTimeout(1500)
  await page.screenshot({ path: info.outputPath('cached-dragon-speech.png'), fullPage: true })
  await page.getByRole('button', { name: 'Stop', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Pause', exact: true })).toHaveCount(0)
  await page.getByRole('button', { name: 'Speak', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Pause', exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Chat', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Pause', exact: true })).toHaveCount(0)
  expect(requests).toBe(1)
  expect(errors).toEqual([])
  await info.attach('cached-speech', {
    body: JSON.stringify({
      voice: response.voice,
      duration_ms: response.duration_ms,
      native_events: response.visemes.length,
      intercepted_requests: requests,
      provider_requests: 0,
    }),
    contentType: 'application/json',
  })
})

test('short serial WebKit frame-cadence comparison', async ({ page }, info) => {
  test.skip(
    info.project.name !== 'webkit-desktop' || process.env.MEASURE_ATTENTION !== '1',
    'Opt-in bounded serial comparison.',
  )
  const rows = []
  await page.route('**/api/v1/speech/status', (route) =>
    route.fulfill({ json: { configured: true } }),
  )
  for (const variant of ['previous', 'attention']) {
    await page.goto(`/?idle=${variant}`)
    await expect(page.getByRole('region', { name: 'Syntelle character preview' })).toHaveAttribute(
      'aria-busy',
      'false',
    )
    const timings = await page.evaluate(
      () =>
        new Promise<number[]>((resolve) => {
          const samples: number[] = []
          let previous = 0,
            start = 0
          function frame(time: number) {
            if (!start) start = time
            if (previous && time - start > 500) samples.push(time - previous)
            previous = time
            if (time - start >= 4500) resolve(samples)
            else requestAnimationFrame(frame)
          }
          requestAnimationFrame(frame)
        }),
    )
    rows.push({
      variant,
      frames: timings.length,
      fps: 1000 / (timings.reduce((a, b) => a + b, 0) / timings.length),
    })
  }
  await info.attach('serial-webkit-cadence', {
    body: JSON.stringify(rows, null, 2),
    contentType: 'application/json',
  })
  expect(rows[1]!.fps).toBeGreaterThan(rows[0]!.fps * 0.9)
})

test('candidate loads on Syntelle, freezes movement and respects reduced motion', async ({
  page,
}, info) => {
  const errors: string[] = [],
    posts: string[] = []
  page.on('pageerror', (e) => errors.push(e.message))
  page.on('request', (r) => {
    if (r.method() === 'POST') posts.push(r.url())
  })
  await page.route('**/api/v1/speech/status', (route) =>
    route.fulfill({ json: { configured: true } }),
  )
  await page.goto('/?idle=attention')
  await page.getByRole('button', { name: 'Read aloud', exact: true }).click()
  for (const name of ['Syntelle']) {
    await expect(page.getByRole('region', { name: `${name} character preview` })).toHaveAttribute(
      'aria-busy',
      'false',
    )
  }
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  await page.getByLabel('Blinking', { exact: true }).uncheck()
  await page.getByLabel('Natural movement', { exact: true }).uncheck()
  await page.getByRole('button', { name: 'Close customization' }).click()
  await page.waitForTimeout(100)
  const frame = page.getByRole('img', { name: 'Animated character', exact: true })
  const frozen = await frame.evaluate((c: HTMLCanvasElement) => c.toDataURL())
  await page.waitForTimeout(250)
  expect(await frame.evaluate((c: HTMLCanvasElement) => c.toDataURL())).toBe(frozen)
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await expect(page.getByRole('region', { name: 'Syntelle character preview' })).toHaveAttribute(
    'aria-busy',
    'false',
  )
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  expect(errors).toEqual([])
  expect(posts).toEqual([])
  await page.screenshot({ path: info.outputPath('attention-workspace.png'), fullPage: true })
})
