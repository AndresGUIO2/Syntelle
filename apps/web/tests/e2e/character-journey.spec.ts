import { expect, test, type Page } from '@playwright/test'
import { readFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import { createHash } from 'node:crypto'

declare global {
  interface Window {
    __characterUploadHashes: string[]
  }
}

const id = `char_${'c'.repeat(32)}`
async function fixture(
  page: Page,
  options: { role?: string; ready?: boolean; expired?: boolean; uncertain?: boolean } = {},
) {
  const photo = await readFile(
    resolve('public/characters/posters/syntelle-light-studio-portrait.webp'),
  )
  let status = options.ready ? 'ready' : 'queued'
  let exists = Boolean(options.ready)
  const writes: { key: string; body: Buffer }[] = []
  let streams = 0
  // WebKit's request interception omits binary multipart contents. Capture the
  // original File passed to fetch instead, in every browser, before the fixture.
  await page.addInitScript(() => {
    window.__characterUploadHashes = []
    const nativeFetch = window.fetch.bind(window)
    window.fetch = async (input, init) => {
      if (String(input) === '/api/v1/characters' && init?.body instanceof FormData) {
        const file = init.body.get('image')
        if (file instanceof File) {
          const hash = await crypto.subtle.digest('SHA-256', await file.arrayBuffer())
          window.__characterUploadHashes.push(
            [...new Uint8Array(hash)].map((value) => value.toString(16).padStart(2, '0')).join(''),
          )
        }
      }
      return nativeFetch(input, init)
    }
  })
  const record = () => ({
    id,
    name: 'Mira',
    status,
    portrait_available: !options.expired,
    created_at: '2026-11-06T00:00:00Z',
  })
  await page.route('https://**', (route) => route.abort())
  await page.route('**/api/v1/**', async (route) => {
    const path = new URL(route.request().url()).pathname
    const method = route.request().method()
    const reply = (body: unknown, status = 200) =>
      route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
    if (path === '/api/v1/auth/me')
      return reply({ account: { id: 'journey-owner', email: 'owner@example.com', name: 'Owner' } })
    if (path === '/api/v1/auth/status')
      return reply({
        email: { configured: true },
        google: { configured: false },
        github: { configured: false },
      })
    if (path === '/api/v1/admin/eligibility') return reply({}, 403)
    if (path === '/api/v1/workspaces')
      return reply({
        items: options.role
          ? [{ id: 'workspace-fixture', name: 'Shared team', role: options.role, suspended: false }]
          : [],
      })
    if (path === '/api/v1/characters' && method === 'GET')
      return reply({ items: exists ? [record()] : [] })
    if (path === '/api/v1/characters' && method === 'POST') {
      writes.push({
        key: route.request().headers()['idempotency-key']!,
        body: route.request().postDataBuffer()!,
      })
      exists = true
      if (options.uncertain && writes.length === 1) return route.abort('failed')
      return reply(record(), 202)
    }
    if (path === `/api/v1/characters/${id}/portrait`)
      return options.expired
        ? reply({}, 404)
        : route.fulfill({ contentType: 'image/webp', body: photo })
    if (path === `/api/v1/characters/${id}`) {
      if (method === 'DELETE') {
        exists = false
        return reply({})
      }
      return reply(record())
    }
    if (path === '/api/v1/streams/status')
      return reply({ configured: true, available: true, time_limits_enabled: false, laughter: {} })
    if (path === '/api/v1/streams' && method === 'POST') {
      streams++
      return reply({}, 503)
    }
    if (
      path.endsWith('/configurations') ||
      path.endsWith('/publications') ||
      path.endsWith('/integrations') ||
      path.endsWith('/keys')
    )
      return reply({ items: [] })
    return reply({ error: { message: 'Fixture unavailable' } }, 404)
  })
  return {
    photo,
    writes,
    originalHash: createHash('sha256').update(photo).digest('hex'),
    streams: () => streams,
    ready: () => {
      status = 'ready'
    },
  }
}

async function selectPhoto(page: Page, photo: Buffer) {
  await expect(page.getByLabel('Portrait', { exact: true })).toBeEnabled()
  await page
    .getByLabel('Portrait', { exact: true })
    .setInputFiles({ name: 'portrait.webp', mimeType: 'image/webp', buffer: photo })
  await expect(page.getByRole('alert')).toHaveCount(0)
  await expect(page.getByAltText('Selected portrait')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Continue', exact: true })).toBeEnabled()
  await page.getByRole('button', { name: 'Continue', exact: true }).click()
}

test('personal characters remain available without workspace and under a billing membership', async ({
  page,
}, testInfo) => {
  const server = await fixture(page, { role: 'billing', ready: true, expired: true })
  await page.goto('/characters')
  await expect(page.getByRole('heading', { name: 'Characters', exact: true })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Mira', exact: true })).toBeVisible()
  await expect(page.getByText('Portrait unavailable', { exact: true })).toBeVisible()
  await expect(
    page.getByRole('link', { name: 'Create character', exact: true }).first(),
  ).toBeVisible()
  await expect(page.getByText('Create your workspace', { exact: true })).toHaveCount(0)
  await expect(page.getByText('Shared in workspace', { exact: true })).toHaveCount(0)
  expect(server.streams()).toBe(0)
  await expect(page.locator('.character-card')).toHaveCount(1)
  await page.screenshot({ path: testInfo.outputPath('personal-collection.png') })
  const sample = page.locator('.included-character')
  await expect(sample.getByRole('link', { name: 'Test', exact: true })).toHaveAttribute(
    'href',
    '/?character=syntelle',
  )
  await sample.getByRole('link', { name: 'Test', exact: true }).click()
  await expect(page).toHaveURL(/character=syntelle$/)
  await expect(page.getByRole('heading', { name: 'Chat with Syntelle' })).toBeVisible()
  expect(server.streams()).toBe(0)
})

test('two-step creation preserves original bytes and draft through navigation and shows one preparation receipt', async ({
  page,
}, testInfo) => {
  const server = await fixture(page)
  await page.goto('/characters/new')
  await expect(page.getByRole('heading', { name: 'Choose a clear portrait' })).toBeVisible()
  await page.screenshot({ path: testInfo.outputPath('portrait-step.png') })
  await selectPhoto(page, server.photo)
  await page.getByLabel('Character name', { exact: true }).fill('Mira')
  expect(server.writes).toHaveLength(0)
  await page
    .locator('.character-breadcrumb')
    .getByRole('link', { name: 'Characters', exact: true })
    .click()
  await expect(page.getByRole('heading', { name: 'Characters', exact: true })).toBeVisible()
  await page.getByRole('link', { name: 'Create character', exact: true }).first().click()
  await expect(page.getByLabel('Character name', { exact: true })).toHaveValue('Mira')
  await expect(page.getByAltText('Selected portrait')).toBeVisible()
  await page.screenshot({ path: testInfo.outputPath('character-review.png'), fullPage: true })
  await page.getByRole('button', { name: 'Create character', exact: true }).scrollIntoViewIfNeeded()
  await page.screenshot({ path: testInfo.outputPath('character-review-actions.png') })
  await page.getByRole('button', { name: 'Create character', exact: true }).click()
  await expect(
    page.getByRole('heading', { name: 'Your portrait is saved', exact: true }),
  ).toBeVisible()
  expect(server.writes).toHaveLength(1)
  expect(await page.evaluate(() => window.__characterUploadHashes)).toEqual([server.originalHash])
  expect(server.writes[0]!.key).toBeTruthy()
  await expect(page.locator('.character-card')).toHaveCount(1)
  await expect(page.getByRole('region', { name: 'Character collection', exact: true })).toHaveCount(
    0,
  )
  server.ready()
  await page.getByRole('button', { name: 'Refresh status' }).click()
  await expect(
    page.getByRole('heading', { name: 'Your character is ready', exact: true }),
  ).toBeVisible()
  await expect(page.getByRole('link', { name: 'Test', exact: true })).toHaveAttribute(
    'href',
    `/?character=${id}`,
  )
  await expect(page.getByRole('button', { name: 'Connect to website', exact: true })).toBeVisible()
  expect(server.streams()).toBe(0)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
})

test('Spanish creation keeps the two steps and explicit confirmation readable', async ({
  page,
}, testInfo) => {
  const server = await fixture(page)
  await page.addInitScript(() => localStorage.setItem('syntelle.locale', 'es'))
  await page.goto('/characters/new')
  const input = page.getByLabel('Retrato', { exact: true })
  await expect(input).toBeEnabled()
  await input.setInputFiles({ name: 'retrato.webp', mimeType: 'image/webp', buffer: server.photo })
  await page.getByRole('button', { name: 'Continuar', exact: true }).click()
  await page.getByLabel('Nombre del personaje', { exact: true }).fill('Mira')
  await expect(page.getByRole('heading', { name: 'Conoce a tu nuevo personaje' })).toBeVisible()
  await page.getByRole('button', { name: 'Crear personaje', exact: true }).scrollIntoViewIfNeeded()
  await page.screenshot({ path: testInfo.outputPath('character-review-spanish.png') })
  expect(server.writes).toHaveLength(0)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
})

test('uncertain upload retains its idempotency key after internal navigation', async ({ page }) => {
  const server = await fixture(page, { uncertain: true })
  await page.goto('/characters/new')
  await selectPhoto(page, server.photo)
  await page.getByLabel('Character name', { exact: true }).fill('Mira')
  await page.getByRole('button', { name: 'Create character', exact: true }).click()
  await expect(page.getByRole('alert')).toBeVisible()
  expect(server.writes).toHaveLength(1)
  await page
    .locator('.character-breadcrumb')
    .getByRole('link', { name: 'Characters', exact: true })
    .click()
  await page.getByRole('link', { name: 'Create character', exact: true }).first().click()
  await page.getByRole('button', { name: 'Create character', exact: true }).click()
  await expect(
    page.getByRole('heading', { name: 'Your portrait is saved', exact: true }),
  ).toBeVisible()
  expect(server.writes).toHaveLength(2)
  expect(server.writes[1]!.key).toBe(server.writes[0]!.key)
  expect(await page.evaluate(() => window.__characterUploadHashes)).toEqual([
    server.originalHash,
    server.originalHash,
  ])
})

test('portrait validation rejects unsupported format, excessive bytes and dimensions before submission', async ({
  page,
}) => {
  const server = await fixture(page)
  await page.goto('/characters/new')
  const input = page.getByLabel('Portrait', { exact: true })
  await expect(input).toBeEnabled()
  await input.setInputFiles({
    name: 'file.svg',
    mimeType: 'image/svg+xml',
    buffer: Buffer.from('<svg/>'),
  })
  await expect(page.getByRole('alert')).toContainText('JPEG, PNG, or WebP')
  await input.setInputFiles({
    name: 'large.jpg',
    mimeType: 'image/jpeg',
    buffer: Buffer.alloc(5_000_001),
  })
  await expect(page.getByRole('alert')).toContainText('5 MB')
  const encoded = await page.evaluate(() => {
    const canvas = document.createElement('canvas')
    canvas.width = 2100
    canvas.height = 2100
    return canvas.toDataURL('image/png').split(',')[1]!
  })
  await input.setInputFiles({
    name: 'wide.png',
    mimeType: 'image/png',
    buffer: Buffer.from(encoded, 'base64'),
  })
  await expect(page.getByRole('alert')).toContainText('4 megapixels')
  await expect(page.getByRole('button', { name: 'Continue', exact: true })).toBeDisabled()
  expect(server.writes).toHaveLength(0)
  await input.setInputFiles({ name: 'valid.webp', mimeType: 'image/webp', buffer: server.photo })
  await expect(page.getByRole('button', { name: 'Continue', exact: true })).toBeEnabled()
})
