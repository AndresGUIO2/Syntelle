import { expect, test } from '@playwright/test'

test('voice workspace preserves personalization without writing or laboratory controls', async ({
  page,
}, info) => {
  const errors: string[] = [],
    resources: string[] = [],
    images: string[] = []
  page.on('pageerror', (error) => errors.push(error.message))
  page.on('request', (request) => {
    resources.push(request.url())
    if (request.resourceType() === 'image') images.push(request.url())
  })
  await page.route('**/api/**', (route) =>
    route.fulfill({ json: { configured: false, recognition: false } }),
  )
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Chat with Syntelle' })).toBeVisible()
  await expect(page.getByRole('textbox')).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Read aloud', exact: true })).toHaveCount(0)
  await expect(page.getByLabel('Language', { exact: true })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Café', exact: true })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Woodland', exact: true })).toHaveCount(0)
  await expect(page.getByRole('img', { name: 'Animated character', exact: true })).toBeVisible()
  expect(images.filter((url) => /\/characters\/(cafe|woodland)\//.test(url))).toEqual([])
  await expect(page.getByRole('region', { name: 'Syntelle character preview' })).toHaveAttribute(
    'aria-busy',
    'false',
  )
  const customize = page.getByRole('button', { name: 'Customize', exact: true })
  await customize.click()
  const dialog = page.getByRole('dialog', { name: 'Customize', exact: true })
  await expect(dialog).toBeVisible()
  await expect(page.getByRole('button', { name: 'Close customization' })).toBeFocused()
  await page.getByRole('button', { name: 'Dark Lounge', exact: true }).click()
  await page.getByLabel('Background blur', { exact: true }).fill('8')
  await page
    .getByLabel('Choose a voice', { exact: true })
    .selectOption('en-US-Andrew:DragonHDLatestNeural')
  await page.getByLabel('Natural movement', { exact: true }).uncheck()
  await page.screenshot({ path: info.outputPath('customization.png'), fullPage: true })
  await page.getByRole('button', { name: 'Close customization' }).press('Escape')
  await expect(dialog).not.toBeVisible()
  await expect(customize).toBeFocused()
  await expect(page.getByTestId('background-layer')).toHaveCSS('filter', 'blur(8px)')
  await expect(page.getByText('Landmark morph', { exact: true })).toHaveCount(0)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  await page.screenshot({ path: info.outputPath('workspace.png'), fullPage: true })
  await page.reload()
  await expect(page.getByRole('textbox')).toHaveCount(0)
  await customize.click()
  await expect(page.getByRole('button', { name: 'Dark Lounge', exact: true })).toHaveAttribute(
    'aria-pressed',
    'true',
  )
  await expect(page.getByLabel('Background blur', { exact: true })).toHaveValue('8')
  await expect(page.getByLabel('Choose a voice', { exact: true })).toHaveValue(
    'en-US-Andrew:DragonHDLatestNeural',
  )
  await expect(page.getByLabel('Natural movement', { exact: true })).not.toBeChecked()
  expect(
    resources.some((url) =>
      /\/(completed|natural-previous|neck|poses|semantic-overlay)\//.test(url),
    ),
  ).toBe(false)
  expect(errors).toEqual([])
})

test('opening the voice workspace never captures or requests paid speech automatically', async ({
  page,
}) => {
  const requests: string[] = []
  await page.addInitScript(() => {
    navigator.mediaDevices.getUserMedia = async () => {
      throw Error('Unexpected automatic microphone access')
    }
  })
  await page.route('**/api/**', (route) => {
    requests.push(route.request().url())
    return route.fulfill({ json: { configured: false, recognition: false } })
  })
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Chat with Syntelle' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Enable microphone' })).toBeDisabled()
  await expect(page.getByRole('textbox')).toHaveCount(0)
  expect(requests.every((url) => url.endsWith('/status'))).toBe(true)
})

test('unavailable voice and reduced motion leave customization usable', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.route('**/api/**', (route) =>
    route.fulfill({ json: { configured: false, recognition: false } }),
  )
  await page.goto('/')
  await expect(page.getByRole('button', { name: 'Enable microphone' })).toBeDisabled()
  const customize = page.getByRole('button', { name: 'Customize', exact: true })
  await customize.click()
  await expect(page.getByLabel('Blinking', { exact: true })).toBeDisabled()
  await expect(page.getByLabel('Natural movement', { exact: true })).toBeDisabled()
  await page.getByRole('button', { name: 'Close customization' }).click()
  await expect(customize).toBeFocused()
})

test('redesigned workspace frames all existing characters in both themes', async ({
  page,
}, info) => {
  test.setTimeout(60_000)
  await page.route('**/api/**', (route) =>
    route.fulfill({ json: { configured: true, recognition: true } }),
  )
  await page.routeWebSocket('**/api/v1/conversation/ws', (ws) => {
    ws.onMessage((raw) => {
      const event = JSON.parse(String(raw))
      if (event.type === 'hello')
        ws.send(JSON.stringify({ type: 'ready', conversation_id: event.conversation_id, data: [] }))
    })
  })
  await page.goto('/')
  await expect(page.locator('.demo-workspace')).toBeVisible()
  for (const [id, name] of [
    ['syntelle', 'Syntelle'],
    ['cafe', 'Café'],
    ['woodland', 'Woodland'],
  ]) {
    // Exercise the existing selection state without adding a product selector.
    await page.evaluate((id) => {
      const host = document.querySelector('.demo-workspace') as any
      host.__vueParentComponent.setupState.selected = id
    }, id)
    await expect(page.getByRole('region', { name: `${name} character preview` })).toHaveAttribute(
      'aria-busy',
      'false',
    )
    await expect(page.getByRole('heading', { name: `Chat with ${name}` })).toBeVisible()
    for (const theme of ['light', 'dark'] as const) {
      await page.emulateMedia({ colorScheme: theme })
      await expect(page.locator('html')).toHaveAttribute('data-theme', theme)
      await expect(page.getByRole('button', { name: 'Enable microphone' })).toBeInViewport({
        ratio: 1,
      })
      const stage = await page.locator('.stage-panel').boundingBox()
      const chat = await page.locator('.chat-panel').boundingBox()
      const viewport = page.viewportSize()!
      if (viewport.width < 900) {
        expect(stage!.height).toBeCloseTo(Math.min(260, Math.max(180, viewport.height * 0.3)), 0)
        expect(chat!.y).toBeGreaterThan(stage!.y + stage!.height)
      } else {
        expect(stage!.width / (stage!.width + chat!.width)).toBeCloseTo(0.55, 2)
      }
      const portrait = await page
        .getByRole('img', { name: 'Animated character', exact: true })
        .boundingBox()
      const definition = await page.evaluate(async (id) => {
        const { loadDemoCharacter } = await import('/src/features/demo/catalog.ts' as string)
        const { canvas, anatomy } = (await loadDemoCharacter(id)).motion.definition
        return { canvas, anatomy }
      }, id)
      // Uniform source scaling, a bounded face size and visible upper body at every viewport.
      const scale = portrait!.height / definition.canvas.height
      expect(portrait!.width / definition.canvas.width).toBeCloseTo(scale, 3)
      expect((definition.anatomy.face_height * scale) / stage!.height).toBeLessThan(0.45)
      const chin = portrait!.y + definition.anatomy.chin_y * scale
      expect(stage!.y + stage!.height - chin).toBeGreaterThan(stage!.height * 0.12)
      if (viewport.width >= 900)
        expect(viewport.height - stage!.y - stage!.height).toBeLessThanOrEqual(24)
      await page.screenshot({ path: info.outputPath(`${id}-${theme}.png`), fullPage: true })
    }
  }
})

test('panels fill the viewport while bust framing stays bounded in wide and tall windows', async ({
  page,
}, info) => {
  await page.route('**/api/**', (route) =>
    route.fulfill({ json: { configured: true, recognition: true } }),
  )
  await page.routeWebSocket('**/api/v1/conversation/ws', (ws) => {
    ws.onMessage((raw) => {
      const event = JSON.parse(String(raw))
      if (event.type === 'hello')
        ws.send(JSON.stringify({ type: 'ready', conversation_id: event.conversation_id, data: [] }))
    })
  })
  await page.goto('/')
  await expect(page.getByRole('region', { name: 'Syntelle character preview' })).toHaveAttribute(
    'aria-busy',
    'false',
  )
  for (const [width, height] of [
    [1900, 945],
    [1280, 1200],
    [1280, 720],
  ]) {
    await page.setViewportSize({ width: width!, height: height! })
    const stage = await page.locator('.stage-panel').boundingBox()
    const portrait = await page
      .getByRole('img', { name: 'Animated character', exact: true })
      .boundingBox()
    const chat = await page.getByRole('region', { name: 'Chat workspace' }).boundingBox()
    expect(Math.abs(chat!.y - stage!.y)).toBeLessThan(1)
    expect(Math.abs(chat!.height - stage!.height)).toBeLessThan(1)
    await expect(page.getByRole('button', { name: 'Enable microphone' })).toBeInViewport({
      ratio: 1,
    })
    expect(height! - stage!.y - stage!.height).toBeGreaterThanOrEqual(0)
    expect(height! - stage!.y - stage!.height).toBeLessThanOrEqual(24)
    // Face size stays bounded even when the stage grows much taller.
    const faceHeight = (portrait!.height / 2400) * 757.437801361084
    expect(faceHeight).toBeLessThan(370)
    expect(faceHeight).toBeLessThan(stage!.width * 0.59)
    // The visible source ends at upper chest, beyond the previous shoulders-only crop.
    const visibleBottom = ((stage!.y + stage!.height - portrait!.y) / portrait!.height) * 2400
    expect(visibleBottom).toBeGreaterThan(1850)
    expect(visibleBottom).toBeLessThan(2050)
    await page.screenshot({ path: info.outputPath(`bust-${width}-${height}.png`), fullPage: true })
  }
})
