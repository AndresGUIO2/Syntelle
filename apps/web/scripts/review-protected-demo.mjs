import assert from 'node:assert/strict'
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs'
import { resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import { createHash } from 'node:crypto'
import { chromium, webkit, expect } from '@playwright/test'
import { preview } from 'vite'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const evidence = resolve(root, '../../.tools/client-protection-31')
mkdirSync(evidence, { recursive: true })
const reports = Object.fromEntries(
  ['baseline', 'protected'].map((mode) => [
    mode,
    JSON.parse(readFileSync(resolve(evidence, `demo-${mode}.json`), 'utf8')),
  ]),
)
const results = { scenarios: [], performance: [], skipped: [], failures: [] }
const servers = [],
  browsers = []
const performanceOnly = process.argv.includes('--performance-only')
const smokeOnly = process.argv.includes('--smoke-only')

function audioFixture(seconds = 4) {
  const rate = 24000,
    count = rate * seconds,
    wav = Buffer.alloc(44 + count * 2)
  wav.write('RIFF')
  wav.writeUInt32LE(wav.length - 8, 4)
  wav.write('WAVEfmt ', 8)
  wav.writeUInt32LE(16, 16)
  wav.writeUInt16LE(1, 20)
  wav.writeUInt16LE(1, 22)
  wav.writeUInt32LE(rate, 24)
  wav.writeUInt32LE(rate * 2, 28)
  wav.writeUInt16LE(2, 32)
  wav.writeUInt16LE(16, 34)
  wav.write('data', 36)
  wav.writeUInt32LE(count * 2, 40)
  for (let i = 0; i < count; i++)
    wav.writeInt16LE(Math.round(Math.sin((i * 220 * Math.PI * 2) / rate) * 2000), 44 + i * 2)
  const ids = [2, 6, 21, 18, 7, 16, 15]
  return {
    audio_base64: wav.toString('base64'),
    mime: 'audio/wav',
    duration_ms: seconds * 1000,
    voice: 'en-US-Ava:DragonHDLatestNeural',
    language: 'en-US',
    sdk_version: 'local-review-fixture',
    visemes: [
      { id: 0, offset_ms: 0 },
      ...Array.from({ length: Math.floor(seconds * 2) }, (_, index) => ({
        id: ids[index % ids.length],
        offset_ms: 100 + index * 480,
      })),
      { id: 0, offset_ms: seconds * 1000 - 50 },
    ],
  }
}

async function instrument(page) {
  await page.addInitScript(() => {
    const state = (window.__demoReview = {
      uniforms: {},
      measure: false,
      bodyFrames: [],
      passes: 0,
    })
    const names = new WeakMap()
    const prototype = WebGLRenderingContext.prototype
    const originalLocation = prototype.getUniformLocation
    prototype.getUniformLocation = function (...args) {
      const location = originalLocation.apply(this, args)
      if (location) names.set(location, args[1])
      return location
    }
    for (const method of ['uniform1f', 'uniform2f', 'uniform3f', 'uniform4f']) {
      const original = prototype[method]
      prototype[method] = function (location, ...values) {
        const name = names.get(location)
        if (name) state.uniforms[name] = values
        return original.call(this, location, ...values)
      }
    }
    const originalDraw = prototype.drawElements
    prototype.drawElements = function (...args) {
      if (state.measure && this.canvas.getAttribute('aria-label') === 'Animated character') {
        if (state.passes++ % 2 === 0) state.bodyFrames.push(performance.now())
      }
      return originalDraw.apply(this, args)
    }
  })
}

async function fixturePage(browser, url, options = {}) {
  const context = await browser.newContext({
    viewport: options.mobile ? { width: 390, height: 844 } : { width: 1440, height: 1000 },
    reducedMotion: options.reduced ? 'reduce' : 'no-preference',
  })
  const page = await context.newPage(),
    state = { unexpected: [], errors: [], requests: [], syntheses: 0, socketMessages: [] }
  await instrument(page)
  page.on('pageerror', (error) => state.errors.push(error.message))
  page.on('request', (request) =>
    state.requests.push({
      path: new URL(request.url()).pathname,
      type: request.resourceType(),
      method: request.method(),
    }),
  )
  // No test request may reach a provider or the owner's existing API. Preview
  // proxies are also disabled, so a missing interception fails locally.
  await page.route('**/*', async (route) => {
    const request = route.request(),
      target = new URL(request.url())
    if (target.origin !== new URL(url).origin) {
      state.unexpected.push(target.origin)
      return route.abort()
    }
    if (!target.pathname.startsWith('/api/')) return route.continue()
    if (target.pathname === '/api/v1/conversation/status')
      return route.fulfill({ json: { configured: !options.unavailable, recognition: false } })
    if (target.pathname === '/api/v1/speech/status')
      return route.fulfill({ json: { configured: !options.unavailable } })
    if (target.pathname === '/api/v1/speech/synthesize' && !options.unavailable) {
      state.syntheses++
      return route.fulfill({ json: audioFixture(options.duration ?? 4) })
    }
    state.unexpected.push(target.pathname)
    return route.abort()
  })
  await page.routeWebSocket('**/api/v1/conversation/ws', (socket) => {
    socket.onMessage((raw) => {
      const event = JSON.parse(String(raw))
      state.socketMessages.push(event.type)
      const send = (type, text = '', data) =>
        socket.send(
          JSON.stringify({
            type,
            text,
            data,
            conversation_id: event.conversation_id,
            turn_id: event.turn_id,
          }),
        )
      if (event.type === 'hello') send('ready', '', [])
      if (event.type === 'submit') {
        send('accepted', event.text)
        send('reply_delta', 'Local fixture reply.')
        send('reply_complete', 'Local fixture reply.', { spoken: 'Local fixture reply.' })
        send('prepared_speech', 'Local fixture reply.', audioFixture())
      }
    })
  })
  return { context, page, state }
}

async function ready(page, name = 'Syntelle') {
  await expect(page.getByRole('region', { name: `${name} character preview` })).toHaveAttribute(
    'aria-busy',
    'false',
    { timeout: 15000 },
  )
  await expect(page.getByRole('img', { name: 'Animated character', exact: true })).toBeVisible()
}
function clean(state) {
  assert.deepEqual(state.unexpected, [])
  assert.deepEqual(state.errors, [])
}

async function smoke(browser, engine, url, mobile = false) {
  const { context, page, state } = await fixturePage(browser, url, { mobile })
  try {
    await page.goto(
      url +
        '/?trial=motion&character=woodland&head=neck-compare&view=validation&expressions=off&idle=previous',
    )
    await ready(page)
    await expect(page.getByRole('heading', { name: 'Chat with Syntelle' })).toBeVisible()
    await expect(page.getByLabel('From pose', { exact: true })).toHaveCount(0)
    for (const path of ['/src/main.ts', '/@vite/client', '/.tools/private.json']) {
      const response = await page.request.get(url + path)
      const body = await response.text()
      assert(
        !/createApp|createHotContext|\.tools\/private/.test(body),
        'Static preview must not expose development source',
      )
    }
    if (!mobile) {
      await page.getByLabel('Message', { exact: true }).fill('A fixture chat turn.')
      await page.getByRole('button', { name: 'Send', exact: true }).click()
      await expect(page.getByText('Local fixture reply.', { exact: true })).toBeVisible()
      assert(state.socketMessages.includes('submit'))
      assert.equal(state.syntheses, 0)
    }
    await page.getByRole('button', { name: 'Read aloud', exact: true }).click()
    const editor = page.getByLabel('Text to speak', { exact: true })
    await editor.fill('A retained draft.')
    for (const name of ['Syntelle']) {
      await ready(page, name)
      await expect(editor).toHaveValue('A retained draft.')
      const first = await page.evaluate(() => window.__demoReview.uniforms.angles)
      await page.waitForTimeout(650)
      const second = await page.evaluate(() => window.__demoReview.uniforms)
      assert.notDeepEqual(second.angles, first, 'Head movement must remain live')
      await expect
        .poll(
          () =>
            page.evaluate(
              () =>
                window.__demoReview.uniforms.offsets?.some((value) => Math.abs(value) > 0) ?? false,
            ),
          {
            timeout: 6000,
            message: 'Gaze offsets must become active beyond the existing quantization threshold',
          },
        )
        .toBe(true)
    }
    await page.getByRole('button', { name: 'Customize', exact: true }).click()
    await page.getByRole('button', { name: 'Dark Lounge', exact: true }).click()
    await page.getByLabel('Background blur', { exact: true }).fill('8')
    await page.getByLabel('Smile', { exact: true }).fill('0.5')
    await page.getByLabel('Raise eyebrows', { exact: true }).fill('0.5')
    await expect
      .poll(() => page.evaluate(() => window.__demoReview.uniforms.expressionAmount))
      .toEqual([0.5, 0.5])
    await page.getByLabel('Natural movement', { exact: true }).uncheck()
    await page.getByRole('button', { name: 'Close customization', exact: true }).click()
    await expect(page.getByTestId('background-layer')).toHaveCSS('filter', 'blur(8px)')
    await page.screenshot({
      path: resolve(evidence, `protected-${engine}${mobile ? '-mobile' : ''}.png`),
      fullPage: true,
    })
    if (await page.evaluate(() => typeof AudioContext !== 'undefined')) {
      await page.getByRole('button', { name: 'Speak', exact: true }).click()
      await expect(page.getByRole('button', { name: 'Pause', exact: true })).toBeVisible()
      await page.getByRole('button', { name: 'Pause', exact: true }).click()
      await expect(page.getByRole('button', { name: 'Resume', exact: true })).toBeVisible()
      await page.getByRole('button', { name: 'Resume', exact: true }).click()
      await page.getByRole('button', { name: 'Stop', exact: true }).click()
      await page.getByRole('button', { name: 'Speak', exact: true }).click()
      await expect(page.getByRole('button', { name: 'Pause', exact: true })).toBeVisible()
      assert.equal(state.syntheses, 1, 'A repeated utterance must reuse the client cache')
      await page.getByRole('button', { name: 'Chat', exact: true }).click()
      await ready(page)
      await expect(page.getByRole('button', { name: 'Pause', exact: true })).toHaveCount(0)
    } else
      results.skipped.push(
        `${engine}: Windows WebKit audio playback unavailable; typed reply checked`,
      )
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    clean(state)
    results.scenarios.push({
      engine,
      mobile,
      checks:
        'query isolation, Syntelle, head/gaze, expressions, customization, chat/read-aloud, cache/cancellation',
      api_requests: state.requests.filter((request) => request.path.startsWith('/api/')),
    })
  } finally {
    await context.close()
  }
}

async function reducedAndIntegrity(browser, url) {
  let item = await fixturePage(browser, url, { reduced: true, unavailable: true })
  try {
    await item.page.goto(url)
    await ready(item.page)
    await item.page.getByRole('button', { name: 'Read aloud', exact: true }).click()
    await item.page.getByLabel('Text to speak', { exact: true }).fill('Editing remains available.')
    await expect(item.page.getByRole('button', { name: 'Speak', exact: true })).toBeDisabled()
    assert.deepEqual(await item.page.evaluate(() => window.__demoReview.uniforms.angles), [0, 0, 0])
    assert.equal(item.state.syntheses, 0)
    clean(item.state)
    results.scenarios.push({ checks: 'reduced motion and unavailable services' })
  } finally {
    await item.context.close()
  }
  for (const kind of ['checksum', 'build-binding']) {
    item = await fixturePage(browser, url)
    try {
      if (kind === 'checksum')
        await item.page.route('**/*.bin', (route) =>
          route.fulfill({ body: Buffer.alloc(64), contentType: 'application/octet-stream' }),
        )
      else {
        const chunk = reports.protected.chunks.find((chunk) =>
          chunk.modules.some((module) => module.includes('syntelle:character/syntelle')),
        )
        assert(chunk)
        await item.page.route(`**/${chunk.file}`, async (route) => {
          const response = await route.fetch()
          const text = await response.text(),
            changed = text.replace(/oral_build_id:[^,}]+/, 'oral_build_id:"mismatch"')
          if (changed === text) {
            item.state.unexpected.push('The build-binding fixture did not alter the manifest')
            return route.abort()
          }
          await route.fulfill({ response, body: changed })
        })
      }
      await item.page.goto(url)
      // The workspace intentionally presents a generic loading error. The two
      // cases alter only a bound checksum resource or the oral-build identity.
      await expect(item.page.getByRole('alert')).toContainText(/couldn.t load/, { timeout: 15000 })
      await expect(
        item.page.getByRole('img', { name: 'Animated character', exact: true }),
      ).toHaveCount(0)
      clean(item.state)
      results.scenarios.push({ checks: `${kind} rejection in protected runtime` })
    } finally {
      await item.context.close()
    }
  }
}

async function measure(browser, engine, mode, url) {
  const { context, page, state } = await fixturePage(browser, url, { duration: 30 })
  try {
    await page.goto(url)
    await ready(page)
    await page.getByRole('button', { name: 'Read aloud', exact: true }).click()
    await page
      .getByLabel('Text to speak', { exact: true })
      .fill('The same local audio and viseme fixture drives both builds.')
    if (await page.evaluate(() => typeof AudioContext !== 'undefined')) {
      await page.getByRole('button', { name: 'Speak', exact: true }).click()
      await expect(page.getByRole('button', { name: 'Pause', exact: true })).toBeVisible()
    }
    await page.waitForTimeout(5000)
    const timings = await page.evaluate(
      () =>
        new Promise((done) => {
          const frames = [],
            started = performance.now()
          let previous
          const state = window.__demoReview
          state.bodyFrames = []
          state.passes = 0
          state.measure = true
          function frame(now) {
            if (previous !== undefined) frames.push(now - previous)
            previous = now
            if (now - started < 20000) return requestAnimationFrame(frame)
            state.measure = false
            done({ frames, duration: now - started, bodyFrames: state.bodyFrames })
          }
          requestAnimationFrame(frame)
        }),
    )
    const sorted = [...timings.frames].sort((a, b) => a - b)
    const result = {
      engine,
      mode,
      frames: timings.frames.length,
      body_frames: timings.bodyFrames.length,
      fps: (1000 * timings.frames.length) / timings.frames.reduce((a, b) => a + b, 0),
      median_frame_ms: sorted[Math.floor(sorted.length * 0.5)],
      p95_frame_ms: sorted[Math.floor(sorted.length * 0.95)],
      duration_ms: timings.duration,
      requests: state.requests.length,
      request_types: Object.fromEntries(
        [...new Set(state.requests.map((request) => request.type))]
          .sort()
          .map((type) => [type, state.requests.filter((request) => request.type === type).length]),
      ),
      api_paths: state.requests
        .filter((request) => request.path.startsWith('/api/'))
        .map((request) => request.path)
        .sort(),
      audio_playback: await page.evaluate(() => typeof AudioContext !== 'undefined'),
      asset_encoded_bytes: await page.evaluate(() =>
        [...performance.getEntriesByType('resource'), ...performance.getEntriesByType('navigation')]
          .filter((entry) => !new URL(entry.name).pathname.startsWith('/api/'))
          .reduce((sum, entry) => sum + entry.encodedBodySize, 0),
      ),
    }
    assert(timings.bodyFrames.length > 300, 'Measure actual active rendering, not an empty page')
    clean(state)
    results.performance.push(result)
    console.log(
      JSON.stringify({
        engine,
        mode,
        fps: result.fps,
        p95_frame_ms: result.p95_frame_ms,
        requests: result.requests,
      }),
    )
  } finally {
    await context.close()
  }
}

try {
  for (const [index, mode] of ['baseline', 'protected'].entries()) {
    const server = await preview({
      configFile: resolve(root, 'vite.config.ts'),
      mode: `demo-${mode}`,
      preview: { host: '127.0.0.1', port: 5181 + index, strictPort: true, proxy: {} },
    })
    servers.push(server)
  }
  for (const [engine, type] of Object.entries({ chromium, webkit })) {
    const browser = await type.launch({ headless: true })
    browsers.push(browser)
    if (!performanceOnly) {
      await smoke(browser, engine, 'http://127.0.0.1:5182')
      if (engine === 'chromium') {
        await smoke(browser, engine, 'http://127.0.0.1:5182', true)
        await reducedAndIntegrity(browser, 'http://127.0.0.1:5182')
      }
      console.log(`${engine}: focused browser checks passed`)
    }
    if (!smokeOnly) {
      for (const [index, mode] of ['baseline', 'protected'].entries())
        await measure(browser, engine, mode, `http://127.0.0.1:${5181 + index}`)
      const [baseline, protectedResult] = results.performance.filter(
        (result) => result.engine === engine,
      )
      assert.deepEqual(protectedResult.request_types, baseline.request_types)
      assert.deepEqual(protectedResult.api_paths, baseline.api_paths)
      assert(
        protectedResult.body_frames / protectedResult.duration_ms >=
          (baseline.body_frames / baseline.duration_ms) * 0.95,
        `${engine}: body draw submission rate regressed by more than 5%`,
      )
      assert(
        protectedResult.fps >= baseline.fps * 0.95,
        `${engine}: frame-rate regression exceeds 5%`,
      )
      assert(
        protectedResult.p95_frame_ms <= baseline.p95_frame_ms + 2,
        `${engine}: p95 regression exceeds 2 ms`,
      )
    }
    await browser.close()
    browsers.pop()
  }
  const originals = JSON.parse(readFileSync(resolve(evidence, 'original-assets.json'), 'utf8'))
  for (const file of originals)
    assert.equal(
      createHash('sha256')
        .update(readFileSync(resolve(root, '../..', file.file)))
        .digest('hex'),
      file.sha256,
      `Frozen resource changed: ${file.file}`,
    )
  results.original_files_verified = originals.length
  results.downloads = Object.fromEntries(
    Object.entries(reports).map(([mode, report]) => [
      mode,
      {
        files: report.files.length,
        bytes: report.files.reduce((sum, file) => sum + file.bytes, 0),
        gzip_bytes: report.files.reduce((sum, file) => sum + file.gzip_bytes, 0),
        javascript_gzip_bytes: report.files
          .filter((file) => file.file.endsWith('.js'))
          .reduce((sum, file) => sum + file.gzip_bytes, 0),
      },
    ]),
  )
  results.passed = true
} catch (error) {
  results.passed = false
  results.failures.push(String(error.stack ?? error))
  process.exitCode = 1
  console.error(error)
} finally {
  writeFileSync(
    resolve(evidence, `review-attempt-${Date.now()}.json`),
    JSON.stringify(results, null, 2),
  )
  writeFileSync(
    resolve(
      evidence,
      performanceOnly
        ? 'performance-review.json'
        : smokeOnly
          ? 'browser-review.json'
          : 'review.json',
    ),
    JSON.stringify(results, null, 2),
  )
  for (const browser of browsers) await browser.close()
  for (const server of servers)
    await new Promise((done, reject) =>
      server.httpServer.close((error) => (error ? reject(error) : done())),
    )
}
