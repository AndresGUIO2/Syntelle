import { expect, test, type Page } from '@playwright/test'

async function fixture(
  page: Page,
  options: { error?: boolean; role?: string; guest?: boolean } = {},
) {
  const writes: string[] = []
  await page.route('**/api/**', async (route) => {
    const path = new URL(route.request().url()).pathname
    if (route.request().method() !== 'GET') writes.push(path)
    const reply = (body: unknown, status = 200) =>
      route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
    if (path === '/api/v1/auth/me')
      return reply({
        account: options.guest
          ? null
          : { id: 'owner-review', name: 'Alex', email: 'alex@example.test' },
      })
    if (path === '/api/v1/auth/status')
      return reply({
        email: { configured: true },
        google: { configured: false },
        github: { configured: false },
      })
    if (path === '/api/v1/admin/eligibility') return reply({}, 403)
    if (path === '/api/v1/workspaces')
      return options.error
        ? reply({ error: { message: 'Workspace service unavailable' } }, 503)
        : reply({
            items: options.role
              ? [{ id: 'workspace-one', name: 'Studio', role: options.role, suspended: false }]
              : [],
          })
    if (path === '/api/v1/characters') return reply({ items: [] })
    if (path.endsWith('/billing'))
      return reply({
        configured: false,
        environment: 'sandbox',
        reason: 'configuration_missing',
        can_manage: true,
        catalog: { plans: [], topups: [] },
        subscription: null,
        review_required: false,
      })
    if (path.endsWith('/usage'))
      return reply({
        included_seconds_remaining: 0,
        topup_seconds_remaining: 0,
        by_integration: [],
      })
    return reply({ items: [] })
  })
  return writes
}
async function nav(page: Page) {
  const trigger = page.getByRole('button', { name: 'Open navigation', exact: true })
  if ((page.viewportSize()?.width ?? 1440) < 1024) {
    await expect(trigger).toBeVisible()
    await trigger.click()
  }
}

test('personal collection and website connections remain distinct without a workspace', async ({
  page,
}, info) => {
  const writes = await fixture(page)
  await page.goto('/characters')
  await expect(page.getByRole('heading', { name: 'Characters', exact: true })).toBeVisible()
  await expect(
    page.getByRole('link', { name: 'Create character', exact: true }).first(),
  ).toBeVisible()
  await expect(page.getByRole('textbox', { name: 'Workspace name' })).toHaveCount(0)
  await page.screenshot({ path: info.outputPath('characters-empty.png'), fullPage: true })
  await nav(page)
  await page.getByRole('link', { name: 'Integrations', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Integrations', exact: true })).toBeVisible()
  await expect(page.getByRole('textbox', { name: 'Workspace name' })).toHaveCount(0)
  await page.screenshot({ path: info.outputPath('integrations-empty.png'), fullPage: true })
  await page.goBack()
  await expect(page.getByRole('heading', { name: 'Characters', exact: true })).toBeVisible()
  expect(writes).toEqual([])
})

test('missing workspace preserves section context and never invents metrics', async ({
  page,
}, info) => {
  const writes = await fixture(page)
  await page.goto('/console')
  await expect(page.getByRole('heading', { name: 'Overview', exact: true })).toBeVisible()
  await expect(page.getByRole('definition')).toHaveCount(0)
  await page.screenshot({ path: info.outputPath('overview-first-visit.png'), fullPage: true })
  for (const [route, title] of [
    ['usage', 'Usage'],
    ['billing', 'Billing'],
    ['team', 'Team'],
    ['settings', 'Workspace settings'],
  ]) {
    await page.goto('/console/' + route)
    await expect(page.getByRole('heading', { name: title!, exact: true })).toBeVisible()
    await expect(
      page.getByRole('heading', { name: 'Available when you connect a website' }),
    ).toBeVisible()
    await expect(page.getByRole('textbox', { name: 'Workspace name' })).toHaveCount(0)
  }
  expect(writes).toEqual([])
})

test('workspace service error is not first-workspace onboarding', async ({ page }) => {
  const writes = await fixture(page, { error: true })
  await page.goto('/console/usage')
  await expect(
    page.getByRole('heading', { name: 'Your workspace could not be loaded' }),
  ).toBeVisible()
  await expect(page.getByRole('textbox', { name: 'Workspace name' })).toHaveCount(0)
  await page.getByRole('button', { name: 'Try again', exact: true }).click()
  await expect(page.getByRole('alert')).toContainText('Workspace service unavailable')
  expect(writes).toEqual([])
})

test('personal characters remain accessible with a billing-only workspace role', async ({
  page,
}) => {
  await fixture(page, { role: 'billing' })
  await page.goto('/characters')
  await expect(page.getByRole('heading', { name: 'Characters', exact: true })).toBeVisible()
  await nav(page)
  await expect(page.getByRole('link', { name: 'Characters', exact: true })).toBeVisible()
})

test('graphite navigation keeps original branding in both content themes and compact layout', async ({
  page,
}, info) => {
  await fixture(page)
  await page.goto('/console')
  await nav(page)
  const panel = page.locator('.navigation-panel').filter({ visible: true })
  await expect(panel.getByAltText('Colors Studio').first()).toBeVisible()
  await expect(panel.getByAltText('Colors Studio').first()).toHaveAttribute(
    'src',
    '/brand/colors-studio.svg',
  )
  if (!info.project.name.includes('mobile')) {
    await expect(page.locator('.desktop-sidebar')).toHaveCSS('background-color', 'rgb(13, 16, 22)')
    for (const theme of ['dark', 'light']) {
      await page.evaluate((theme) => {
        document.documentElement.dataset.theme = theme
      }, theme)
      await expect(page.locator('.desktop-sidebar')).toHaveCSS(
        'background-color',
        'rgb(13, 16, 22)',
      )
      await expect(panel.getByAltText('Colors Studio').first()).toHaveAttribute(
        'src',
        '/brand/colors-studio.svg',
      )
      await page.screenshot({ path: info.outputPath('console-' + theme + '.png'), fullPage: true })
    }
    await page.getByRole('button', { name: 'Collapse navigation', exact: true }).click()
    const characters = page.getByRole('link', { name: 'Characters', exact: true })
    await characters.focus()
    await expect(page.getByRole('tooltip').filter({ hasText: 'Characters' })).toBeVisible()
    await expect(page.locator('.desktop-sidebar').getByAltText('Colors Studio')).toBeVisible()
    await page.screenshot({ path: info.outputPath('collapsed-tooltip.png'), fullPage: true })
  } else {
    await page.keyboard.press('Escape')
    await expect(page.getByRole('button', { name: 'Open navigation', exact: true })).toBeFocused()
    await expect(page.locator('.mobile-header').getByAltText('Colors Studio')).toBeVisible()
  }
})

test('creation return survives authentication without an open redirect', async ({ page }) => {
  await fixture(page)
  await page.addInitScript(() =>
    sessionStorage.setItem('syntelle.console.auth-return', '/characters/new'),
  )
  await page.goto('/?auth')
  await expect(page).toHaveURL(/\/characters\/new$/)
  await expect(page.getByRole('heading', { name: 'Create character', exact: true })).toBeVisible()
})

test('revoked workspace handoff reports the lost access and does not create a replacement', async ({
  page,
}) => {
  const writes = await fixture(page, { role: 'owner' })
  await page.addInitScript(() =>
    sessionStorage.setItem(
      'syntelle.console.connection-intent',
      JSON.stringify({
        accountId: 'owner-review',
        destination: '/console/integrations',
        characterId: 'syntelle',
        workspaceId: 'revoked-workspace',
      }),
    ),
  )
  await page.goto('/console/integrations')
  await expect(
    page.getByRole('alert').filter({ hasText: 'This workspace is no longer available.' }),
  ).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Name your workspace' })).toHaveCount(0)
  expect(
    await page.evaluate(() => sessionStorage.getItem('syntelle.console.connection-intent')),
  ).toBeNull()
  expect(writes).toEqual([])
})

test('malformed stored handoff cannot forward arbitrary identifiers', async ({ page }) => {
  const writes = await fixture(page)
  await page.addInitScript(() =>
    sessionStorage.setItem(
      'syntelle.console.connection-intent',
      JSON.stringify({
        accountId: 'owner-review',
        destination: '/console/integrations',
        workspaceId: 12345,
        characterId: 'syntelle',
      }),
    ),
  )
  await page.goto('/console/integrations')
  await expect(page.getByRole('heading', { name: 'Integrations', exact: true })).toBeVisible()
  expect(
    await page.evaluate(() => sessionStorage.getItem('syntelle.console.connection-intent')),
  ).toBeNull()
  expect(writes).toEqual([])
})
