import { expect, test, type Page } from '@playwright/test'
import { readFile } from 'node:fs/promises'
import { resolve } from 'node:path'

const characterId = 'char_' + 'a'.repeat(32)
const appearance = {
  placement: 'floating',
  position: 'bottom-right',
  theme: 'system',
  language: 'en-US',
  launcher_label: 'Ask Syntelle',
}
const publication = {
  id: 'pub-one',
  workspace_id: 'workspace-1',
  name: 'Syntelle guide',
  character_id: characterId,
  package_hash: 'fixture-package',
  portrait_available: true,
  settings: {},
  created_at: '2026-11-01T00:00:00Z',
}
const integration = {
  id: 'int-one',
  workspace_id: 'workspace-1',
  name: 'Studio website',
  publication_id: publication.id,
  widget_id: 'widget-one',
  allowed_origins: ['https://studio.example'],
  capabilities: ['message', 'speak', 'interrupt', 'suspend'],
  appearance,
  revision: 1,
  published_revision: 1,
  enabled: true,
  daily_limit_seconds: 3600,
  created_at: '2026-11-01T00:00:00Z',
  updated_at: '2026-11-01T00:00:00Z',
}
const usage = {
  included_seconds_remaining: 2700,
  topup_seconds_remaining: 1800,
  used_seconds: 900,
  reserved_seconds: 0,
  active_sessions: 0,
  concurrency_limit: 2,
  max_session_seconds: 300,
  period_start: '2026-11-01T00:00:00Z',
  period_end: '2026-12-01T00:00:00Z',
  by_integration: [{ integration_id: 'Studio website', seconds: 900, sessions: 8 }],
}
const catalog = {
  plans: [
    {
      id: 'test_starter',
      name: 'Test Starter',
      amount: 1000,
      currency: 'USD',
      interval: 'month',
      included_minutes: 60,
      concurrency: 1,
      max_session_seconds: 300,
    },
    {
      id: 'test_team',
      name: 'Test Team',
      amount: 2000,
      currency: 'USD',
      interval: 'month',
      included_minutes: 180,
      concurrency: 2,
      max_session_seconds: 300,
    },
  ],
  topups: [{ id: 'test_topup', name: 'Test top-up', amount: 500, currency: 'USD', minutes: 30 }],
}
async function fixture(
  page: Page,
  options: { empty?: boolean; configured?: boolean; role?: string } = {},
) {
  if (options.configured)
    await page.addInitScript(() => {
      const state = window as unknown as {
        Paddle: unknown
        paddleFixture: {
          environment: string
          transactions: string[]
          callback?: (event: { name: string }) => void
        }
      }
      state.paddleFixture = { environment: '', transactions: [] }
      state.Paddle = {
        Environment: {
          set: (value: string) => {
            state.paddleFixture.environment = value
          },
        },
        Initialize: (input: { eventCallback: (event: { name: string }) => void }) => {
          state.paddleFixture.callback = input.eventCallback
        },
        Checkout: {
          open: (input: { transactionId: string }) => {
            state.paddleFixture.transactions.push(input.transactionId)
          },
          close: () => undefined,
        },
      }
    })
  const writes: { path: string; body: Record<string, unknown>; key?: string }[] = []
  const calls: string[] = []
  const workspaces = options.empty
    ? []
    : [
        {
          id: 'workspace-1',
          name: 'Colors Studio',
          role: options.role ?? 'owner',
          suspended: false,
        },
        { id: 'workspace-2', name: 'Second project', role: 'owner', suspended: false },
      ]
  const publications = [{ ...publication }]
  const integrations = [{ ...integration }]
  const keys: Record<string, unknown>[] = []
  const portrait = await readFile(resolve('public/characters/syntelle-poster.jpg'))
  await page.route('**/api/v1/**', async (route) => {
    const path = new URL(route.request().url()).pathname
    const method = route.request().method()
    calls.push(`${method} ${path}`)
    const reply = (body: unknown, status = 200) =>
      route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
    let body: Record<string, unknown> = {}
    if (method !== 'GET') {
      body = route.request().postDataJSON() ?? {}
      writes.push({ path, body, key: route.request().headers()['idempotency-key'] })
    }
    if (path === '/api/v1/auth/me')
      return reply({ account: { id: 'owner-1', email: 'alex@colors.example', name: 'Alex' } })
    if (path === '/api/v1/auth/status')
      return reply({
        email: { configured: true },
        google: { configured: false },
        github: { configured: false },
      })
    if (path === '/api/v1/admin/eligibility')
      return reply({ error: { message: 'Unavailable' } }, 403)
    if (path === '/api/v1/workspaces' && method === 'GET') return reply({ items: workspaces })
    if (path === '/api/v1/workspaces' && method === 'POST') {
      const item = {
        id: 'workspace-created',
        name: String(body.name),
        role: 'owner',
        suspended: false,
      }
      workspaces.push(item)
      return reply(item, 201)
    }
    if (path.endsWith('/portrait'))
      return route.fulfill({ status: 200, contentType: 'image/jpeg', body: portrait })
    if (path === '/api/v1/characters')
      return reply({
        items: [
          {
            id: characterId,
            name: 'Syntelle personal',
            status: 'ready',
            portrait_available: true,
            created_at: '2026-11-01T00:00:00Z',
          },
        ],
      })
    if (path.endsWith('/publications') && method === 'POST') {
      const result = { ...publication, id: 'pub-new', name: String(body.name) }
      publications.push(result)
      return reply(result, 201)
    }
    if (path.endsWith('/publications'))
      return reply({ items: path.includes('workspace-2') ? [] : publications })
    if (path.endsWith('/integrations') && method === 'POST') {
      const result = {
        ...integration,
        ...body,
        id: 'int-new',
        widget_id: 'widget-new',
        revision: 1,
        published_revision: 0,
        enabled: false,
      }
      integrations.push(result as typeof integration)
      return reply(result, 201)
    }
    if (path.endsWith('/integrations'))
      return reply({ items: path.includes('workspace-2') ? [] : integrations })
    if (path.endsWith('/publish')) {
      const item = integrations.find((item) => path.includes('/' + item.id + '/'))!
      item.enabled = true
      item.published_revision = item.revision
      return reply(item)
    }
    if (path.endsWith('/keys') && method === 'POST') {
      keys.push({
        id: 'key-one',
        name: body.name,
        integration_id: body.integration_id,
        status: 'active',
        revoked_at: null,
      })
      return reply({ secret: 'fixture-secret-do-not-leak' }, 201)
    }
    if (path.endsWith('/keys')) return reply({ items: keys })
    if (path.endsWith('/usage')) return reply(usage)
    if (path.endsWith('/billing'))
      return reply({
        configured: options.configured ?? false,
        environment: 'sandbox',
        client_token: options.configured ? 'test_fixture' : '',
        reason: 'configuration_missing',
        can_manage: true,
        catalog,
        subscription: null,
        review_required: false,
      })
    if (path.endsWith('/billing/checkout'))
      return reply({
        transaction_id: 'txn_' + 'a'.repeat(26),
        environment: 'sandbox',
      })
    if (/\/workspaces\/[^/]+$/.test(path))
      return reply({
        workspace: workspaces.find((item) => path.endsWith(item.id)),
        commercial: { status: 'unavailable' },
      })
    if (path.endsWith('/members'))
      return reply({
        items: [
          { account_id: 'owner-1', name: 'Alex', email: 'alex@colors.example', role: 'owner' },
        ],
      })
    if (
      ['/conversations', '/conversations/claimable', '/configurations', '/activity'].some((end) =>
        path.endsWith(end),
      )
    )
      return reply({ items: [] })
    return reply({ error: { message: `Unexpected fixture request: ${path}` } }, 500)
  })
  return { writes, calls }
}
async function showNavigation(page: Page) {
  const open = page.getByRole('button', { name: 'Open navigation', exact: true })
  if (await open.isVisible()) await open.click()
}
async function selectWorkspace(page: Page, id: string) {
  await showNavigation(page)
  const collapsed = page.getByRole('button', { name: 'Choose workspace', exact: true })
  if (await collapsed.isVisible()) await collapsed.click()
  await page.getByRole('combobox', { name: 'Workspace', exact: true }).selectOption(id)
  const close = page.getByRole('button', { name: 'Close navigation', exact: true })
  if (await close.isVisible()) await close.click()
}

test('console overview, intentional navigation and browser history', async ({ page }, info) => {
  const fixtureState = await fixture(page)
  await page.goto('/console')
  await expect(page.getByRole('heading', { name: 'Overview', exact: true })).toBeVisible()
  await expect(page.getByRole('definition').filter({ hasText: /^75 minutes$/ })).toBeVisible()
  if (info.project.name === 'chromium-desktop') {
    const resizer = page.getByRole('separator', { name: 'Resize navigation' })
    await resizer.focus()
    await page.keyboard.press('End')
    await expect(resizer).toHaveAttribute('aria-valuenow', '320')
    await page.keyboard.press('Home')
    await expect(resizer).toHaveAttribute('aria-valuenow', '240')
  }
  await page.screenshot({ path: info.outputPath('overview.png'), fullPage: true })
  await showNavigation(page)
  await page
    .getByRole('navigation', { name: 'Main navigation' })
    .getByRole('link', { name: 'Integrations', exact: true })
    .click()
  await expect(page.getByRole('heading', { name: 'Integrations', exact: true })).toBeVisible()
  await page.goBack()
  await expect(page.getByRole('heading', { name: 'Overview', exact: true })).toBeVisible()
  expect(fixtureState.calls.some((call) => call === 'POST /api/v1/streams')).toBeFalsy()
})

test('explicit publication preserves personal collection', async ({ page }, info) => {
  const state = await fixture(page)
  await page.goto('/characters')
  await expect(page.getByRole('heading', { name: 'Syntelle guide', exact: true })).toBeVisible()
  await page.screenshot({ path: info.outputPath('characters.png'), fullPage: true })
  await page.getByRole('tab', { name: 'Personal library' }).click()
  await page.getByRole('button', { name: 'Publish to workspace', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: 'Publish a character', exact: true })
  await expect(dialog.getByRole('combobox')).toHaveValue(characterId)
  await dialog.getByRole('textbox', { name: 'Character name' }).fill('Website guide')
  await dialog.getByRole('button', { name: 'Publish to workspace', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Website guide' })).toBeVisible()
  expect(state.writes.filter((item) => item.path.endsWith('/publications'))).toHaveLength(1)
  expect(state.writes.some((item) => item.path.includes('/characters/'))).toBeFalsy()
})

test('integration draft, publish, one-time key and workspace isolation', async ({ page }, info) => {
  const state = await fixture(page)
  await page.goto('/console/integrations')
  await expect(page.getByRole('heading', { name: 'Studio website' })).toBeVisible()
  await page.screenshot({ path: info.outputPath('integrations.png'), fullPage: true })
  await page.getByRole('button', { name: 'New integration', exact: true }).click()
  const form = page.getByRole('dialog', { name: 'New integration', exact: true })
  await form.getByRole('textbox', { name: 'Integration name' }).fill('Help center')
  await form.getByRole('textbox', { name: 'Allowed websites' }).fill('https://help.example')
  await form.getByRole('spinbutton', { name: 'Daily allowance in minutes' }).fill('15')
  await form.getByRole('button', { name: 'Save draft' }).click()
  await expect(page.getByRole('heading', { name: 'Help center' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Copy snippet' })).toBeDisabled()
  await page.getByRole('button', { name: 'Publish', exact: true }).click()
  await page
    .getByRole('dialog', { name: 'Publish integration' })
    .getByRole('button', { name: 'Confirm', exact: true })
    .click()
  await expect(page.getByRole('button', { name: 'Copy snippet' })).toBeEnabled()
  await page.getByRole('textbox', { name: 'Key name' }).fill('Backend')
  await page.getByRole('button', { name: 'Create API key', exact: true }).click()
  await expect(page.getByText('fixture-secret-do-not-leak', { exact: true })).toBeVisible()
  const input = state.writes.find((item) => item.path.endsWith('/integrations'))?.body
  expect(input?.daily_limit_seconds).toBe(900)
  await selectWorkspace(page, 'workspace-2')
  await expect(page.getByText('fixture-secret-do-not-leak', { exact: true })).toHaveCount(0)
  await expect(
    page.getByRole('heading', { name: 'One character. Anywhere you need it.' }),
  ).toBeVisible()
})

test('billing disconnected state is honest and has no payment side effects', async ({
  page,
}, info) => {
  const state = await fixture(page)
  await page.goto('/console/billing')
  await expect(page.getByText('Payments are not connected yet.', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Start test subscription' }).first()).toBeDisabled()
  await page.screenshot({ path: info.outputPath('billing.png'), fullPage: true })
  expect(state.writes).toHaveLength(0)
})

test('billing confirmation creates one scoped checkout and leaves completion to webhook', async ({
  page,
}) => {
  const state = await fixture(page, { configured: true })
  await page.goto('/console/billing')
  await page.getByRole('button', { name: 'Start test subscription' }).first().click()
  await page
    .getByRole('dialog', { name: 'Start test subscription' })
    .getByRole('button', { name: 'Confirm', exact: true })
    .click()
  await expect
    .poll(() =>
      page.evaluate(
        () =>
          (window as unknown as { paddleFixture: { transactions: string[] } }).paddleFixture
            .transactions,
      ),
    )
    .toEqual(['txn_' + 'a'.repeat(26)])
  expect(
    await page.evaluate(
      () =>
        (window as unknown as { paddleFixture: { environment: string } }).paddleFixture.environment,
    ),
  ).toBe('sandbox')
  await page.evaluate(() =>
    (
      window as unknown as { paddleFixture: { callback: (event: { name: string }) => void } }
    ).paddleFixture.callback({ name: 'checkout.completed' }),
  )
  await expect(
    page
      .getByRole('status')
      .filter({ hasText: 'Checkout finished. Waiting for server confirmation.' }),
  ).toBeVisible()
  expect(state.writes.filter((item) => item.path.endsWith('/billing/checkout'))).toHaveLength(1)
  expect(state.writes[0]?.key).toBeTruthy()
  await expect(page.getByText('Current plan', { exact: true })).toHaveCount(0)
})

test('billing-only role does not request operational characters', async ({ page }) => {
  const state = await fixture(page, { role: 'billing' })
  await page.goto('/console')
  await expect(page.getByRole('heading', { name: 'Overview', exact: true })).toBeVisible()
  expect(
    state.calls.some((call) => call.endsWith('/publications') || call.endsWith('/integrations')),
  ).toBeFalsy()
  await showNavigation(page)
  await expect(
    page
      .getByRole('navigation', { name: 'Main navigation' })
      .getByRole('link', { name: 'Characters', exact: true }),
  ).toHaveCount(0)
})

test('empty workspace creation is a complete onboarding surface', async ({ page }, info) => {
  const state = await fixture(page, { empty: true })
  await page.goto('/console')
  await expect(
    page.getByRole('heading', { name: 'Bring your character to your website.' }),
  ).toBeVisible()
  await page.screenshot({ path: info.outputPath('workspace-onboarding.png'), fullPage: true })
  await page.getByRole('textbox', { name: 'Workspace name' }).fill('My product')
  await page.getByRole('button', { name: 'Create workspace', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Overview', exact: true })).toBeVisible()
  expect(state.writes.filter((item) => item.path === '/api/v1/workspaces')).toHaveLength(1)
})

test('Spanish mobile drawer restores focus and fits the viewport', async ({ page }, info) => {
  test.skip(!info.project.name.includes('mobile'))
  await page.addInitScript(() => localStorage.setItem('syntelle.locale', 'es'))
  await fixture(page)
  await page.goto('/console')
  const trigger = page.getByRole('button', { name: 'Abrir navegación', exact: true })
  await trigger.click()
  await expect(page.getByRole('dialog')).toBeVisible()
  await page.screenshot({ path: info.outputPath('mobile-navigation-es.png') })
  await page.keyboard.press('Escape')
  await expect(trigger).toBeFocused()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy()
})

test('late integration response cannot populate the next workspace', async ({ page }) => {
  await fixture(page)
  let finish: (() => void) | undefined
  const held = new Promise<void>((resolve) => {
    finish = resolve
  })
  await page.route('**/api/v1/workspaces/workspace-1/integrations', async (route) => {
    await held
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ items: [integration] }),
    })
  })
  await page.goto('/console/integrations')
  await expect(page.getByRole('heading', { name: 'Integrations', exact: true })).toBeVisible()
  await selectWorkspace(page, 'workspace-2')
  finish?.()
  await expect(
    page.getByRole('heading', { name: 'One character. Anywhere you need it.' }),
  ).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Studio website' })).toHaveCount(0)
})

test('workspace creation rejects duplicate submit and preserves the failed input', async ({
  page,
}) => {
  await fixture(page, { empty: true })
  let finish: (() => void) | undefined
  let submissions = 0
  const held = new Promise<void>((resolve) => {
    finish = resolve
  })
  await page.route('**/api/v1/workspaces', async (route) => {
    if (route.request().method() !== 'POST') return route.fallback()
    submissions++
    await held
    await route.fulfill({
      status: 503,
      contentType: 'application/json',
      body: JSON.stringify({ error: { message: 'Workspace creation is unavailable.' } }),
    })
  })
  await page.goto('/console')
  await page.getByRole('textbox', { name: 'Workspace name' }).fill('My studio')
  await page.getByRole('button', { name: 'Create workspace', exact: true }).click()
  await expect(
    page.getByRole('button', { name: 'Creating workspace…', exact: true }),
  ).toBeDisabled()
  await page
    .locator('.creation-form')
    .evaluate((element: HTMLFormElement) => element.requestSubmit())
  finish?.()
  await expect(page.getByRole('alert')).toContainText('Workspace creation is unavailable.')
  await expect(page.getByRole('textbox', { name: 'Workspace name' })).toHaveValue('My studio')
  expect(submissions).toBe(1)
})
