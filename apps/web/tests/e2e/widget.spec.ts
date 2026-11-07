import { test, expect, type Page } from '@playwright/test'
import { createServer as httpServer, type Server } from 'node:http'
import { createServer, type ViteDevServer } from 'vite'
import type { AddressInfo } from 'node:net'

// Real separate-origin embedding and production framing middleware. Only API,
// challenge and media providers are fixtures; no paid requests can escape.
test.describe.configure({ mode: 'serial' })
let api: Server, customer: Server, vite: ViteDevServer
let customerOrigin = '',
  playerOrigin = ''
const id = 'fixture-widget-126'
const listen = (server: Server) =>
  new Promise<string>((resolve) => {
    server.listen(0, '127.0.0.1', () =>
      resolve(`http://127.0.0.1:${(server.address() as AddressInfo).port}`),
    )
  })
const metadata = () => ({
  id,
  name: 'Meet Syntelle',
  allowed_origins: [customerOrigin],
  available: true,
  turnstile_site_key: 'fixture-site-key',
  appearance: {
    placement: 'inline',
    position: 'bottom-right',
    theme: 'light',
    language: 'en-US',
    launcher_label: 'Talk with Syntelle',
  },
  player: {
    character_name: 'Syntelle',
    background: 'natural-space',
    blur: 0,
    poster: '/characters/syntelle-poster.jpg',
  },
})
test.beforeAll(async () => {
  customer = httpServer((req, res) => {
    res.setHeader('Content-Type', 'text/html')
    const mode = req.url?.includes('floating') ? 'floating' : 'inline'
    res.end(
      `<!doctype html><html><head><meta name="viewport" content="width=device-width,initial-scale=1"></head><body style="margin:24px;font:16px system-ui;background:#f6f5f8"><h1>A customer website</h1><p>The character is embedded in this page.</p><div style="max-width:440px"><script src="${playerOrigin}/widget/v1.js" data-widget="${id}" data-mode="${mode}" async></script></div></body></html>`,
    )
  })
  customerOrigin = await listen(customer)
  api = httpServer((req, res) => {
    res.setHeader('Content-Type', 'application/json')
    if (req.url === `/api/v1/widgets/${id}`) res.end(JSON.stringify(metadata()))
    else {
      res.statusCode = 404
      res.end('{}')
    }
  })
  process.env.API_PROXY_TARGET = await listen(api)
  vite = await createServer({ server: { port: 0, strictPort: false, host: '127.0.0.1' } })
  await vite.listen()
  playerOrigin = `http://127.0.0.1:${(vite.httpServer!.address() as AddressInfo).port}`
})
test.afterAll(async () => {
  await vite?.close()
  await Promise.all(
    [api, customer]
      .filter(Boolean)
      .map((server) => new Promise<void>((resolve) => server.close(() => resolve()))),
  )
})

async function fixture(
  page: Page,
  pixels: boolean,
  options: {
    slowChallenge?: boolean
    denied?: boolean
    spanish?: boolean
    theme?: 'system' | 'light' | 'dark'
  } = {},
) {
  const starts: Record<string, unknown>[] = [],
    deletes: string[] = []
  const commands: { type: string; data?: { type: string; text?: string } }[] = []
  const requests: { path: string; cookie?: string; auth?: string }[] = []
  const unexpected: string[] = []
  await page.addInitScript((pixels) => {
    const tracks: MediaStreamTrack[] = []
    Object.defineProperty(window, 'widgetFixtureTracks', { value: tracks })
    class Peer {
      connectionState = 'new'
      onconnectionstatechange?: () => void
      ontrack?: (event: { track: MediaStreamTrack }) => void
      constructor() {
        setTimeout(() => {
          if (this.connectionState === 'closed') return
          this.connectionState = 'connected'
          this.onconnectionstatechange?.()
          if (!pixels) return
          const canvas = document.createElement('canvas')
          canvas.width = 1440
          canvas.height = 960
          const ctx = canvas.getContext('2d')!
          ctx.fillStyle = 'black'
          ctx.fillRect(0, 0, 1440, 960)
          ctx.fillStyle = '#bba3f5'
          ctx.fillRect(200, 150, 200, 400)
          ctx.fillStyle = 'white'
          ctx.fillRect(920, 150, 200, 400)
          const video = canvas.captureStream(30).getVideoTracks()[0]!
          tracks.push(video)
          this.ontrack?.({ track: video })
          const audio = new AudioContext(),
            destination = audio.createMediaStreamDestination()
          const oscillator = audio.createOscillator(),
            gain = audio.createGain()
          gain.gain.value = 0
          oscillator.connect(gain).connect(destination)
          oscillator.start()
          const sound = destination.stream.getAudioTracks()[0]!
          const stop = sound.stop.bind(sound)
          sound.stop = () => {
            stop()
            void audio.close()
          }
          tracks.push(sound)
          this.ontrack?.({ track: sound })
        }, 30)
      }
      close() {
        this.connectionState = 'closed'
      }
    }
    Object.defineProperty(window, 'RTCPeerConnection', { value: Peer })
    if (pixels) {
      navigator.mediaDevices.getUserMedia = async () => {
        const audio = new AudioContext(),
          destination = audio.createMediaStreamDestination()
        const track = destination.stream.getAudioTracks()[0]!,
          stop = track.stop.bind(track)
        track.stop = () => {
          stop()
          void audio.close()
        }
        tracks.push(track)
        return destination.stream
      }
    } else {
      Object.defineProperty(window, 'MediaStream', {
        value: class extends EventTarget {
          getTracks() {
            return []
          }
          getAudioTracks() {
            return []
          }
          getVideoTracks() {
            return []
          }
        },
      })
      Object.defineProperty(HTMLMediaElement.prototype, 'srcObject', {
        writable: true,
        value: null,
      })
      HTMLMediaElement.prototype.play = () => Promise.resolve()
    }
  }, pixels)
  await page
    .context()
    .addCookies([{ name: 'syntelle_account', value: 'must-not-be-sent', url: playerOrigin }])
  await page.route('https://challenges.cloudflare.com/**', (route) =>
    route.fulfill({
      contentType: 'application/javascript',
      body: `window.turnstile={render:(_e,o)=>{setTimeout(()=>o.callback('fixture-token'),${options.slowChallenge ? 900 : 10});return 'fixture'},remove:()=>{}}`,
    }),
  )
  await page.route('**/api/**', async (route) => {
    const req = route.request(),
      path = new URL(req.url()).pathname,
      headers = await req.allHeaders()
    requests.push({ path, cookie: headers.cookie, auth: headers.authorization })
    if (path === `/api/v1/widgets/${id}`) {
      const data = metadata()
      if (options.theme) data.appearance.theme = options.theme
      if (options.spanish) {
        data.appearance.language = 'es-CO'
        data.appearance.launcher_label = 'Hablar con Syntelle'
        data.appearance.theme = 'dark'
      }
      await route.fulfill({ json: data })
    } else if (path.endsWith('/bootstrap')) {
      expect(req.postDataJSON()).toEqual({ parent_origin: customerOrigin })
      await route.fulfill({
        json: {
          bootstrap_id: 'bootstrap-126',
          nonce: 'server-bound-nonce',
          expires_at: new Date(Date.now() + 60_000).toISOString(),
        },
      })
    } else if (path.endsWith('/sessions')) {
      starts.push(req.postDataJSON())
      await route.fulfill(
        options.denied
          ? {
              status: 402,
              json: { error: { message: 'This workspace has no available minutes.' } },
            }
          : {
              json: {
                id: `widget-session-${starts.length}`,
                capability: `syntelle_wp_fixture_${starts.length}`,
                ticket: 'one-use-ticket',
                state: 'idle',
                configured: true,
                recognition: pixels,
              },
            },
      )
    } else if (/\/streams\/widget-session-\d+$/.test(path) && req.method() === 'DELETE') {
      deletes.push(path)
      await route.fulfill({ json: {} })
    } else {
      unexpected.push(`${req.method()} ${path}`)
      await route.fulfill({ status: 500, json: {} })
    }
  })
  await page.routeWebSocket('**/api/v1/streams/*/ws', (socket) => {
    socket.onMessage((raw) => {
      const message = JSON.parse(String(raw))
      commands.push(message)
      if (message.type === 'authenticate')
        socket.send(
          JSON.stringify({
            type: 'ready',
            session_id: `widget-session-${starts.length}`,
            data: { state: 'idle' },
          }),
        )
    })
  })
  return { starts, deletes, commands, requests, unexpected }
}

test('floating widget requires explicit Start, uses scoped capability and releases on close', async ({
  page,
  browserName,
}) => {
  const state = await fixture(page, browserName === 'chromium')
  await page.goto(`${customerOrigin}/floating`)
  const launcher = page.getByRole('button', { name: 'Talk with Syntelle', exact: true })
  await expect(launcher).toBeVisible()
  expect(state.starts).toHaveLength(0)
  await launcher.click()
  const player = page.frameLocator('iframe')
  await expect(
    player.getByRole('button', { name: 'Talk with Syntelle', exact: true }),
  ).toBeEnabled()
  expect(state.starts).toHaveLength(0)
  await expect(
    player.getByRole('img', { name: 'Syntelle', exact: true }).filter({ visible: true }),
  ).toBeVisible()
  await expect
    .poll(() =>
      player.locator('picture img').evaluate((image: HTMLImageElement) => image.naturalWidth),
    )
    .toBeGreaterThan(0)
  await player.locator('picture img').evaluate((image: HTMLImageElement) => image.decode())
  await test.info().attach('portrait-layout.json', {
    body: JSON.stringify(
      await player
        .locator('.widget-stage,.remote-stage,.remote-media,.remote-poster,.remote-poster img')
        .evaluateAll((elements) =>
          elements.map((element) => ({
            name: element.className || element.tagName,
            rect: element.getBoundingClientRect().toJSON(),
            style: {
              position: getComputedStyle(element).position,
              height: getComputedStyle(element).height,
              width: getComputedStyle(element).width,
              display: getComputedStyle(element).display,
              opacity: getComputedStyle(element).opacity,
            },
          })),
        ),
      null,
      2,
    ),
    contentType: 'application/json',
  })
  await page.screenshot({ path: test.info().outputPath('floating-before-start.png') })
  await player.getByRole('button', { name: 'Talk with Syntelle', exact: true }).click()
  await expect(player.getByRole('textbox', { name: 'Message', exact: true })).toBeVisible()
  expect(state.starts).toHaveLength(1)
  expect(state.starts[0]).toMatchObject({
    parent_origin: customerOrigin,
    bootstrap_id: 'bootstrap-126',
    nonce: 'server-bound-nonce',
    turnstile_token: 'fixture-token',
  })
  expect(state.starts[0]).not.toHaveProperty('settings')
  await player
    .getByRole('textbox', { name: 'Message', exact: true })
    .fill('Hello from an embedded website')
  await player.getByRole('button', { name: 'Send', exact: true }).click()
  await expect
    .poll(() =>
      state.commands.some((event) => event.data?.text === 'Hello from an embedded website'),
    )
    .toBe(true)
  if (browserName === 'chromium') {
    await player.getByRole('button', { name: 'Turn on microphone', exact: true }).click()
    await expect(
      player.getByRole('button', { name: 'Turn off microphone', exact: true }),
    ).toBeVisible()
  }
  await page.getByRole('button', { name: 'Close character', exact: true }).click()
  await expect.poll(() => state.deletes.length).toBe(1)
  await expect(launcher).toBeFocused()
  const iframe = page.frames().find((frame) => frame.url().includes('/embed/'))!
  if (browserName === 'chromium')
    expect(
      await iframe.evaluate(() =>
        (
          Object.getOwnPropertyDescriptor(window, 'widgetFixtureTracks')!
            .value as MediaStreamTrack[]
        ).every((track) => track.readyState === 'ended'),
      ),
    ).toBe(true)
  await launcher.click()
  await expect(
    player.getByRole('button', { name: 'Talk with Syntelle', exact: true }),
  ).toBeEnabled()
  expect(state.starts).toHaveLength(1)
  expect(state.requests.every((req) => !req.cookie)).toBe(true)
  expect(state.requests.find((req) => req.path.endsWith('widget-session-1'))?.auth).toBe(
    'Bearer syntelle_wp_fixture_1',
  )
  expect(state.unexpected).toEqual([])
})

test('inline Spanish widget keeps media stopped and cannot be started by a forged parent message', async ({
  page,
}) => {
  const state = await fixture(page, false, { spanish: true })
  await page.goto(customerOrigin)
  const frame = page.frameLocator('iframe')
  await expect(
    frame.getByRole('button', { name: 'Hablar con Syntelle', exact: true }),
  ).toBeEnabled()
  await page.evaluate(() =>
    document
      .querySelector('[data-syntelle-widget]')!
      .shadowRoot!.querySelector('iframe')!
      .contentWindow!.postMessage(
        { channel: 'syntelle-widget-v1', type: 'start', nonce: 'forged' },
        '*',
      ),
  )
  expect(state.starts).toHaveLength(0)
  await expect(frame.locator('html')).toHaveAttribute('data-theme', 'dark')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  await page.screenshot({ path: test.info().outputPath('inline-spanish.png') })
  expect(state.unexpected).toEqual([])
})

test('closing during verification cancels preparation without starting a session', async ({
  page,
}) => {
  const state = await fixture(page, false, { slowChallenge: true })
  await page.goto(`${customerOrigin}/floating`)
  await page.getByRole('button', { name: 'Talk with Syntelle', exact: true }).click()
  const frame = page.frameLocator('iframe')
  await frame.getByRole('button', { name: 'Talk with Syntelle', exact: true }).click()
  await expect(frame.getByRole('button', { name: 'Connecting…', exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Close character', exact: true }).click()
  await page.waitForTimeout(1000)
  expect(state.starts).toHaveLength(0)
  expect(state.unexpected).toEqual([])
})

test('quota refusal stays in the widget and never falls back to personal or demo access', async ({
  page,
}) => {
  const state = await fixture(page, false, { denied: true })
  await page.goto(customerOrigin)
  const frame = page.frameLocator('iframe')
  await frame.getByRole('button', { name: 'Talk with Syntelle', exact: true }).click()
  await expect(frame.getByRole('alert')).toHaveText('This workspace has no available minutes.')
  expect(state.starts).toHaveLength(1)
  expect(state.commands).toEqual([])
  expect(state.unexpected).toEqual([])
})

for (const theme of ['system', 'light', 'dark'] as const) {
  for (const mode of ['inline', 'floating'] as const) {
    test(`night theme stays synchronized for ${theme} ${mode} without starting media`, async ({
      page,
    }) => {
      await page.emulateMedia({ colorScheme: 'dark' })
      await page.addInitScript(() => {
        window.addEventListener('message', (event) => {
          if (event.data?.type === 'ready' && event.data?.channel === 'syntelle-widget-v1')
            (window as Window & { themeReviewNonce?: string }).themeReviewNonce = event.data.nonce
        })
      })
      const state = await fixture(page, false, { theme })
      await page.goto(mode === 'floating' ? `${customerOrigin}/floating` : customerOrigin)
      const host = page.locator('[data-syntelle-widget]')
      const frame = page.frameLocator('iframe')
      const initial = theme === 'light' ? 'light' : 'dark'
      await expect(host).toHaveAttribute('data-syntelle-theme', initial)
      if (mode === 'floating')
        await page.getByRole('button', { name: 'Talk with Syntelle', exact: true }).click()
      await expect(frame.locator('html')).toHaveAttribute('data-theme', initial)
      if (initial === 'dark') {
        await expect(frame.locator('.start-action')).toHaveCSS(
          'background-color',
          'rgb(238, 234, 226)',
        )
        const mirrors = await frame.locator('html').evaluate((element) => {
          const style = getComputedStyle(element)
          return Object.fromEntries(
            ['panel', 'text', 'divider', 'primary', 'primary-hover', 'on-primary', 'accent'].map(
              (name) => [name, style.getPropertyValue(`--night-${name}`).trim()],
            ),
          )
        })
        const hostColors = await host.evaluate((element) => {
          const style = (element as HTMLElement).style
          return Object.fromEntries(
            ['surface', 'text', 'border', 'primary', 'primaryHover', 'onPrimary', 'accent'].map(
              (name) => [name, style.getPropertyValue(`--syntelle-widget-${name}`).trim()],
            ),
          )
        })
        expect(hostColors).toEqual({
          surface: mirrors.panel,
          text: mirrors.text,
          border: mirrors.divider,
          primary: mirrors.primary,
          primaryHover: mirrors['primary-hover'],
          onPrimary: mirrors['on-primary'],
          accent: mirrors.accent,
        })
      }
      await page.emulateMedia({ colorScheme: 'light' })
      const next = theme === 'system' ? 'light' : initial
      await expect(frame.locator('html')).toHaveAttribute('data-theme', next)
      await expect(host).toHaveAttribute('data-syntelle-theme', next)
      const nonce = await page.evaluate(
        () => (window as Window & { themeReviewNonce?: string }).themeReviewNonce,
      )
      const player = page.frames().find((item) => item.url().includes('/embed/'))!
      const forgedTheme = next === 'dark' ? 'light' : 'dark'
      await player.evaluate(
        ({ origin, forgedTheme }) =>
          window.parent.postMessage(
            {
              channel: 'syntelle-widget-v1',
              type: 'theme-changed',
              nonce: 'forged',
              theme: forgedTheme,
            },
            origin,
          ),
        { origin: customerOrigin, forgedTheme },
      )
      await expect(host).toHaveAttribute('data-syntelle-theme', next)
      await player.evaluate(
        ({ origin, nonce }) =>
          window.parent.postMessage(
            { channel: 'syntelle-widget-v1', type: 'theme-changed', nonce, theme: 'invalid' },
            origin,
          ),
        { origin: customerOrigin, nonce },
      )
      await expect(host).toHaveAttribute('data-syntelle-theme', next)
      await page.evaluate(
        ({ nonce, forgedTheme }) =>
          window.postMessage(
            { channel: 'syntelle-widget-v1', type: 'theme-changed', nonce, theme: forgedTheme },
            '*',
          ),
        { nonce, forgedTheme },
      )
      await expect(host).toHaveAttribute('data-syntelle-theme', next)
      await page.evaluate(
        ({ nonce, forgedTheme }) => {
          const iframe = document
            .querySelector('[data-syntelle-widget]')!
            .shadowRoot!.querySelector('iframe')!
          window.dispatchEvent(
            new MessageEvent('message', {
              source: iframe.contentWindow,
              origin: 'https://forged.example',
              data: {
                channel: 'syntelle-widget-v1',
                type: 'theme-changed',
                nonce,
                theme: forgedTheme,
              },
            }),
          )
        },
        { nonce, forgedTheme },
      )
      await expect(host).toHaveAttribute('data-syntelle-theme', next)
      await page.emulateMedia({ colorScheme: 'dark' })
      await expect(host).toHaveAttribute('data-syntelle-theme', initial)
      await expect(frame.locator('html')).toHaveAttribute('data-theme', initial)
      await page.screenshot({ path: test.info().outputPath(`${theme}-${mode}.png`) })
      expect(state.starts).toHaveLength(0)
      expect(state.commands).toHaveLength(0)
      expect(state.unexpected).toEqual([])
    })
  }
}
