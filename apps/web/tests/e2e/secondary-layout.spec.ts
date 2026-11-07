import { expect, test, type Page } from '@playwright/test'
import { defaultSettings } from '../../src/features/streaming/types'

async function fixture(page: Page, staffRole = 'administrator') {
  const writes: string[] = []
  await page.addInitScript(() => localStorage.setItem('syntelle.navigation.collapsed', 'false'))
  const workspace = {
    id: 'workspace-layout',
    name: 'Colors Studio — International customer experience team',
    role: 'owner',
    suspended: false,
  }
  await page.route('**/api/**', async (route) => {
    const path = new URL(route.request().url()).pathname
    const reply = (json: unknown, status = 200) => route.fulfill({ json, status })
    if (route.request().method() !== 'GET') writes.push(path)
    if (path === '/api/v1/auth/me')
      return reply({
        account: {
          id: 'layout-owner',
          name: 'Alex',
          email: 'international.customer.experience.owner@example.test',
        },
      })
    if (path === '/api/v1/auth/status')
      return reply({
        email: { configured: true },
        google: { configured: false },
        github: { configured: false },
      })
    if (path === '/api/v1/admin/eligibility' || path === '/api/v1/admin')
      return reply({ role: staffRole })
    if (path === '/api/v1/admin/challenge/start') return reply({ challenge_id: 'fixture' })
    if (path === '/api/v1/admin/challenge/verify') return reply({ access_token: 'fixture-token' })
    if (path === '/api/v1/admin/exit') return reply({})
    if (path === '/api/v1/admin/customers')
      return reply({
        items: Array.from({ length: 5 }, (_, i) => ({
          id: `customer-${i}`,
          name: `${workspace.name} ${i + 1}`,
          suspended: i === 2,
          created_at: '2026-11-06T12:00:00Z',
        })),
      })
    if (path === '/api/v1/admin/audit')
      return reply({
        items: [
          {
            workspace_id: workspace.id,
            action: 'workspace.created',
            target_id: workspace.id,
            reason: 'Fixture review',
            actor_account_id: 'layout-owner',
            created_at: '2026-11-06T12:00:00Z',
          },
        ],
      })
    if (path === '/api/v1/workspaces') return reply({ items: [workspace] })
    if (path === '/api/v1/workspaces/' + workspace.id)
      return reply({ workspace, commercial: { status: 'unavailable' } })
    if (path.endsWith('/members'))
      return reply({
        items: [
          {
            account_id: 'layout-owner',
            name: 'Alex',
            email: 'international.customer.experience.owner@example.test',
            role: 'owner',
          },
          {
            account_id: 'layout-editor',
            name: 'María Fernanda del Valle',
            email: 'customer.experience.administration@example.test',
            role: 'editor',
          },
        ],
      })
    if (path.endsWith('/configurations'))
      return reply({
        items: Array.from({ length: 5 }, (_, i) => ({
          id: `setup-${i}`,
          name: `Customer experience — ${'Long unbroken name '.repeat(i === 0 ? 4 : 1)}${i + 1}`,
          settings: defaultSettings(),
          package_hash: 'fixture',
          created_at: '2026-11-06T12:00:00Z',
          updated_at: '2026-11-06T12:00:00Z',
        })),
      })
    return reply({ items: [] })
  })
  return writes
}

test('guest sign-in dialog remains usable at desktop and mobile sizes', async ({ page }, info) => {
  const writes = await fixture(page)
  await page.route('**/api/v1/auth/me', (route) => route.fulfill({ json: { account: null } }))
  for (const viewport of [
    { width: 1440, height: 1000 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport)
    await page.emulateMedia({ colorScheme: viewport.width === 390 ? 'light' : 'dark' })
    await page.goto('/characters')
    const dialog = page.getByRole('dialog', { name: 'Welcome to Syntelle', exact: true })
    await expect(dialog).toBeVisible()
    await expect(
      dialog.getByRole('textbox', { name: 'Email address', exact: true }),
    ).toBeInViewport()
    await expect(
      dialog.getByRole('button', { name: 'Send sign-in code', exact: true }),
    ).toBeInViewport()
    await expect(
      dialog.getByRole('button', { name: 'Close account', exact: true }),
    ).toBeInViewport()
    expect(await dialog.evaluate((el) => el.scrollWidth <= el.clientWidth + 1)).toBe(true)
    await page.screenshot({ path: info.outputPath(`guest-sign-in-${viewport.width}.png`) })
    await page.keyboard.press('Escape')
    await expect(dialog).not.toBeVisible()
  }
  expect(writes).toEqual([])
})

async function noHorizontalOverflow(page: Page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  const sample = await page.locator('.main-content').evaluate((el) => {
    const bounds = el.getBoundingClientRect()
    return {
      fits: el.scrollWidth <= el.clientWidth + 1,
      overflow: Array.from(el.querySelectorAll('*'))
        .filter((child) => child.getBoundingClientRect().right > bounds.right + 1)
        .slice(0, 6)
        .map((child) => ({
          tag: child.tagName,
          class: child.className,
          right: child.getBoundingClientRect().right,
        })),
    }
  })
  expect(sample.fits, JSON.stringify(sample.overflow)).toBe(true)
}
async function centeredInContent(page: Page, selector: string) {
  const offset = await page.locator(selector).evaluate((element) => {
    const main = document.querySelector<HTMLElement>('.main-content')!
    const content = main.getBoundingClientRect()
    const bounds = element.getBoundingClientRect()
    return Math.abs(
      (bounds.left + bounds.right) / 2 - (content.left + main.clientLeft + main.clientWidth / 2),
    )
  })
  expect(
    offset,
    `${selector} should have balanced margins within the full canvas`,
  ).toBeLessThanOrEqual(1.5)
}
async function section(page: Page, value: string, label: string) {
  const picker = page.getByRole('combobox', { name: 'Workspace sections' })
  if (await picker.isVisible()) await picker.selectOption(value)
  else
    await page
      .getByRole('navigation', { name: 'Workspace sections' })
      .getByRole('button', { name: label, exact: true })
      .click()
}

for (const size of [
  { width: 2556, height: 1314 },
  { width: 1440, height: 1000 },
  { width: 390, height: 844 },
  { width: 1440, height: 520 },
]) {
  for (const theme of ['light', 'dark'] as const) {
    test(`secondary layouts ${size.width}x${size.height} ${theme}`, async ({ page }, info) => {
      const writes = await fixture(page)
      await page.setViewportSize(size)
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' })
      await page.goto('/?view=admin')
      const verification = page.getByRole('heading', { name: 'Verify by email' })
      await expect(verification).toBeVisible()
      await noHorizontalOverflow(page)
      await centeredInContent(page, '.staff-panel')
      await page.screenshot({ path: info.outputPath('staff-verification.png') })
      const verificationWidth = await verification.evaluate(
        (el) => el.parentElement!.getBoundingClientRect().width,
      )
      expect
        .soft(verificationWidth, 'Email verification is a compact form, not a full-width banner')
        .toBeLessThanOrEqual(640)
      await page.getByRole('button', { name: 'Send verification code', exact: true }).click()
      await page.getByRole('textbox', { name: 'Email code' }).fill('123456')
      await page.getByRole('button', { name: 'Verify', exact: true }).click()
      await expect(page.getByRole('heading', { name: 'Customers', exact: true })).toBeVisible()
      await noHorizontalOverflow(page)
      await centeredInContent(page, '.staff-panel')
      await page.screenshot({ path: info.outputPath('staff-customers.png') })
      const operationalInput = page.getByLabel('Reason for operational change')
      await expect(operationalInput).toBeVisible()
      expect.soft((await operationalInput.boundingBox())!.width).toBeLessThanOrEqual(680)
      const staffPicker = page.getByRole('combobox', { name: 'Administration sections' })
      if (await staffPicker.isVisible()) await staffPicker.selectOption('audit')
      else
        await page
          .getByRole('navigation', { name: 'Administration sections' })
          .getByRole('button', { name: 'Audit', exact: true })
          .click()
      await expect(page.getByRole('heading', { name: 'Audit', exact: true })).toBeVisible()
      await noHorizontalOverflow(page)
      await page.screenshot({ path: info.outputPath('staff-audit.png') })

      // Invitation links preserve the supported legacy workspace URL.
      await page.goto('/?view=workspaces&invite=fixture-layout')
      await expect(page.getByLabel('Active workspace')).toBeVisible()
      await section(page, 'team', 'Team')
      await expect(page.getByRole('heading', { name: 'Team', exact: true })).toBeVisible()
      await noHorizontalOverflow(page)
      await centeredInContent(page, '.main-content > .workspace-panel')
      await page.screenshot({ path: info.outputPath('workspace-team.png') })
      const inviteEmail = page.getByRole('textbox', { name: 'Invite by email', exact: true })
      await inviteEmail.scrollIntoViewIfNeeded()
      await expect(inviteEmail).toBeInViewport()
      expect
        .soft((await inviteEmail.boundingBox())!.width)
        .toBeGreaterThanOrEqual(size.width < 500 ? 200 : 240)
      await page
        .locator('.workspace-panel')
        .getByRole('button', { name: 'New workspace', exact: true })
        .click()
      const creation = page.getByRole('dialog', { name: 'Create workspace', exact: true })
      await expect(creation).toBeVisible()
      await page.screenshot({ path: info.outputPath('workspace-creation.png') })
      expect(await creation.evaluate((el) => el.scrollWidth <= el.clientWidth + 1)).toBe(true)
      await page.keyboard.press('Escape')
      await expect(creation).not.toBeVisible()
      await expect(
        page
          .locator('.workspace-panel')
          .getByRole('button', { name: 'New workspace', exact: true }),
      ).toBeFocused()

      await page.goto('/console/team')
      await expect(page.getByRole('heading', { name: 'Team', exact: true })).toHaveCount(1)
      await expect(page.locator('.details.embedded .card')).toHaveCSS('border-left-width', '0px')
      await noHorizontalOverflow(page)
      await centeredInContent(page, '.main-content > .console-page')
      await page.screenshot({ path: info.outputPath('embedded-team.png') })
      await page.goto('/console/settings')
      await expect(
        page.getByRole('heading', { name: 'Shared settings', exact: true }),
      ).toBeVisible()
      await noHorizontalOverflow(page)
      await centeredInContent(page, '.main-content > .console-page')
      await page.screenshot({ path: info.outputPath('embedded-settings.png') })

      const drawer = page.getByRole('button', { name: 'Open navigation', exact: true })
      if (await drawer.isVisible()) {
        await drawer.focus()
        await drawer.press('Enter')
      }
      const accountTrigger = page.getByRole('button', { name: 'Your account: Alex', exact: true })
      await accountTrigger.focus()
      await accountTrigger.press('Enter')
      const account = page.getByRole('dialog', { name: 'Your account', exact: true })
      await expect(account).toBeVisible()
      await expect(account.getByText(/Customer experience —/).first()).toBeVisible()
      expect(await account.evaluate((el) => el.scrollWidth <= el.clientWidth + 1)).toBe(true)
      await page.screenshot({ path: info.outputPath('account-library.png') })
      const lastAction = account
        .getByRole('button', { name: 'Delete saved setup', exact: true })
        .last()
      await lastAction.scrollIntoViewIfNeeded()
      await expect(lastAction).toBeInViewport()
      await expect(
        account.getByRole('button', { name: 'Close account', exact: true }),
      ).toBeInViewport()
      await expect(account.getByRole('button', { name: 'Sign out', exact: true })).toBeInViewport()
      await page.keyboard.press('Escape')
      await expect(account).not.toBeVisible()
      await expect(
        page.getByRole('button', {
          name: size.width < 1024 ? 'Open navigation' : 'Your account: Alex',
          exact: true,
        }),
      ).toBeFocused()
      expect(
        writes.every((path) =>
          [
            '/api/v1/admin/challenge/start',
            '/api/v1/admin/challenge/verify',
            '/api/v1/admin/exit',
          ].includes(path),
        ),
      ).toBe(true)
    })
  }
}

test('legacy first workspace keeps its whole composition centered', async ({ page }, info) => {
  const writes = await fixture(page)
  await page.route('**/api/v1/workspaces', (route) => route.fulfill({ json: { items: [] } }))
  for (const viewport of [
    { width: 2556, height: 1314 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport)
    await page.emulateMedia({ colorScheme: viewport.width === 390 ? 'light' : 'dark' })
    await page.goto('/?view=workspaces&invite=fixture-layout')
    await expect(
      page.getByRole('heading', { name: 'Create a workspace', exact: true }),
    ).toBeVisible()
    await centeredInContent(page, '.workspace-panel.empty-state')
    await expect(page.locator('.workspace-panel.empty-state')).toHaveCSS('max-width', '520px')
    await noHorizontalOverflow(page)
    await page.screenshot({ path: info.outputPath(`empty-workspace-${viewport.width}.png`) })
  }
  expect(writes).toEqual([])
})
