import { expect, test, type Page } from '@playwright/test'
import { writeFile } from 'node:fs/promises'

async function layoutFixture(page: Page, theme: string, locale: string) {
  let populated = false
  let guest = false
  const writes: string[] = []
  const workspace = {
    id: 'layout-workspace',
    name: 'Colors Studio',
    role: 'owner',
    suspended: false,
  }
  await page.addInitScript(
    ({ theme, locale }) => {
      localStorage.setItem('syntelle.navigation.collapsed', 'false')
      localStorage.setItem('syntelle.locale', locale)
      localStorage.setItem('syntelle.theme', theme)
    },
    { theme, locale },
  )
  await page.route('**/api/**', (route) => {
    const path = new URL(route.request().url()).pathname
    const reply = (body: unknown, status = 200) => route.fulfill({ json: body, status })
    if (route.request().method() !== 'GET') {
      writes.push(path)
      return reply({ error: { message: 'Read-only layout fixture' } }, 403)
    }
    if (path.endsWith('/auth/me'))
      return guest
        ? reply({}, 401)
        : reply({ account: { id: 'layout-owner', name: 'Alex', email: 'alex@example.test' } })
    if (path.endsWith('/auth/status'))
      return reply({
        email: { configured: true },
        google: { configured: false },
        github: { configured: false },
      })
    if (path.endsWith('/admin/eligibility')) return reply({}, 403)
    if (path === '/api/v1/workspaces') return reply({ items: populated ? [workspace] : [] })
    if (path.endsWith('/layout-workspace'))
      return reply({ workspace, commercial: { status: 'unavailable' } })
    if (path.endsWith('/members'))
      return reply({
        items: [
          { account_id: 'layout-owner', name: 'Alex', email: 'alex@example.test', role: 'owner' },
        ],
      })
    if (path.endsWith('/usage'))
      return reply({
        used_seconds: 900,
        included_seconds_remaining: 2700,
        topup_seconds_remaining: 1800,
        reserved_seconds: 120,
        active_sessions: 1,
        concurrency_limit: 2,
        max_session_seconds: 300,
        period_start: '2026-11-01T00:00:00Z',
        period_end: '2026-12-01T00:00:00Z',
        by_integration: [{ integration_id: 'int_' + 'a'.repeat(32), seconds: 900, sessions: 8 }],
      })
    if (path.endsWith('/billing'))
      return reply({
        configured: false,
        environment: 'sandbox',
        client_token: '',
        can_manage: true,
        review_required: false,
        subscription: null,
        catalog: {
          plans: [
            {
              id: 'starter',
              name: 'Test Starter',
              amount: 1000,
              currency: 'USD',
              interval: 'month',
              included_minutes: 60,
              concurrency: 1,
              max_session_seconds: 300,
            },
            {
              id: 'team',
              name: 'Test Team',
              amount: 2000,
              currency: 'USD',
              interval: 'month',
              included_minutes: 180,
              concurrency: 2,
              max_session_seconds: 300,
            },
          ],
          topups: [{ id: 'topup', name: 'Test top-up', minutes: 30, amount: 500, currency: 'USD' }],
        },
      })
    return reply({ items: [] })
  })
  return {
    writes,
    populate: () => {
      populated = true
    },
    guest: () => {
      guest = true
    },
  }
}

const screens = [
  { width: 2556, height: 1309, theme: 'light', locale: 'en' },
  { width: 2556, height: 1309, theme: 'dark', locale: 'es' },
  { width: 1440, height: 1000, theme: 'dark', locale: 'en' },
  { width: 1280, height: 720, theme: 'light', locale: 'es' },
  { width: 390, height: 844, theme: 'light', locale: 'en' },
  { width: 390, height: 844, theme: 'dark', locale: 'es' },
]
for (const screen of screens)
  test(`screen distribution ${screen.width} ${screen.theme} ${screen.locale}`, async ({
    page,
  }, info) => {
    test.setTimeout(60_000)
    await page.setViewportSize(screen)
    await page.emulateMedia({ reducedMotion: 'reduce' })
    const fixture = await layoutFixture(page, screen.theme, screen.locale)
    const measurements: unknown[] = []
    async function inspect(path: string, label: string) {
      await page.goto(path)
      await expect(page.locator('main h1')).toBeVisible()
      await page.evaluate((theme) => {
        document.documentElement.dataset.theme = theme
      }, screen.theme)
      await page
        .locator('main [role="status"]')
        .filter({ hasText: /Loading|Cargando/ })
        .waitFor({ state: 'hidden' })
        .catch(() => {})
      const bounds = await page.evaluate(() => {
        const main = document.querySelector('main')!
        const content = main.querySelector('.console-page, .pricing-page')!
        const rect = content.getBoundingClientRect()
        const heading = main.querySelector('h1')!.getBoundingClientRect()
        return {
          width: innerWidth,
          documentWidth: document.documentElement.scrollWidth,
          mainOverflow: main.scrollWidth - main.clientWidth,
          contentWidth: rect.width,
          contentLeft: rect.left,
          headingLeft: heading.left,
          headingTop: heading.top,
          centeredOffset: Math.abs(
            (rect.left + rect.right) / 2 -
              (main.getBoundingClientRect().left + main.getBoundingClientRect().right) / 2,
          ),
          tableOverflow: Array.from(main.querySelectorAll('.console-table-wrap')).map(
            (el) => el.scrollWidth - el.clientWidth,
          ),
        }
      })
      measurements.push({ label, ...bounds })
      await page.screenshot({ path: info.outputPath(`${label}.png`), fullPage: true })
      const scrollable = await page
        .locator('main')
        .evaluate((main) => main.scrollHeight > main.clientHeight + 1)
      if (scrollable) {
        await page.locator('main').evaluate((main) => {
          main.scrollTop = main.scrollHeight
        })
        await page.screenshot({ path: info.outputPath(`${label}-bottom.png`), fullPage: true })
      }
      expect.soft(bounds.documentWidth, label + ' page overflow').toBeLessThanOrEqual(screen.width)
      expect.soft(bounds.mainOverflow, label + ' main overflow').toBeLessThanOrEqual(1)
      expect.soft(bounds.contentWidth, label + ' readable page width').toBeLessThanOrEqual(1280)
      expect
        .soft(bounds.centeredOffset, label + ' balanced within application canvas')
        .toBeLessThanOrEqual(9)
    }
    await inspect('/console', 'overview-empty')
    await inspect('/console/usage', 'usage-unavailable')
    fixture.populate()
    await inspect('/console', 'overview')
    await inspect('/console/usage', 'usage')
    await inspect('/console/billing', 'billing')
    await inspect('/console/team', 'team')
    await inspect('/console/settings', 'settings')
    fixture.guest()
    await inspect('/pricing', 'pricing-guest')
    await writeFile(info.outputPath('measurements.json'), JSON.stringify(measurements, null, 2))
    expect(fixture.writes).toEqual([])
  })
