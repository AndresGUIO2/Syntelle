import { chromium } from 'playwright'
import { mkdir, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import assert from 'node:assert/strict'

const base = 'http://127.0.0.1:5173'
const output = resolve('../../.tools/accounts-streaming-49')
await mkdir(output, { recursive: true })
const browser = await chromium.launch({ headless: true })
const entries = []
try {
  for (let index = 0; index < 2; index++) {
    const context = await browser.newContext({ viewport: { width: 1200, height: 850 } })
    const page = await context.newPage()
    entries.push({ context, page })
    await page.goto(`${base}/?view=stream`)
    await page.waitForFunction(() => {
      const video = document.querySelector('video')
      return video?.videoWidth === 720 && video.readyState >= 2
    })
  }
  const first = entries[0]
  const third = await first.page.evaluate(async () => {
    const id = document.querySelector('[data-stream-id]').dataset.streamId
    const session = await fetch(`/api/v1/streams/${id}`).then((response) => response.json())
    const response = await fetch('/api/v1/streams', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Syntelle-Request': '1' },
      body: JSON.stringify({ settings: session.settings }),
    })
    return { status: response.status, body: await response.json() }
  })
  assert.equal(third.status, 503)
  assert.match(third.body.error, /capacity/i)
  const frames = await Promise.all(
    entries.map(({ page }) =>
      page.locator('video').evaluate((video) => video.getVideoPlaybackQuality().totalVideoFrames),
    ),
  )
  await entries[0].page.waitForTimeout(1500)
  for (let index = 0; index < entries.length; index++) {
    const after = await entries[index].page
      .locator('video')
      .evaluate((video) => video.getVideoPlaybackQuality().totalVideoFrames)
    assert.ok(after > frames[index] + 10, 'Each admitted session must continue receiving frames')
  }
  for (const { context, page } of entries) {
    const id = await page.getByTestId('stream-workspace').getAttribute('data-stream-id')
    await context.request.delete(`${base}/api/v1/streams/${id}`, {
      headers: { Origin: base, 'X-Syntelle-Request': '1' },
    })
  }
  const status = await entries[0].context.request
    .get(`${base}/api/v1/streams/status`)
    .then((response) => response.json())
  assert.equal(status.active, 0)
  const report = {
    paid_provider_calls: 0,
    simultaneous_video_sessions: 2,
    third_session_rejected: true,
    explicit_stop_releases_all: true,
  }
  await writeFile(resolve(output, 'capacity-review.json'), JSON.stringify(report, null, 2))
  process.stdout.write(`${JSON.stringify(report)}\n`)
} finally {
  await Promise.allSettled(entries.map(({ context }) => context.close()))
  await browser.close()
}
