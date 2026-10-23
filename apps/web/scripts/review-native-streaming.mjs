// Bounded integration proof: real Vue player, real Go fixture API and native media.
// It uses only cached speech, a verified isolated PostgreSQL database and a local
// static player. No login/provider/microphone/concurrency matrix runs here.
import assert from 'node:assert/strict'
import { createServer } from 'node:http'
import { createReadStream } from 'node:fs'
import { mkdir, stat, writeFile } from 'node:fs/promises'
import { resolve, dirname, extname, sep } from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright'

const argument = (name, fallback) => {
  const index = process.argv.indexOf(name)
  return index < 0 ? fallback : process.argv[index + 1]
}
const base = argument('--base', 'http://127.0.0.1:5173')
const fixture = argument('--fixture', 'http://127.0.0.1:8083')
const playerDirectory = resolve(
  argument(
    '--player-directory',
    resolve(dirname(fileURLToPath(import.meta.url)), '../dist-stream-player'),
  ),
)
const output = resolve(argument('--output', '../../.tools/native-efficiency-57/player'))
const reconnectCount = Number(argument('--reconnect-count', '2'))
assert.ok(Number.isInteger(reconnectCount) && reconnectCount >= 1 && reconnectCount <= 3)
for (const url of [base, fixture]) {
  const value = new URL(url)
  if (value.protocol !== 'http:' || !['127.0.0.1', 'localhost'].includes(value.hostname))
    throw Error('Native player proof requires loopback-only fixture URLs.')
}
const origin = new URL(base).origin
assert.equal(origin, 'http://127.0.0.1:5173', 'Keep the existing private API Origin policy.')
await mkdir(output, { recursive: true })
const report = {
  version: 'native-vue-player-fixture/1',
  fixture: true,
  implementation: 'native-cpu/1',
  paid_provider_calls: 0,
  checks: [],
  browser_errors: [],
  private_resource_requests: 0,
  transport_trace: [],
}
const trace = (kind, values = {}) =>
  report.transport_trace.push({ ms: Math.round(performance.now()), kind, ...values })
const bridges = new Set()
let websocketConnections = 0
let context, page, browser, sessionID
let webServer
async function fixtureCounts() {
  const response = await fetch(`${fixture}/__fixture__/counts`, {
    headers: { Authorization: 'Bearer syntelle-offline-control-fixture' },
  })
  assert.equal(response.status, 200)
  return response.json()
}
async function waitUntil(action, predicate, milliseconds = 15000) {
  const end = performance.now() + milliseconds
  while (performance.now() < end) {
    const value = await action()
    if (predicate(value)) return value
    await new Promise((resolve) => setTimeout(resolve, 150))
  }
  throw Error('Bounded native player condition timed out.')
}
async function waitForVideo() {
  await page.waitForFunction(
    () => {
      const video = document.querySelector('video')
      return (
        video?.readyState >= 2 &&
        video.videoWidth === 720 &&
        video.videoHeight === 960 &&
        video.srcObject?.getVideoTracks().some((track) => track.readyState === 'live')
      )
    },
    undefined,
    { timeout: 30000 },
  )
}
async function mediaCounters() {
  return page.evaluate(async () => {
    const video = document.querySelector('video')
    const peer = window.__nativeReviewPeers?.at(-1)
    const stats = peer ? [...(await peer.getStats()).values()] : []
    const inbound = stats.filter((value) => value.type === 'inbound-rtp')
    const codecs = inbound
      .map((value) => stats.find((codec) => codec.id === value.codecId)?.mimeType)
      .filter(Boolean)
    return {
      frames: video.getVideoPlaybackQuality().totalVideoFrames,
      rvfc: window.__nativeReviewFrames?.length ?? 0,
      frame_samples: window.__nativeReviewFrames ?? [],
      decoded: inbound.find((value) => value.kind === 'video')?.framesDecoded ?? 0,
      decoded_drops: inbound.find((value) => value.kind === 'video')?.framesDropped ?? 0,
      stamp: performance.now(),
      size: [video.videoWidth, video.videoHeight],
      codecs,
      connection: peer?.connectionState,
    }
  })
}
async function status() {
  const response = await context.request.get(`${fixture}/api/v1/streams/status`, {
    headers: { Origin: origin },
  })
  assert.equal(response.status(), 200)
  return response.json()
}
async function removeSession() {
  if (!sessionID || !context) return
  const response = await context.request.delete(`${fixture}/api/v1/streams/${sessionID}`, {
    headers: { Origin: origin, 'X-Syntelle-Request': '1' },
  })
  assert.ok([204, 404].includes(response.status()))
  sessionID = undefined
}
try {
  // Serving the actual public build avoids changing Vite or production Origin
  // rules. The API, worker and browser can share an unpublished namespace.
  const types = {
    '.html': 'text/html',
    '.js': 'text/javascript',
    '.css': 'text/css',
    '.svg': 'image/svg+xml',
    '.png': 'image/png',
    '.webp': 'image/webp',
    '.woff2': 'font/woff2',
    '.json': 'application/json',
  }
  webServer = createServer(async (request, response) => {
    try {
      const path = decodeURIComponent(new URL(request.url, base).pathname)
      const target = resolve(playerDirectory, `.${path === '/' ? '/index.html' : path}`)
      if (
        !['GET', 'HEAD'].includes(request.method) ||
        !target.startsWith(playerDirectory + sep) ||
        !(await stat(target)).isFile()
      ) {
        response.writeHead(404).end()
        return
      }
      response.writeHead(200, {
        'Content-Type': types[extname(target)] ?? 'application/octet-stream',
        'Cache-Control': 'no-store',
      })
      if (request.method === 'HEAD') response.end()
      else createReadStream(target).pipe(response)
    } catch {
      response.writeHead(404).end()
    }
  })
  await new Promise((resolve, reject) => {
    webServer.once('error', reject)
    webServer.listen(5173, '127.0.0.1', resolve)
  })
  report.counts_before = await fixtureCounts()
  assert.deepEqual(report.counts_before, { model: 0, speech: 0, recognition: 0 })
  browser = await chromium.launch({
    headless: true,
    args: [
      '--autoplay-policy=no-user-gesture-required',
      '--disable-features=WebRtcHideLocalIpsWithMdns',
    ],
  })
  context = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
  await context.addInitScript(() => {
    const Peer = window.RTCPeerConnection
    window.__nativeReviewPeers = []
    window.RTCPeerConnection = new Proxy(Peer, {
      construct(Target, args) {
        const peer = new Target(...args)
        window.__nativeReviewPeers.push(peer)
        return peer
      },
    })
  })
  await context.route('**/api/v1/**', async (route) => {
    const url = new URL(route.request().url())
    try {
      const response = await route.fetch({ url: `${fixture}${url.pathname}${url.search}` })
      trace('http', {
        method: route.request().method(),
        // Session IDs, tickets and credential payloads are deliberately absent.
        endpoint: url.pathname.replace(/\/streams\/[^/]+/, '/streams/:id'),
        status: response.status(),
      })
      await route.fulfill({ response })
    } catch {
      await route.abort().catch(() => undefined)
    }
  })
  await context.routeWebSocket(/\/api\/v1\/streams\/[^/]+\/ws$/, (route) => {
    websocketConnections++
    const connection = websocketConnections
    trace('socket_opening', { connection })
    const url = new URL(route.url())
    const upstreamURL = new URL(url.pathname, fixture)
    upstreamURL.protocol = 'ws:'
    const upstream = new WebSocket(upstreamURL, { headers: { Origin: origin } })
    const bridge = { upstream, route, closed: false }
    bridges.add(bridge)
    const queue = []
    route.onMessage((message) => {
      try {
        const event = JSON.parse(message)
        trace('player_event', { connection, type: event.type })
      } catch {
        /* Never record a raw payload. */
      }
      if (upstream.readyState === WebSocket.OPEN) upstream.send(message)
      else queue.push(message)
    })
    upstream.addEventListener('open', () => {
      for (const message of queue) upstream.send(message)
      queue.length = 0
      trace('upstream_open', { connection })
    })
    upstream.addEventListener('message', ({ data }) => {
      if (typeof data !== 'string' || bridge.closed) return
      try {
        const event = JSON.parse(data)
        trace('server_event', {
          connection,
          type: event.type,
          ...(event.type === 'state' ? { state: event.data?.state } : {}),
        })
        if (event.type === 'ready') report.renderer = event.data?.renderer
      } catch {
        /* The actual client owns protocol validation. */
      }
      route.send(data)
    })
    upstream.addEventListener('close', (event) => {
      trace('upstream_close', { connection, code: event.code })
      bridge.closed = true
      bridges.delete(bridge)
      void route.close()
    })
    upstream.addEventListener('error', () => {
      void route.close({ code: 1011 })
    })
    route.onClose(() => {
      trace('player_close', { connection })
      bridge.closed = true
      upstream.close()
      bridges.delete(bridge)
    })
  })
  page = await context.newPage()
  page.on('pageerror', (error) => report.browser_errors.push(error.message.split('\n')[0]))
  page.on('request', (request) => {
    if (/\/experiments\/|\/render-worker\/|projection\.bin/.test(request.url()))
      report.private_resource_requests++
  })
  const begin = performance.now()
  await page.goto(`${base}/?view=stream`)
  await waitForVideo()
  await page.evaluate(() => {
    const video = document.querySelector('video')
    window.__nativeReviewFrames = []
    function frame(now, metadata) {
      window.__nativeReviewFrames.push({
        now,
        expected: metadata.expectedDisplayTime,
        media: metadata.mediaTime,
        presented: metadata.presentedFrames,
        processing: metadata.processingDuration,
      })
      video.requestVideoFrameCallback(frame)
    }
    video.requestVideoFrameCallback(frame)
  })
  sessionID = await page.getByTestId('stream-workspace').getAttribute('data-stream-id')
  assert.ok(sessionID)
  report.startup_ms = performance.now() - begin
  const before = await mediaCounters()
  await page.waitForTimeout(1500)
  const after = await mediaCounters()
  report.received_idle_fps = (after.frames - before.frames) / 1.5
  assert.ok(after.frames > before.frames + 20, 'Actual idle media must keep advancing.')
  assert.equal(after.connection, 'connected')
  assert.deepEqual(after.size, [720, 960])
  assert.ok(after.codecs.includes('video/H264'))
  report.checks.push(
    'actual public Vue player receives native H264 at720x960 through the real Go visitor session API',
  )
  // The current public player is microphone-only. Submit the existing public
  // speech envelope through the authenticated fixture bridge without restoring
  // retired text controls or starting a real microphone/provider.
  report.command_submission = 'public WebSocket speech envelope; actual microphone-only player receives media and state'
  await page.evaluate(async () => {
    const context = new AudioContext(),
      analyser = context.createAnalyser(),
      mute = context.createGain()
    mute.gain.value = 0
    const input = context.createMediaStreamSource(document.querySelector('video').srcObject)
    input.connect(analyser).connect(mute).connect(context.destination)
    await context.resume()
    const state = { context, input, peak: 0, current: 0 }
    window.__nativeReviewAudio = state
    const data = new Float32Array(analyser.fftSize)
    function sample() {
      analyser.getFloatTimeDomainData(data)
      state.current = Math.sqrt(data.reduce((sum, v) => sum + v * v, 0) / data.length)
      state.peak = Math.max(state.peak, ...data.map(Math.abs))
      if (context.state !== 'closed') requestAnimationFrame(sample)
    }
    sample()
  })
  const speechBridge = [...bridges].find((bridge) => !bridge.closed && bridge.upstream.readyState === WebSocket.OPEN)
  assert.ok(speechBridge, 'The actual player must authenticate its public stream before speech.')
  speechBridge.upstream.send(JSON.stringify({ type: 'command', session_id: sessionID,
    command_id: crypto.randomUUID(), data: { type: 'speak', text: 'Offline native streaming fixture.' } }))
  await page.getByRole('button', { name: 'Stop reply', exact: true }).waitFor({ timeout: 15000 })
  await page.waitForFunction(() => window.__nativeReviewAudio?.peak > 0.01, undefined, {
    timeout: 15000,
  })
  await page.waitForTimeout(200)
  const speechBefore = await mediaCounters()
  await page.waitForTimeout(1500)
  const speechAfter = await mediaCounters()
  const speechSeconds = (speechAfter.stamp - speechBefore.stamp) / 1000
  report.speech_cadence = {
    seconds: speechSeconds,
    video_quality_fps: (speechAfter.frames - speechBefore.frames) / speechSeconds,
    rvfc_fps: (speechAfter.rvfc - speechBefore.rvfc) / speechSeconds,
    decoded_fps: (speechAfter.decoded - speechBefore.decoded) / speechSeconds,
    decoded_drops: speechAfter.decoded_drops - speechBefore.decoded_drops,
    frames: speechAfter.frame_samples.slice(speechBefore.frame_samples.length),
  }
  for (const field of ['video_quality_fps', 'rvfc_fps', 'decoded_fps'])
    assert.ok(report.speech_cadence[field] >= 28, `${field} must sustain the existing 28fps gate.`)
  assert.equal(report.speech_cadence.decoded_drops, 0)
  report.decoded_audio_peak = await page.evaluate(() => window.__nativeReviewAudio.peak)
  const spoken = await mediaCounters()
  assert.ok(spoken.codecs.includes('audio/opus'))
  await page.screenshot({ path: resolve(output, 'native-player-speaking.png') })
  report.checks.push(
    'visitor literal-speech command reaches cached fixture speech through Go and native RTC; WebAudio verifies decoded Opus samples',
  )
  const interrupted = performance.now()
  await page.getByRole('button', { name: 'Stop reply', exact: true }).click()
  report.interrupt_control_ms = performance.now() - interrupted
  await page.waitForTimeout(250)
  await page.waitForFunction(() => window.__nativeReviewAudio?.current < 0.002, undefined, {
    timeout: 3000,
  })
  report.decoded_silence_after_interrupt_ms = performance.now() - interrupted
  report.counts_after_speech = await fixtureCounts()
  assert.deepEqual(report.counts_after_speech, { model: 0, speech: 1, recognition: 0 })
  report.checks.push(
    'the actual Stop reply control makes the interrupted native source silent',
  )
  await page.evaluate(async () => {
    window.__nativeReviewAudio.input.disconnect()
    await window.__nativeReviewAudio.context.close()
  })
  report.reconnects = []
  for (let attempt = 0; attempt < reconnectCount; attempt++) {
    // Every injected loss must pass; this is not a retry of a failed assertion.
    const oldID = sessionID,
      oldConnections = websocketConnections,
      started = performance.now()
    trace('reconnect_injected', { attempt: attempt + 1 })
    for (const bridge of [...bridges])
      await bridge.route.close({ code: 1012, reason: 'Offline reconnect fixture' })
    await waitUntil(
      async () => websocketConnections,
      (value) => value > oldConnections,
    )
    await waitForVideo()
    await page.waitForFunction(
      () => window.__nativeReviewPeers?.at(-1)?.connectionState === 'connected',
      undefined,
      { timeout: 20000 },
    )
    assert.equal(await page.getByTestId('stream-workspace').getAttribute('data-stream-id'), oldID)
    const recovered = performance.now() - started
    const resumedBefore = await mediaCounters()
    await page.waitForTimeout(1000)
    const resumedAfter = await mediaCounters()
    const seconds = (resumedAfter.stamp - resumedBefore.stamp) / 1000
    report.received_reconnected_fps = (resumedAfter.frames - resumedBefore.frames) / seconds
    assert.ok(report.received_reconnected_fps >= 28)
    report.reconnects.push({ recovered_ms: recovered, fps: report.received_reconnected_fps })
    report.counts_after_reconnect = await fixtureCounts()
    assert.deepEqual(report.counts_after_reconnect, report.counts_after_speech)
  }
  report.checks.push(
    'transport reconnection reattaches the same Go session, restores actual video and makes no speech/model retry',
  )
  await page.screenshot({ path: resolve(output, 'native-player-reconnected.png') })
  await removeSession()
  const final = await waitUntil(status, (value) => value.active === 0)
  assert.equal(final.active, 0)
  report.checks.push('explicit stop releases the Go stream session')
  assert.deepEqual(report.browser_errors, [])
  assert.equal(report.private_resource_requests, 0)
  report.passed = true
} catch (error) {
  trace('assertion_failed')
  report.passed = false
  report.failure = error.message.split('\n')[0]
  if (page)
    await page
      .screenshot({ path: resolve(output, 'native-player-failure.png') })
      .catch(() => undefined)
  throw error
} finally {
  await removeSession().catch(() => undefined)
  for (const bridge of bridges) bridge.upstream.close()
  await context?.close().catch(() => undefined)
  await browser?.close()
  webServer?.closeAllConnections()
  if (webServer) await new Promise((resolve) => webServer.close(resolve))
  if (process.argv.includes('--stop-fixture')) {
    trace('fixture_stop_requested')
    const response = await fetch(`${fixture}/__fixture__/stop`, {
      headers: { Authorization: 'Bearer syntelle-offline-control-fixture' },
    }).catch(() => undefined)
    report.fixture_stop_requested = response?.status === 204
  }
  await writeFile(
    resolve(output, 'native-player-review.json'),
    JSON.stringify(report, null, 2) + '\n',
  )
  process.stdout.write(
    JSON.stringify({ ...report, speech_cadence: { ...report.speech_cadence, frames: undefined } }) +
      '\n',
  )
}
