// Offline receivers run in a different Linux container from the limited worker.
import { createRequire } from 'node:module'
import { readFile, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
const requireWeb = createRequire(
  new URL('../../../apps/web/package.json', import.meta.url),
)
const { chromium } = requireWeb('playwright')
const args = process.argv.slice(2)
const option = (name, fallback) => {
  const index = args.indexOf(name)
  return index < 0 ? fallback : args[index + 1]
}
const base = option('--host', 'http://worker:3000')
const count = Number(option('--sessions', '1'))
if (![1, 2, 4].includes(count))
  throw Error('Bounded probe supports 1, 2 or 4 sessions')
const output = resolve(option('--output', '/results/cpu-review.json'))
const speechMilliseconds = Number(option('--speech-ms', '3500'))
const speechParts = Number(option('--speech-parts', '1'))
if (
  !Number.isInteger(speechMilliseconds) ||
  speechMilliseconds < 3500 ||
  speechMilliseconds > 60000 ||
  !Number.isInteger(speechParts) ||
  speechParts < 1 ||
  speechParts > 32
)
  throw Error('Offline speech replay exceeds its bounded duration or queue')
const speech = JSON.parse(await readFile(option('--speech-file','/work/cached-speech.json'), 'utf8'))
const command = (value) => request('/command', value)
async function request(path, value) {
  const response = await fetch(
    base + path,
    value === undefined
      ? undefined
      : {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify(value),
        },
  )
  const data = await response.json()
  if (!response.ok) throw Error(data.error ?? 'Probe host failed')
  return data
}
let health
for (let attempt = 0; attempt < 20; attempt++) {
  try {
    health = await request('/health')
    break
  } catch (error) {
    if (attempt === 19) throw error
    await new Promise((resolve) => setTimeout(resolve, 250))
  }
}
const renderer = option('--renderer', 'native')
if (renderer !== 'native')
  throw Error('Only the native Go render worker is supported')
if (!(health.cpu_only === true && health.renderer === 'native-cpu/1') &&
    !['', 'auto', 'cpu', 'gpu'].includes(health.renderer_configured))
  throw Error('Expected native renderer configuration not established')
// Chromium is the receiving test browser; rendering and encoding run in Go.
const browser = await chromium.launch({
  headless: true,
  ...(args.includes('--hardware-browser')?{channel:'chromium'}:{}),
  args: [
    ...(args.includes('--hardware-browser')?['--enable-gpu','--use-angle=d3d11']:[]),
    '--autoplay-policy=no-user-gesture-required',
    '--disable-features=WebRtcHideLocalIpsWithMdns',
  ],
})
const pages = [],
  ids = [],
  errors = []
const runID = crypto.randomUUID().slice(0, 8)
let cursor = 0,
  stopping = false,
  poll
async function relay() {
  while (!stopping) {
    const events = await request(`/events?after=${cursor}`)
    for (const event of events) {
      cursor = Math.max(cursor, event.cursor)
      if (event.type === 'error') errors.push(event.data?.message)
      if (event.type === 'signal') {
        const index = ids.indexOf(event.session_id)
        if (index >= 0)
          await pages[index].evaluate(
            (value) => window.signal(value),
            event.data,
          )
      }
    }
    await new Promise((resolve) => setTimeout(resolve, 15))
  }
}
try {
  for (let index = 0; index < count; index++) {
    const id = `bounded_cpu_${runID}_${index}`
    ids.push(id)
    const context = await browser.newContext({
      viewport: { width: 720, height: 960 },
    })
    const page = await context.newPage()
    pages.push(page)
    page.on('pageerror', (error) => errors.push(error.message))
    await page.exposeBinding('sendSignal', (_source, data) =>
      command({ type: 'signal', session_id: id, data }),
    )
    await page.setContent(
      '<style>body{margin:0}video{width:720px;height:960px}</style><video muted autoplay playsinline></video>',
    )
    await page.evaluate(() => {
      let peer = new RTCPeerConnection({ iceServers: [] })
      const video = document.querySelector('video')
      const audio = new AudioContext({ sampleRate: 48000 }),
        analyser = audio.createAnalyser(),
        mute = audio.createGain()
      mute.gain.value = 0
      analyser.fftSize = 256
      analyser.connect(mute).connect(audio.destination)
      const samples = new Float32Array(analyser.fftSize)
      const state = { times: [], peak: 0 }
      const timingCanvas = document.createElement('canvas')
      timingCanvas.width = timingCanvas.height = 32
      const timingContext = timingCanvas.getContext('2d', {
        willReadFrequently: true,
      })
      function listen() {
        analyser.getFloatTimeDomainData(samples)
        const rms = Math.sqrt(
          samples.reduce((sum, value) => sum + value * value, 0) /
            samples.length,
        )
        state.instant_audio = rms
        if (state.timing && state.timing.audio === null && rms > 0.03)
          state.timing.audio = performance.now()
        state.peak = Math.max(
          state.peak,
          Math.sqrt(
            samples.reduce((sum, value) => sum + value * value, 0) /
              samples.length,
          ),
        )
        requestAnimationFrame(listen)
      }
      listen()
      function frame(now) {
        state.times.push(now)
        if (state.timing && state.timing.video === null) {
          timingContext.drawImage(video, 0, 0, 32, 32, 0, 0, 32, 32)
          const pixel = timingContext.getImageData(16, 16, 1, 1).data
          if (pixel[0] > 180 && pixel[1] < 100 && pixel[2] > 160)
            state.timing.video = now
        }
        video.requestVideoFrameCallback(frame)
      }
      video.requestVideoFrameCallback(frame)
      peer.ontrack = ({ streams }) => {
        video.srcObject = streams[0]
        if (streams[0].getAudioTracks().length) {
          audio.createMediaStreamSource(streams[0]).connect(analyser)
          void audio.resume()
        }
        void video.play().catch((error) => {
          if (error.name !== 'AbortError') throw error
        })
      }
      peer.onicecandidate = ({ candidate }) =>
        void window.sendSignal({ candidate: candidate?.toJSON() ?? null })
      const pending = []
      window.resetPeer = () => {
        const track = peer.ontrack,
          ice = peer.onicecandidate
        peer.ontrack = peer.onicecandidate = null
        peer.close()
        peer = new RTCPeerConnection({ iceServers: [] })
        peer.ontrack = track
        peer.onicecandidate = ice
        pending.splice(0)
      }
      window.signal = async ({ description, candidate }) => {
        if (description) {
          state.signal_stage = 'remote-offer'
          await peer.setRemoteDescription(description)
          state.signal_stage = 'local-answer'
          await peer.setLocalDescription(await peer.createAnswer())
          state.signal_stage = 'send-answer'
          await window.sendSignal({
            description: peer.localDescription.toJSON(),
          })
          state.signal_stage = 'answered'
          for (const item of pending.splice(0))
            await peer.addIceCandidate(item ?? undefined)
        }
        if (candidate !== undefined) {
          if (peer.remoteDescription)
            await peer.addIceCandidate(candidate ?? undefined)
          else pending.push(candidate)
        }
      }
      window.probe = state
      window.snapshot = async () => {
        const values = [...(await peer.getStats()).values()]
        const videoStats = values.find(
          (value) => value.type === 'inbound-rtp' && value.kind === 'video',
        )
        return {
          ...state,
          connection: peer.connectionState,
          ice: peer.iceConnectionState,
          signaling: peer.signalingState,
          gathering: peer.iceGatheringState,
          video: videoStats,
          codec: values.find((value) => value.id === videoStats?.codecId)
            ?.mimeType,
          size: [video.videoWidth, video.videoHeight],
        }
      }
    })
  }
  poll = relay().catch((error) => {
    errors.push('Relay: ' + error.message)
  })
  for (const id of ids)
    await command({
      type: 'create',
      session_id: id,
      data: {
        settings: { character: 'syntelle', background: 'dark-lounge', blur: 0 },
      },
    })
  for (const page of pages)
    await page.waitForFunction(
      () => window.probe.times.length >= 15,
      undefined,
      { timeout: 15000 },
    )
  const snapshots = () =>
    Promise.all(pages.map((page) => page.evaluate(() => window.snapshot())))
  async function sample(label, milliseconds) {
    const before = {
      host: await request('/snapshot'),
      receivers: await snapshots(),
      time: performance.now(),
    }
    await new Promise((resolve) => setTimeout(resolve, milliseconds))
    const after = {
      host: await request('/snapshot'),
      receivers: await snapshots(),
      time: performance.now(),
    }
    const receiverSeconds = (after.time - before.time) / 1000
    const seconds = Number.isFinite(after.host.raw_monotonic_ns) && Number.isFinite(before.host.raw_monotonic_ns) ? (after.host.raw_monotonic_ns-before.host.raw_monotonic_ns)/1e9 : receiverSeconds
    const usage = (value, key) =>
      Number(
        value
          .split('\n')
          .find((line) => line.startsWith(key + ' '))
          ?.split(' ')[1] ?? 0,
      )
    const delta = (current, previous) =>
      Number.isFinite(current) &&
      Number.isFinite(previous) &&
      current >= previous
        ? current - previous
        : null
    const average = (total, frames) =>
      total !== null && frames !== null && frames > 0 ? total / frames : null
    return {
      label,
      seconds,
      measurement_clock: after.host.raw_monotonic_ns ? 'worker CLOCK_MONOTONIC_RAW; cgroup CPU / raw seconds' : 'receiver monotonic',
      worker_cpu_cores:
        (usage(after.host.cgroup.cpu, 'usage_usec') -
          usage(before.host.cgroup.cpu, 'usage_usec')) /
        1e6 /
        seconds,
      throttled_seconds:
        (usage(after.host.cgroup.cpu, 'throttled_usec') -
          usage(before.host.cgroup.cpu, 'throttled_usec')) /
        1e6,
      worker_memory: after.host.cgroup.memory,
      worker_peak_memory: after.host.cgroup.peak,
      cgroup: after.host.cgroup,
      renderers: after.host.pages.map((value, index) => {
        const previous = before.host.pages[index]
        const video = value.reports.filter(
          (report) => report.type === 'outbound-rtp' && report.kind === 'video',
        )
        const previousVideo = previous.reports.find(
          (report) => report.type === 'outbound-rtp' && report.kind === 'video',
        )
        const captured = delta(value.captured_frames, previous.captured_frames)
        const rendered =
          delta(value.render_stage_frames, previous.render_stage_frames) ??
          captured
        const encoded = delta(
          video[0]?.framesEncoded,
          previousVideo?.framesEncoded,
        )
        const audioFrames = delta(value.audio_frames, previous.audio_frames)
        const renderMS = delta(value.render_total_ms, previous.render_total_ms)
        const videoMS = delta(
          value.video_encode_total_ms,
          previous.video_encode_total_ms,
        )
        const audioMS = delta(
          value.audio_encode_total_ms,
          previous.audio_encode_total_ms,
        )
        return {
          captured_fps: captured === null ? null : captured / seconds,
          device_surface_frames: value.device_surface_frames ?? null,
          repeated_video_timestamps: value.repeated_video_timestamps ?? null,
          video,
          stages: {
            measurement:
              'elapsed-wall-time; not CPU accounting; stages may overlap',
            render_method: 'native-scene-render-call',
            rendered_frames: rendered,
            render_total_ms: renderMS,
            render_ms_per_frame: average(renderMS, rendered),
            video_encoded_frames: encoded,
            video_encode_total_ms: videoMS,
            video_encode_ms_per_frame: average(videoMS, encoded),
            audio_encoded_frames: audioFrames,
            audio_encode_total_ms: audioMS,
            audio_encode_ms_per_frame: average(audioMS, audioFrames),
          },
        }
      }),
      sessions: after.receivers.map((value, index) => {
        const previous = before.receivers[index],
          times = value.times.slice(previous.times.length)
        const gaps = times
          .slice(1)
          .map((value, index) => (value - times[index])*seconds/receiverSeconds)
          .sort((a, b) => a - b)
        return {
          fps: times.length / seconds,
          decoded_fps:
            (value.video.framesDecoded - previous.video.framesDecoded) /
            seconds,
          frame_gap_p95_ms: gaps[Math.floor(gaps.length * 0.95)] ?? 0,
          max_gap_ms: gaps.at(-1) ?? 0,
          video_kbps:
            ((value.video.bytesReceived - previous.video.bytesReceived) * 8) /
            seconds /
            1000,
          dropped_frames:
            value.video.framesDropped - previous.video.framesDropped,
          freezes:
            (value.video.freezeCount ?? 0) - (previous.video.freezeCount ?? 0),
          codec: value.codec,
          size: value.size,
          audio_peak: value.peak,
        }
      }),
    }
  }
  await new Promise((resolve) => setTimeout(resolve, 1000))
  const idle = await sample('idle', 4000)
  for (const [index, id] of ids.entries()) {
    const turn = `cached_${index}`
    await command({
      type: 'conversation',
      session_id: id,
      data: { type: 'accepted', turn_id: turn },
    })
    for (let part = 0; part < speechParts; part++)
      await command({
        type: 'conversation',
        session_id: id,
        data:
          speechParts === 1
            ? {
                type: 'prepared_speech',
                turn_id: turn,
                text: 'Cached offline utterance',
                data: speech,
              }
            : {
                type: 'speech_part',
                turn_id: turn,
                data: { index: part, text: 'Cached offline utterance', speech },
              },
      })
    if (speechParts > 1)
      await command({
        type: 'conversation',
        session_id: id,
        data: { type: 'speech_complete', turn_id: turn, data: { parts: speechParts } },
      })
  }
  const speaking = await sample('simultaneous-cached-speech', speechMilliseconds)
  for (const id of ids) await command({ type: 'interrupt', session_id: id })
  const quality = []
  for (const id of ids)
    await command({
      type: 'configure',
      session_id: id,
      data: {
        paused: true,
        blinking: false,
        movement: false,
        automatic_expressions: false,
      },
    })
  await new Promise((resolve) => setTimeout(resolve, 1000))
  for (const [index, page] of pages.entries()) {
    const source = await request('/source', { index })
    const result = await page.evaluate(async (url) => {
      const original = new Image()
      original.src = url
      await original.decode()
      const canvas = document.createElement('canvas')
      canvas.width = 720
      canvas.height = 960
      const context = canvas.getContext('2d', { willReadFrequently: true })
      context.drawImage(original, 0, 0)
      const a = context.getImageData(0, 0, 720, 960).data
      context.drawImage(document.querySelector('video'), 0, 0)
      const b = context.getImageData(0, 0, 720, 960).data
      let sum = 0,
        count = 0
      for (let y = 180; y < 680; y++)
        for (let x = 180; x < 540; x++)
          for (let channel = 0; channel < 3; channel++) {
            const p = (y * 720 + x) * 4 + channel
            sum += (a[p] - b[p]) ** 2
            count++
          }
      return {
        face_psnr_db: 10 * Math.log10(255 ** 2 / Math.max(sum / count, 1e-10)),
        color_space: (() => {
          if (typeof VideoFrame === 'undefined') return null
          const frame = new VideoFrame(document.querySelector('video'), {
            timestamp: 0,
          })
          try {
            return frame.colorSpace.toJSON()
          } finally {
            frame.close()
          }
        })(),
      }
    }, source)
    quality.push(result)
    await writeFile(
      output.replace(/\.json$/, `-source-${index}.png`),
      Buffer.from(source.split(',')[1], 'base64'),
    )
    await page
      .locator('video')
      .screenshot({ path: output.replace(/\.json$/, `-decoded-${index}.png`) })
  }
  let controls = null
  if (args.includes('--controls')) {
    if (ids.length !== 1)
      throw Error('Offline controls require one native session')
    const id = ids[0],
      page = pages[0]
    await command({
      type: 'configure',
      session_id: id,
      data: {
        paused: false,
        blinking: false,
        movement: false,
        automatic_expressions: false,
      },
    })
    await new Promise((resolve) => setTimeout(resolve, 300))
    await page.evaluate(() => {
      window.probe.timing = { audio: null, video: null }
    })
    await request('/timing', { session_id: id })
    await page.waitForFunction(
      () =>
        window.probe.timing.audio !== null &&
        window.probe.timing.video !== null,
      undefined,
      { timeout: 3000 },
    )
    const timing = await page.evaluate(() => window.probe.timing)
    const offset = timing.video - timing.audio
    await page.evaluate(() => {
      window.probe.timing = null
    })
    const turn = 'cached_controls_' + runID
    await command({
      type: 'conversation',
      session_id: id,
      data: { type: 'accepted', turn_id: turn },
    })
    await command({
      type: 'conversation',
      session_id: id,
      data: {
        type: 'prepared_speech',
        turn_id: turn,
        text: 'Cached offline utterance',
        data: speech,
      },
    })
    await new Promise((resolve) => setTimeout(resolve, 450))
    await command({ type: 'configure', session_id: id, data: { paused: true } })
    const paused = (await request('/snapshot')).pages[0]
    await new Promise((resolve) => setTimeout(resolve, 220))
    const frozen = (await request('/snapshot')).pages[0]
    await command({
      type: 'configure',
      session_id: id,
      data: { paused: false },
    })
    await new Promise((resolve) => setTimeout(resolve, 220))
    const resumed = (await request('/snapshot')).pages[0]
    const interruptStart = performance.now()
    await command({ type: 'interrupt', session_id: id })
    const interruptControlMS = performance.now() - interruptStart
    await page.waitForFunction(
      () => window.probe.instant_audio < 0.002,
      undefined,
      { timeout: 700 },
    )
    const quietMS = performance.now() - interruptStart
    const interrupted = (await request('/snapshot')).pages[0]
    const beforeReconnect = await page.evaluate(() => window.probe.times.length)
    const reconnectStart = performance.now()
    await page.evaluate(() => window.resetPeer())
    await command({
      type: 'connect',
      session_id: id,
      data: { reconnect: true },
    })
    await page.waitForFunction(
      (before) => window.probe.times.length > before + 8,
      beforeReconnect,
      { timeout: 5000 },
    )
    const reconnectMS = performance.now() - reconnectStart
    controls = {
      video_minus_audio_ms: offset,
      interrupt_control_ms: interruptControlMS,
      interrupt_quiet_ms: quietMS,
      reconnect_ms: reconnectMS,
      pause_position_delta_seconds:
        frozen.speech_position_seconds - paused.speech_position_seconds,
      resume_position_delta_seconds:
        resumed.speech_position_seconds - frozen.speech_position_seconds,
      interruption_generation_advanced:
        interrupted.speech_generation > resumed.speech_generation,
      passed:
        Math.abs(offset) <= 80 &&
        Math.abs(
          frozen.speech_position_seconds - paused.speech_position_seconds,
        ) < 0.001 &&
        resumed.speech_position_seconds - frozen.speech_position_seconds >
          0.1 &&
        interruptControlMS < 100 &&
        quietMS < 700 &&
        interrupted.speech_generation > resumed.speech_generation &&
        reconnectMS < 5000,
    }
  }
  for (const id of ids) await command({ type: 'stop', session_id: id })
  const cleanup = await request('/snapshot')
  const released = cleanup.active === 0
  const gates = {
    idle: idle.sessions.every(
      (value) => value.fps >= 28 && value.freezes === 0,
    ),
    speech: speaking.sessions.every(
      (value) =>
        value.fps >= 28 &&
        value.frame_gap_p95_ms <= 66.8 &&
        value.freezes === 0 &&
        value.dropped_frames === 0 &&
        value.audio_peak > 0.002,
    ),
    size: speaking.sessions.every(
      (value) => ['720x960', '1440x960'].includes(value.size.join('x')),
    ),
    quality: quality.every((value) => value.face_psnr_db >= 40),
    released,
    no_errors: errors.length === 0,
    controls: controls === null || controls.passed,
  }
  const report = {
    version: 'linux-cpu-concurrency/1',
    sessions: count,
    health,
    implementation: renderer,
    speech_replay: { milliseconds: speechMilliseconds, parts: speechParts },
    idle,
    speech: speaking,
    quality,
    controls,
    cleanup: { active: cleanup.active, cgroup: cleanup.cgroup },
    gates,
    passed: Object.values(gates).every(Boolean),
    errors,
    paid_provider_calls: 0,
  }
  await writeFile(output, JSON.stringify(report, null, 2) + '\n')
  console.log(
    JSON.stringify({
      output,
      sessions: count,
      passed: report.passed,
      idle_fps: idle.sessions.map((value) => value.fps),
      speech_fps: speaking.sessions.map((value) => value.fps),
      cpu: speaking.worker_cpu_cores,
      memory_mib: speaking.worker_peak_memory / 2 ** 20,
      gates,
    }),
  )
} catch (error) {
  await writeFile(
    output,
    JSON.stringify(
      {
        failed: true,
        message: error.message,
        errors,
        host: await request('/snapshot').catch(() => null),
        receivers: await Promise.all(
          pages.map((page) =>
            page.evaluate(() => window.snapshot()).catch(() => null),
          ),
        ),
        paid_provider_calls: 0,
      },
      null,
      2,
    ),
  )
  throw error
} finally {
  stopping = true
  await poll?.catch(() => {})
  for (const id of ids)
    await command({ type: 'stop', session_id: id }).catch(() => {})
  await browser.close()
}
