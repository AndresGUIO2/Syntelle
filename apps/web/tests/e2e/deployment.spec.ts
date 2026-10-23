import { test, expect } from '@playwright/test'
test('cloud ICE reaches the peer and explicit stop releases the stream', async ({ page }) => {
  let creates = 0,
    deletes = 0
  const ice = [
    {
      urls: ['turns:relay.example:443?transport=tcp'],
      username: 'expiring-fixture',
      credential: 'fixture',
    },
  ]
  await page.route('**/api/**', (route) => {
    if (route.request().method() === 'DELETE') {
      deletes++
      return route.fulfill({ json: {} })
    }
    if (route.request().method() === 'POST') {
      creates++
      return route.fulfill({
        status: 201,
        json: {
          id: 'fixture_session',
          ticket: 'one-use',
          state: 'starting',
          configured: true,
          recognition: false,
          startup_timeout_ms: 95000,
          ice_servers: ice,
        },
      })
    }
    return route.fulfill({ json: {} })
  })
  await page.goto('/pricing')
  const result = await page.evaluate(async () => {
    const modulePath = '/src/features/streaming/composables/useRemoteStream.ts'
    const typesPath = '/src/features/streaming/types.ts'
    const { useRemoteStream } = await import(modulePath)
    const { defaultSettings } = await import(typesPath)
    let received: unknown
    const originalPeer = window.RTCPeerConnection,
      originalSocket = window.WebSocket
    class Peer {
      constructor(config: unknown) {
        received = config
      }
      close() {}
    }
    class Socket {
      readyState = 1
      close() {}
      send() {}
    }
    Object.assign(window, { RTCPeerConnection: Peer, WebSocket: Socket })
    try {
      const remote = useRemoteStream(defaultSettings)
      await remote.connect()
      await remote.stop()
      return received
    } finally {
      Object.assign(window, { RTCPeerConnection: originalPeer, WebSocket: originalSocket })
    }
  })
  expect(result).toEqual({ iceServers: ice })
  expect(creates).toBe(1)
  expect(deletes).toBe(1)
})
test('startup rejection has no automatic paid-command retry', async ({ page }) => {
  await page.clock.install()
  let creates = 0
  await page.route('**/api/**', (route) => {
    if (route.request().method() === 'POST') creates++
    return route.fulfill({
      status: 503,
      json: { error: { message: 'Offline renderer launch rejected' } },
    })
  })
  await page.goto('/pricing')
  const message = await page.evaluate(async () => {
    const path = '/src/features/streaming/composables/useRemoteStream.ts',
      types = '/src/features/streaming/types.ts'
    const { useRemoteStream } = await import(path),
      { defaultSettings } = await import(types)
    const remote = useRemoteStream(defaultSettings)
    await remote.connect()
    const error = remote.error.value
    await remote.stop()
    return error
  })
  expect(message).toContain('Offline renderer launch rejected')
  await page.clock.fastForward(30_000)
  expect(creates).toBe(1)
})

test('reconnect reuses the admitted stream and requests a fresh ticket', async ({ page }) => {
  let creates = 0,
    tickets = 0
  await page.route('**/api/**', (route) => {
    const path = new URL(route.request().url()).pathname
    if (path.endsWith('/ticket')) {
      tickets++
      return route.fulfill({ json: { ticket: 'fresh-fixture-ticket' } })
    }
    if (path === '/api/v1/streams' && route.request().method() === 'POST') creates++
    return route.fulfill({
      json: {
        id: 'fixture_session',
        ticket: 'first-ticket',
        state: 'starting',
        configured: false,
        recognition: false,
      },
    })
  })
  await page.goto('/pricing')
  await page.evaluate(async () => {
    const path = '/src/features/streaming/composables/useRemoteStream.ts',
      types = '/src/features/streaming/types.ts'
    const { useRemoteStream } = await import(path),
      { defaultSettings } = await import(types)
    const originalPeer = window.RTCPeerConnection,
      originalSocket = window.WebSocket
    class Peer {
      close() {}
    }
    class Socket {
      readyState = 1
      close() {}
      send() {}
    }
    Object.assign(window, { RTCPeerConnection: Peer, WebSocket: Socket })
    try {
      const remote = useRemoteStream(defaultSettings)
      await remote.connect()
      await remote.connect(true)
      await remote.stop()
    } finally {
      Object.assign(window, { RTCPeerConnection: originalPeer, WebSocket: originalSocket })
    }
  })
  expect(creates).toBe(1)
  expect(tickets).toBe(1)
})

for (const signedIn of [false, true]) {
  test(`${signedIn ? 'account' : 'guest'} fixture opens without allocating a renderer`, async ({
    page,
  }) => {
    let starts = 0
    await page.route('**/api/**', (route) => {
      const path = new URL(route.request().url()).pathname
      if (route.request().method() === 'POST' && path === '/api/v1/streams') starts++
      if (path === '/api/v1/auth/me')
        return route.fulfill({
          json: {
            account: signedIn
              ? { id: 'fixture-account', email: 'fixture@example.test', name: 'Fixture' }
              : null,
          },
        })
      return route.fulfill({
        json: {
          items: [],
          available: false,
          configured: false,
          time_limits_enabled: true,
          providers: {},
        },
      })
    })
    await page.goto('/')
    await expect(
      page.getByRole('button', { name: signedIn ? 'Your account' : 'Sign in', exact: true }),
    ).toBeVisible()
    expect(starts).toBe(0)
  })
}
