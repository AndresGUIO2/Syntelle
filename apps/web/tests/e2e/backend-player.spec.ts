import { expect, test, type Page } from '@playwright/test'

async function openStream(page: Page, url = '/') {
  await page.goto(url)
  await page.getByRole('button', { name: /^(Start conversation|Iniciar conversación)$/ }).click()
}

async function closeCustomization(page: Page) {
  await page
    .getByRole('dialog', { name: /^(Customize Syntelle|Personalizar Syntelle)$/ })
    .getByRole('button', { name: /^(Close|Cerrar)$/ })
    .click()
}

async function showAppearance(page: Page) {
  await page.getByRole('tab', { name: /^(Appearance|Apariencia)$/ }).click()
}
async function showAdvanced(page: Page) {
  await page.getByRole('tab', { name: /^(Behavior|Comportamiento)$/ }).click()
  const button = page.getByRole('button', { name: /^(Advanced|Avanzado)$/ })
  if ((await button.getAttribute('aria-expanded')) !== 'true') await button.click()
}
async function chooseMood(page: Page, state: string, label: string) {
  const select = page.getByRole('combobox', { name: 'Mood', exact: true })
  if (await select.isVisible()) await select.selectOption(state)
  else await page.getByRole('radio', { name: label, exact: true }).check()
}

function languageTrigger(page: Page) {
  return page.getByRole('button', { name: /^(Platform language|Idioma de la plataforma):/ })
}
async function chooseLanguage(page: Page, language: 'en' | 'es') {
  await languageTrigger(page).click()
  await page
    .getByRole('menuitemradio', { name: language === 'en' ? 'English' : 'Español', exact: true })
    .click()
}

test('local static preview uses the chosen background without rendering or time-cap copy', async ({
  page,
}) => {
  const remote = await fixture(page)
  await page.route('**/api/v1/streams/status', (route) =>
    route.fulfill({ json: { time_limits_enabled: false } }),
  )
  await page.goto('/')
  const portrait = page.getByRole('img', { name: 'Syntelle portrait' })
  await expect(portrait).toBeVisible()
  await expect(
    page.getByText('Up to 5 minutes per session. 15 minutes total.', { exact: true }),
  ).toHaveCount(0)
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  await showAppearance(page)
  for (const [label, id] of [
    ['Dark Lounge', 'dark-lounge'],
    ['Light Studio', 'light-studio'],
    ['Natural Space', 'natural-space'],
  ]) {
    await page.getByRole('button', { name: label, exact: true }).click()
    await expect
      .poll(() => portrait.evaluate((image: HTMLImageElement) => image.currentSrc))
      .toContain(`syntelle-${id}-`)
  }
  await closeCustomization(page)
  await page.screenshot({ path: test.info().outputPath('static-background-preview.png') })
  expect(remote.sessionRequests).toEqual([])
  expect(remote.commands).toEqual([])
})

test('arrival shows a static portrait and opens no rendering session until Start', async ({
  page,
}) => {
  const remote = await fixture(page)
  await page.goto('/')
  const start = page.getByRole('button', { name: 'Start conversation', exact: true })
  await expect(start).toBeVisible()
  await expect(page.locator('.remote-stage')).toHaveClass(/remote-stage-still/)
  await expect(page.locator('.still-mark')).toHaveCount(0)
  await expect(page.getByText('Select Start conversation', { exact: false })).toHaveCount(0)
  await expect(page.getByText('Ready when you are.', { exact: true })).toBeVisible()
  await expect(page.getByRole('img', { name: 'Syntelle portrait' })).toBeVisible()
  await expect(page.locator('video.remote-source')).toBeHidden()
  await expect(
    page.getByRole('button', { name: 'Start with microphone', exact: true }),
  ).toBeEnabled()
  await chooseLanguage(page, 'es')
  await expect(
    page.getByRole('button', { name: 'Iniciar conversación', exact: true }),
  ).toBeVisible()
  await expect(page.getByText('Lista cuando quieras.', { exact: true })).toBeVisible()
  await expect(
    page.getByRole('button', { name: 'Iniciar con micrófono', exact: true }),
  ).toBeEnabled()
  await page.waitForTimeout(500)
  expect(remote.sessionRequests).toEqual([])
  expect(remote.commands).toEqual([])
  await page.screenshot({ path: test.info().outputPath('static-arrival.png') })
  await page.getByRole('button', { name: 'Iniciar conversación', exact: true }).click()
  await expect(page.locator('.remote-stage')).not.toHaveClass(/remote-stage-still/)
  await expect(page.getByRole('button', { name: 'Activar micrófono', exact: true })).toBeEnabled()
  expect(remote.sessionRequests).toHaveLength(1)
})

test('microphone starts the conversation and capture in one action', async ({
  page,
  browserName,
}) => {
  test.skip(browserName !== 'chromium', 'Windows WebKit does not provide Web Audio capture.')
  const remote = await fixture(page, false, true)
  await page.addInitScript(() => {
    navigator.mediaDevices.getUserMedia = async () => {
      const audio = new AudioContext()
      const output = audio.createMediaStreamDestination()
      const oscillator = audio.createOscillator()
      const gain = audio.createGain()
      gain.gain.value = 0
      oscillator.connect(gain).connect(output)
      oscillator.start()
      output.stream.getTracks().forEach((track) => {
        const stop = track.stop.bind(track)
        track.stop = () => {
          stop()
          oscillator.stop()
          void audio.close()
        }
      })
      return output.stream
    }
  })
  await page.goto('/')
  await page.getByRole('button', { name: 'Start with microphone', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Microphone on', exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Start conversation', exact: true })).toHaveCount(0)
  expect(remote.sessionRequests).toHaveLength(1)
  expect(remote.commands.filter((event) => event.data?.type === 'mic_start')).toHaveLength(1)
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  await showAdvanced(page)
  await page.getByRole('button', { name: /Play gesture/ }).click()
  await expect(page.locator('.microphone-control')).toHaveAttribute('data-capture', 'false')
  await expect
    .poll(
      () => remote.commands.filter((event) => event.data?.type === 'preview').at(-1)?.data?.text,
    )
    .toBe('nod')
  expect(remote.commands.filter((event) => event.data?.type === 'mic_stop')).toHaveLength(1)
})

test('five-minute end returns to a portrait and never silently restarts', async ({ page }) => {
  const remote = await fixture(page)
  await openStream(page)
  await expect(page.getByRole('button', { name: 'Enable microphone', exact: true })).toBeEnabled()
  remote.publish({ type: 'ended', data: { reason: 'session time limit' } })
  await expect(page.getByRole('button', { name: 'Start new session', exact: true })).toBeVisible()
  await expect(page.getByRole('img', { name: 'Syntelle portrait' })).toBeVisible()
  await page.waitForTimeout(700)
  expect(remote.sessionRequests).toHaveLength(1)
  await page.getByRole('button', { name: 'Start new session', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Enable microphone', exact: true })).toBeEnabled()
  expect(remote.sessionRequests).toHaveLength(2)
  expect(remote.sessionRequests[1]?.resume_from).toBeUndefined()
})

for (const reducedMotion of ['no-preference', 'reduce'] as const) {
  test(`start holds the portrait until composed video and gently fades (${reducedMotion})`, async ({
    page,
    browserName,
  }) => {
    test.skip(browserName !== 'chromium', 'Windows WebKit does not provide actual canvas capture.')
    const remote = await fixture(page, false, true)
    await page.emulateMedia({ reducedMotion })
    let releaseBackground: (() => void) | undefined
    await page.route(/\/natural-space\.webp$/, async (route) => {
      await new Promise<void>((resolve) => (releaseBackground = resolve))
      await route.continue()
    })
    await openStream(page)
    await expect.poll(() => Boolean(releaseBackground)).toBe(true)
    await expect(page.getByRole('button', { name: 'Enable microphone', exact: true })).toBeEnabled()
    const poster = page.locator('.remote-poster')
    await expect(page.getByRole('img', { name: 'Syntelle portrait' })).toBeVisible()
    await expect(poster).toHaveCSS('opacity', '1')
    expect(
      await page
        .locator('video')
        .evaluate(
          (video: HTMLVideoElement) => (video.srcObject as MediaStream).getVideoTracks().length,
        ),
    ).toBe(0)
    releaseBackground!()
    await expect
      .poll(() =>
        page.locator('video.remote-source').evaluate((video: HTMLVideoElement) => video.videoWidth),
      )
      .toBe(720)
    await expect(poster).toHaveCSS('opacity', '0')
    await expect(poster).toHaveCSS(
      'transition-duration',
      reducedMotion === 'reduce' ? '0.08s' : '0.24s',
    )
    await expect(page.getByRole('region', { name: 'Syntelle live video' })).toHaveAttribute(
      'aria-busy',
      'false',
    )
    expect(remote.sessionRequests).toHaveLength(1)
    expect(
      remote.commands.filter((event) => event.type === 'command' || event.type === 'microphone'),
    ).toEqual([])
    await page.screenshot({ path: test.info().outputPath('gentle-start.png') })
  })
}

test('exhausted accumulated allowance keeps the static portrait without a reconnect loop', async ({
  page,
}) => {
  const remote = await fixture(page)
  let attempts = 0
  await page.route('**/api/v1/streams', async (route) => {
    attempts++
    await route.fulfill({
      status: 429,
      json: { error: 'Your 15-minute demo allowance is exhausted or in use by another session.' },
    })
  })
  await openStream(page)
  await expect(page.getByRole('img', { name: 'Syntelle portrait' })).toBeVisible()
  await expect(page.getByRole('alert')).toContainText('15-minute demo allowance')
  await expect(page.getByRole('button', { name: 'Start conversation', exact: true })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Reconnect', exact: true })).toHaveCount(0)
  await page.waitForTimeout(700)
  expect(attempts).toBe(1)
  expect(remote.commands).toEqual([])
})

test('packed live pixels become a moving composed video with picture in picture', async ({
  page,
  browserName,
}) => {
  test.skip(
    browserName !== 'chromium',
    'Windows WebKit does not provide MediaStream/canvas capture.',
  )
  const errors: string[] = []
  page.on('pageerror', (error) => errors.push(error.message))
  const remote = await fixture(page, false, true)
  await page.addInitScript(() => {
    Object.defineProperty(document, 'pictureInPictureEnabled', { configurable: true, value: true })
    HTMLVideoElement.prototype.requestPictureInPicture = async function () {
      Object.defineProperty(document, 'pictureInPictureElement', {
        configurable: true,
        value: this,
      })
      this.dispatchEvent(new Event('enterpictureinpicture'))
      return {} as PictureInPictureWindow
    }
    document.exitPictureInPicture = async () => {
      const video = document.pictureInPictureElement
      Object.defineProperty(document, 'pictureInPictureElement', {
        configurable: true,
        value: null,
      })
      video?.dispatchEvent(new Event('leavepictureinpicture'))
    }
  })
  await openStream(page)
  const video = page.locator('video.remote-source')
  await expect
    .poll(() => video.evaluate((v: HTMLVideoElement) => [v.videoWidth, v.videoHeight]))
    .toEqual([1440, 960])
  const before = await video.evaluate(
    (v: HTMLVideoElement) => v.getVideoPlaybackQuality().totalVideoFrames,
  )
  expect(
    await video.evaluate((v: HTMLVideoElement) => {
      const original = Object.getOwnPropertyDescriptor(window, 'backgroundFixtureAudio')?.value
      return (v.srcObject as MediaStream).getAudioTracks()[0] === original
    }),
  ).toBe(true)
  remote.publish({
    type: 'conversation',
    data: { type: 'accepted', turn_id: 'pixel-audio', text: 'Fixture' },
  })
  remote.publish({ type: 'playback', data: { event: 'playback_started', turn_id: 'pixel-audio' } })
  const unmute = page.getByRole('button', { name: 'Unmute sound', exact: true })
  if (await unmute.isVisible()) await unmute.click()
  await expect(video).toHaveJSProperty('muted', false)
  const baselineAlerts = await page.getByRole('alert').allTextContents()
  await test.info().attach('before-customization-alerts', {
    body: JSON.stringify(baselineAlerts),
    contentType: 'application/json',
  })
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  await showAppearance(page)
  await page.getByRole('button', { name: 'Dark Lounge', exact: true }).click()
  await page.getByRole('slider', { name: /blur/i }).fill('10')
  await closeCustomization(page)
  await expect
    .poll(() =>
      video.evaluate((v: HTMLVideoElement) => v.getVideoPlaybackQuality().totalVideoFrames),
    )
    .toBeGreaterThan(before + 3)
  await page.getByRole('button', { name: 'Picture in picture', exact: true }).click()
  await expect
    .poll(() =>
      page.evaluate(
        () =>
          document.pictureInPictureElement ===
          document.querySelector<HTMLVideoElement>('video.remote-pip-source'),
      ),
    )
    .toBe(true)
  await page.getByRole('button', { name: 'Return to tab', exact: true }).click()
  expect(remote.commands.filter((event) => event.data?.type === 'configure')).toEqual([])
  expect(errors).toEqual([])
  await expect(page.getByRole('alert')).toHaveText(baselineAlerts)
})

test('browser backgrounds preserve opaque pixels and fractional hair coverage', async ({
  page,
}) => {
  await page.route('**/api/**', (route) =>
    route.fulfill({ status: 503, json: { error: { message: 'Offline pixel check' } } }),
  )
  await openStream(page)
  const results = await page.evaluate(async () => {
    const modulePath = '/src/features/streaming/backgroundCompositor.ts'
    const { createBackgroundCompositor } = await import(modulePath)
    const bg = document.createElement('canvas')
    bg.width = 720
    bg.height = 960
    const b = bg.getContext('2d')!
    b.fillStyle = 'rgb(20,40,80)'
    b.fillRect(0, 0, 360, 960)
    b.fillStyle = 'rgb(240,200,160)'
    b.fillRect(360, 0, 360, 960)
    const image = new Image()
    image.src = bg.toDataURL()
    await image.decode()
    const read = (canvas: HTMLCanvasElement) => {
      const copy = document.createElement('canvas')
      copy.width = 720
      copy.height = 960
      const c = copy.getContext('2d')!
      c.drawImage(canvas, 0, 0)
      return c.getImageData(0, 0, 720, 960).data
    }
    const pixel = (data: Uint8ClampedArray, x: number, y: number) =>
      Array.from(data.slice((y * 720 + x) * 4, (y * 720 + x) * 4 + 3))
    const results = []
    for (const height of [960, 720]) {
      const canvas = document.createElement('canvas')
      const renderer = createBackgroundCompositor(canvas, height * 1.5, height)
      const packed = document.createElement('canvas')
      packed.width = height * 1.5
      packed.height = height
      const p = packed.getContext('2d')!
      p.scale(height / 960, height / 960)
      p.fillStyle = 'black'
      p.fillRect(0, 0, 1440, 960)
      p.fillStyle = 'rgb(100,40,20)'
      p.fillRect(100, 100, 100, 100)
      p.fillStyle = 'white'
      p.fillRect(820, 100, 100, 100)
      p.fillStyle = 'rgb(60,20,10)'
      p.fillRect(300, 100, 40, 100)
      p.fillStyle = 'rgb(128,128,128)'
      p.fillRect(1020, 100, 40, 100)
      renderer.setBackground(image, 0)
      renderer.draw(packed)
      const before = read(canvas)
      renderer.setBackground(image, 20)
      renderer.draw(packed)
      const after = read(canvas)
      results.push({
        opaque: pixel(before, 150, 150),
        edge: pixel(before, 320, 150),
        background: pixel(before, 10, 800),
        sharp: pixel(before, 355, 800),
        blurred: pixel(after, 355, 800),
        opaqueBlurred: pixel(after, 150, 150),
      })
      renderer.dispose()
    }
    return results
  })
  for (const result of results) {
    expect(result.opaque).toEqual([100, 40, 20])
    expect(result.opaqueBlurred).toEqual(result.opaque)
    expect(result.background).toEqual([20, 40, 80])
    result.edge.forEach((value, index) =>
      expect(Math.abs(value - [70, 40, 50][index]!)).toBeLessThanOrEqual(1),
    )
    expect(result.sharp).toEqual([20, 40, 80])
    expect(result.blurred[0]).toBeGreaterThan(60)
    expect(result.blurred[0]).toBeLessThan(180)
  }
})

test('background and blur changes stay local while character controls reach the backend', async ({
  page,
}) => {
  const remote = await fixture(page)
  await openStream(page)
  await expect(page.getByRole('button', { name: 'Customize', exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  await showAppearance(page)
  await page.getByRole('button', { name: 'Light Studio', exact: true }).click()
  await page.getByRole('slider', { name: /blur/i }).fill('12')
  await page.waitForTimeout(250)
  expect(remote.commands.filter((event) => event.data?.type === 'configure')).toEqual([])
  await page.getByRole('checkbox', { name: 'Head movement and breathing' }).uncheck()
  await expect
    .poll(() => remote.commands.filter((event) => event.data?.type === 'configure').length)
    .toBe(1)
  expect(remote.settings).toHaveLength(1)
})

test('inline composition preserves native audio/video tracks without recapture or rate changes', async ({
  page,
  browserName,
}) => {
  test.skip(
    browserName !== 'chromium',
    'Actual media tracks are required for the presentation-clock check.',
  )
  const remote = await fixture(page, false, true)
  await page.addInitScript(() => {
    const capture = HTMLCanvasElement.prototype.captureStream
    Object.defineProperty(window, 'inlineCaptureCount', { value: { count: 0 }, configurable: true })
    HTMLCanvasElement.prototype.captureStream = function (...args) {
      if (this.width === 720 && this.height === 960)
        Object.getOwnPropertyDescriptor(window, 'inlineCaptureCount')!.value.count++
      return capture.apply(this, args)
    }
  })
  await openStream(page)
  await expect(page.locator('canvas.remote-video')).toBeVisible()
  await expect
    .poll(() => page.locator('canvas.remote-video').evaluate((c: HTMLCanvasElement) => c.width))
    .toBe(720)
  await expect
    .poll(() =>
      page.evaluate(() => {
        const v = document.querySelector<HTMLVideoElement>('video.remote-source')!
        const stream = v.srcObject as MediaStream
        return (
          v.videoWidth > 0 &&
          stream?.getVideoTracks()[0] ===
            Object.getOwnPropertyDescriptor(window, 'backgroundFixtureVideo')?.value
        )
      }),
    )
    .toBe(true)
  await expect(page.getByRole('button', { name: 'Mute sound', exact: true })).toBeVisible()
  remote.publish({
    type: 'conversation',
    data: { type: 'accepted', turn_id: 'sync', text: 'Sync' },
  })
  remote.publish({ type: 'playback', data: { event: 'playback_started', turn_id: 'sync' } })
  await expect(page.locator('video.remote-source')).toHaveJSProperty('muted', false)
  const state = await page.evaluate(() => {
    const v = document.querySelector<HTMLVideoElement>('video.remote-source')!,
      stream = v.srcObject as MediaStream
    return {
      audio:
        stream.getAudioTracks()[0] ===
        Object.getOwnPropertyDescriptor(window, 'backgroundFixtureAudio')?.value,
      rate: v.playbackRate,
      muted: v.muted,
      captures: Object.getOwnPropertyDescriptor(window, 'inlineCaptureCount')!.value.count,
    }
  })
  expect(state).toEqual({ audio: true, rate: 1, muted: false, captures: 0 })
})

async function fixture(page: Page, unavailable = false, pixels = false) {
  const commands: { type: string; data?: { type?: string; text?: string; settings?: unknown } }[] =
    []
  const settings: Record<string, unknown>[] = []
  const sessionRequests: Record<string, unknown>[] = []
  const deleted: string[] = []
  let publish = (_data: unknown) => {}
  let close = () => {}
  const downloads: string[] = []
  page.on('request', (request) => downloads.push(request.url()))
  await page.addInitScript((pixels) => {
    // This fixture exercises the UI/control protocol without a renderer or provider.
    class Peer {
      connectionState = 'new'
      onconnectionstatechange: (() => void) | null = null
      ontrack: ((event: { track: MediaStreamTrack }) => void) | null = null
      constructor() {
        setTimeout(() => {
          if (this.connectionState === 'closed') return
          this.connectionState = 'connected'
          this.onconnectionstatechange?.()
          if (pixels) {
            const c = document.createElement('canvas')
            c.width = 1440
            c.height = 960
            const ctx = c.getContext('2d')!
            let tick = 0
            const draw = () => {
              ctx.fillStyle = 'black'
              ctx.fillRect(0, 0, 1440, 960)
              ctx.fillStyle = tick++ % 2 ? '#782814' : '#802c18'
              ctx.fillRect(200, 150, 200, 400)
              ctx.fillStyle = 'white'
              ctx.fillRect(920, 150, 200, 400)
            }
            draw()
            const timer = setInterval(draw, 33)
            const track = c.captureStream(30).getVideoTracks()[0]!
            Object.defineProperty(window, 'backgroundFixtureVideo', {
              value: track,
              configurable: true,
            })
            track.addEventListener('ended', () => clearInterval(timer))
            this.ontrack?.({ track })
            const audio = new AudioContext()
            const oscillator = audio.createOscillator(),
              destination = audio.createMediaStreamDestination()
            oscillator.connect(destination)
            oscillator.start()
            const audioTrack = destination.stream.getAudioTracks()[0]!
            Object.defineProperty(window, 'backgroundFixtureAudio', {
              value: audioTrack,
              configurable: true,
            })
            this.ontrack?.({ track: audioTrack })
          }
        }, 20)
      }
      close() {
        this.connectionState = 'closed'
      }
    }
    Object.defineProperty(window, 'RTCPeerConnection', { value: Peer })
    if (pixels) return
    // Windows WebKit has no WebRTC/MediaStream implementation. These tests
    // verify UI/protocol behavior; actual decoding is checked separately.
    Object.defineProperty(window, 'MediaStream', {
      value: class extends EventTarget {
        getAudioTracks() {
          return []
        }
        getVideoTracks() {
          return []
        }
        getTracks() {
          return []
        }
      },
    })
    Object.defineProperty(HTMLMediaElement.prototype, 'srcObject', { writable: true, value: null })
    HTMLMediaElement.prototype.play = () => Promise.resolve()
    HTMLCanvasElement.prototype.getContext = () => {
      throw Error('Empty fixture streams must not start the pixel compositor')
    }
  }, pixels)
  await page.route('**/api/**', async (route) => {
    const request = route.request()
    const path = new URL(request.url()).pathname
    if (path === '/api/v1/auth/status') {
      await route.fulfill({
        json: {
          email: { configured: false },
          google: { configured: false },
          github: { configured: false },
        },
      })
    } else if (path === '/api/v1/auth/me') {
      await route.fulfill({ json: { account: null } })
    } else if (path === '/api/v1/streams/status') {
      await route.fulfill({
        json: { time_limits_enabled: true, laughter: { 'en-US': true, 'es-CO': true } },
      })
    } else if (path === '/api/v1/streams' && request.method() === 'POST') {
      sessionRequests.push(request.postDataJSON())
      settings.push(request.postDataJSON().settings)
      await route.fulfill(
        unavailable
          ? { status: 403, json: { error: { message: 'The backend stream is unavailable.' } } }
          : {
              json: {
                id: 'fixture-stream',
                ticket: 'fixture-ticket',
                laughter_available: true,
                state: 'idle',
                configured: true,
                recognition: true,
              },
            },
      )
    } else if (path === '/api/v1/streams/fixture-stream' && request.method() === 'GET') {
      await route.fulfill({ status: 404, json: { error: 'Stream not found.' } })
    } else if (
      /^\/api\/v1\/streams\/fixture-(?:stream|stale)$/.test(path) &&
      request.method() === 'DELETE'
    ) {
      deleted.push(path)
      await route.fulfill({ json: {} })
    } else throw Error(`Unexpected provider/API request: ${request.method()} ${path}`)
  })
  await page.routeWebSocket('**/api/v1/streams/*/ws', (socket) => {
    close = () => socket.close()
    publish = (data) =>
      socket.send(JSON.stringify({ session_id: 'fixture-stream', ...(data as object) }))
    socket.onMessage((raw) => {
      const event = JSON.parse(String(raw))
      commands.push(event)
      if (event.type === 'authenticate') publish({ type: 'ready', data: { state: 'idle' } })
    })
  })
  return {
    commands,
    settings,
    sessionRequests,
    deleted,
    downloads,
    publish: (data: unknown) => publish(data),
    close: () => close(),
  }
}

test('inactivity pause releases the player and resumes only after explicit action', async ({
  page,
}) => {
  const remote = await fixture(page)
  await openStream(page)
  await expect(page.getByRole('button', { name: 'Enable microphone', exact: true })).toBeEnabled()
  remote.publish({
    type: 'conversation',
    data: { type: 'accepted', turn_id: 'paused-turn', text: 'Keep my conversation.' },
  })
  await expect(page.getByText('Keep my conversation.', { exact: true })).toBeVisible()
  remote.publish({ type: 'paused', data: { reason: 'user inactivity' } })
  const resume = page.getByRole('button', { name: 'Resume', exact: true })
  await expect(resume).toBeVisible()
  await expect(page.locator('.remote-stage')).toHaveClass(/remote-stage-still/)
  await expect(page.locator('.connection-state [role="status"]')).toHaveText(
    'Syntelle is paused due to inactivity',
  )
  await expect(
    page.getByRole('button', { name: 'Start with microphone', exact: true }),
  ).toBeEnabled()
  await expect(page.locator('[data-testid="stream-workspace"]')).toHaveAttribute(
    'data-stream-id',
    '',
  )
  await expect(page.getByText('Keep my conversation.', { exact: true })).toBeVisible()
  await page.waitForTimeout(800)
  expect(remote.sessionRequests).toHaveLength(1)
  expect(remote.deleted).toEqual([])
  await page.screenshot({ path: test.info().outputPath('paused-resume-control.png') })
  await resume.click()
  await expect(page.getByRole('button', { name: 'Enable microphone', exact: true })).toBeEnabled()
  await expect(page.locator('.remote-stage')).not.toHaveClass(/remote-stage-still/)
  expect(remote.sessionRequests).toHaveLength(2)
  expect(remote.sessionRequests[1]?.resume_from).toBe('fixture-stream')
  await expect(page.getByText('Keep my conversation.', { exact: true })).toBeVisible()
})

test('lost inactivity notices cannot automatically create a new render session', async ({
  page,
}) => {
  const remote = await fixture(page)
  await openStream(page)
  await expect(page.getByRole('button', { name: 'Enable microphone', exact: true })).toBeEnabled()
  remote.close()
  await expect(page.getByRole('button', { name: 'Reconnect', exact: true })).toBeVisible()
  expect(remote.sessionRequests).toHaveLength(1)
})

test('switching tabs keeps the same media and in-flight reply', async ({ page }) => {
  const remote = await fixture(page)
  await openStream(page)
  await expect(page.getByRole('button', { name: 'Enable microphone', exact: true })).toBeEnabled()
  remote.publish({
    type: 'conversation',
    data: { type: 'accepted', turn_id: 'background', text: 'Hello' },
  })
  await expect(page.getByText('Hello', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Unmute sound', exact: true }).click()
  await page.evaluate(() => {
    Object.defineProperty(
      document.querySelector<HTMLVideoElement>('video.remote-source'),
      'fixtureOriginalMedia',
      {
        value: document.querySelector<HTMLVideoElement>('video.remote-source')!.srcObject,
      },
    )
    Object.defineProperty(document, 'hidden', { configurable: true, value: true })
    document.dispatchEvent(new Event('visibilitychange'))
  })
  remote.publish({
    type: 'conversation',
    data: { type: 'reply_delta', turn_id: 'background', text: 'Still replying' },
  })
  remote.publish({ type: 'playback', data: { event: 'playback_started', turn_id: 'background' } })
  await expect(page.getByText('Still replying', { exact: true })).toBeVisible()
  await expect(page.locator('video.remote-source')).toHaveJSProperty('muted', false)
  const media = await page.locator('video.remote-source').evaluate((element: HTMLVideoElement) => {
    Object.defineProperty(document, 'hidden', { configurable: true, value: false })
    document.dispatchEvent(new Event('visibilitychange'))
    return (
      Object.getOwnPropertyDescriptor(element, 'fixtureOriginalMedia')?.value === element.srcObject
    )
  })
  expect(media).toBe(true)
  expect(remote.settings).toHaveLength(1)
  expect(remote.deleted).toEqual([])
  expect(remote.commands.filter((event) => event.type === 'authenticate')).toHaveLength(1)
  expect(remote.commands.filter((event) => event.data?.type === 'interrupt')).toEqual([])
})

for (const phase of ['capturing', 'permission', 'worklet'] as const) {
  test(`switching tabs preserves the microphone during ${phase}`, async ({ page, browserName }) => {
    test.skip(browserName !== 'chromium', 'Windows WebKit does not provide Web Audio capture.')
    const remote = await fixture(page, false, true)
    await page.addInitScript((phase) => {
      const state = window as typeof window & {
        captureTracks: MediaStreamTrack[]
        continueMicrophone?: () => void
      }
      state.captureTracks = []
      const wait = () => new Promise<void>((resolve) => (state.continueMicrophone = resolve))
      navigator.mediaDevices.getUserMedia = async () => {
        const audio = new AudioContext()
        const output = audio.createMediaStreamDestination()
        const oscillator = audio.createOscillator()
        const gain = audio.createGain()
        gain.gain.value = 0
        oscillator.connect(gain).connect(output)
        oscillator.start()
        output.stream.getTracks().forEach((track) => {
          const stop = track.stop.bind(track)
          track.stop = () => {
            stop()
            void audio.close()
          }
        })
        state.captureTracks.push(...output.stream.getTracks())
        if (phase === 'permission') await wait()
        return output.stream
      }
      if (phase === 'worklet') {
        const addModule = Worklet.prototype.addModule
        Worklet.prototype.addModule = async function (...args) {
          await addModule.apply(this, args)
          await wait()
        }
      }
    }, phase)
    await openStream(page)
    await page.getByRole('button', { name: 'Enable microphone', exact: true }).click()
    const audioPackets = () =>
      remote.commands.filter((event) => event.type === 'microphone' && event.data?.type === 'audio')
        .length
    if (phase === 'capturing') {
      await expect(page.getByRole('button', { name: 'Microphone on', exact: true })).toBeVisible()
      await expect.poll(audioPackets).toBeGreaterThan(1)
    } else {
      await expect
        .poll(() =>
          page.evaluate(
            () => typeof Object.getOwnPropertyDescriptor(window, 'continueMicrophone')?.value,
          ),
        )
        .toBe('function')
    }
    const before = audioPackets()
    await page.evaluate(() => {
      Object.defineProperty(document, 'hidden', { configurable: true, value: true })
      document.dispatchEvent(new Event('visibilitychange'))
      Object.getOwnPropertyDescriptor(window, 'continueMicrophone')?.value?.()
    })
    await expect(page.getByRole('button', { name: 'Microphone on', exact: true })).toBeVisible()
    await expect.poll(audioPackets).toBeGreaterThan(before + 1)
    expect(remote.commands.filter((event) => event.data?.type === 'mic_start')).toHaveLength(1)
    expect(remote.commands.filter((event) => event.data?.type === 'mic_stop')).toEqual([])
    expect(remote.deleted).toEqual([])
    expect(remote.sessionRequests).toHaveLength(1)
    expect(
      await page.evaluate(() =>
        (
          Object.getOwnPropertyDescriptor(window, 'captureTracks')!.value as MediaStreamTrack[]
        ).every((track) => track.readyState === 'live'),
      ),
    ).toBe(true)
    await page.evaluate(() => {
      Object.defineProperty(document, 'hidden', { configurable: true, value: false })
      document.dispatchEvent(new Event('visibilitychange'))
    })
    await expect(page.getByRole('button', { name: 'Microphone on', exact: true })).toBeVisible()
    if (phase === 'capturing')
      await page.getByRole('button', { name: 'Microphone on', exact: true }).click()
    else remote.publish({ type: 'paused', data: { reason: 'user inactivity' } })
    await expect
      .poll(() =>
        page.evaluate(() =>
          (
            Object.getOwnPropertyDescriptor(window, 'captureTracks')!.value as MediaStreamTrack[]
          ).every((track) => track.readyState === 'ended'),
        ),
      )
      .toBe(true)
    await expect
      .poll(() => remote.commands.filter((event) => event.data?.type === 'mic_stop').length)
      .toBe(1)
  })
}

test('sound control stays visible and preserves the selected mute state during replies', async ({
  page,
}) => {
  const remote = await fixture(page)
  await openStream(page)
  const initialMute = page.getByRole('button', { name: 'Mute sound', exact: true })
  await expect(initialMute).toBeVisible()
  await initialMute.click()
  const unmute = page.getByRole('button', { name: 'Unmute sound', exact: true })
  await expect(unmute).toBeVisible()
  remote.publish({
    type: 'conversation',
    data: { type: 'accepted', turn_id: 'sound-initial', text: 'Initial reply' },
  })
  remote.publish({
    type: 'playback',
    data: { event: 'playback_started', turn_id: 'sound-initial' },
  })
  await expect(page.getByText('Initial reply', { exact: true })).toBeVisible()
  await expect(unmute).toHaveText('')
  await expect(unmute).toHaveAttribute('aria-pressed', 'false')
  await expect(page.locator('video.remote-source')).toHaveJSProperty('muted', true)
  await unmute.focus()
  await unmute.press('Enter')
  const mute = page.getByRole('button', { name: 'Mute sound', exact: true })
  await expect(mute).toBeVisible()
  await expect(mute).toHaveAttribute('aria-pressed', 'true')
  await expect(mute).toBeFocused()
  await expect(page.locator('video.remote-source')).toHaveJSProperty('muted', false)
  await mute.click()
  remote.publish({
    type: 'conversation',
    data: { type: 'accepted', turn_id: 'muted-reply', text: 'Hello' },
  })
  remote.publish({ type: 'playback', data: { event: 'playback_started', turn_id: 'muted-reply' } })
  await expect(page.getByText('Hello', { exact: true })).toBeVisible()
  await expect(page.locator('video.remote-source')).toHaveJSProperty('muted', true)
  await expect(unmute).toBeVisible()
  await unmute.click()
  await expect(page.locator('video.remote-source')).toHaveJSProperty('muted', false)
  await expect(mute).toBeVisible()
  await page.screenshot({ path: test.info().outputPath('sound-control.png') })
  expect(remote.settings).toHaveLength(1)
  expect(remote.deleted).toEqual([])
})

test('failed sound activation keeps the muted control available for retry', async ({ page }) => {
  const remote = await fixture(page)
  await openStream(page)
  const initialMute = page.getByRole('button', { name: 'Mute sound', exact: true })
  await expect(initialMute).toBeVisible()
  await initialMute.click()
  const unmute = page.getByRole('button', { name: 'Unmute sound', exact: true })
  await expect(unmute).toBeVisible()
  remote.publish({
    type: 'conversation',
    data: { type: 'accepted', turn_id: 'sound-retry', text: 'Retry reply' },
  })
  remote.publish({ type: 'playback', data: { event: 'playback_started', turn_id: 'sound-retry' } })
  await expect(page.getByText('Retry reply', { exact: true })).toBeVisible()
  await page.locator('video.remote-source').evaluate((element: HTMLVideoElement) => {
    element.play = () => Promise.reject(new DOMException('Blocked', 'NotAllowedError'))
  })
  await unmute.click()
  await expect(page.getByRole('status').filter({ hasText: 'Sound could not start.' })).toBeVisible()
  await expect(unmute).toBeVisible()
  await expect(page.locator('video.remote-source')).toHaveJSProperty('muted', true)
  await page.locator('video.remote-source').evaluate((element: HTMLVideoElement) => {
    element.play = () => Promise.resolve()
  })
  await unmute.click()
  await expect(page.getByRole('button', { name: 'Mute sound', exact: true })).toBeVisible()
  await expect(page.locator('video.remote-source')).toHaveJSProperty('muted', false)
  await expect(
    page.getByText('Sound could not start. Select Unmute sound to try again.'),
  ).toHaveCount(0)
})

test('picture in picture handles enter, browser close and denial without reconnecting', async ({
  page,
  browserName,
}) => {
  test.skip(browserName !== 'chromium', 'Actual PiP capture needs a media-capable browser.')
  const remote = await fixture(page, false, true)
  await page.addInitScript(() => {
    Object.defineProperty(document, 'pictureInPictureEnabled', { configurable: true, value: true })
    HTMLVideoElement.prototype.requestPictureInPicture = async function () {
      if (this.dataset.deny) throw new DOMException('Denied', 'NotAllowedError')
      Object.defineProperty(document, 'pictureInPictureElement', {
        configurable: true,
        value: this,
      })
      this.dispatchEvent(new Event('enterpictureinpicture'))
      return {} as PictureInPictureWindow
    }
    document.exitPictureInPicture = async () => {
      const element = document.pictureInPictureElement
      Object.defineProperty(document, 'pictureInPictureElement', {
        configurable: true,
        value: null,
      })
      element?.dispatchEvent(new Event('leavepictureinpicture'))
    }
  })
  await openStream(page)
  await page.getByRole('button', { name: 'Picture in picture', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Return to tab', exact: true })).toHaveAttribute(
    'aria-pressed',
    'true',
  )
  await page.getByRole('button', { name: 'Return to tab', exact: true }).click()
  await page.getByRole('button', { name: 'Picture in picture', exact: true }).click()
  await page.evaluate(() => document.exitPictureInPicture())
  await expect(
    page.getByRole('button', { name: 'Picture in picture', exact: true }),
  ).toHaveAttribute('aria-pressed', 'false')
  await page.locator('video.remote-pip-source').evaluate((element) => {
    element.dataset.deny = 'true'
  })
  await page.getByRole('button', { name: 'Picture in picture', exact: true }).click()
  await expect(
    page.getByText('Picture in picture could not open. Try again or use this tab.'),
  ).toBeVisible()
  expect(remote.settings).toHaveLength(1)
  expect(remote.deleted).toEqual([])
})

test('unsupported picture in picture has no unusable control', async ({ page }) => {
  await fixture(page)
  await page.addInitScript(() => {
    Object.defineProperty(document, 'pictureInPictureEnabled', { configurable: true, value: false })
    Object.defineProperty(HTMLVideoElement.prototype, 'webkitSupportsPresentationMode', {
      configurable: true,
      value: () => false,
    })
  })
  await openStream(page)
  await expect(page.getByRole('button', { name: 'Enable microphone', exact: true })).toBeEnabled()
  await expect(page.getByRole('button', { name: 'Picture in picture', exact: true })).toHaveCount(0)
})

test('browser automatic PiP action opens once while hidden with a quiet accessible control', async ({
  page,
  browserName,
}) => {
  test.skip(browserName !== 'chromium', 'Actual PiP capture needs a media-capable browser.')
  const remote = await fixture(page, false, true)
  await page.addInitScript(() => {
    Object.defineProperty(navigator, 'mediaSession', {
      value: {
        setActionHandler(action: string, handler: (() => void) | null) {
          if (action === 'enterpictureinpicture')
            Object.defineProperty(window, 'fixtureAutoPip', { configurable: true, value: handler })
        },
      },
    })
    Object.defineProperty(document, 'pictureInPictureEnabled', { configurable: true, value: true })
    HTMLVideoElement.prototype.requestPictureInPicture = async function () {
      this.dataset.pipRequests = String(Number(this.dataset.pipRequests ?? 0) + 1)
      if (this.dataset.deny) throw new DOMException('Denied', 'NotAllowedError')
      Object.defineProperty(document, 'pictureInPictureElement', {
        configurable: true,
        value: this,
      })
      this.dispatchEvent(new Event('enterpictureinpicture'))
      return {} as PictureInPictureWindow
    }
    document.exitPictureInPicture = async () => {
      const element = document.pictureInPictureElement
      Object.defineProperty(document, 'pictureInPictureElement', {
        configurable: true,
        value: null,
      })
      element?.dispatchEvent(new Event('leavepictureinpicture'))
    }
  })
  await openStream(page)
  const button = page.getByRole('button', { name: 'Picture in picture', exact: true })
  await expect(button).toBeVisible()
  await expect(button).toHaveText('')
  expect(await button.evaluate((e) => e.getBoundingClientRect().width)).toBe(44)
  // The real browser, not the visibility listener, dispatches this action.
  await page.evaluate(() => Object.getOwnPropertyDescriptor(window, 'fixtureAutoPip')!.value())
  await expect(page.locator('video.remote-pip-source')).not.toHaveAttribute('data-pip-requests')
  await page.evaluate(() => {
    Object.defineProperty(document, 'hidden', { configurable: true, value: true })
    document.dispatchEvent(new Event('visibilitychange'))
    Object.getOwnPropertyDescriptor(window, 'fixtureAutoPip')!.value()
  })
  await expect(page.getByRole('button', { name: 'Return to tab', exact: true })).toHaveAttribute(
    'aria-pressed',
    'true',
  )
  await page.evaluate(() => Object.getOwnPropertyDescriptor(window, 'fixtureAutoPip')!.value())
  await expect(page.locator('video.remote-pip-source')).toHaveAttribute('data-pip-requests', '1')
  await page.evaluate(async () => {
    await document.exitPictureInPicture()
    document.querySelector<HTMLVideoElement>('video.remote-pip-source')!.dataset.deny = 'true'
    Object.getOwnPropertyDescriptor(window, 'fixtureAutoPip')!.value()
  })
  await expect(page.locator('video.remote-pip-source')).toHaveAttribute('data-pip-requests', '2')
  await expect(
    page.getByText('Picture in picture could not open. Try again or use this tab.'),
  ).toHaveCount(0)
  expect(remote.settings).toHaveLength(1)
  expect(remote.deleted).toEqual([])
})

test('all former workspace/trial links use only the remote player', async ({ page }) => {
  const remote = await fixture(page)
  for (const url of [
    '/',
    '/?view=stream',
    '/?trial=motion&head=natural',
    '/?trial=viseme-a',
    '/?view=validation',
  ]) {
    await openStream(page, url)
    await expect(page.getByRole('heading', { name: 'Chat with Syntelle' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Enable microphone', exact: true })).toBeEnabled()
    await expect(page.locator('video.remote-source')).toHaveCount(1)
    await expect(page.locator('canvas, .demo-workspace, .stream-composer')).toHaveCount(0)
    const layout = await page.locator('.voice-actions').evaluate((element) => {
      const row = element.getBoundingClientRect()
      const mic = element.querySelector('.microphone-button')!.getBoundingClientRect()
      return {
        offset: Math.abs(mic.x + mic.width / 2 - row.x - row.width / 2),
        overflow: document.documentElement.scrollWidth > innerWidth,
      }
    })
    expect(layout.offset).toBeLessThan(1)
    expect(layout.overflow).toBe(false)
  }
  expect(
    remote.downloads.filter((url) =>
      /\/src\/(?:features\/(?:character|demo)\/|render-worker\/)/.test(url),
    ),
  ).toEqual([])
  expect(
    remote.downloads.filter(
      (url) => /\/src\/experiments\//.test(url) && !url.includes('-thumb.webp'),
    ),
  ).toEqual([])
  for (const path of [
    '/src/features/demo/DemoWorkspace.vue',
    '/src/features/character/rendering/poseMorph.ts',
    '/src/experiments/motion/assets/syntelle/package.json',
    '/src/render-worker/main.ts',
  ]) {
    expect((await page.request.get(path)).status()).toBe(403)
  }
})

test('current appearance, transcript and controls use the backend protocol', async ({ page }) => {
  const remote = await fixture(page)
  await page.addInitScript(() => {
    localStorage.setItem(
      'syntelle.demo.appearance',
      JSON.stringify({
        movement: false,
        voices: { syntelle: 'en-US-Andrew:DragonHDLatestNeural' },
      }),
    )
    localStorage.setItem(
      'syntelle.motion.background',
      JSON.stringify({ id: 'dark-lounge', blur: 5 }),
    )
    localStorage.setItem('syntelle.instructions.syntelle', 'Preserve these instructions.')
  })
  await openStream(page)
  await expect(page.getByRole('button', { name: 'Enable microphone', exact: true })).toBeEnabled()
  expect(remote.settings[0]).toMatchObject({
    movement: false,
    voice: 'en-US-Emma:DragonHDLatestNeural',
    background: 'dark-lounge',
    blur: 5,
    instructions: 'Preserve these instructions.',
  })
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  await showAppearance(page)
  await expect(page.getByRole('region', { name: 'Customize Syntelle', exact: true })).toBeVisible()
  await expect(page.getByRole('combobox', { name: 'Voice', exact: true })).toHaveCount(0)
  await page.getByRole('checkbox', { name: 'Head movement and breathing' }).check()
  await expect
    .poll(() =>
      remote.commands.some((event) => event.type === 'command' && event.data?.type === 'configure'),
    )
    .toBe(true)
  await closeCustomization(page)
  await expect(page.getByRole('button', { name: 'Customize', exact: true })).toBeFocused()
  remote.publish({
    type: 'conversation',
    data: { type: 'accepted', turn_id: 'turn', text: 'Hello Syntelle' },
  })
  remote.publish({
    type: 'conversation',
    data: { type: 'reply_delta', turn_id: 'turn', text: 'Hello from the backend.' },
  })
  await expect(page.getByText('Hello from the backend.', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Stop reply', exact: true }).click()
  await expect
    .poll(() =>
      remote.commands.some((event) => event.type === 'command' && event.data?.type === 'interrupt'),
    )
    .toBe(true)
  await page.getByRole('button', { name: 'New chat', exact: true }).click()
  await expect(page.getByText('Hello from the backend.', { exact: true })).toHaveCount(0)
  await page.getByRole('button', { name: 'Start conversation', exact: true }).click()
  await expect.poll(() => remote.settings.length).toBe(2)
})

test('platform language translates the workspace, pricing and guest account and persists', async ({
  page,
}) => {
  const remote = await fixture(page)
  await page.route('**/api/v1/auth/status', (route) =>
    route.fulfill({
      json: {
        email: { configured: true },
        google: { configured: false },
        github: { configured: false },
      },
    }),
  )
  await openStream(page)
  await expect(page.getByRole('button', { name: 'Enable microphone', exact: true })).toBeEnabled()
  const language = languageTrigger(page)
  await expect(language).toHaveText('English')
  await chooseLanguage(page, 'es')
  await page.getByRole('button', { name: 'Iniciar conversación', exact: true }).click()
  await expect.poll(() => remote.settings.length).toBe(2)
  expect(remote.settings[1]).toMatchObject({
    language: 'es-CO',
    voice: 'en-US-Emma:DragonHDLatestNeural',
  })
  expect(remote.deleted).toHaveLength(1)
  await expect.poll(() => remote.commands.filter((e) => e.type === 'authenticate').length).toBe(2)
  const spanish = languageTrigger(page)
  await expect(spanish).toHaveText('Español')
  await expect(page.locator('html')).toHaveAttribute('lang', 'es')
  await expect(page.getByRole('heading', { name: 'Habla con Syntelle' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Activar micrófono', exact: true })).toBeEnabled()
  await page.screenshot({ path: test.info().outputPath('platform-spanish.png') })
  await page.getByRole('button', { name: 'Iniciar sesión', exact: true }).click()
  await expect(
    page.getByRole('heading', { name: 'Te damos la bienvenida a Syntelle' }),
  ).toBeVisible()
  await expect(page.getByRole('textbox', { name: 'Correo electrónico', exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Continuar como visitante' })).toBeVisible()
  await page.getByRole('button', { name: 'Cerrar cuenta', exact: true }).click()
  await page.getByRole('button', { name: 'Personalizar', exact: true }).click()
  await expect(
    page.getByRole('combobox', { name: 'Estilo de reacción', exact: true }),
  ).toBeVisible()
  await expect(page.getByRole('combobox', { name: 'Voz', exact: true })).toHaveCount(0)
  await expect(page.getByRole('textbox', { name: 'Instrucciones del personaje' })).toHaveValue(
    String(remote.settings[0]!.instructions),
  )
  await showAppearance(page)
  await expect(page.getByRole('checkbox', { name: 'Parpadeo natural', exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Estudio claro', exact: true })).toBeVisible()
  await showAdvanced(page)
  await expect(page.getByRole('slider', { name: 'Sonrisa suave', exact: true })).toBeVisible()
  await page.screenshot({ path: test.info().outputPath('platform-spanish-settings.png') })
  await page.getByRole('button', { name: 'Volver al chat', exact: true }).click()

  remote.publish({
    type: 'conversation',
    data: { type: 'accepted', turn_id: 'language-fixture', text: 'Preserve this original text.' },
  })
  await expect(page.getByText('Preserve this original text.', { exact: true })).toBeVisible()
  remote.publish({ type: 'state', data: { state: 'speaking' } })
  await expect(spanish).toBeEnabled()
  await chooseLanguage(page, 'en')
  await page.getByRole('button', { name: 'Start conversation', exact: true }).click()
  await expect.poll(() => remote.settings.length).toBe(3)
  expect(remote.settings[2]).toMatchObject({
    language: 'en-US',
    voice: 'en-US-Emma:DragonHDLatestNeural',
  })
  await expect(page.getByRole('heading', { name: 'Chat with Syntelle' })).toBeVisible()
  await expect(page.getByText('Preserve this original text.', { exact: true })).toBeVisible()
  await chooseLanguage(page, 'es')
  await page.getByRole('button', { name: 'Iniciar conversación', exact: true }).click()
  await expect.poll(() => remote.settings.length).toBe(4)

  await page.reload()
  await expect(
    page.getByRole('button', { name: 'Iniciar conversación', exact: true }),
  ).toBeVisible()
  expect(remote.settings).toHaveLength(4)
  await page.getByRole('button', { name: 'Iniciar conversación', exact: true }).click()
  await expect.poll(() => remote.settings.length).toBe(5)
  expect(remote.settings[4]).toMatchObject({ language: 'es-CO' })
  await expect(spanish).toHaveText('Español')
  expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)).toBe(false)
  await page.getByRole('link', { name: 'Precios', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Pronto tendrás precios claros.' })).toBeVisible()
  await page.getByRole('link', { name: 'Volver a la demostración' }).click()
  await expect(
    page.getByRole('button', { name: 'Iniciar conversación', exact: true }),
  ).toBeVisible()
  expect(remote.settings).toHaveLength(5)
})

test('platform language applies to saved account controls without adopting a setup language', async ({
  page,
}) => {
  const remote = await fixture(page)
  await page.route('**/api/v1/auth/me', (route) =>
    route.fulfill({
      json: { account: { id: 'fixture-account', email: 'owner@example.test', name: 'Owner' } },
    }),
  )
  await page.route('**/api/v1/configurations', (route) =>
    route.fulfill({
      json: {
        items: [
          {
            id: 'fixture-setup',
            name: 'Original setup name',
            settings: { ...remote.settings[0], language: 'en-US' },
          },
        ],
      },
    }),
  )
  await page.route('**/api/v1/keys', (route) =>
    route.fulfill(
      route.request().method() === 'POST'
        ? { status: 400, json: { error: { message: 'Use a key name with 1 to 80 characters.' } } }
        : { json: { items: [] } },
    ),
  )
  await page.addInitScript(() =>
    localStorage.setItem(
      'syntelle.stream.settings',
      JSON.stringify({
        character: 'syntelle',
        emotion_baseline: { state: 'fearful', intensity: 0.7 },
        emotion_regulation: { enabled: false, temperament: 'warm' },
      }),
    ),
  )
  await openStream(page)
  await expect(page.getByRole('button', { name: 'Enable microphone', exact: true })).toBeEnabled()
  await chooseLanguage(page, 'es')
  await page.getByRole('button', { name: 'Iniciar conversación', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Activar micrófono', exact: true })).toBeEnabled()
  await page.getByRole('button', { name: 'Tu cuenta', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Guardar', exact: true })).toBeEnabled()
  await expect(page.getByText('Original setup name', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: /Acceso para desarrolladores/ })).toBeDisabled()
  const count = remote.settings.length
  await page.getByRole('button', { name: 'Usar esta configuración', exact: true }).click()
  expect(remote.settings).toHaveLength(count)
  await page.getByRole('button', { name: 'Iniciar conversación', exact: true }).click()
  await expect.poll(() => remote.settings.length).toBe(count + 1)
  expect(remote.settings.at(-1)).toMatchObject({ language: 'es-CO' })
  expect(remote.settings.at(-1)).toMatchObject({
    emotion_baseline: { state: 'fearful', intensity: 0.7 },
    emotion_regulation: { enabled: false, temperament: 'warm' },
  })
  await page.getByRole('button', { name: 'Personalizar', exact: true }).click()
  await expect(page.getByRole('slider', { name: 'Intensidad emocional', exact: true })).toHaveValue(
    '0.7',
  )
  await expect(page.getByRole('combobox', { name: 'Estilo de reacción', exact: true })).toHaveValue(
    'warm',
  )
  await expect(
    page.getByRole('combobox', { name: 'Estilo de reacción', exact: true }),
  ).toBeDisabled()
  await expect(languageTrigger(page)).toHaveText('Español')
})

for (const language of ['en', 'es'] as const) {
  test(`account history opens and continues in ${language} without auto-start`, async ({
    page,
  }) => {
    const remote = await fixture(page)
    let signedIn = true
    await page.route('**/api/v1/auth/me', (route) =>
      route.fulfill({
        json: {
          account: signedIn
            ? { id: 'fixture-account', email: 'owner@example.test', name: 'Owner' }
            : null,
        },
      }),
    )
    await page.route('**/api/v1/auth/logout', (route) => {
      signedIn = false
      return route.fulfill({ json: {} })
    })
    await page.route('**/api/v1/configurations', (route) => route.fulfill({ json: { items: [] } }))
    await page.route('**/api/v1/keys', (route) => route.fulfill({ json: { items: [] } }))
    let claimed = false
    await page.route('**/api/v1/conversations**', (route) => {
      const path = new URL(route.request().url()).pathname
      if (path.endsWith('/claimable'))
        return route.fulfill({
          json: {
            items: claimed ? [] : [{ id: 'guest-chat', title: 'Guest note', language: 'en-US' }],
          },
        })
      if (path.endsWith('/guest-chat/claim')) {
        claimed = true
        return route.fulfill({ json: { saved: true } })
      }
      if (path.endsWith('/saved-chat'))
        return route.fulfill({
          json: {
            conversation: {
              id: 'saved-chat',
              title: 'Earlier chat',
              language: language === 'es' ? 'es-CO' : 'en-US',
              settings: {
                character: 'syntelle',
                instructions: 'Saved instructions',
                language: language === 'es' ? 'es-CO' : 'en-US',
                voice: 'en-US-Emma:DragonHDLatestNeural',
                background: 'natural-space',
                blur: 0,
                blinking: true,
                movement: true,
                automatic_expressions: true,
                smile: 0,
                brows: 0,
                reduced_motion: false,
              },
            },
            turns: new URL(route.request().url()).searchParams.has('before')
              ? [
                  {
                    id: 'turn-1',
                    sequence: 1,
                    status: 'ready',
                    user: 'Earlier question',
                    reply: 'Earlier answer',
                    spoken: '',
                  },
                ]
              : [
                  {
                    id: 'turn-2',
                    sequence: 2,
                    status: 'ready',
                    user: 'Later question',
                    reply: 'Later answer',
                    spoken: '',
                  },
                ],
            before: new URL(route.request().url()).searchParams.has('before') ? 0 : 2,
          },
        })
      if (
        path === '/api/v1/conversations' &&
        new URL(route.request().url()).searchParams.has('before')
      )
        return route.fulfill({ json: { items: [], next: '' } })
      if (path === '/api/v1/conversations')
        return route.fulfill({
          json: {
            items: [
              {
                id: 'saved-chat',
                title: 'Earlier chat',
                language: language === 'es' ? 'es-CO' : 'en-US',
                updated_at: new Date().toISOString(),
              },
            ],
            next: '',
          },
        })
      throw Error(`Unexpected history request: ${path}`)
    })
    await page.goto('/')
    if (language === 'es') await chooseLanguage(page, 'es')
    await page
      .getByRole('button', {
        name: language === 'es' ? 'Historial de conversaciones' : 'Conversation history',
      })
      .click()
    await expect(
      page.getByRole('heading', {
        name: language === 'es' ? 'Historial de conversaciones' : 'Conversation history',
      }),
    ).toBeVisible()
    await page
      .getByRole('button', { name: language === 'es' ? 'Guardar en la cuenta' : 'Save to account' })
      .click()
    expect(claimed).toBe(true)
    await page.getByRole('button', { name: /Earlier chat/ }).click()
    await expect(page.getByText('Later answer')).toBeVisible()
    await page
      .getByRole('button', { name: language === 'es' ? 'Mensajes anteriores' : 'Older messages' })
      .click()
    await expect(page.getByText('Earlier answer')).toBeVisible()
    expect(remote.sessionRequests).toHaveLength(0)
    await page
      .getByRole('button', {
        name: language === 'es' ? 'Continuar en español' : 'Continue in English',
      })
      .click()
    await expect.poll(() => remote.sessionRequests.length).toBe(1)
    expect(remote.sessionRequests[0]).toMatchObject({ continue_conversation_id: 'saved-chat' })
    await page
      .getByRole('button', { name: language === 'es' ? 'Tu cuenta' : 'Your account' })
      .click()
    await page
      .getByRole('button', { name: language === 'es' ? 'Cerrar sesión' : 'Sign out' })
      .click()
    await expect(
      page.getByRole('button', {
        name: language === 'es' ? 'Historial de conversaciones' : 'Conversation history',
      }),
    ).toHaveCount(0)
    await expect(page.getByText('Later answer')).toHaveCount(0)
  })
}

test('platform language cancels an in-flight connection and waits for Start', async ({ page }) => {
  const remote = await fixture(page)
  await page.addInitScript(() => {
    localStorage.setItem(
      'syntelle.stream.settings',
      JSON.stringify({ character: 'syntelle', language: 'es-CO' }),
    )
  })
  let release!: () => void
  const hold = new Promise<void>((resolve) => {
    release = resolve
  })
  let arrived!: () => void
  const blocked = new Promise<void>((resolve) => {
    arrived = resolve
  })
  await page.route(
    '**/api/v1/streams',
    async (route) => {
      arrived()
      await hold
      await route.fulfill({
        json: {
          id: 'fixture-stale',
          ticket: 'fixture-ticket',
          state: 'idle',
          configured: true,
          recognition: true,
        },
      })
    },
    { times: 1 },
  )
  await openStream(page)
  await blocked
  const language = languageTrigger(page)
  await expect(language).toHaveText('Español')
  await expect(page.getByText('Conectando…', { exact: true })).toBeVisible()
  await chooseLanguage(page, 'en')
  release()
  await expect.poll(() => remote.deleted.includes('/api/v1/streams/fixture-stale')).toBe(true)
  await page.getByRole('button', { name: 'Start conversation', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Enable microphone', exact: true })).toBeEnabled()
  expect(remote.settings.at(-1)).toMatchObject({ language: 'en-US' })
  await expect.poll(() => remote.deleted.includes('/api/v1/streams/fixture-stale')).toBe(true)
  await expect(page.locator('html')).toHaveAttribute('lang', 'en')
})

test('expression previews use a direct command without sending a chat message', async ({
  page,
}) => {
  const remote = await fixture(page)
  await openStream(page)
  await expect(page.getByRole('button', { name: 'Customize', exact: true })).toBeEnabled()
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  await showAdvanced(page)
  await page.getByRole('button', { name: 'Play laughter', exact: true }).click()
  await expect
    .poll(() => remote.commands.some((e) => e.data?.type === 'preview' && e.data?.text === 'laugh'))
    .toBe(true)
  expect(remote.commands.some((e) => e.data?.type === 'message' || e.data?.type === 'speak')).toBe(
    false,
  )
  await expect(page.getByRole('button', { name: 'Playing…', exact: true })).toBeDisabled()
  await page.waitForTimeout(3000)
  await expect(page.getByRole('button', { name: 'Playing…', exact: true })).toBeDisabled()
  remote.publish({ type: 'preview_audio', data: { turn_id: 'laugh-lifecycle' } })
  remote.publish({
    type: 'playback',
    data: { event: 'playback_started', turn_id: 'laugh-lifecycle' },
  })
  await expect(page.getByRole('button', { name: 'Playing…', exact: true })).toBeDisabled()
  remote.publish({
    type: 'playback',
    data: { event: 'playback_ended', turn_id: 'laugh-lifecycle' },
  })
  await expect(page.getByRole('button', { name: 'Play laughter', exact: true })).toBeEnabled()
  await page.getByRole('button', { name: 'Play laughter', exact: true }).click()
  remote.publish({
    type: 'playback',
    data: { event: 'playback_ended', turn_id: 'laugh-lifecycle' },
  })
  await expect(page.getByRole('button', { name: 'Playing…', exact: true })).toBeDisabled()
  remote.publish({ type: 'error', data: { message: 'Preview fixture stopped.' } })
  await expect(page.getByRole('button', { name: 'Play laughter', exact: true })).toBeEnabled()
})

test('expression previews start a stream without microphone capture, including reduced motion', async ({
  page,
}) => {
  const remote = await fixture(page)
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.goto('/')
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  await showAdvanced(page)
  await expect(page.getByRole('button', { name: 'Play laughter', exact: true })).toBeEnabled()
  await expect(page.getByRole('button', { name: /Play gesture/ })).toBeEnabled()
  await page.getByRole('button', { name: /Play gesture/ }).click()
  await expect
    .poll(
      () => remote.commands.filter((event) => event.data?.type === 'preview').at(-1)?.data?.text,
    )
    .toBe('nod')
  await expect(page.getByRole('button', { name: 'Play laughter', exact: true })).toBeEnabled()
  await page.getByRole('button', { name: 'Play laughter', exact: true }).click()
  await expect
    .poll(
      () => remote.commands.filter((event) => event.data?.type === 'preview').at(-1)?.data?.text,
    )
    .toBe('laugh')
  expect(remote.sessionRequests).toHaveLength(1)
  expect(remote.settings[0]).toMatchObject({ reduced_motion: true })
  expect(remote.commands.some((event) => event.type === 'microphone')).toBe(false)
})

test('backend unavailability has no browser rendering fallback', async ({ page }) => {
  await fixture(page, true)
  await openStream(page)
  await expect(page.getByRole('alert')).toHaveText('The backend stream is unavailable.')
  await expect(page.getByRole('button', { name: 'Reconnect', exact: true })).toBeVisible()
  await expect(
    page.getByRole('button', { name: 'Start with microphone', exact: true }),
  ).toBeEnabled()
  await expect(page.locator('video.remote-source')).toHaveCount(1)
  await expect(page.locator('canvas')).toHaveCount(0)
})

test('expression controls keep continuous baselines and scale direct head previews', async ({
  page,
}) => {
  const remote = await fixture(page)
  await openStream(page)
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  await showAdvanced(page)
  const smile = page.getByRole('slider', { name: 'Soft smile' })
  await smile.fill('0.7')
  await page.getByRole('slider', { name: 'Raise eyebrows' }).fill('0.5')
  await expect
    .poll(() => remote.commands.filter((e) => e.data?.type === 'configure').at(-1)?.data?.settings)
    .toMatchObject({ smile: 0.7, brows: 0.5 })
  await page.getByRole('combobox', { name: 'Gesture', exact: true }).selectOption('tilt')
  await page.getByRole('slider', { name: 'Intensity', exact: true }).fill('0.5')
  await page.getByRole('button', { name: /Play gesture/ }).click()
  await expect
    .poll(() => remote.commands.filter((e) => e.data?.type === 'preview').at(-1)?.data)
    .toMatchObject({ text: 'tilt', amount: 0.5 })
  await expect(smile).toHaveValue('0.7')
  await expect(page.getByRole('button', { name: 'Smile', exact: true })).toHaveCount(0)
  await page.screenshot({ path: test.info().outputPath('expression-controls.png') })
  expect(remote.commands.some((e) => ['message', 'speak'].includes(e.data?.type ?? ''))).toBe(false)
})

test('emotion regulation defaults, explicit disable and temperament persist without providers', async ({
  page,
}) => {
  const remote = await fixture(page)
  await page.goto('/')
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  await showAdvanced(page)
  const stable = page.getByRole('checkbox', { name: 'Emotion stability', exact: true })
  const temperament = page.getByRole('combobox', { name: 'Reaction style', exact: true })
  await expect(stable).toBeChecked()
  await expect(temperament).toHaveValue('calm')
  await temperament.selectOption('expressive')
  await stable.uncheck()
  await expect(temperament).toBeDisabled()
  await page.reload()
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  await showAdvanced(page)
  await expect(stable).not.toBeChecked()
  await expect(temperament).toHaveValue('expressive')
  await stable.check()
  await expect(temperament).toBeEnabled()
  await chooseLanguage(page, 'es')
  await expect(
    page.getByRole('checkbox', { name: 'Estabilidad emocional', exact: true }),
  ).toBeChecked()
  expect(remote.sessionRequests).toEqual([])
  expect(remote.commands).toEqual([])
})

test('emotion controls persist the baseline without starting providers', async ({ page }) => {
  const remote = await fixture(page)
  await page.goto('/')
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  const intensity = page.getByRole('slider', { name: 'Emotion intensity', exact: true })
  await chooseMood(page, 'sad', 'Sad')
  await expect(intensity).toHaveValue('0.5')
  await intensity.fill('0.8')
  await page.getByRole('checkbox', { name: 'React to the conversation', exact: true }).uncheck()
  await closeCustomization(page)
  await page.reload()
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  const saved = await page.evaluate(() =>
    JSON.parse(localStorage.getItem('syntelle.stream.settings')!),
  )
  expect(saved.emotion_baseline).toEqual({ state: 'sad', intensity: 0.8 })
  await expect(intensity).toHaveValue('0.8')
  await expect(
    page.getByRole('checkbox', { name: 'React to the conversation', exact: true }),
  ).not.toBeChecked()
  await page.screenshot({ path: test.info().outputPath('emotion-baseline.png') })
  await page.getByRole('button', { name: 'Reset emotion', exact: true }).click()
  await expect(intensity).toHaveValue('0')
  await expect(intensity).toBeDisabled()
  expect(remote.sessionRequests).toEqual([])
  expect(remote.commands).toEqual([])
})

test('customization simple controls cover all moods and preserve advanced values across sections', async ({
  page,
}) => {
  const remote = await fixture(page)
  await page.goto('/')
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  const moodHelp = page.locator('summary[aria-label="About moods"]')
  await moodHelp.click()
  await expect(
    page.getByText('Preview: some emotions still need visual calibration.', { exact: true }),
  ).toBeVisible()
  await moodHelp.click()
  await expect(
    page.getByText('Preview: some emotions still need visual calibration.', { exact: true }),
  ).toBeHidden()
  expect(remote.commands).toEqual([])
  await expect(page.getByRole('radio')).toHaveCount(10)
  const advanced = page.getByRole('button', { name: 'Advanced', exact: true })
  await expect(advanced).toHaveAttribute('aria-expanded', 'false')
  await expect(page.getByRole('slider', { name: 'Soft smile', exact: true })).toBeHidden()
  for (const [state, label] of [
    ['neutral', 'Neutral'],
    ['joyful', 'Joyful'],
    ['playful', 'Playful'],
    ['affectionate', 'Affectionate'],
    ['curious', 'Curious'],
    ['sad', 'Sad'],
    ['angry', 'Angry'],
    ['fearful', 'Fearful'],
    ['surprised', 'Surprised'],
    ['disgusted', 'Disgusted'],
  ]) {
    await chooseMood(page, state!, label!)
    await expect(page.locator('.customization-current-mood')).toHaveText(label!)
    await expect
      .poll(() =>
        page.evaluate(
          () =>
            JSON.parse(localStorage.getItem('syntelle.stream.settings') ?? 'null')?.emotion_baseline
              .state ?? 'neutral',
        ),
      )
      .toBe(state)
  }
  const style = page.getByRole('combobox', { name: 'Reaction style', exact: true })
  for (const value of ['warm', 'expressive', 'calm']) await style.selectOption(value)
  await style.selectOption('expressive')
  const auto = page.getByRole('checkbox', { name: 'React to the conversation', exact: true })
  await auto.uncheck()
  await expect(style).toBeDisabled()
  await auto.check()
  await expect(style).toHaveValue('expressive')
  await advanced.click()
  await page.getByRole('checkbox', { name: 'Emotion stability', exact: true }).uncheck()
  await expect(style).toBeDisabled()
  await page.getByRole('slider', { name: 'Soft smile', exact: true }).fill('0.35')
  await page.getByRole('slider', { name: 'Raise eyebrows', exact: true }).fill('0.25')
  await page.getByRole('combobox', { name: 'Gesture', exact: true }).selectOption('shake')
  await page.getByRole('slider', { name: 'Intensity', exact: true }).fill('0.65')
  await page.getByRole('slider', { name: 'Repetitions', exact: true }).fill('3')
  await showAppearance(page)
  await page.getByRole('button', { name: 'Dark Lounge', exact: true }).click()
  await page.getByRole('slider', { name: 'Background blur', exact: true }).fill('9')
  await page.getByRole('checkbox', { name: 'Natural blinking', exact: true }).uncheck()
  await page.getByRole('checkbox', { name: 'Head movement and breathing', exact: true }).uncheck()
  await page.getByRole('tab', { name: 'Behavior', exact: true }).click()
  await expect(page.getByRole('slider', { name: 'Repetitions', exact: true })).toHaveValue('3')
  await expect(page.getByRole('slider', { name: 'Intensity', exact: true })).toHaveValue('0.65')
  await page
    .getByRole('textbox', { name: 'Character instructions', exact: true })
    .fill('Keep this role verbatim.')
  await closeCustomization(page)
  await expect(page.getByRole('button', { name: 'Customize', exact: true })).toBeFocused()
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  await expect(advanced).toHaveAttribute('aria-expanded', 'false')
  await showAdvanced(page)
  await expect(page.getByRole('slider', { name: 'Repetitions', exact: true })).toHaveValue('3')
  await page.reload()
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  const saved = await page.evaluate(() =>
    JSON.parse(localStorage.getItem('syntelle.stream.settings')!),
  )
  expect(saved).toMatchObject({
    instructions: 'Keep this role verbatim.',
    emotion_regulation: { enabled: false, temperament: 'expressive' },
    emotion_baseline: { state: 'disgusted' },
    smile: 0.35,
    brows: 0.25,
    background: 'dark-lounge',
    blur: 9,
    blinking: false,
    movement: false,
  })
  await page.screenshot({ path: test.info().outputPath('customization-behavior.png') })
  await showAppearance(page)
  await page.screenshot({ path: test.info().outputPath('customization-appearance.png') })
  expect(remote.sessionRequests).toEqual([])
  expect(remote.commands).toEqual([])
})

test('customization navigation preserves the live session, transcript scroll and ongoing reply', async ({
  page,
}) => {
  const remote = await fixture(page)
  await openStream(page)
  await expect(page.getByRole('button', { name: 'Enable microphone', exact: true })).toBeEnabled()
  for (let i = 0; i < 16; i++) {
    remote.publish({
      type: 'conversation',
      data: { type: 'accepted', turn_id: `custom-${i}`, text: `Question ${i}` },
    })
    remote.publish({
      type: 'conversation',
      data: {
        type: 'reply_delta',
        turn_id: `custom-${i}`,
        text: `Response ${i}: ` + 'Cached fixture text. '.repeat(15),
      },
    })
    remote.publish({ type: 'state', data: { state: 'idle' } })
  }
  await expect(page.getByText('Question 15', { exact: true })).toBeVisible()
  const transcript = page.getByRole('log', { name: 'Conversation', exact: true })
  await transcript.evaluate((el) => {
    el.scrollTop = 60
    el.dispatchEvent(new Event('scroll'))
  })
  await page.evaluate(() =>
    Object.defineProperty(document, 'originalCustomizeVideo', {
      value: document.querySelector<HTMLVideoElement>('video.remote-source'),
    }),
  )
  const count = remote.commands.length
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  await expect(transcript).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Chat with Syntelle', exact: true })).toBeVisible()
  const drawer = page.getByRole('dialog', { name: 'Customize Syntelle', exact: true })
  await expect(drawer).toBeVisible()
  expect(await transcript.evaluate((el) => el.closest('dialog'))).toBe(null)
  await expect(page.getByRole('button', { name: 'Enable microphone', exact: true })).toBeVisible()
  await expect(
    page.getByRole('textbox', { name: 'Character instructions', exact: true }),
  ).toBeEnabled()
  remote.publish({
    type: 'conversation',
    data: { type: 'accepted', turn_id: 'custom-live', text: 'Live question' },
  })
  remote.publish({
    type: 'conversation',
    data: { type: 'reply_delta', turn_id: 'custom-live', text: 'A continuing response.' },
  })
  remote.publish({ type: 'playback', data: { event: 'playback_started', turn_id: 'custom-live' } })
  await expect(
    page.getByRole('textbox', { name: 'Character instructions', exact: true }),
  ).toBeDisabled()
  await expect(
    page.getByText(
      'Turn off the microphone and wait for any reply or connection to finish before editing instructions.',
      { exact: true },
    ),
  ).toBeVisible()
  await expect(page.getByRole('button', { name: 'Stop reply', exact: true })).toBeVisible()
  await showAppearance(page)
  const appearance = page.getByRole('tab', { name: 'Appearance', exact: true })
  await appearance.focus()
  await page.keyboard.press('ArrowLeft')
  await expect(page.getByRole('tab', { name: 'Behavior', exact: true })).toBeFocused()
  await showAdvanced(page)
  await expect(page.getByRole('button', { name: /Play gesture/ })).toBeDisabled()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('button', { name: 'Customize', exact: true })).toBeFocused()
  await expect.poll(() => transcript.evaluate((el) => el.scrollTop)).toBe(60)
  expect(
    await page.evaluate(
      () =>
        Object.getOwnPropertyDescriptor(document, 'originalCustomizeVideo')?.value ===
        document.querySelector<HTMLVideoElement>('video.remote-source'),
    ),
  ).toBe(true)
  expect(remote.commands.slice(count)).toEqual([])
  expect(remote.sessionRequests).toHaveLength(1)
  expect(remote.deleted).toEqual([])
  await expect(page.getByRole('button', { name: 'Stop reply', exact: true })).toBeVisible()
  remote.publish({ type: 'playback', data: { event: 'playback_ended', turn_id: 'custom-live' } })
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  await expect(
    page.getByRole('textbox', { name: 'Character instructions', exact: true }),
  ).toBeEnabled()
  await expect(page.getByRole('button', { name: 'Advanced', exact: true })).toHaveAttribute(
    'aria-expanded',
    'false',
  )
})

test('customization localized layout supports themes, reduced motion and keyboard navigation', async ({
  page,
}) => {
  const remote = await fixture(page)
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.goto('/')
  await chooseLanguage(page, 'es')
  await page.getByRole('button', { name: 'Personalizar', exact: true }).click()
  await expect(
    page.getByRole('heading', { name: 'Personalizar Syntelle', exact: true }),
  ).toBeFocused()
  await expect(page.getByRole('img', { name: 'Retrato de Syntelle', exact: true })).toBeVisible()
  const first = page.getByRole('tab', { name: 'Comportamiento', exact: true })
  await first.focus()
  await page.keyboard.press('End')
  await expect(page.getByRole('tab', { name: 'Apariencia', exact: true })).toBeFocused()
  await expect(page.getByRole('checkbox', { name: 'Parpadeo natural', exact: true })).toBeDisabled()
  await expect(
    page.getByRole('checkbox', { name: 'Movimiento de cabeza y respiración', exact: true }),
  ).toBeDisabled()
  expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)).toBe(false)
  await page.screenshot({ path: test.info().outputPath('customization-spanish-theme.png') })
  await page.getByRole('button', { name: 'Modo oscuro', exact: true }).click()
  await page.screenshot({ path: test.info().outputPath('customization-spanish-other-theme.png') })
  await page.keyboard.press('Escape')
  await expect(page.getByRole('button', { name: 'Personalizar', exact: true })).toBeFocused()
  await page.getByRole('button', { name: 'Personalizar', exact: true }).click()
  const moodSelect = page.getByRole('combobox', { name: 'Ánimo', exact: true })
  if (await moodSelect.isVisible()) await moodSelect.focus()
  else await page.getByRole('combobox', { name: 'Estilo de reacción', exact: true }).focus()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('button', { name: 'Personalizar', exact: true })).toBeFocused()
  expect(remote.sessionRequests).toEqual([])
  expect(remote.commands).toEqual([])
})
