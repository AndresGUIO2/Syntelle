import { expect, test, type Page } from '@playwright/test'

const id = `char_${'a'.repeat(32)}`
const portrait = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/l8sAAAAASUVORK5CYII=',
  'base64',
)

async function fixture(page: Page) {
  let status = 'queued'
  let exists = false
  let streamRequests = 0
  let submissions = 0
  const record = () => ({
    id,
    name: 'Mira',
    status,
    portrait_available: true,
    created_at: new Date().toISOString(),
  })
  await page.route('**/api/v1/**', async (route) => {
    const path = new URL(route.request().url()).pathname
    const method = route.request().method()
    const reply = (body: unknown, code = 200) =>
      route.fulfill({ status: code, contentType: 'application/json', body: JSON.stringify(body) })
    if (path === '/api/v1/auth/me')
      return reply({ account: { id: 'owner', email: 'owner@example.com', name: 'Owner' } })
    if (path === '/api/v1/auth/status')
      return reply({
        email: { configured: true },
        google: { configured: false },
        github: { configured: false },
      })
    if (path === '/api/v1/admin/eligibility') return reply({}, 403)
    if (path === '/api/v1/characters' && method === 'GET')
      return reply({ items: exists ? [record()] : [] })
    if (path === '/api/v1/characters' && method === 'POST') {
      submissions++
      expect(route.request().headers()['content-type']).toContain('multipart/form-data; boundary=')
      expect(route.request().headers()['idempotency-key']).toBeTruthy()
      expect(route.request().postDataBuffer()?.includes(portrait)).toBeTruthy()
      exists = true
      return reply(record(), 202)
    }
    if (path === `/api/v1/characters/${id}/portrait`)
      return route.fulfill({ contentType: 'image/png', body: portrait })
    if (path === `/api/v1/characters/${id}` && method === 'GET') return reply(record())
    if (path === '/api/v1/streams/status')
      return reply({ laughter: { 'en-US': true }, time_limits_enabled: false })
    if (path === '/api/v1/streams' && method === 'POST') {
      streamRequests++
      return reply({}, 503)
    }
    if (path.endsWith('/configurations')) return reply({ items: [] })
    return reply({ error: { message: 'Unavailable' } }, 404)
  })
  return {
    ready: () => {
      status = 'ready'
    },
    streams: () => streamRequests,
    submissions: () => submissions,
  }
}

test('portrait upload persists a queued character, opens only when ready and never substitutes Syntelle', async ({
  page,
}) => {
  const server = await fixture(page)
  await page.goto('/characters/new')
  await expect(page.getByRole('heading', { name: 'Characters', exact: true })).toBeVisible()
  await page.getByLabel('Character name', { exact: true }).fill('Mira')
  await page
    .locator('#character-photo')
    .setInputFiles({ name: 'portrait.png', mimeType: 'image/png', buffer: portrait })
  await page.getByRole('button', { name: 'Create character', exact: true }).click()
  await expect(page.getByText('Waiting to prepare', { exact: true })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Open character' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Create character', exact: true })).toBeDisabled()
  expect(server.submissions()).toBe(1)
  server.ready()
  await page.getByRole('button', { name: 'Refresh', exact: true }).click()
  await page.getByRole('link', { name: 'Open character' }).click()
  await expect(page).toHaveURL(new RegExp(`character=${id}$`))
  await expect(page.getByRole('heading', { name: 'Chat with Mira' })).toBeVisible()
  await expect(page.locator('.remote-poster img')).toHaveAttribute(
    'src',
    `/api/v1/characters/${id}/portrait`,
  )
  expect(server.streams()).toBe(0)
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  await expect(
    page.getByText(
      'This character supports speech, blinking, head movement, smiles and brows. Emotional expressions and laughter are not available yet.',
    ),
  ).toBeVisible()
  await expect(page.getByRole('combobox', { name: 'Emotion' })).toHaveCount(0)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy()
})

test('unknown private character cannot start a stream or show the sample portrait', async ({
  page,
}) => {
  const server = await fixture(page)
  await page.goto(`/?character=char_${'b'.repeat(32)}`)
  await expect(
    page.getByRole('alert').filter({ hasText: 'This character is unavailable.' }),
  ).toBeVisible()
  await expect(page.getByRole('button', { name: 'Start conversation', exact: true })).toBeDisabled()
  await expect(page.locator('.remote-poster img')).toHaveCount(0)
  expect(server.streams()).toBe(0)
})
