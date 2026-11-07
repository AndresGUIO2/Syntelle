import { expect, test, type Page } from '@playwright/test'

async function selectSection(page: Page, value: string, label: string) {
  const picker = page.getByRole('combobox', { name: 'Workspace sections' })
  if (await picker.isVisible()) await picker.selectOption(value)
  else
    await page
      .getByRole('navigation', { name: 'Workspace sections' })
      .getByRole('button', { name: label, exact: true })
      .click()
}

async function fixture(
  page: Page,
  role: 'owner' | 'billing' | 'support',
  secondWorkspace = false,
  empty = false,
) {
  const created: string[] = []
  const workspaces = empty
    ? []
    : [
        { id: 'workspace-1', name: 'Example team', role, suspended: false },
        ...(secondWorkspace
          ? [{ id: 'workspace-2', name: 'Other team', role: 'billing', suspended: false }]
          : []),
      ]
  await page.route('**/api/v1/**', async (route) => {
    const path = new URL(route.request().url()).pathname
    const method = route.request().method()
    const reply = (status: number, body: unknown) =>
      route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
    if (path === '/api/v1/auth/status')
      return reply(200, {
        email: { configured: true },
        google: { configured: false },
        github: { configured: false },
      })
    if (path === '/api/v1/auth/me')
      return reply(200, { account: { id: 'person-1', email: 'owner@example.com', name: 'Owner' } })
    if (path === '/api/v1/conversations' || path === '/api/v1/conversations/claimable')
      return reply(200, { items: [] })
    if (path === '/api/v1/admin/eligibility')
      return role === 'support'
        ? reply(200, { role: 'support' })
        : reply(403, { error: { message: 'Unavailable' } })
    if (path === '/api/v1/workspaces' && method === 'GET') return reply(200, { items: workspaces })
    if (path === '/api/v1/workspaces' && method === 'POST') {
      const name = route.request().postDataJSON().name
      created.push(name)
      const workspace = {
        id: `workspace-created-${created.length}`,
        name,
        role: 'owner',
        suspended: false,
      }
      workspaces.push(workspace)
      return reply(201, workspace)
    }
    if (/\/api\/v1\/workspaces\/workspace-[^/]+$/.test(path))
      return reply(200, {
        workspace: workspaces.find((item) => path.endsWith('/' + item.id)),
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
              {
                account_id: 'person-2',
                email: 'maya@example.com',
                name: 'Maya Rivera',
                role: 'editor',
              },
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
  return { created }
}

test('empty workspace has a complete creation surface and selects the created team', async ({
  page,
}, info) => {
  await page.addInitScript(() => localStorage.setItem('syntelle-theme', 'dark'))
  const server = await fixture(page, 'owner', false, true)
  let submitCount = 0
  let release = () => {}
  const pending = new Promise<void>((resolve) => {
    release = resolve
  })
  await page.route('**/api/v1/workspaces', async (route) => {
    if (route.request().method() === 'POST') {
      submitCount++
      await pending
    }
    await route.fallback()
  })
  await page.goto('/?view=workspaces')
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  await expect(page.getByRole('heading', { name: 'A shared home for your team' })).toBeVisible()
  await expect(
    page.getByText(
      'Your personal characters and conversations stay private. Share only the settings you choose.',
    ),
  ).toBeVisible()
  await expect(page.getByRole('link', { name: 'Back to conversation' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Create workspace', exact: true })).toBeDisabled()
  await page.getByLabel('Workspace name', { exact: true }).fill('Colors Studio')
  await page.screenshot({ path: info.outputPath('workspace-empty-dark.png') })
  await page.getByRole('button', { name: 'Create workspace', exact: true }).click()
  await expect(
    page.getByRole('button', { name: 'Creating workspace…', exact: true }),
  ).toBeDisabled()
  await page.locator('.creation-form').dispatchEvent('submit')
  expect(submitCount).toBe(1)
  release()
  await expect(page.getByLabel('Active workspace')).toHaveValue('workspace-created-1')
  await expect(page.getByRole('heading', { name: 'Colors Studio', exact: true })).toBeVisible()
  await expect(page.getByLabel('Workspace name', { exact: true })).toHaveCount(0)
  expect(server.created).toEqual(['Colors Studio'])
  expect(submitCount).toBe(1)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
})

test('existing workspaces use a focused creation dialog with cancellation and keyboard return', async ({
  page,
}, info) => {
  await page.addInitScript(() => localStorage.setItem('syntelle-theme', 'light'))
  const server = await fixture(page, 'owner')
  await page.goto('/?view=workspaces')
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
  const open = page.getByRole('button', { name: 'New workspace', exact: true })
  await expect(open).toBeVisible()
  await expect(page.getByLabel('Workspace name', { exact: true })).toHaveCount(0)
  await page.screenshot({ path: info.outputPath('workspace-overview-light.png') })
  await open.click()
  const dialog = page.getByRole('dialog', { name: 'Create workspace', exact: true })
  await expect(dialog).toBeVisible()
  await expect(page.getByLabel('Workspace name', { exact: true })).toBeFocused()
  await page.getByLabel('Workspace name', { exact: true }).fill('Northstar Design')
  await page.screenshot({ path: info.outputPath('workspace-creation-dialog.png') })
  await page.keyboard.press('Escape')
  await expect(dialog).not.toBeVisible()
  await expect(open).toBeFocused()
  expect(server.created).toEqual([])
  await open.click()
  await page.getByLabel('Workspace name', { exact: true }).fill('Northstar Design')
  await page.getByRole('button', { name: 'Create workspace', exact: true }).click()
  await expect(dialog).not.toBeVisible()
  await expect(page.getByLabel('Active workspace')).toHaveValue('workspace-created-1')
  await expect(page.getByLabel('Active workspace')).toBeFocused()
  await selectSection(page, 'team', 'Team')
  await expect(page.getByText('Maya Rivera', { exact: true })).toBeVisible()
  await page.screenshot({ path: info.outputPath('workspace-team-light.png') })
  expect(server.created).toEqual(['Northstar Design'])
})

test('failed workspace creation keeps the entered name and offers another deliberate submission', async ({
  page,
}) => {
  await fixture(page, 'owner', false, true)
  await page.route('**/api/v1/workspaces', (route) =>
    route.request().method() === 'POST'
      ? route.fulfill({
          status: 503,
          json: { error: { message: 'Workspace could not be created.' } },
        })
      : route.fallback(),
  )
  await page.goto('/?view=workspaces')
  await page.getByLabel('Workspace name', { exact: true }).fill('Colors Studio')
  await page.getByRole('button', { name: 'Create workspace', exact: true }).click()
  await expect(page.locator('.workspace-panel').getByRole('alert')).toHaveText(
    'Workspace could not be created.',
  )
  await expect(page.getByLabel('Workspace name', { exact: true })).toHaveValue('Colors Studio')
  await expect(page.getByRole('button', { name: 'Create workspace', exact: true })).toBeEnabled()
})

test('owner sees workspace controls and truthful billing state', async ({ page }) => {
  await fixture(page, 'owner')
  await page.goto('/?view=workspaces')
  await expect(page.getByRole('heading', { name: 'Workspaces' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Example team' })).toBeVisible()
  await selectSection(page, 'team', 'Team')
  await expect(page.getByRole('heading', { name: 'Team' })).toBeVisible()
  await selectSection(page, 'billing', 'Billing')
  await expect(
    page.getByText('Subscriptions and invoices will appear when payments are available.'),
  ).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
})

test('billing role sees no team or integration controls', async ({ page }) => {
  await fixture(page, 'billing')
  await page.goto('/?view=workspaces')
  await expect(page.getByRole('heading', { name: 'Example team' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Team', exact: true })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Integrations', exact: true })).toHaveCount(0)
  await expect(page.locator('.section-picker option[value="team"]')).toHaveCount(0)
  await expect(page.locator('.section-picker option[value="integrations"]')).toHaveCount(0)
})

test('switching workspaces drops sections from the prior role', async ({ page }) => {
  await fixture(page, 'owner', true)
  await page.goto('/?view=workspaces')
  await expect(page.getByRole('heading', { name: 'Example team' })).toBeVisible()
  await selectSection(page, 'team', 'Team')
  await expect(page.getByRole('heading', { name: 'Team' })).toBeVisible()
  await page.getByLabel('Active workspace').selectOption('workspace-2')
  await expect(page.getByRole('heading', { name: 'Other team' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Team', exact: true })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Team', exact: true })).toHaveCount(0)
  await expect(page.locator('.section-picker option[value="team"]')).toHaveCount(0)
})

test('support must verify email and has read only navigation', async ({ page }) => {
  await fixture(page, 'support')
  await page.goto('/?view=admin')
  await expect(page.getByRole('heading', { name: 'Verify by email' })).toBeVisible()
  await page.getByRole('button', { name: 'Send verification code' }).click()
  await page.getByRole('textbox', { name: 'Email code' }).fill('123456')
  await page.getByRole('button', { name: 'Verify', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Customers' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Audit', exact: true })).toHaveCount(0)
  await expect(page.locator('.section-picker option[value="audit"]')).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Suspend' })).toHaveCount(0)
})

test('workspace panel respects Spanish and dark appearance', async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('syntelle.locale', 'es'))
  await page.emulateMedia({ colorScheme: 'dark' })
  await fixture(page, 'owner')
  await page.goto('/?view=workspaces')
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  await expect(page.getByRole('heading', { name: 'Espacios de trabajo' })).toBeVisible()
  const picker = page.getByRole('combobox', { name: 'Secciones del espacio' })
  if (await picker.isVisible()) {
    await expect(picker.locator('option[value="team"]')).toHaveText('Equipo')
    await picker.selectOption('team')
    await expect(page.getByRole('heading', { name: 'Equipo', exact: true })).toBeVisible()
  } else await expect(page.getByRole('button', { name: 'Equipo', exact: true })).toBeVisible()
})
