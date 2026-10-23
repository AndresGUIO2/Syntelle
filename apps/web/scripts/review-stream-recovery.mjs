import assert from 'node:assert/strict'
import { mkdir, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import { chromium, webkit } from 'playwright'

const base = 'http://127.0.0.1:5173'
const output = resolve('../../.tools/stream-recovery-50')
await mkdir(output, { recursive: true })
const report = { paid_provider_calls: 0, checks: [], page_errors: [] }
const browser = await chromium.launch({ headless: true })
const context = await browser.newContext({ viewport: { width: 1340, height: 940 } })
await context.addInitScript(() => {
  window.reviewSockets = []
  window.reviewMicRequests = 0
  const Original = window.WebSocket
  window.WebSocket = class extends Original {
    constructor(...args) {
      super(...args)
      if (String(args[0]).includes('/api/v1/streams/')) window.reviewSockets.push(this)
    }
  }
  navigator.mediaDevices.getUserMedia = async () => {
    window.reviewMicRequests++
    throw Error('Microphone must stay off during recovery checks')
  }
})
const page = await context.newPage()
page.on('pageerror', (error) => report.page_errors.push(error.message))
const commands = []
page.on('websocket', (socket) =>
  socket.on('framesent', ({ payload }) => {
    try {
      const event = JSON.parse(String(payload))
      if (event.type === 'command') commands.push(event.data.type)
    } catch {
      /* Binary/non-protocol frames are irrelevant. */
    }
  }),
)
async function videoReady() {
  await page.waitForFunction(
    () => {
      const video = document.querySelector('video')
      return (
        video?.videoWidth === 720 &&
        video.readyState >= 2 &&
        document.querySelector('.conversation-heading [role="status"]')?.textContent === 'Ready'
      )
    },
    { timeout: 25000 },
  )
}
try {
  await page.goto(`${base}/?view=stream`)
  await videoReady()
  const first = await page.getByTestId('stream-workspace').getAttribute('data-stream-id')
  await page.getByLabel('Message Syntelle', { exact: true }).fill('Keep this unsent draft')
  await page.evaluate(() => window.reviewSockets.at(-1).close())
  await page.waitForFunction(() => window.reviewSockets.length >= 2)
  await videoReady()
  assert.equal(await page.getByTestId('stream-workspace').getAttribute('data-stream-id'), first)
  report.checks.push(
    'Control disconnection restores the same authorized stream with a fresh ticket',
  )

  await page.evaluate(() => {
    Object.defineProperty(document, 'hidden', { configurable: true, value: true })
    document.dispatchEvent(new Event('visibilitychange'))
  })
  await page.getByText('Connect to continue', { exact: true }).waitFor()
  assert.equal(
    await page.getByRole('button', { name: 'Enable microphone', exact: true }).isDisabled(),
    true,
  )
  assert.equal(await page.locator('.microphone-control').getAttribute('data-state'), 'off')
  await page.evaluate(() => {
    Object.defineProperty(document, 'hidden', { configurable: true, value: false })
    document.dispatchEvent(new Event('visibilitychange'))
  })
  await videoReady()
  const second = await page.getByTestId('stream-workspace').getAttribute('data-stream-id')
  assert.notEqual(second, first)
  assert.equal(
    await page.getByLabel('Message Syntelle', { exact: true }).inputValue(),
    'Keep this unsent draft',
  )
  report.checks.push(
    'Hidden tab releases stream; returning restores idle video and preserves the unsent draft',
  )

  // Expiration exercises the same missing-session path as a service restart.
  await context.request.delete(`${base}/api/v1/streams/${second}`, {
    headers: { Origin: base, 'X-Syntelle-Request': '1' },
  })
  await page.waitForFunction(
    (old) =>
      document.querySelector('[data-stream-id]')?.dataset.streamId &&
      document.querySelector('[data-stream-id]').dataset.streamId !== old,
    second,
  )
  await videoReady()
  assert.equal(await page.evaluate(() => window.reviewMicRequests), 0)
  assert.ok(
    commands.every((type) => type === 'interrupt'),
    'Recovery must never replay speech/model commands',
  )
  report.checks.push(
    'Expired session recreates idle video without replaying messages, speech or microphone capture',
  )
  await page.route('**/api/v1/auth/email/start', (route) =>
    route.fulfill({
      status: 202,
      contentType: 'application/json',
      body: JSON.stringify({
        challenge_id: 'browser-fixture-only',
        expires_in: 600,
        resend_after: 60,
      }),
    }),
  )
  await page.route('**/api/v1/auth/email/verify', (route) =>
    route.fulfill({
      status: 401,
      contentType: 'application/json',
      body: JSON.stringify({ error: { message: 'This preview code is invalid.' } }),
    }),
  )
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await page.getByLabel('Email address', { exact: true }).fill('fixture@example.com')
  await page.getByRole('button', { name: 'Send sign-in code', exact: true }).click()
  await page.getByLabel('Sign-in code', { exact: true }).fill('000000')
  await page.getByRole('button', { name: 'Continue', exact: true }).click()
  await page.getByText('This preview code is invalid.', { exact: true }).waitFor()
  await videoReady()
  await page.getByRole('button', { name: 'Close account', exact: true }).click()
  assert.equal(await page.evaluate(() => window.reviewMicRequests), 0)
  report.checks.push(
    'Rejected sign-in restores guest idle video while preserving the account error; fixture mail only',
  )
  await page.waitForTimeout(1000)
  await page.screenshot({ path: resolve(output, 'recovered-desktop.png') })
} finally {
  const id = await page
    .getByTestId('stream-workspace')
    .getAttribute('data-stream-id')
    .catch(() => '')
  if (id)
    await context.request
      .delete(`${base}/api/v1/streams/${id}`, {
        headers: { Origin: base, 'X-Syntelle-Request': '1' },
      })
      .catch(() => undefined)
  await context.close()
  await browser.close()
}
const safari = await webkit.launch({ headless: true })
const mobile = await safari.newContext({
  viewport: { width: 390, height: 844 },
  isMobile: true,
  hasTouch: true,
})
try {
  const page = await mobile.newPage()
  await page.route('**/api/v1/streams', (route) =>
    route.fulfill({
      status: 503,
      contentType: 'application/json',
      body: JSON.stringify({ error: 'The private renderer is not running.' }),
    }),
  )
  await page.goto(`${base}/?view=stream`)
  await page.getByText('Reconnecting to Syntelle…', { exact: true }).waitFor()
  assert.equal(await page.locator('.stream-error').count(), 0)
  await page.getByText('Connect to continue', { exact: true }).waitFor({ timeout: 15000 })
  await page
    .getByText('Syntelle is temporarily unavailable. Select Reconnect to try again.', {
      exact: true,
    })
    .waitFor()
  assert.equal(
    await page.getByText('The private renderer is not running.', { exact: true }).count(),
    0,
  )
  assert.equal(
    await page.getByRole('button', { name: 'Enable microphone', exact: true }).isDisabled(),
    true,
  )
  assert.equal(
    await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1),
    false,
  )
  await page.screenshot({ path: resolve(output, 'unavailable-mobile.png') })
  report.checks.push(
    'Bounded retries end with an honest unavailable state and a disabled microphone on WebKit mobile layout',
  )
} finally {
  await mobile.close()
  await safari.close()
}
assert.deepEqual(report.page_errors, [])
await writeFile(resolve(output, 'review.json'), JSON.stringify(report, null, 2) + '\n')
console.log(JSON.stringify(report))
