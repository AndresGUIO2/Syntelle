import { expect, test, type Page } from '@playwright/test'
import { writeFile } from 'node:fs/promises'

async function canvasFixture(page: Page, theme: 'light' | 'dark') {
  const mutations: string[] = []
  await page.addInitScript(() => {
    localStorage.setItem('syntelle.navigation.collapsed', 'false')
    localStorage.setItem('syntelle.locale', 'en')
  })
  await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' })
  await page.route('**/api/**', async (route) => {
    const path = new URL(route.request().url()).pathname
    if (route.request().method() !== 'GET') mutations.push(path)
    const reply = (body: unknown, status = 200) => route.fulfill({ json: body, status })
    if (path === '/api/v1/auth/me')
      return reply({ account: { id: 'canvas-owner', name: 'Alex', email: 'canvas@example.test' } })
    if (path === '/api/v1/auth/status')
      return reply({
        email: { configured: true },
        google: { configured: false },
        github: { configured: false },
      })
    if (path === '/api/v1/admin/eligibility') return reply({}, 403)
    return reply({ items: [] })
  })
  return mutations
}

function luminance(rgb: string) {
  const channels = rgb
    .match(/[\d.]+/g)!
    .slice(0, 3)
    .map(Number)
    .map((channel) => {
      const value = channel / 255
      return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4
    })
  return channels[0]! * 0.2126 + channels[1]! * 0.7152 + channels[2]! * 0.0722
}

const viewports = [
  { width: 2545, height: 1314 },
  { width: 1920, height: 1080 },
  { width: 1440, height: 1000 },
  { width: 390, height: 844 },
]

for (const viewport of viewports) {
  for (const theme of ['light', 'dark'] as const) {
    test(`console canvas fills ${viewport.width}px in ${theme} with distinct navigation`, async ({
      page,
    }, info) => {
      const mutations = await canvasFixture(page, theme)
      await page.setViewportSize(viewport)
      await page.goto('/characters')
      await expect(page.getByRole('heading', { name: 'Characters', exact: true })).toBeVisible()
      // Exercise both themes independently of the owner's persisted preference.
      await page.evaluate((value) => {
        document.documentElement.dataset.theme = value
      }, theme)
      const sample = await page.evaluate(() => {
        function background(element: Element | null): string {
          while (element) {
            const color = getComputedStyle(element).backgroundColor
            if (color !== 'rgba(0, 0, 0, 0)' && color !== 'transparent') return color
            element = element.parentElement
          }
          return 'transparent'
        }
        const frame = document.querySelector('.navigation-content')!
        const main = document.querySelector('.main-content.console-content')!
        const navigation = document.querySelector(
          innerWidth >= 1024 ? '.desktop-sidebar .navigation-panel' : '.mobile-header',
        )!
        const bounds = frame.getBoundingClientRect()
        return {
          canvas: background(frame),
          main: background(main),
          navigation: background(navigation),
          frame: { left: bounds.left, right: bounds.right, top: bounds.top, bottom: bounds.bottom },
          viewport: { width: innerWidth, height: innerHeight },
          documentWidth: document.documentElement.scrollWidth,
          mainOverflow: main.scrollWidth - main.clientWidth,
        }
      })
      const canvasLuminance = luminance(sample.canvas)
      const navigationLuminance = luminance(sample.navigation)
      const surfaceContrast =
        (Math.max(canvasLuminance, navigationLuminance) + 0.05) /
        (Math.min(canvasLuminance, navigationLuminance) + 0.05)
      await page.screenshot({ path: info.outputPath('console-canvas.png'), fullPage: true })
      await writeFile(
        info.outputPath('canvas-sample.json'),
        JSON.stringify({ ...sample, surfaceContrast }, null, 2),
      )
      expect(
        sample.main,
        'Main content and its surrounding canvas must not create contrasting gutters',
      ).toBe(sample.canvas)
      expect(sample.frame.right).toBeCloseTo(viewport.width, 0)
      expect(sample.frame.bottom).toBeCloseTo(viewport.height, 0)
      expect(sample.documentWidth).toBeLessThanOrEqual(viewport.width)
      expect(sample.mainOverflow).toBeLessThanOrEqual(1)
      // The owner rejected the midtone gray and saturated purple canvases.
      // A deeper neutral dark canvas keeps a smaller luminance separation.
      expect(
        surfaceContrast,
        'Navigation needs a perceptibly distinct surface; this is a product target, not WCAG text contrast',
      ).toBeGreaterThanOrEqual(theme === 'dark' ? 1.2 : 1.5)
      expect(mutations).toEqual([])
    })
  }
}
