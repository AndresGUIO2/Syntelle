import { test, expect } from '@playwright/test'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const fixturePath = (name: string) => fileURLToPath(new URL(`../fixtures/${name}`, import.meta.url))
const fixture = (name: string) => readFileSync(fixturePath(name))

test.beforeEach(async ({ page }) => {
  await page.goto('/?view=validation')
})

test('keyboard upload completes the browser → Go → FastAPI → browser flow', async ({
  page,
}, info) => {
  const errors: string[] = []
  page.on('pageerror', (error) => errors.push(error.message))
  const choose = page.getByRole('button', { name: 'Choose an image', exact: true })
  await choose.focus()
  const chooserPromise = page.waitForEvent('filechooser')
  await choose.press('Enter')
  const chooser = await chooserPromise
  const responsePromise = page.waitForResponse((response) =>
    response.url().endsWith('/api/v1/images/validate'),
  )
  await chooser.setFiles(fixturePath('image.png'))
  const response = await responsePromise
  expect(response.status()).toBe(200)
  expect(await response.json()).toMatchObject({ format: 'PNG', width: 32, height: 48 })
  await expect(page.getByText('Image file validated', { exact: true })).toBeVisible()
  await expect(page.getByText('32 × 48 px', { exact: true })).toBeVisible()
  const image = page.getByRole('img', { name: 'Preview of image.png' })
  await expect(image).toBeVisible()
  await expect
    .poll(() => image.evaluate((element: HTMLImageElement) => element.naturalWidth))
    .toBe(32)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(
    true,
  )
  expect(errors).toEqual([])
  await page.screenshot({ path: info.outputPath('validated.png'), fullPage: true })
  await page.getByRole('button', { name: 'Remove image' }).click()
  await expect(page.getByRole('img', { name: /^Preview of / })).toHaveCount(0)
  await expect(page.getByText('Your portrait goes here', { exact: true })).toBeVisible()
})

test('unsupported and corrupted content produces clear errors', async ({ page }) => {
  await page
    .getByLabel('Portrait image', { exact: true })
    .setInputFiles({ name: 'pretend.png', mimeType: 'image/png', buffer: fixture('image.gif') })
  await expect(page.getByRole('alert')).toContainText('Choose a JPEG, PNG, or WebP image.')
  await page.getByLabel('Portrait image', { exact: true }).setInputFiles(fixturePath('corrupt.png'))
  await expect(page.getByRole('alert')).toContainText('may be damaged')
  await expect(page.getByRole('img', { name: /^Preview of / })).toHaveCount(0)
})

test('oversized files are rejected before a request is sent', async ({ page }) => {
  let requests = 0
  page.on('request', (request) => {
    if (request.url().endsWith('/api/v1/images/validate')) requests++
  })
  await page
    .getByLabel('Portrait image', { exact: true })
    .setInputFiles({ name: 'large.png', mimeType: 'image/png', buffer: Buffer.alloc(5_000_001) })
  await expect(page.getByRole('alert')).toContainText('5,000,000 bytes')
  expect(requests).toBe(0)
})

test('a real in-flight request shows loading and then success', async ({ page }) => {
  let release!: () => void
  let started!: () => void
  const ready = new Promise<void>((resolve) => {
    started = resolve
  })
  const hold = new Promise<void>((resolve) => {
    release = resolve
  })
  await page.route('**/api/v1/images/validate', async (route) => {
    started()
    await hold
    await route.continue()
  })
  await page.getByLabel('Portrait image', { exact: true }).setInputFiles(fixturePath('image.png'))
  await ready
  await expect(page.getByRole('status')).toContainText('Validating image')
  release()
  await expect(page.getByText('Image file validated', { exact: true })).toBeVisible()
})

test('replacing an in-flight upload does not restore stale metadata', async ({ page }) => {
  let first = true
  let release!: () => void
  let started!: () => void
  let finished!: () => void
  const ready = new Promise<void>((resolve) => {
    started = resolve
  })
  const hold = new Promise<void>((resolve) => {
    release = resolve
  })
  const done = new Promise<void>((resolve) => {
    finished = resolve
  })
  await page.route('**/api/v1/images/validate', async (route) => {
    if (!first) {
      await route.continue()
      return
    }
    first = false
    started()
    await hold
    await route
      .fulfill({
        json: {
          format: 'PNG',
          mime_type: 'image/png',
          width: 32,
          height: 48,
          size_bytes: fixture('image.png').length,
        },
      })
      .catch(() => {})
    finished()
  })
  const input = page.getByLabel('Portrait image', { exact: true })
  await input.setInputFiles(fixturePath('image.png'))
  await ready
  await input.setInputFiles(fixturePath('rotated.jpg'))
  await expect(page.getByText('48 × 32 px', { exact: true })).toBeVisible()
  release()
  await done
  await expect(page.getByText('JPEG', { exact: true })).toBeVisible()
  await expect(page.getByRole('img', { name: 'Preview of rotated.jpg' })).toBeVisible()
})

test('drag and drop validates one image and rejects multiple images', async ({ page }) => {
  const zone = page.locator('.upload-zone')
  for (const count of [1, 2]) {
    const transfer = await page.evaluateHandle(
      ({ base64, count }) => {
        const data = new DataTransfer()
        const bytes = Uint8Array.from(atob(base64), (character) => character.charCodeAt(0))
        for (let index = 0; index < count; index++)
          data.items.add(new File([bytes], `image-${index}.png`, { type: 'image/png' }))
        return data
      },
      { base64: fixture('image.png').toString('base64'), count },
    )
    await zone.dispatchEvent('drop', { dataTransfer: transfer })
    await transfer.dispose()
    if (count === 1)
      await expect(page.getByText('Image file validated', { exact: true })).toBeVisible()
    else await expect(page.getByRole('alert')).toContainText('Choose one image at a time.')
  }
})

test('theme follows the system and a manual choice survives reload', async ({ page }, info) => {
  const initial = info.project.use.colorScheme === 'dark' ? 'dark' : 'light'
  const next = initial === 'dark' ? 'light' : 'dark'
  await expect(page.locator('html')).toHaveAttribute('data-theme', initial)
  await page.getByRole('button', { name: 'Dark mode', exact: true }).click()
  await expect(page.locator('html')).toHaveAttribute('data-theme', next)
  await page.reload()
  await expect(page.locator('html')).toHaveAttribute('data-theme', next)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(
    true,
  )
  const chooseBounds = await page
    .getByRole('button', { name: 'Choose an image', exact: true })
    .boundingBox()
  expect(chooseBounds).not.toBeNull()
  expect(chooseBounds!.y + chooseBounds!.height).toBeLessThanOrEqual(page.viewportSize()!.height)
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.screenshot({ path: info.outputPath('empty-opposite-theme.png'), fullPage: true })
})

test('company branding loads and the workspace header opens the real file picker', async ({
  page,
}, info) => {
  const mobile = info.project.use.isMobile === true
  const logo = page.getByRole('img', { name: 'Colors Studio', exact: true })
  await expect(logo).toBeVisible()
  const expectedAsset =
    mobile || info.project.use.colorScheme !== 'dark'
      ? 'colors-studio-symbol.svg'
      : 'colors-studio.svg'
  await expect
    .poll(() => logo.evaluate((element: HTMLImageElement) => element.currentSrc))
    .toContain(`/brand/${expectedAsset}`)
  await expect
    .poll(() =>
      logo.evaluate((element: HTMLImageElement) => element.complete && element.naturalWidth > 0),
    )
    .toBe(true)
  const assetResponse = await page.request.get(`/brand/${expectedAsset}`)
  expect(assetResponse.ok()).toBe(true)
  expect(await assetResponse.body()).toEqual(
    readFileSync(fileURLToPath(new URL(`../../public/brand/${expectedAsset}`, import.meta.url))),
  )

  await expect(page.getByRole('textbox', { name: 'Message', exact: true })).toBeDisabled()
  await expect(page.getByRole('button', { name: 'Use microphone', exact: true })).toBeDisabled()
  await expect(page.getByRole('button', { name: 'Send message', exact: true })).toBeDisabled()
  await expect(page.getByText('Not available yet', { exact: true })).toBeVisible()
  if (!mobile) {
    const portrait = await page
      .getByRole('region', { name: 'Your character', exact: true })
      .boundingBox()
    const conversation = await page
      .getByRole('region', { name: 'Conversation', exact: true })
      .boundingBox()
    expect(portrait).not.toBeNull()
    expect(conversation).not.toBeNull()
    expect(Math.abs(portrait!.y - conversation!.y)).toBeLessThan(2)
    expect(Math.abs(portrait!.height - conversation!.height)).toBeLessThan(2)
    expect(portrait!.height).toBeGreaterThan(page.viewportSize()!.height * 0.7)
    for (const height of [943, 600]) {
      await page.setViewportSize({ width: 1024, height })
      const overflow = await page.evaluate(() => ({
        vertical: document.documentElement.scrollHeight - innerHeight,
        horizontal: document.documentElement.scrollWidth - innerWidth,
      }))
      expect(overflow.vertical).toBeLessThanOrEqual(0)
      expect(overflow.horizontal).toBeLessThanOrEqual(0)
      await expect(page.getByRole('textbox', { name: 'Message', exact: true })).toBeInViewport()
    }
  } else {
    const footer = page.getByText('Conversation is coming in a later step.', { exact: true })
    await footer.scrollIntoViewIfNeeded()
    await expect(footer).toBeInViewport()
  }
  const chooserPromise = page.waitForEvent('filechooser')
  await page.getByRole('button', { name: 'Upload photo', exact: true }).click()
  await (await chooserPromise).setFiles(fixturePath('image.jpg'))
  await expect(page.getByText('Image file validated', { exact: true })).toBeVisible()
  await expect(page.getByRole('img', { name: 'Preview of image.jpg' })).toBeVisible()
})
