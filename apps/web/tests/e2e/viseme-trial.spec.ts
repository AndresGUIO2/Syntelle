import { expect, test } from '@playwright/test'
import { readFileSync } from 'node:fs'

test('automatic dental source selection identifies the Argon contour composition trial', async ({
  page,
}) => {
  const posts: string[] = []
  page.on('request', (request) => {
    if (request.method() === 'POST') posts.push(request.url())
  })
  await page.goto('/?trial=viseme-a&oral=previous')
  const summary = page.getByTestId('dental-reference')
  await expect(summary).toHaveText(
    'Dental reference: EI · automatically selected from 8 original poses',
  )
  await summary.click()
  const source = page.getByRole('img', { name: 'Original EI dental source', exact: true })
  const masks = page.getByRole('img', { name: 'Argon dental and tongue contours', exact: true })
  for (const image of [source, masks]) {
    await expect(image).toBeVisible()
    await expect
      .poll(() =>
        image.evaluate((element: HTMLImageElement) => element.complete && element.naturalWidth > 0),
      )
      .toBe(true)
  }
  await expect(page.getByRole('row', { name: /^EI / })).toContainText('Selected')
  await expect(page.getByRole('row', { name: /^A / })).toContainText('Eligible alternative')
  await expect(page.getByRole('row', { name: /^FV / })).toContainText(
    'cannot be separated confidently',
  )
  await expect(page.getByRole('row', { name: /^SZ / })).toContainText('central gap')
  await expect(page.getByRole('row')).toHaveCount(9)
  await expect(page.getByText(/Argon contour trial/)).toBeVisible()
  await expect(page.getByText(/visible tongue in orange/)).toBeVisible()
  expect(await masks.getAttribute('src')).toMatch(/[?&]v=[a-f0-9]{64}$/)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  expect(posts).toEqual([])
})

test('viseme endpoints preserve outside pixels and playback stays in place', async ({
  page,
}, info) => {
  const errors: string[] = []
  const paidRequests: string[] = []
  page.on('pageerror', (error) => errors.push(error.message))
  page.on('request', (request) => {
    if (request.method() === 'POST') paidRequests.push(request.url())
  })
  await page.goto('/?trial=viseme-a&oral=previous')
  const portrait = page.getByRole('img', { name: 'Character transition', exact: true })
  await expect(portrait).toBeVisible()
  const layout = () =>
    portrait.evaluate((element) => {
      const rect = element.getBoundingClientRect()
      const parent = element.parentElement!.getBoundingClientRect()
      return { x: rect.x - parent.x, y: rect.y - parent.y, width: rect.width, height: rect.height }
    })
  const origin = await layout()
  await portrait.evaluate((element: HTMLCanvasElement) => {
    const context = element.getContext('2d')!
    const data = context.getImageData(0, 0, element.width, element.height)
    ;(window as any).__restPixels = data.data
  })
  await page.getByRole('button', { name: 'A', exact: true }).click()
  await expect(page.getByTestId('pose')).toHaveText('A')
  const changes = await portrait.evaluate((element: HTMLCanvasElement) => {
    const before: Uint8ClampedArray = (window as any).__restPixels
    const after = element.getContext('2d')!.getImageData(0, 0, element.width, element.height).data
    let outside = 0,
      inside = 0
    for (let y = 0; y < element.height; y++) {
      for (let x = 0; x < element.width; x++) {
        const i = (y * element.width + x) * 4
        if (
          after[i] !== before[i] ||
          after[i + 1] !== before[i + 1] ||
          after[i + 2] !== before[i + 2]
        ) {
          if (x >= 740 && x < 1060 && y >= 998 && y < 1208) inside++
          else outside++
        }
      }
    }
    return { outside, inside }
  })
  expect(changes.outside).toBe(0)
  expect(changes.inside).toBeGreaterThan(1000)
  expect(await layout()).toEqual(origin)
  await page.screenshot({ path: info.outputPath('viseme-a.png'), fullPage: true })

  await page.getByRole('slider', { name: 'Mouth blend' }).evaluate((element: HTMLInputElement) => {
    element.value = '0.5'
    element.dispatchEvent(new Event('input', { bubbles: true }))
  })
  await expect(page.getByTestId('pose')).toHaveText('Transition')
  await page
    .getByRole('img', { name: 'Mouth detail', exact: true })
    .screenshot({ path: info.outputPath('blend-50.png') })
  for (const transition of ['0', '120', '300']) {
    await page.getByLabel('Transition', { exact: true }).selectOption(transition)
    await page.getByRole('button', { name: 'Rest', exact: true }).click()
    await page.getByRole('button', { name: 'Play', exact: true }).click()
    // Poll faster than the 500 ms hold: default assertion backoff can sample
    // the same phase of a periodic loop repeatedly and miss the other pose.
    await expect.poll(() => page.getByTestId('pose').textContent(), { intervals: [50] }).toBe('A')
    await expect
      .poll(() => page.getByTestId('pose').textContent(), { intervals: [50] })
      .toBe('Rest')
    await page.getByRole('button', { name: 'Pause', exact: true }).click()
    await expect(page.getByRole('button', { name: 'Play', exact: true })).toBeVisible()
    expect(await layout()).toEqual(origin)
  }
  await page.getByLabel('Show crop boundary').check()
  await expect(page.getByLabel('Show crop boundary')).toBeChecked()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  expect(errors).toEqual([])
  expect(paidRequests).toEqual([])
})

test('viseme reduced-motion preference keeps playback manual', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.goto('/?trial=viseme-a&oral=previous')
  await expect(page.getByRole('button', { name: 'Play', exact: true })).toBeDisabled()
  await page.getByRole('button', { name: 'A', exact: true }).click()
  await expect(page.getByTestId('pose')).toHaveText('A')
  await page.getByRole('button', { name: 'Rest', exact: true }).click()
  await expect(page.getByTestId('pose')).toHaveText('Rest')
  await page.emulateMedia({ reducedMotion: 'no-preference' })
  await expect(page.getByRole('button', { name: 'Play', exact: true })).toBeEnabled()
})

test('viseme image load failure leaves a readable error', async ({ page }) => {
  await page.route('**/base.webp*', (route) =>
    route.request().resourceType() === 'image' ? route.abort() : route.continue(),
  )
  await page.goto('/?trial=viseme-a&oral=previous')
  await expect(page.getByRole('alert')).toContainText('trial images could not load')
  await expect(page.getByRole('button', { name: 'Play', exact: true })).toHaveCount(0)
})

test('landmark morph keeps endpoints exact and protects the fixed alpha mask between them', async ({
  page,
}, info) => {
  await page.goto('/?trial=viseme-a&oral=previous')
  const portrait = page.getByRole('img', { name: 'Character transition', exact: true })
  await expect(portrait).toBeVisible()
  await expect(page.getByLabel('Method', { exact: true })).toHaveValue('morph')
  const alphaUrl = `data:image/webp;base64,${readFileSync(new URL('../../src/experiments/viseme-a/mouth-a.webp', import.meta.url)).toString('base64')}`
  await portrait.evaluate(async (element: HTMLCanvasElement, url: string) => {
    const mask = new Image()
    mask.src = url
    await mask.decode()
    const canvas = document.createElement('canvas')
    canvas.width = 320
    canvas.height = 210
    const context = canvas.getContext('2d')!
    context.drawImage(mask, 0, 0)
    ;(window as any).__mask = context.getImageData(0, 0, 320, 210).data
    ;(window as any).__base = element.getContext('2d')!.getImageData(0, 0, 1792, 2400).data
  }, alphaUrl)
  for (const value of ['0.01', '0.25', '0.5', '0.75', '0.99']) {
    await page
      .getByRole('slider', { name: 'Mouth blend' })
      .evaluate((element: HTMLInputElement, value: string) => {
        element.value = value
        element.dispatchEvent(new Event('input', { bubbles: true }))
      }, value)
    await expect(page.getByTestId('pose')).toHaveText('Transition')
    const check = await portrait.evaluate((element: HTMLCanvasElement) => {
      const pixels = element.getContext('2d')!.getImageData(0, 0, 1792, 2400).data
      const base: Uint8ClampedArray = (window as any).__base
      const mask: Uint8ClampedArray = (window as any).__mask
      let changedOutsideMask = 0,
        transparent = 0,
        changedInside = 0
      for (let y = 0; y < 2400; y++)
        for (let x = 0; x < 1792; x++) {
          const i = (y * 1792 + x) * 4
          const alpha =
            x >= 740 && x < 1060 && y >= 998 && y < 1208
              ? mask[((y - 998) * 320 + (x - 740)) * 4 + 3]
              : 0
          const changed =
            pixels[i] !== base[i] || pixels[i + 1] !== base[i + 1] || pixels[i + 2] !== base[i + 2]
          if (changed && alpha === 0) changedOutsideMask++
          if (changed && alpha > 0) changedInside++
          if (pixels[i + 3] !== 255) transparent++
        }
      return { changedOutsideMask, transparent, changedInside }
    })
    expect(check.changedOutsideMask).toBe(0)
    expect(check.transparent).toBe(0)
    expect(check.changedInside).toBeGreaterThan(100)
    if (value === '0.5')
      await page
        .getByRole('img', { name: 'Mouth detail', exact: true })
        .screenshot({ path: info.outputPath('morph-50.png') })
  }
  for (const endpoint of ['Rest', 'A']) {
    await page.getByRole('button', { name: endpoint, exact: true }).click()
    await portrait.evaluate((element: HTMLCanvasElement) => {
      ;(window as any).__morphEndpoint = element
        .getContext('2d')!
        .getImageData(0, 0, 1792, 2400).data
    })
    await page.getByLabel('Method', { exact: true }).selectOption('crossfade')
    const identical = await portrait.evaluate((element: HTMLCanvasElement) => {
      const actual = element.getContext('2d')!.getImageData(0, 0, 1792, 2400).data
      return actual.every((value, i) => value === (window as any).__morphEndpoint[i])
    })
    expect(identical).toBe(true)
    await page.getByLabel('Method', { exact: true }).selectOption('morph')
  }
  await page.getByRole('slider', { name: 'Mouth blend' }).evaluate((element: HTMLInputElement) => {
    element.value = '0.5'
    element.dispatchEvent(new Event('input', { bubbles: true }))
  })
  await page.getByLabel('Method', { exact: true }).selectOption('crossfade')
  await page
    .getByRole('img', { name: 'Mouth detail', exact: true })
    .screenshot({ path: info.outputPath('crossfade-50.png') })
})

test('missing WebGL leaves the crossfade controls usable', async ({ page }) => {
  await page.addInitScript(() => {
    const original = HTMLCanvasElement.prototype.getContext
    HTMLCanvasElement.prototype.getContext = function (
      this: HTMLCanvasElement,
      type: string,
      ...args: unknown[]
    ) {
      if (type === 'webgl') return null
      return (original as any).call(this, type, ...args)
    } as typeof original
  })
  await page.goto('/?trial=viseme-a&oral=previous')
  await expect(page.getByLabel('Method', { exact: true })).toHaveValue('crossfade')
  await expect(page.getByRole('status')).toContainText('deformation is unavailable')
  await page.getByRole('button', { name: 'A', exact: true }).click()
  await expect(page.getByTestId('pose')).toHaveText('A')
})

test('visible upper teeth retain their reference pixels while the lips open', async ({
  page,
}, info) => {
  await page.goto('/?trial=viseme-a&oral=previous')
  const portrait = page.getByRole('img', { name: 'Character transition', exact: true })
  await expect(portrait).toBeVisible()
  await expect(page.getByLabel('Method', { exact: true })).toHaveValue('morph')
  const textureUrl = `data:image/webp;base64,${readFileSync(new URL('../../src/experiments/viseme-a/poses/upper-teeth.webp', import.meta.url)).toString('base64')}`
  const expected = await page.evaluate(async (url: string) => {
    const image = new Image()
    image.src = url
    await image.decode()
    const canvas = document.createElement('canvas')
    canvas.width = image.naturalWidth
    canvas.height = image.naturalHeight
    const context = canvas.getContext('2d')!
    context.drawImage(image, 0, 0)
    // Reviewed contours make y=82 an antialiased tooth edge. Sample the fully
    // opaque enamel core; its RGB must stay exact while the lips move.
    return [140, 150, 160, 170].flatMap((x) =>
      Array.from(context.getImageData(x + 6, 80, 1, 1).data),
    )
  }, textureUrl)
  expect(expected.filter((_, index) => index % 4 === 3)).toEqual([255, 255, 255, 255])
  for (const value of ['0.25', '0.5', '0.75', '0.99']) {
    await page
      .getByRole('slider', { name: 'Mouth blend' })
      .evaluate((element: HTMLInputElement, value: string) => {
        element.value = value
        element.dispatchEvent(new Event('input', { bubbles: true }))
      }, value)
    await expect(page.getByTestId('pose')).toHaveText('Transition')
    const actual = await portrait.evaluate((element: HTMLCanvasElement) => {
      const context = element.getContext('2d')!
      return [140, 150, 160, 170].flatMap((x) =>
        Array.from(context.getImageData(740 + x, 998 + 80, 1, 1).data),
      )
    })
    expect(actual).toEqual(expected)
    await page
      .getByRole('img', { name: 'Mouth detail', exact: true })
      .screenshot({ path: info.outputPath(`fixed-teeth-${value}.png`) })
  }
})

test('prepared poses have inspectable endpoints and safe representative transitions', async ({
  page,
}, info) => {
  const requests: string[] = []
  const errors: string[] = []
  page.on('request', (request) => {
    if (request.method() === 'POST') requests.push(request.url())
  })
  page.on('pageerror', (error) => errors.push(error.message))
  await page.goto('/?trial=viseme-a&oral=previous')
  await expect(page.getByRole('img', { name: 'Mouth detail', exact: true })).toBeVisible()
  for (const id of ['EI', 'O', 'U', 'MBP', 'SZ', 'CHSHR']) {
    await page.getByLabel('To pose', { exact: true }).selectOption(id)
    await page.getByRole('button', { name: id, exact: true }).click()
    await expect(page.getByTestId('pose')).toHaveText(id)
    await page
      .getByRole('img', { name: 'Mouth detail', exact: true })
      .screenshot({ path: info.outputPath(`${id}-endpoint.png`) })
    if (id === 'SZ')
      await page.screenshot({ path: info.outputPath('sz-spacing-browser.png'), fullPage: true })
  }
  for (const [source, target] of [
    ['A', 'EI'],
    ['O', 'U'],
    ['MBP', 'A'],
    ['A', 'U'],
    ['SZ', 'FV'],
    ['FV', 'CHSHR'],
  ]) {
    await page.getByLabel('From pose', { exact: true }).selectOption(source)
    await page.getByLabel('To pose', { exact: true }).selectOption(target)
    await expect(page.getByRole('button', { name: 'Play', exact: true })).toBeEnabled()
    await page
      .getByRole('slider', { name: 'Mouth blend' })
      .evaluate((element: HTMLInputElement) => {
        element.value = '0.5'
        element.dispatchEvent(new Event('input', { bubbles: true }))
      })
    await expect(page.getByTestId('pose')).toHaveText('Transition')
    await page
      .getByRole('img', { name: 'Mouth detail', exact: true })
      .screenshot({ path: info.outputPath(`${source}-${target}-50.png`) })
    if (source === 'A' && target === 'EI')
      await page.screenshot({ path: info.outputPath('argon-browser-review.png'), fullPage: true })
    await page.getByRole('button', { name: 'Play', exact: true }).click()
    await expect
      .poll(() => page.getByTestId('pose').textContent(), { intervals: [50] })
      .toBe(target)
    await expect
      .poll(() => page.getByTestId('pose').textContent(), { intervals: [50] })
      .toBe(source)
    await page.getByRole('button', { name: 'Pause', exact: true }).click()
  }
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  expect(requests).toEqual([])
  expect(errors).toEqual([])
})

test('phonetic candidates can play while invalid geometry remains blocked', async ({ page }) => {
  await page.goto('/?trial=viseme-a&oral=previous')
  for (const [source, target] of [
    ['SZ', 'FV'],
    ['FV', 'CHSHR'],
  ]) {
    await page.getByLabel('From pose', { exact: true }).selectOption(source)
    await page.getByLabel('To pose', { exact: true }).selectOption(target)
    await expect(page.getByRole('button', { name: 'Play', exact: true })).toBeEnabled()
    await expect(page.getByRole('slider', { name: 'Mouth blend' })).toBeEnabled()
    await expect(page.getByRole('option', { name: /review only/ })).toHaveCount(0)
  }
  for (const [source, target] of [
    ['U', 'CHSHR'],
    ['CHSHR', 'U'],
  ]) {
    await page.getByLabel('From pose', { exact: true }).selectOption(source)
    await page.getByLabel('To pose', { exact: true }).selectOption(target)
    await expect(page.getByRole('button', { name: 'Play', exact: true })).toBeDisabled()
    await expect(page.getByRole('slider', { name: 'Mouth blend' })).toBeDisabled()
    await expect(page.getByRole('status')).toContainText('Direct morph unavailable')
    await page.getByRole('button', { name: target, exact: true }).click()
    await expect(page.getByTestId('pose')).toHaveText(target)
  }
})

test('shared upper teeth stay fixed across A to EI', async ({ page }) => {
  await page.goto('/?trial=viseme-a&oral=previous')
  await page.getByLabel('From pose', { exact: true }).selectOption('A')
  await page.getByLabel('To pose', { exact: true }).selectOption('EI')
  const portrait = page.getByRole('img', { name: 'Character transition', exact: true })
  const sampleTeeth = () =>
    portrait.evaluate((element: HTMLCanvasElement) => {
      const context = element.getContext('2d')!
      return [140, 150, 160, 170].flatMap((x) =>
        Array.from(context.getImageData(740 + x, 1078, 1, 1).data),
      )
    })
  await page.getByRole('button', { name: 'A', exact: true }).click()
  const expected = await sampleTeeth()
  for (const value of ['0.25', '0.5', '0.75', '1']) {
    await page
      .getByRole('slider', { name: 'Mouth blend' })
      .evaluate((element: HTMLInputElement, value: string) => {
        element.value = value
        element.dispatchEvent(new Event('input', { bubbles: true }))
      }, value)
    expect(await sampleTeeth()).toEqual(expected)
  }
})

test('A to U hides the dental overlay continuously instead of flashing a floating row at the endpoint', async ({
  page,
}) => {
  await page.goto('/?trial=viseme-a&oral=previous')
  async function setLateFrame() {
    await page.getByLabel('From pose', { exact: true }).selectOption('A')
    await page.getByLabel('To pose', { exact: true }).selectOption('U')
    await page
      .getByRole('slider', { name: 'Mouth blend' })
      .evaluate((element: HTMLInputElement) => {
        element.value = '0.99'
        element.dispatchEvent(new Event('input', { bubbles: true }))
      })
    await expect(page.getByTestId('pose')).toHaveText('Transition')
  }
  await setLateFrame()
  const detail = page.getByRole('img', { name: 'Mouth detail', exact: true })
  await detail.evaluate((element: HTMLCanvasElement) =>
    sessionStorage.setItem('__withDental', element.toDataURL('image/png')),
  )
  const transparent = readFileSync(new URL('../fixtures/transparent-teeth.png', import.meta.url))
  let replaced = 0
  await page.route(/\/(upper|lower)-teeth\.webp(?:\?.*)?$/, async (route) => {
    if (route.request().resourceType() !== 'image') return route.continue()
    replaced++
    return route.fulfill({ contentType: 'image/png', body: transparent })
  })
  await page.reload()
  await setLateFrame()
  const largestDifference = await detail.evaluate(async (element: HTMLCanvasElement) => {
    const reference = new Image()
    reference.src = sessionStorage.getItem('__withDental')!
    await reference.decode()
    const canvas = document.createElement('canvas')
    canvas.width = element.width
    canvas.height = element.height
    const context = canvas.getContext('2d')!
    context.drawImage(reference, 0, 0)
    const before = context.getImageData(0, 0, canvas.width, canvas.height).data
    const after = element.getContext('2d')!.getImageData(0, 0, canvas.width, canvas.height).data
    let maximum = 0
    for (let i = 0; i < before.length; i++)
      maximum = Math.max(maximum, Math.abs(before[i] - after[i]))
    sessionStorage.removeItem('__withDental')
    return maximum
  })
  expect(replaced).toBe(2)
  // The geometric cover has completely hidden enamel before the endpoint.
  expect(largestDifference).toBe(0)
})

test('exposed enamel stays opaque while transitioning into a toothless rounded pose', async ({
  page,
}, info) => {
  await page.goto('/?trial=viseme-a&oral=previous')
  await page.getByLabel('From pose', { exact: true }).selectOption('EI')
  await page.getByLabel('To pose', { exact: true }).selectOption('O')
  const portrait = page.getByRole('img', { name: 'Character transition', exact: true })
  await expect(portrait).toBeVisible()
  const imageUrl = `data:image/webp;base64,${readFileSync(new URL('../../src/experiments/viseme-a/poses/upper-teeth.webp', import.meta.url)).toString('base64')}`
  const reference = await portrait.evaluate(async (_element, url: string) => {
    const image = new Image()
    image.src = url
    await image.decode()
    const canvas = document.createElement('canvas')
    canvas.width = image.naturalWidth
    canvas.height = image.naturalHeight
    const context = canvas.getContext('2d')!
    context.drawImage(image, 0, 0)
    return Array.from(context.getImageData(166, 80, 1, 1).data)
  }, imageUrl)
  expect(reference[3]).toBe(255)
  for (const value of ['0.25', '0.5']) {
    await page
      .getByRole('slider', { name: 'Mouth blend' })
      .evaluate((element: HTMLInputElement, value: string) => {
        element.value = value
        element.dispatchEvent(new Event('input', { bubbles: true }))
      }, value)
    await expect(page.getByTestId('pose')).toHaveText('Transition')
    const actual = await portrait.evaluate((element: HTMLCanvasElement) =>
      Array.from(element.getContext('2d')!.getImageData(900, 1078, 1, 1).data),
    )
    expect(actual).toEqual(reference)
    if (value === '0.5')
      await page.screenshot({ path: info.outputPath('opaque-teeth-browser.png'), fullPage: true })
  }
})
