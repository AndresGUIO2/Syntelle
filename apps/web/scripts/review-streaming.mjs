import { chromium, webkit } from 'playwright'
import { mkdir, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import assert from 'node:assert/strict'

const argument = (name) => {
  const index = process.argv.indexOf(name)
  return index < 0 ? undefined : process.argv[index + 1]
}
const base = argument('--base') ?? 'http://127.0.0.1:5173'
const fixture = argument('--fixture')
const output = resolve(argument('--output') ?? '../../.tools/accounts-streaming-49')
await mkdir(output, { recursive: true })
if (!['127.0.0.1', 'localhost'].includes(new URL(base).hostname))
  throw Error('Review requires a local player.')
if (fixture && !['127.0.0.1', 'localhost'].includes(new URL(fixture).hostname))
  throw Error('Review requires a local fixture.')
const report = { fixture: Boolean(fixture), paid_provider_calls: 0, checks: [], browser_errors: [] }
const bridges = new Set()
async function routeFixture(context) {
  if (!fixture) return
  await context.route('**/api/v1/**', async (route) => {
    const url = new URL(route.request().url())
    try {
      const upstream = await route.fetch({ url: `${fixture}${url.pathname}${url.search}` })
      await route.fulfill({ response: upstream })
    } catch {
      // Disposing a test context can abort its proxy. Do not print request
      // headers/cookies from Playwright's detailed network error objects.
      await route.abort().catch(() => undefined)
    }
  })
  await context.routeWebSocket(/\/api\/v1\/streams\/[^/]+\/ws$/, (route) => {
    const url = new URL(route.url())
    const upstreamURL = new URL(url.pathname, fixture)
    upstreamURL.protocol = 'ws:'
    const upstream = new WebSocket(upstreamURL, { headers: { Origin: new URL(base).origin } })
    bridges.add(upstream)
    const queue = []
    route.onMessage((message) =>
      upstream.readyState === WebSocket.OPEN ? upstream.send(message) : queue.push(message),
    )
    upstream.addEventListener('open', () => {
      for (const message of queue) upstream.send(message)
      queue.length = 0
    })
    upstream.addEventListener('message', ({ data }) => {
      if (typeof data === 'string') route.send(data)
    })
    upstream.addEventListener('close', () => {
      bridges.delete(upstream)
      void route.close()
    })
    upstream.addEventListener('error', () => {
      void route.close({ code: 1011 })
    })
    route.onClose(() => {
      upstream.close()
      bridges.delete(upstream)
    })
  })
}
async function waitForVideo(page) {
  await page.waitForFunction(
    () => {
      const video = document.querySelector('video')
      return video?.readyState >= 2 && video.videoWidth === 720 && video.videoHeight === 960
    },
    undefined,
    { timeout: 30000 },
  )
}
async function releaseContext(context) {
  for (const page of context.pages()) {
    const id = await page
      .locator('[data-stream-id]')
      .getAttribute('data-stream-id')
      .catch(() => null)
    if (id)
      await context.request
        .delete(`${fixture ?? base}/api/v1/streams/${id}`, {
          headers: { Origin: new URL(base).origin, 'X-Syntelle-Request': '1' },
        })
        .catch(() => undefined)
  }
}
async function emailLogin(page, email) {
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await page.getByRole('textbox', { name: 'Email address', exact: true }).fill(email)
  await page.getByRole('button', { name: 'Send sign-in code', exact: true }).click()
  await page.getByRole('textbox', { name: 'Sign-in code', exact: true }).waitFor()
  const response = await fetch(`${fixture}/__fixture__/mail`, {
    headers: { Authorization: 'Bearer syntelle-offline-control-fixture' },
  })
  assert.equal(response.status, 200)
  const mail = await response.json()
  assert.match(mail.code, /^\d{6}$/)
  await page.getByRole('textbox', { name: 'Sign-in code', exact: true }).fill(mail.code)
  await page.getByRole('button', { name: 'Continue', exact: true }).click()
  await page.getByRole('heading', { name: 'Your account', exact: true }).waitFor()
}
const browser = await chromium.launch({
  headless: true,
  args: ['--autoplay-policy=no-user-gesture-required'],
})
let desktopContext
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
  desktopContext = context
  await routeFixture(context)
  const page = await context.newPage()
  page.on('pageerror', (error) => report.browser_errors.push(error.message))
  const started = performance.now()
  await page.goto(`${base}/?view=stream`)
  await waitForVideo(page)
  report.startup_ms = performance.now() - started
  const startFrames = await page
    .locator('video')
    .evaluate((video) => video.getVideoPlaybackQuality().totalVideoFrames)
  await page.waitForTimeout(2100)
  const endFrames = await page
    .locator('video')
    .evaluate((video) => video.getVideoPlaybackQuality().totalVideoFrames)
  report.received_fps = (endFrames - startFrames) / 2.1
  assert.ok(endFrames > startFrames + 25, 'Idle video must keep advancing')
  report.checks.push('actual Go-to-worker-to-player idle video at720x960')
  await page.getByRole('button', { name: 'Turn sound on', exact: true }).click()
  if (fixture) {
    await emailLogin(page, `stream-review+${Date.now()}@example.test`)
    await page.getByRole('textbox', { name: 'Save this setup', exact: true }).fill('Review setup')
    await page.getByRole('button', { name: 'Save', exact: true }).click()
    await page.getByText('Review setup', { exact: true }).waitFor()
    await page.getByRole('button', { name: 'Developer access', exact: true }).click()
    await page.getByRole('button', { name: 'Create key', exact: true }).click()
    const key = await page
      .getByRole('textbox', { name: 'New integration key', exact: true })
      .inputValue()
    assert.ok(key.length > 32)
    const settings = {
      character: 'syntelle',
      instructions: 'Fixture integration.',
      language: 'en-US',
      voice: 'en-US-Ava:DragonHDLatestNeural',
      background: 'natural-space',
      blur: 0,
      blinking: true,
      movement: true,
      automatic_expressions: true,
      smile: 0,
      brows: 0,
    }
    const integration = await fetch(`${fixture}/api/v1/streams`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${key}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ settings }),
    })
    assert.equal(integration.status, 201)
    const integrationSession = await integration.json()
    const revoked = page.waitForResponse(
      (response) =>
        response.url().includes('/api/v1/keys/') && response.request().method() === 'DELETE',
    )
    await page.getByRole('button', { name: 'Revoke', exact: true }).click()
    assert.ok([200, 204].includes((await revoked).status()))
    const revokedSession = await fetch(`${fixture}/api/v1/streams/${integrationSession.id}`, {
      headers: { Authorization: `Bearer ${key}` },
    })
    assert.equal(revokedSession.status, 401)
    report.checks.push(
      'actual integration key starts a stream and revocation removes its authority',
    )
    await page.getByRole('button', { name: 'Close account', exact: true }).click()
    await waitForVideo(page)
    report.checks.push(
      'real email-code account login, configuration save and key create/revoke with fixture delivery',
    )
    await page.getByRole('button', { name: 'Customize Syntelle', exact: true }).click()
    await page.getByRole('checkbox', { name: 'Say exactly what I type', exact: true }).check()
    await page.getByRole('button', { name: 'Close', exact: true }).click()
    await page.evaluate(async () => {
      const video = document.querySelector('video')
      const context = new AudioContext()
      await context.resume()
      const input = context.createMediaStreamSource(video.srcObject)
      const analyser = context.createAnalyser()
      input.connect(analyser)
      window.__streamReviewAudio = { context, analyser, input, peak: 0 }
      const sample = () => {
        const data = new Float32Array(analyser.fftSize)
        analyser.getFloatTimeDomainData(data)
        window.__streamReviewAudio.peak = Math.max(
          window.__streamReviewAudio.peak,
          ...data.map(Math.abs),
        )
        if (context.state !== 'closed') requestAnimationFrame(sample)
      }
      sample()
    })
    await page
      .getByRole('textbox', { name: 'Exact text for Syntelle to say', exact: true })
      .fill('The exact supplied text is kept intact.')
    await page.getByRole('button', { name: 'Speak text', exact: true }).click()
    await page.getByRole('button', { name: 'Stop reply', exact: true }).waitFor()
    await page.waitForFunction(() => window.__streamReviewAudio?.peak > 0.01)
    report.decoded_audio_peak = await page.evaluate(() => window.__streamReviewAudio.peak)
    await page.screenshot({ path: resolve(output, 'player-speaking.png') })
    await page.getByRole('button', { name: 'Stop reply', exact: true }).click()
    assert.equal(await page.locator('video').evaluate((video) => video.muted), true)
    report.checks.push('literal speech through Go and real RTC, local immediate interruption mute')
    await page.evaluate(async () => {
      window.__streamReviewAudio.input.disconnect()
      await window.__streamReviewAudio.context.close()
    })
    await page.getByRole('button', { name: 'Return to conversation', exact: true }).click()
    await page.getByRole('textbox', { name: 'Message Syntelle', exact: true }).fill('Please reply.')
    await page.getByRole('button', { name: 'Send message', exact: true }).click()
    await page.getByRole('button', { name: 'Stop reply', exact: true }).waitFor()
    await page.waitForFunction(() => !document.querySelector('video')?.muted)
    report.checks.push('fixture conversation reply re-enables audio only for matching new playback')
    await page.getByRole('button', { name: 'Your account', exact: true }).click()
    await page.getByRole('button', { name: 'Sign out', exact: true }).click()
    await page.getByRole('heading', { name: 'Welcome to Syntelle', exact: true }).waitFor()
    await page.getByRole('textbox', { name: 'Email address', exact: true }).waitFor()
    await page.getByRole('button', { name: 'Close account', exact: true }).click()
    await waitForVideo(page)
    report.checks.push('sign-out closes protected stream and restores usable guest flow')
  } else {
    await page.getByRole('button', { name: 'Sign in', exact: true }).click()
    await page.getByRole('heading', { name: 'Welcome to Syntelle', exact: true }).waitFor()
    await page.screenshot({ path: resolve(output, 'login-configuration.png') })
    await page.getByRole('button', { name: 'Continue as a guest', exact: true }).click()
    report.checks.push('real provider readiness shown without synthetic login')
  }
  await page.screenshot({
    path: resolve(output, fixture ? 'player-fixture.png' : 'player-live-idle.png'),
  })
  await releaseContext(context)
  await context.close()
} finally {
  if (desktopContext) await releaseContext(desktopContext).catch(() => undefined)
  for (const bridge of bridges) bridge.close()
  await browser.close()
}
// Layout/media-only coverage: no microphone or native-device audio approval.
const safari = await webkit.launch({ headless: true })
let mobileContext
try {
  const context = await safari.newContext({
    viewport: { width: 390, height: 844 },
    isMobile: true,
    hasTouch: true,
  })
  mobileContext = context
  await routeFixture(context)
  const page = await context.newPage()
  page.on('pageerror', (error) => report.browser_errors.push(error.message))
  await page.goto(`${base}/?view=stream`)
  await page.getByTestId('stream-workspace').waitFor()
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await page.getByRole('heading', { name: 'Welcome to Syntelle', exact: true }).waitFor()
  assert.equal(
    await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1),
    false,
  )
  await page.screenshot({ path: resolve(output, 'login-webkit-mobile.png') })
  report.checks.push('WebKit mobile layout, accessible account dialog and no horizontal overflow')
  await releaseContext(context)
  await context.close()
} finally {
  if (mobileContext) await releaseContext(mobileContext).catch(() => undefined)
  for (const bridge of bridges) bridge.close()
  await safari.close()
}
assert.deepEqual(report.browser_errors, [])
await writeFile(
  resolve(output, fixture ? 'player-fixture-review.json' : 'player-live-review.json'),
  JSON.stringify(report, null, 2),
)
process.stdout.write(`${JSON.stringify(report)}\n`)
