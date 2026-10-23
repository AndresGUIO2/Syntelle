import assert from 'node:assert/strict'
import { mkdir, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import { chromium, webkit } from 'playwright'

const base = 'http://127.0.0.1:5173'
const output = resolve('../../.tools/stream-ui-52')
await mkdir(output, { recursive: true })
const report = { paid_provider_calls: 0, fixture_messages: true, checks: [], page_errors: [] }
const browser = await chromium.launch({ headless: true })
const context = await browser.newContext({
  viewport: { width: 1440, height: 1000 },
  colorScheme: 'dark',
})
await context.addInitScript(() => {
  window.fixtureCommands = []
  const Original = WebSocket
  window.WebSocket = class extends Original {
    send(data) {
      let event
      try {
        event = JSON.parse(String(data))
      } catch {
        return super.send(data)
      }
      if (!['message', 'speak'].includes(event.data?.type) || event.type !== 'command')
        return super.send(data)
      // UI-only fixture: no speech/model submission reaches the real service.
      window.fixtureCommands.push(event.data)
      const emit = (type, data) =>
        this.dispatchEvent(
          new MessageEvent('message', {
            data: JSON.stringify({ type, session_id: event.session_id, data }),
          }),
        )
      setTimeout(() => {
        emit('conversation', { type: 'accepted', turn_id: event.command_id, text: event.data.text })
        emit('conversation', {
          type: 'reply_complete',
          turn_id: event.command_id,
          text:
            event.data.type === 'speak'
              ? event.data.text
              : 'I’m doing well, thank you. What would you like to explore today?',
        })
        emit('state', { state: 'idle' })
      }, 0)
    }
  }
})
const page = await context.newPage()
page.on('pageerror', (error) => report.page_errors.push(error.message))
async function ready() {
  await page.waitForFunction(
    () =>
      document.querySelector('video')?.videoWidth === 720 &&
      document.querySelector('.conversation-heading [role="status"]')?.textContent === 'Ready',
  )
}
try {
  await page.goto(`${base}/?view=stream`)
  await ready()
  assert.equal(await page.locator('.workspace-name, .workspace-toolbar, .mode-switch').count(), 0)
  assert.equal(await page.locator('.wordmark').count(), 1)
  assert.equal(await page.getByRole('heading', { name: 'Conversation', exact: true }).count(), 1)
  assert.equal(await page.getByRole('button', { name: 'Read aloud', exact: true }).count(), 0)
  assert.equal(
    await page.getByRole('button', { name: 'Send message', exact: true }).isDisabled(),
    true,
  )
  const newChat = page.getByRole('button', { name: 'New chat', exact: true })
  assert.equal(await newChat.evaluate((button) => getComputedStyle(button).borderTopWidth), '0px')
  report.checks.push(
    'One brand header, a single conversation header, quiet new-chat action and no mode tabs',
  )

  const input = page.getByLabel('Message Syntelle', { exact: true })
  await input.fill('How are you?')
  await input.press('Enter')
  await page
    .getByText('I’m doing well, thank you. What would you like to explore today?', { exact: true })
    .waitFor()
  assert.equal(await input.inputValue(), '')
  await input.fill('What can we create together?')
  await page.waitForTimeout(1200)
  await page.screenshot({ path: resolve(output, 'desktop-dark.png') })
  await page.emulateMedia({ colorScheme: 'light' })
  await page.waitForTimeout(100)
  await page.screenshot({ path: resolve(output, 'desktop-light.png') })
  await page.emulateMedia({ colorScheme: 'dark' })
  await input.fill('First line')
  await input.press('Shift+Enter')
  await input.press('End')
  await input.type('Second line')
  assert.equal(await page.evaluate(() => window.fixtureCommands.length), 1)
  assert.equal(await input.inputValue(), 'First line\nSecond line')
  await page.getByRole('button', { name: 'Send message', exact: true }).click()
  assert.equal(await page.evaluate(() => window.fixtureCommands.at(-1).type), 'message')
  assert.equal(
    await page.evaluate(() => window.fixtureCommands.at(-1).text),
    'First line\nSecond line',
  )
  report.checks.push(
    'Integrated arrow sends by Enter or click; Shift+Enter retains a multiline draft',
  )

  await page.getByRole('button', { name: 'Customize Syntelle', exact: true }).click()
  await page.getByRole('checkbox', { name: 'Say exactly what I type', exact: true }).check()
  await page.getByRole('button', { name: 'Close', exact: true }).click()
  await page.getByLabel('Exact text for Syntelle to say', { exact: true }).fill('Say this exactly.')
  await page.getByRole('button', { name: 'Speak text', exact: true }).click()
  assert.equal(await page.evaluate(() => window.fixtureCommands.at(-1).type), 'speak')
  assert.equal(await page.evaluate(() => window.fixtureCommands.at(-1).text), 'Say this exactly.')
  await page.getByRole('button', { name: 'Return to conversation', exact: true }).click()
  report.checks.push(
    'Literal playback remains a secondary Customize option and preserves the exact-text command',
  )

  await input.fill('A cleaner place to start a conversation.')
  assert.equal(
    await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1),
    false,
  )
  const oldID = await page.getByTestId('stream-workspace').getAttribute('data-stream-id')
  await newChat.click()
  await page.waitForFunction(
    (old) =>
      document.querySelector('[data-stream-id]')?.dataset.streamId &&
      document.querySelector('[data-stream-id]').dataset.streamId !== old,
    oldID,
  )
  await ready()
  assert.equal(await page.locator('.exchange').count(), 0)
  assert.equal(await input.inputValue(), 'A cleaner place to start a conversation.')
  report.checks.push('New conversation resets exchanges while preserving an unsent draft')
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
  colorScheme: 'dark',
})
try {
  const page = await mobile.newPage()
  page.on('pageerror', (error) => report.page_errors.push(error.message))
  await page.route('**/api/v1/streams', (route) =>
    route.fulfill({
      status: 503,
      contentType: 'application/json',
      body: JSON.stringify({ error: 'Local media fixture is unavailable.' }),
    }),
  )
  await page.goto(`${base}/?view=stream`)
  await page.getByRole('heading', { name: 'Conversation', exact: true }).waitFor()
  await page.getByLabel('Message Syntelle', { exact: true }).fill('A short message')
  assert.equal(
    await page.getByRole('button', { name: 'Send message', exact: true }).isDisabled(),
    true,
  )
  assert.equal(
    await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1),
    false,
  )
  const size = await page.getByRole('button', { name: 'New chat', exact: true }).boundingBox()
  assert.ok(size.width >= 44 && size.height >= 44)
  await page.screenshot({ path: resolve(output, 'mobile-dark.png') })
  await page.getByRole('button', { name: 'Customize Syntelle', exact: true }).click()
  await page.getByRole('checkbox', { name: 'Say exactly what I type', exact: true }).check()
  await page.getByRole('button', { name: 'Close', exact: true }).click()
  await page.getByRole('button', { name: 'Return to conversation', exact: true }).waitFor()
  report.checks.push(
    'WebKit mobile layout fits, retains 44px actions, and keeps unavailable sending disabled',
  )
} finally {
  await mobile.close()
  await safari.close()
}
assert.deepEqual(report.page_errors, [])
await writeFile(resolve(output, 'review.json'), JSON.stringify(report, null, 2) + '\n')
console.log(JSON.stringify(report))
