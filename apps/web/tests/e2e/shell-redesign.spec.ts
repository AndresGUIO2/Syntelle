import { expect, test, type Page } from '@playwright/test'
import { createHash } from 'node:crypto'
import { readFile, writeFile } from 'node:fs/promises'

async function shellFixture(page: Page, spanish = false) {
  const writes: string[] = []
  await page.addInitScript((es) => {
    localStorage.setItem('syntelle.navigation.collapsed', 'false')
    localStorage.setItem('syntelle.locale', es ? 'es' : 'en')
  }, spanish)
  await page.route('**/api/**', async (route) => {
    const path = new URL(route.request().url()).pathname
    if (route.request().method() !== 'GET') writes.push(path)
    const reply = (body: unknown, status = 200) =>
      route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
    if (path === '/api/v1/auth/me')
      return reply({ account: { id: 'shell-review', name: 'Alex', email: 'review@example.test' } })
    if (path === '/api/v1/auth/status')
      return reply({
        email: { configured: true },
        google: { configured: false },
        github: { configured: false },
      })
    if (path === '/api/v1/admin/eligibility') return reply({}, 403)
    if (path === '/api/v1/workspaces')
      return reply({
        items: [
          {
            id: 'workspace-shell',
            name: 'Colors Studio — Equipo de experiencias digitales',
            role: 'owner',
            suspended: false,
          },
        ],
      })
    if (path.endsWith('/usage'))
      return reply({
        included_seconds_remaining: 0,
        topup_seconds_remaining: 0,
        active_sessions: 0,
        by_integration: [],
      })
    return reply({ items: [] })
  })
  return writes
}

function contrast(a: string, b: string) {
  const luminance = (value: string) => {
    const channels = value
      .match(/[\d.]+/g)!
      .slice(0, 3)
      .map(Number)
      .map((value) => {
        const channel = value / 255
        return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4
      })
    return channels[0]! * 0.2126 + channels[1]! * 0.7152 + channels[2]! * 0.0722
  }
  const l1 = luminance(a),
    l2 = luminance(b)
  return (Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05)
}

test('sidebar brand bytes and actual neutral selected text contrast remain qualified', async ({
  page,
}, info) => {
  test.skip(
    info.project.name.includes('mobile'),
    'Expanded desktop contrast and source identity check',
  )
  const writes = await shellFixture(page)
  await page.goto('/console')
  await expect(page.getByRole('heading', { name: 'Overview', exact: true })).toBeVisible()
  const hashes: Record<string, string> = {
    'colors-studio.svg': '98a8d1da1055f4cc2e6d027d1376c1a4f9a4a5a7f97043afe230a100c2f70f5a',
    'colors-studio-symbol.svg': 'f3ded775158ed1947c9c2411638af9728948202ea3c665d607fa5beb4db1a17d',
  }
  for (const [file, expected] of Object.entries(hashes))
    expect(
      createHash('sha256')
        .update(await readFile('public/brand/' + file))
        .digest('hex'),
    ).toBe(expected)
  const samples = []
  for (const theme of ['light', 'dark']) {
    await page.evaluate((theme) => (document.documentElement.dataset.theme = theme), theme)
    const sample = await page.locator('.desktop-sidebar').evaluate((el) => {
      const panel = el.querySelector('.navigation-panel')!
      const neutral = el.querySelector('a[href="/characters"]')!
      const active = el.querySelector('a[aria-current="page"]')!
      const style = (element: Element) => getComputedStyle(element)
      return {
        background: style(panel).backgroundColor,
        neutral: style(neutral).color,
        active: style(active).color,
        activeBackground: style(active).backgroundColor,
      }
    })
    expect(sample.background).toBe('rgb(13, 16, 22)')
    expect(contrast(sample.neutral, sample.background)).toBeGreaterThanOrEqual(4.5)
    expect(contrast(sample.active, sample.activeBackground)).toBeGreaterThanOrEqual(4.5)
    const logo = page.locator('.desktop-sidebar .company-signature img')
    await expect(logo).toHaveCSS('width', '120px')
    await expect(logo).toHaveAttribute('src', '/brand/colors-studio.svg')
    samples.push({
      theme,
      ...sample,
      neutralContrast: contrast(sample.neutral, sample.background),
      activeContrast: contrast(sample.active, sample.activeBackground),
    })
    await page.screenshot({ path: info.outputPath(`shell-${theme}.png`), fullPage: true })
  }
  await info.attach('contrast', {
    body: JSON.stringify(samples, null, 2),
    contentType: 'application/json',
  })
  await writeFile(info.outputPath('contrast.json'), JSON.stringify({ hashes, samples }, null, 2))
  expect(writes).toEqual([])
})

test('short Spanish desktop preserves scrolling resizing and unclipped compact labels', async ({
  page,
}, info) => {
  test.skip(info.project.name.includes('mobile'), 'Desktop sidebar interaction')
  const writes = await shellFixture(page, true)
  await page.setViewportSize({ width: 1280, height: 600 })
  await page.goto('/console')
  await expect(page.locator('.desktop-sidebar')).toBeVisible()
  const account = page.locator('.desktop-sidebar .account-button')
  await account.scrollIntoViewIfNeeded()
  await expect(account).toBeInViewport()
  expect(await page.locator('.desktop-sidebar').evaluate((el) => el.scrollTop)).toBeGreaterThan(0)
  await page.screenshot({ path: info.outputPath('spanish-short-scrolled.png'), fullPage: true })
  const resize = page.getByRole('separator')
  await resize.focus()
  await page.keyboard.press('End')
  await expect(resize).toHaveAttribute('aria-valuenow', '320')
  await page.keyboard.press('Home')
  await expect(resize).toHaveAttribute('aria-valuenow', '240')
  await page.locator('.desktop-sidebar .collapse-control').click()
  const characters = page.locator('.desktop-sidebar a[href="/characters"]')
  await characters.focus()
  const tooltip = page.getByRole('tooltip')
  await expect(tooltip).toBeVisible()
  const box = await tooltip.boundingBox()
  expect(box!.x).toBeGreaterThanOrEqual(72)
  expect(box!.x + box!.width).toBeLessThanOrEqual(1280)
  await expect(page.locator('.desktop-sidebar .brand-by')).toHaveText('by')
  await expect(page.locator('.desktop-sidebar .company-signature img')).toHaveAttribute(
    'src',
    '/brand/colors-studio-symbol.svg',
  )
  await page.screenshot({ path: info.outputPath('spanish-compact-tooltip.png'), fullPage: true })
  expect(writes).toEqual([])
})

test('Spanish mobile and 200 percent equivalent reflow retain grouped branding and focus', async ({
  page,
}, info) => {
  const writes = await shellFixture(page, true)
  const mobile = info.project.name.includes('mobile')
  await page.setViewportSize(mobile ? { width: 390, height: 700 } : { width: 720, height: 500 })
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.goto('/console')
  const trigger = page.locator('.drawer-trigger')
  await expect(trigger).toBeVisible()
  await expect(page.locator('.main-content.console-content')).toHaveCSS('padding', '24px 16px')
  await expect(page.locator('.mobile-header .brand-by')).toHaveText('by')
  await expect(page.locator('.mobile-header .company-signature img')).toHaveAttribute(
    'src',
    '/brand/colors-studio-symbol.svg',
  )
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  await trigger.click()
  const drawer = page.locator('.mobile-drawer')
  await expect(drawer).toBeVisible()
  await expect(drawer.locator('.company-signature img')).toHaveCSS('width', '120px')
  await page.screenshot({
    path: info.outputPath(
      mobile ? 'spanish-mobile-drawer.png' : 'spanish-200-equivalent-reflow.png',
    ),
    fullPage: true,
  })
  const account = drawer.locator('.account-button')
  await account.scrollIntoViewIfNeeded()
  await expect(account).toBeInViewport()
  await page.keyboard.press('Escape')
  await expect(trigger).toBeFocused()
  await expect(drawer).not.toBeVisible()
  expect(await page.evaluate(() => document.body.style.overflow)).not.toBe('hidden')
  await page.screenshot({ path: info.outputPath('spanish-mobile-header.png'), fullPage: true })
  expect(writes).toEqual([])
})
