import { expect, test, type Page } from '@playwright/test'
import { defaultSettings } from '../../src/features/streaming/types'

async function fixture(page: Page, signedIn = true) {
  const requests: Record<string, unknown>[] = [],
    commands: string[] = [],
    deleted: string[] = []
  let publish = (_data: unknown) => {},
    pause: 'success' | 'failure' | 'pending' = 'success'
  let sessionDelay: Promise<void> | undefined
  let owner = { id: 'owner', email: 'owner@example.com', name: 'Alex' }
  await page.addInitScript(() => {
    const activity = { plays: 0, captures: 0 }
    Object.assign(window, { navigationMedia: activity })
    class Peer {
      connectionState = 'new'
      onconnectionstatechange: (() => void) | null = null
      constructor() {
        setTimeout(() => {
          if (this.connectionState !== 'closed') {
            this.connectionState = 'connected'
            this.onconnectionstatechange?.()
          }
        }, 20)
      }
      close() {
        this.connectionState = 'closed'
      }
    }
    Object.defineProperty(window, 'RTCPeerConnection', { value: Peer })
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
    HTMLMediaElement.prototype.play = () => {
      activity.plays++
      return Promise.resolve()
    }
    Object.defineProperty(navigator, 'mediaDevices', {
      value: {
        getUserMedia: () => {
          activity.captures++
          return Promise.reject(new Error('Fixture: microphone capture is not expected'))
        },
      },
    })
  })
  await page.route('**/api/**', async (route) => {
    const request = route.request(),
      path = new URL(request.url()).pathname
    const reply = (json: unknown, status = 200) => route.fulfill({ json, status })
    if (path === '/api/v1/auth/me')
      return reply({
        account: signedIn ? owner : null,
      })
    if (path === '/api/v1/auth/status')
      return reply({
        email: { configured: true },
        google: { configured: false },
        github: { configured: false },
      })
    if (path === '/api/v1/auth/email/start')
      return reply({ challenge_id: 'fixture-code', resend_after: 30 })
    if (path === '/api/v1/auth/email/verify') {
      signedIn = true
      return reply({ account: owner })
    }
    if (path === '/api/v1/auth/logout') {
      signedIn = false
      return reply({})
    }
    if (path === '/api/v1/admin/eligibility') return reply({}, 403)
    if (path === '/api/v1/streams/status')
      return reply({ configured: true, time_limits_enabled: false })
    if (path === '/api/v1/streams' && request.method() === 'POST') {
      requests.push(request.postDataJSON())
      await sessionDelay
      return reply({
        id: 'navigation-stream',
        ticket: 'fixture',
        state: 'idle',
        configured: true,
        recognition: false,
      })
    }
    if (path === '/api/v1/streams/navigation-stream' && request.method() === 'DELETE') {
      deleted.push(path)
      return reply({})
    }
    if (
      [
        '/api/v1/characters',
        '/api/v1/workspaces',
        '/api/v1/configurations',
        '/api/v1/keys',
        '/api/v1/conversations',
        '/api/v1/conversations/claimable',
      ].includes(path)
    )
      return reply({ items: [] })
    return reply({ error: { message: 'Fixture: unavailable' } }, 404)
  })
  await page.routeWebSocket('**/api/v1/streams/*/ws', (socket) => {
    publish = (data) =>
      socket.send(JSON.stringify({ session_id: 'navigation-stream', ...(data as object) }))
    socket.onMessage((raw) => {
      const event = JSON.parse(String(raw))
      if (event.type === 'authenticate') publish({ type: 'ready', data: { state: 'idle' } })
      if (event.type === 'command') {
        commands.push(event.data.type)
        if (event.data.type === 'suspend' && pause !== 'pending')
          publish(
            pause === 'success'
              ? { type: 'paused', data: { reason: 'navigation' } }
              : { type: 'error', data: { code: 'suspension_failed', message: 'Pause failed' } },
          )
      }
    })
  })
  return {
    requests,
    commands,
    deleted,
    owner: () => (signedIn ? owner.id : null),
    setOwner: (id: string) => {
      signedIn = true
      owner = { id, email: `${id}@example.invalid`, name: id === 'owner' ? 'Alex' : 'Sam' }
    },
    publish: (data: unknown) => publish(data),
    pause: (value: typeof pause) => {
      pause = value
    },
    delay: (value: Promise<void>) => {
      sessionDelay = value
    },
  }
}

async function openNavigation(page: Page) {
  await expect(page.locator('.navigation-shell')).toBeVisible()
  if (await page.locator('.mobile-drawer[open]').count()) return
  const open = page.getByRole('button', { name: 'Open navigation', exact: true })
  if (await open.isVisible()) await open.click()
}
async function visit(page: Page, name: string) {
  const historyPanel = page.getByRole('dialog', { name: 'Conversation history', exact: true })
  if (await historyPanel.isVisible()) await page.keyboard.press('Escape')
  await openNavigation(page)
  await page.getByRole('link', { name, exact: true }).click()
}

async function populatedHistory(page: Page, server: Awaited<ReturnType<typeof fixture>>) {
  await page.clock.setFixedTime(new Date('2026-11-05T17:00:00Z'))
  const titles = [
    'A walk through the forest and the small things we almost missed',
    'Choosing the next chapter of our adventure',
    'An unfinished conversation about tomorrow',
    'Cómo hacer una pausa cuando el día se siente demasiado largo',
    'A little encouragement before the interview',
    'Ideas para cocinar con lo que queda en la nevera',
    'The song that brought back a memory',
    '¿Qué podemos aprender de una conversación difícil?',
    'Planning a weekend by the sea',
    'Preguntas sobre astronomía y noches sin luna',
    'A story about a lighthouse keeper',
    'Practiquemos inglés sin miedo a equivocarnos',
    'Something funny happened on the train',
    'Pequeños hábitos para un proyecto grande',
    'Finding the right words for an apology',
    'La biblioteca que abría solamente cuando llovía',
    'The difference between rest and giving up',
    'Una idea que todavía no tiene nombre',
    'What would you do with an extra hour?',
    'Recuerdos de un viaje por las montañas',
    'An older conversation worth coming back to',
    'Volver a empezar después de una semana difícil',
    'The mystery of the missing blue notebook',
    'Una historia larga sobre un pueblo junto al río y sus habitantes',
  ]
  const items = titles.map((title, index) => ({
    id: `history-${index}`,
    title,
    language: index % 2 === 1 && index > 2 ? 'es-CO' : 'en-US',
    updated_at: new Date(
      Date.parse('2026-11-05T16:00:00Z') -
        (index < 5 ? 0 : index < 9 ? 1 : index < 15 ? 3 : 20) * 86400000 -
        index * 60000,
    ).toISOString(),
  }))
  const deleted = new Set<string>()
  const details: string[] = [],
    pages: string[] = []
  const gates = new Map<string, Promise<void>>()
  await page.route('**/api/v1/conversations**', async (route) => {
    const url = new URL(route.request().url()),
      path = url.pathname
    const owner = server.owner()
    if (path.endsWith('/claimable')) return route.fulfill({ json: { items: [] } })
    if (path === '/api/v1/conversations') {
      const before = url.searchParams.get('before') ?? ''
      pages.push(before)
      if (owner !== 'owner')
        return route.fulfill({
          json: {
            items: [{ ...items[0], id: 'other-owner', title: 'Sam’s private conversation' }],
            next: '',
          },
        })
      return route.fulfill({
        json: {
          items: (before ? items.slice(19) : items.slice(0, 20)).filter(
            (item) => !deleted.has(item.id),
          ),
          next: before ? '' : 'fixture-page-two',
        },
      })
    }
    const id = path.split('/').at(-1)!
    if (route.request().method() === 'DELETE') {
      deleted.add(id)
      return route.fulfill({ status: 204 })
    }
    const item = items.find((item) => item.id === id)
    if (!item) return route.fulfill({ status: 404, json: { error: 'Conversation not found.' } })
    details.push(id)
    await gates.get(id)
    const turn = (sequence: number) => ({
      id: `${id}-turn-${sequence}`,
      sequence,
      status: 'ready',
      user: sequence === 1 ? `How did we begin ${id}?` : `Let's return to ${item.title}.`,
      reply: sequence === 1 ? `Earlier context for ${id}.` : `The saved answer for ${id}.`,
      spoken: sequence === 1 ? `Earlier context for ${id}.` : `The saved answer for ${id}.`,
    })
    return route.fulfill({
      json: {
        conversation: { ...item, settings: { ...defaultSettings(), language: item.language } },
        turns: [turn(url.searchParams.has('before') ? 1 : 2)],
        before: url.searchParams.has('before') ? 0 : 2,
      },
    })
  })
  return {
    items,
    deleted,
    details,
    pages,
    delay: (id: string) => {
      let release!: () => void
      gates.set(
        id,
        new Promise<void>((resolve) => {
          release = resolve
        }),
      )
      return () => {
        gates.delete(id)
        release()
      }
    },
  }
}

async function openHistory(page: Page) {
  if (new URL(page.url()).pathname !== '/') await visit(page, 'Test character')
  const closeNavigation = page.getByRole('button', { name: 'Close navigation', exact: true })
  if (await closeNavigation.isVisible()) await closeNavigation.click()
  const panel = page.getByRole('dialog', { name: 'Conversation history', exact: true })
  if (!(await panel.isVisible()))
    await page.getByRole('button', { name: 'Conversation history', exact: true }).click()
  await expect(panel).toBeVisible()
  return panel
}

async function selectSaved(page: Page, title: string) {
  const panel = await openHistory(page)
  await panel.locator('.conversation-title').filter({ hasText: title }).click()
}

async function expectNoMedia(page: Page) {
  expect(
    await page.evaluate(
      () =>
        (
          window as unknown as {
            navigationMedia: { plays: number; captures: number }
          }
        ).navigationMedia,
    ),
  ).toEqual({ plays: 0, captures: 0 })
}

async function releaseHistory(page: Page, id: string, release: () => void) {
  const response = page.waitForResponse(
    (response) => new URL(response.url()).pathname === `/api/v1/conversations/${id}`,
  )
  release()
  await (await response).finished()
  // Let the response and Vue's next presentation commit before inspecting for leaks.
  await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => resolve())))
}

test('navigation separates collection/create and opens account in place, with responsive accessible shell', async ({
  page,
}, info) => {
  const server = await fixture(page)
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Chat with Syntelle' })).toBeVisible()
  await openNavigation(page)
  await page.screenshot({ path: info.outputPath('navigation-shell.png') })
  await page.getByRole('link', { name: 'Characters', exact: true }).click()
  await expect(page).toHaveURL(/\/characters$/)
  await expect(page.getByRole('heading', { name: 'Characters', exact: true })).toBeVisible()
  await expect(page.getByLabel('Character name', { exact: true })).toHaveCount(0)
  await page.getByRole('link', { name: 'Create character', exact: true }).click()
  await expect(page.getByLabel('Character name', { exact: true })).toBeVisible()
  await page.getByRole('link', { name: 'Back to characters', exact: true }).click()
  await openNavigation(page)
  await page.getByRole('button', { name: 'Your account: Alex', exact: true }).click()
  await expect(page.getByRole('dialog', { name: 'Your account', exact: true })).toBeVisible()
  await expect(page).toHaveURL(/\/characters$/)
  await page.getByRole('button', { name: 'Close account', exact: true }).click()
  await visit(page, 'Conversation')
  await expect(page.getByRole('heading', { name: 'Chat with Syntelle' })).toBeVisible()
  expect(server.requests).toHaveLength(0)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy()
  await page.screenshot({ path: info.outputPath('conversation-shell.png') })
})

test('guest character entry signs in and stays at the requested destination', async ({ page }) => {
  const server = await fixture(page, false)
  await page.goto('/pricing')
  await openNavigation(page)
  await expect(page.getByRole('link', { name: 'Pricing', exact: true })).toBeVisible()
  await page.goto('/characters/new')
  await expect(page.getByRole('dialog', { name: 'Welcome to Syntelle' })).toBeVisible()
  await page.getByLabel('Email address', { exact: true }).fill('owner@example.com')
  await page.getByRole('button', { name: 'Send sign-in code', exact: true }).click()
  await page.getByLabel('Sign-in code', { exact: true }).fill('123456')
  await page.getByRole('button', { name: 'Continue', exact: true }).click()
  await expect(page.getByLabel('Character name', { exact: true })).toBeVisible()
  await expect(page).toHaveURL(/\/characters\/new$/)
  await expect(page.getByRole('link', { name: 'Pricing', exact: true })).toHaveCount(0)
  await page.goto('/pricing')
  await expect(page).toHaveURL(/\/$/)
  await expect(page.getByRole('heading', { name: 'Chat with Syntelle', exact: true })).toBeVisible()
  expect(server.requests).toHaveLength(0)
})

test('leaving conversation suspends once and explicit continuation preserves context and transcript', async ({
  page,
}) => {
  const server = await fixture(page)
  await page.goto('/?character=syntelle')
  await page.getByRole('button', { name: 'Start conversation', exact: true }).click()
  await expect(page.locator('[data-testid="stream-workspace"]')).toHaveAttribute(
    'data-stream-id',
    'navigation-stream',
  )
  server.publish({
    type: 'conversation',
    data: { type: 'accepted', turn_id: 'turn-1', text: 'Remember this conversation.' },
  })
  await expect(page.getByText('Remember this conversation.', { exact: true })).toBeVisible()
  await visit(page, 'Characters')
  await expect.poll(() => server.commands.filter((command) => command === 'suspend').length).toBe(1)
  await visit(page, 'Test character')
  await expect(page.getByText('Remember this conversation.', { exact: true })).toBeVisible()
  await expect(
    page.getByRole('button', { name: 'Continue conversation', exact: true }),
  ).toBeVisible()
  expect(server.requests).toHaveLength(1)
  expect(server.deleted).toEqual([])
  await page.getByRole('button', { name: 'Continue conversation', exact: true }).click()
  await expect.poll(() => server.requests.length).toBe(2)
  expect(server.requests[1]?.resume_from).toBe('navigation-stream')
})

test('rapid return waits for suspension and a failed pause cannot silently start another conversation', async ({
  page,
}) => {
  const server = await fixture(page)
  server.pause('pending')
  await page.goto('/?character=syntelle')
  await page.getByRole('button', { name: 'Start conversation', exact: true }).click()
  await expect(page.locator('[data-testid="stream-workspace"]')).toHaveAttribute(
    'data-stream-id',
    'navigation-stream',
  )
  await visit(page, 'Characters')
  await visit(page, 'Test character')
  expect(server.requests).toHaveLength(1)
  await expect(
    page.getByRole('button', { name: 'Continue conversation', exact: true }),
  ).toHaveCount(0)
  server.publish({ type: 'error', data: { code: 'suspension_failed', message: 'Pause failed' } })
  await expect(
    page.getByText(
      'Your conversation could not be paused safely. Start a new conversation to reconnect.',
      { exact: true },
    ),
  ).toBeVisible()
  expect(server.requests).toHaveLength(1)
})

test('leaving during allocation suspends the late session without starting hidden playback', async ({
  page,
}) => {
  const server = await fixture(page)
  let release!: () => void
  server.delay(
    new Promise<void>((resolve) => {
      release = resolve
    }),
  )
  await page.goto('/?character=syntelle')
  await page.getByRole('button', { name: 'Start conversation', exact: true }).click()
  await expect.poll(() => server.requests.length).toBe(1)
  await visit(page, 'Characters')
  release()
  await expect.poll(() => server.commands.includes('suspend')).toBeTruthy()
  await visit(page, 'Test character')
  await expect(
    page.getByRole('button', { name: 'Continue conversation', exact: true }),
  ).toBeVisible()
  expect(server.requests).toHaveLength(1)
  expect(server.commands.filter((value) => value === 'mic_start' || value === 'preview')).toEqual(
    [],
  )
})

test('sidebar preference and drawer keyboard focus work with Spanish and reduced motion', async ({
  page,
}, info) => {
  await fixture(page)
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.goto('/')
  const trigger = page.getByRole('button', { name: 'Open navigation', exact: true })
  if (await trigger.isVisible()) {
    await trigger.click()
    await page.keyboard.press('Escape')
    await expect(trigger).toBeFocused()
    await trigger.click()
  } else {
    await page.getByRole('button', { name: 'Collapse navigation', exact: true }).click()
    await expect(page.locator('.desktop-sidebar')).toHaveCSS('width', '72px')
    await page.reload()
    await expect(page.locator('.desktop-sidebar')).toHaveCSS('width', '72px')
    await page.getByRole('button', { name: 'Expand navigation', exact: true }).click()
  }
  await page.getByRole('button', { name: 'Platform language: English' }).click()
  await page.getByRole('menuitemradio', { name: 'Español', exact: true }).click()
  await expect(page.getByRole('link', { name: 'Conversación', exact: true })).toBeVisible()
  await page.screenshot({ path: info.outputPath('navigation-spanish.png') })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy()
})

test('shell history browses read-only, deletes inline and explicitly continues from another section', async ({
  page,
}) => {
  const server = await fixture(page)
  const history = await populatedHistory(page, server)
  await page.goto('/characters')
  await selectSaved(page, history.items[1]!.title)
  await expect(page.getByText('The saved answer for history-1.', { exact: true })).toBeVisible()
  await expect(page).toHaveURL(/\/$/)
  await expect(
    page.getByRole('dialog', { name: 'Conversation history', exact: true }),
  ).not.toBeVisible()
  expect(server.requests).toHaveLength(0)
  await expectNoMedia(page)
  const panel = await openHistory(page)
  await panel
    .getByRole('button', { name: `Delete conversation: ${history.items[1]!.title}`, exact: true })
    .click()
  expect(history.deleted.size).toBe(0)
  await expect(panel.getByRole('button', { name: 'Delete', exact: true })).toBeFocused()
  await panel.getByRole('button', { name: 'Delete', exact: true }).click()
  await expect(
    page.locator('.conversation-title').filter({ hasText: history.items[1]!.title }),
  ).toHaveCount(0)
  expect(history.deleted.has('history-1')).toBe(true)
  await expect(page.getByText('The saved answer for history-1.', { exact: true })).toHaveCount(0)
  await selectSaved(page, history.items[0]!.title)
  await expect(page.getByText('The saved answer for history-0.', { exact: true })).toBeVisible()
  expect(server.requests).toHaveLength(0)
  await page.getByRole('button', { name: 'Continue conversation', exact: true }).click()
  await expect(page.locator('[data-testid="stream-workspace"]')).toHaveAttribute(
    'data-stream-id',
    'navigation-stream',
  )
  expect(server.requests).toHaveLength(1)
  expect(server.requests[0]).toMatchObject({ continue_conversation_id: 'history-0' })
  await page.goBack()
  await expect(page).toHaveURL(/\/characters$/)
  await expect.poll(() => server.commands.filter((command) => command === 'suspend').length).toBe(1)
  await page.goForward()
  await expect(page.getByText('The saved answer for history-0.', { exact: true })).toBeVisible()
  await expect(
    page.getByRole('button', { name: 'Continue conversation', exact: true }),
  ).toBeVisible()
  expect(server.requests).toHaveLength(1)
})

test('shell saved setup selects its character and survives back and forward without starting media', async ({
  page,
}) => {
  const server = await fixture(page)
  const character = 'char_11111111111111111111111111111111'
  const settings = {
    ...defaultSettings(),
    character,
    instructions: 'Explain the constellations clearly.',
  }
  await page.route(`**/api/v1/characters/${character}`, (route) =>
    route.fulfill({
      json: {
        id: character,
        name: 'Lyra',
        status: 'ready',
        portrait_available: false,
        created_at: '2026-11-05T12:00:00Z',
      },
    }),
  )
  await page.route('**/api/v1/configurations', (route) =>
    route.fulfill({
      json: {
        items: [
          {
            id: 'lyra-setup',
            name: 'Lyra astronomy',
            package_hash: 'fixture-package',
            settings,
            created_at: '2026-11-05T12:00:00Z',
            updated_at: '2026-11-05T12:00:00Z',
          },
        ],
      },
    }),
  )
  await page.goto('/characters')
  await openNavigation(page)
  await page.getByRole('button', { name: 'Your account: Alex', exact: true }).click()
  const account = page.getByRole('dialog', { name: 'Your account', exact: true })
  await expect(account.getByText('Lyra astronomy', { exact: true })).toBeVisible()
  await expect(page).toHaveURL(/\/characters$/)
  expect(server.requests).toHaveLength(0)
  await account.getByRole('button', { name: 'Use this setup', exact: true }).click()
  await expect(page).toHaveURL(new RegExp(`character=${character}$`))
  await expect(page.getByRole('heading', { name: 'Chat with Lyra', exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  await expect(page.getByLabel('Character instructions', { exact: true })).toHaveValue(
    settings.instructions,
  )
  await page
    .getByRole('dialog', { name: 'Customize character', exact: true })
    .getByRole('button', { name: 'Close', exact: true })
    .click()
  await visit(page, 'Characters')
  await page.goBack()
  await expect(page.getByRole('heading', { name: 'Chat with Lyra', exact: true })).toBeVisible()
  await page.goForward()
  await expect(page).toHaveURL(/\/characters$/)
  await visit(page, 'Conversation')
  await expect(page).toHaveURL(new RegExp(`character=${character}$`))
  await expect(page.getByRole('heading', { name: 'Chat with Lyra', exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Start conversation', exact: true }).click()
  await expect.poll(() => server.requests.length).toBe(1)
  expect(server.requests[0]).toMatchObject({
    settings: { character, instructions: settings.instructions },
  })
})

test('shell sign-out from another section clears the retained account conversation', async ({
  page,
}) => {
  const server = await fixture(page)
  await page.goto('/?character=syntelle')
  await page.getByRole('button', { name: 'Start conversation', exact: true }).click()
  await expect(page.locator('[data-testid="stream-workspace"]')).toHaveAttribute(
    'data-stream-id',
    'navigation-stream',
  )
  server.publish({
    type: 'conversation',
    data: { type: 'accepted', turn_id: 'private-turn', text: 'Private account conversation.' },
  })
  await expect(page.getByText('Private account conversation.', { exact: true })).toBeVisible()
  await visit(page, 'Characters')
  await expect.poll(() => server.commands.filter((command) => command === 'suspend').length).toBe(1)
  await openNavigation(page)
  await page.getByRole('button', { name: 'Your account: Alex', exact: true }).click()
  await page
    .getByRole('dialog', { name: 'Your account', exact: true })
    .getByRole('button', { name: 'Sign out', exact: true })
    .click()
  await expect(page.getByRole('dialog', { name: 'Your account', exact: true })).toHaveCount(0)
  await expect(page).toHaveURL(/\/characters$/)
  await visit(page, 'Test character')
  await expect(page.getByRole('heading', { name: 'Chat with Syntelle', exact: true })).toBeVisible()
  await expect(page.getByText('Private account conversation.', { exact: true })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Start conversation', exact: true })).toBeVisible()
  await expect(
    page.getByRole('button', { name: 'Continue conversation', exact: true }),
  ).toHaveCount(0)
  expect(server.requests).toHaveLength(1)
})

test('Test history opens only in a temporary right panel and preserves read-only preview', async ({
  page,
}, info) => {
  const server = await fixture(page)
  const history = await populatedHistory(page, server)
  await page.goto('/?character=syntelle')
  await expect(page.getByRole('heading', { name: 'Chat with Syntelle', exact: true })).toBeVisible()
  const trigger = page.getByRole('button', { name: 'Conversation history', exact: true })
  const panel = page.getByRole('dialog', { name: 'Conversation history', exact: true })
  await expect(page.locator('.navigation-panel .conversation-list')).toHaveCount(0)
  await trigger.focus()
  await page.keyboard.press('Enter')
  await expect(panel).toBeVisible()
  const bounds = await panel.boundingBox()
  expect(bounds!.x + bounds!.width).toBeCloseTo(page.viewportSize()!.width, 0)
  await expect(panel.locator('.conversation-title')).toHaveCount(20)
  await page.keyboard.press('Escape')
  await expect(panel).not.toBeVisible()
  await expect(trigger).toBeFocused()
  await trigger.click()
  await panel.getByRole('button', { name: 'More conversations', exact: true }).click()
  await expect(panel.locator('.conversation-title')).toHaveCount(24)
  expect(history.pages).toContain('fixture-page-two')
  await expect(panel.getByRole('button', { name: 'Close', exact: true })).toBeInViewport()
  await panel.locator('.conversation-title').filter({ hasText: history.items[0]!.title }).click()
  await expect(panel).not.toBeVisible()
  await expect(page.getByText('The saved answer for history-0.', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Load earlier messages', exact: true }).click()
  await expect(page.getByText('Earlier context for history-0.', { exact: true })).toBeVisible()
  for (const theme of ['dark', 'light']) {
    await page.evaluate((value) => {
      document.documentElement.dataset.theme = value
    }, theme)
    await trigger.click()
    await panel.locator('.history-panel-body').evaluate((element) => {
      element.scrollTop = 0
    })
    await expect(panel.locator('.conversation-title[aria-current="true"]')).toHaveText(
      history.items[0]!.title,
    )
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
    ).toBeTruthy()
    await page.screenshot({ path: info.outputPath(`history-panel-${theme}.png`) })
    await panel.getByRole('button', { name: 'Close', exact: true }).click()
    await page.screenshot({ path: info.outputPath(`test-preview-${theme}.png`) })
  }
  await trigger.click()
  await panel
    .getByRole('button', { name: `Delete conversation: ${history.items[1]!.title}`, exact: true })
    .click()
  await panel.getByRole('button', { name: 'Delete', exact: true }).click()
  await expect(panel.locator('.conversation-title')).toHaveCount(23)
  expect(history.deleted.has(history.items[1]!.id)).toBeTruthy()
  if (page.viewportSize()!.width > 600) {
    await page.mouse.click(10, 10)
    await expect(panel).not.toBeVisible()
  } else await page.keyboard.press('Escape')
  expect(server.requests).toHaveLength(0)
  expect(server.commands).toEqual([])
  await expectNoMedia(page)
})

test('late history selection, navigation and owner replacement cannot reveal stale transcripts', async ({
  page,
}) => {
  const server = await fixture(page)
  const history = await populatedHistory(page, server)
  await page.goto('/?character=syntelle')
  const first = history.delay('history-0')
  await selectSaved(page, history.items[0]!.title)
  await expect.poll(() => history.details.includes('history-0')).toBeTruthy()
  await selectSaved(page, history.items[1]!.title)
  await expect(page.getByText('The saved answer for history-1.', { exact: true })).toBeVisible()
  await releaseHistory(page, 'history-0', first)
  await expect(page.locator('.conversation-title[aria-current="true"]')).toHaveText(
    history.items[1]!.title,
  )
  await expect(page.getByText('The saved answer for history-0.', { exact: true })).toHaveCount(0)

  const leaving = history.delay('history-2')
  await selectSaved(page, history.items[2]!.title)
  await expect.poll(() => history.details.includes('history-2')).toBeTruthy()
  await visit(page, 'Characters')
  await releaseHistory(page, 'history-2', leaving)
  await expect(page).toHaveURL(/\/characters$/)
  await expect(page.getByRole('heading', { name: 'Characters', exact: true })).toBeVisible()
  await visit(page, 'Test character')
  await expect(page.getByText('The saved answer for history-2.', { exact: true })).toHaveCount(0)

  const oldOwner = history.delay('history-0')
  await selectSaved(page, history.items[0]!.title)
  await expect.poll(() => history.details.filter((id) => id === 'history-0').length).toBe(2)
  await visit(page, 'Characters')
  server.setOwner('owner-b')
  await visit(page, 'Test character')
  await openNavigation(page)
  await expect(page.getByRole('button', { name: 'Your account: Sam', exact: true })).toBeVisible()
  const panel = await openHistory(page)
  await expect(panel.locator('.conversation-title')).toHaveCount(1)
  await expect(panel.locator('.conversation-title')).toHaveText('Sam’s private conversation')
  await releaseHistory(page, 'history-0', oldOwner)
  await expect(panel.locator('.conversation-title')).toHaveCount(1)
  await expect(page.getByText(/The saved answer for history-/)).toHaveCount(0)
  expect(server.requests).toHaveLength(0)
  await expectNoMedia(page)
})

test('unavailable history preserves the live conversation for explicit resumption', async ({
  page,
}) => {
  const server = await fixture(page)
  const history = await populatedHistory(page, server)
  await page.route('**/api/v1/conversations/history-0', (route) =>
    route.fulfill({ status: 404, json: { error: 'Conversation not found.' } }),
  )
  await page.goto('/?character=syntelle')
  await page.getByRole('button', { name: 'Start conversation', exact: true }).click()
  await expect(page.locator('[data-testid="stream-workspace"]')).toHaveAttribute(
    'data-stream-id',
    'navigation-stream',
  )
  server.publish({
    type: 'conversation',
    data: {
      type: 'accepted',
      turn_id: 'still-current',
      text: 'This is the ongoing conversation.',
    },
  })
  await expect(page.getByText('This is the ongoing conversation.', { exact: true })).toBeVisible()
  await selectSaved(page, history.items[0]!.title)
  await expect(
    page.getByText(
      'This conversation is unavailable. Choose another conversation or start a new one.',
      { exact: true },
    ),
  ).toBeVisible()
  expect(server.requests).toHaveLength(1)
  expect(server.deleted).toEqual([])
  expect(server.commands.filter((command) => command === 'suspend')).toHaveLength(1)
  await page
    .getByRole('dialog', { name: 'Conversation history', exact: true })
    .getByRole('button', { name: 'Close', exact: true })
    .click()
  await expect(page.getByText('This is the ongoing conversation.', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Continue conversation', exact: true }).click()
  await expect.poll(() => server.requests.length).toBe(2)
  expect(server.requests[1]).toMatchObject({ resume_from: 'navigation-stream' })
  expect(server.requests[1]).not.toHaveProperty('continue_conversation_id')
})
