import { expect, test, type Page } from '@playwright/test'

async function fixture(page: Page, role: 'owner' | 'billing' | 'support', secondWorkspace = false) {
  await page.route('**/api/v1/**', async (route) => {
    const path = new URL(route.request().url()).pathname
    const method = route.request().method()
    const reply = (status: number, body: unknown) =>
      route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
    if (path === '/api/v1/auth/me')
      return reply(200, { account: { id: 'person-1', email: 'owner@example.com', name: 'Owner' } })
    if (path === '/api/v1/admin/eligibility')
      return role === 'support'
        ? reply(200, { role: 'support' })
        : reply(403, { error: { message: 'Unavailable' } })
    if (path === '/api/v1/workspaces' && method === 'GET')
      return reply(200, {
        items: [
          { id: 'workspace-1', name: 'Example team', role, suspended: false },
          ...(secondWorkspace
            ? [{ id: 'workspace-2', name: 'Other team', role: 'billing', suspended: false }]
            : []),
        ],
      })
    if (path === '/api/v1/workspaces/workspace-1' || path === '/api/v1/workspaces/workspace-2')
      return reply(200, {
        workspace: {
          id: path.endsWith('workspace-2') ? 'workspace-2' : 'workspace-1',
          name: path.endsWith('workspace-2') ? 'Other team' : 'Example team',
          role: path.endsWith('workspace-2') ? 'billing' : role,
          suspended: false,
        },
        commercial: {
          status: 'unavailable',
          included_minutes: null,
          prepaid_balance: null,
          concurrency: null,
        },
      })
    if (path.endsWith('/members'))
      return role === 'owner'
        ? reply(200, {
            items: [
              { account_id: 'person-1', email: 'owner@example.com', name: 'Owner', role: 'owner' },
            ],
          })
        : reply(403, { error: { message: 'Unavailable' } })
    if (path.endsWith('/activity') || path.endsWith('/keys') || path.endsWith('/configurations'))
      return reply(200, { items: [] })
    if (path === '/api/v1/admin/challenge/start') return reply(202, { challenge_id: 'challenge-1' })
    if (path === '/api/v1/admin/challenge/verify')
      return reply(200, { access_token: 'a'.repeat(43) })
    if (path === '/api/v1/admin')
      return route.request().headers()['x-syntelle-staff-access'] === 'a'.repeat(43)
        ? reply(200, { role: 'support' })
        : reply(403, { error: { message: 'Unavailable' } })
    if (path === '/api/v1/admin/customers')
      return reply(200, {
        items: [
          {
            id: 'workspace-1',
            name: 'Example team',
            suspended: false,
            created_at: new Date().toISOString(),
          },
        ],
      })
    return reply(404, { error: { message: 'Unavailable' } })
  })
}

test('owner sees workspace controls and truthful billing state', async ({ page }) => {
  await fixture(page, 'owner')
  await page.goto('/?view=workspaces')
  await expect(page.getByRole('heading', { name: 'Workspaces' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Example team' })).toBeVisible()
  await page.getByRole('button', { name: 'team', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Team' })).toBeVisible()
  await page.getByRole('button', { name: 'billing', exact: true }).click()
  await expect(
    page.getByText('Subscriptions and invoices will appear when payments are available.'),
  ).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
})

test('billing role sees no team or integration controls', async ({ page }) => {
  await fixture(page, 'billing')
  await page.goto('/?view=workspaces')
  await expect(page.getByRole('heading', { name: 'Example team' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'team', exact: true })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'integrations', exact: true })).toHaveCount(0)
})

test('switching workspaces drops sections from the prior role', async ({ page }) => {
  await fixture(page, 'owner', true)
  await page.goto('/?view=workspaces')
  await expect(page.getByRole('heading', { name: 'Example team' })).toBeVisible()
  await page.getByRole('button', { name: 'team', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Team' })).toBeVisible()
  await page.getByLabel('Active workspace').selectOption('workspace-2')
  await expect(page.getByRole('heading', { name: 'Other team' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Team', exact: true })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'team', exact: true })).toHaveCount(0)
})

test('support must verify email and has read only navigation', async ({ page }) => {
  await fixture(page, 'support')
  await page.goto('/?view=admin')
  await expect(page.getByRole('heading', { name: 'Verify by email' })).toBeVisible()
  await page.getByRole('button', { name: 'Send verification code' }).click()
  await page.getByRole('textbox', { name: 'Email code' }).fill('123456')
  await page.getByRole('button', { name: 'Verify', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Customers' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'audit', exact: true })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Suspend' })).toHaveCount(0)
})

test('workspace panel respects Spanish and dark appearance', async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('syntelle.locale', 'es'))
  await page.emulateMedia({ colorScheme: 'dark' })
  await fixture(page, 'owner')
  await page.goto('/?view=workspaces')
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  await expect(page.getByRole('heading', { name: 'Espacios de trabajo' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Equipo', exact: true })).toBeVisible()
})
