import assert from 'node:assert/strict'
import { test } from 'node:test'
import { createServer } from 'vite'
import { acceptsParentMessage } from '../../src/features/widget/types.ts'
import { widgetFrameAncestors } from '../../config/widget-headers.ts'
import type { createWidgetTransport as Transport } from '../../src/features/widget/transport.ts'

test('widget handshake binds the source, exact parent origin and nonce', () => {
  const parent = {} as Window
  const event = {
    source: parent,
    origin: 'https://customer.example',
    data: { channel: 'syntelle-widget-v1', nonce: 'bound' },
  }
  assert.equal(acceptsParentMessage(event, parent, ['https://customer.example'], 'bound'), true)
  for (const changed of [
    { ...event, origin: 'https://customer.example.evil.test' },
    { ...event, source: {} as Window },
    { ...event, data: { ...event.data, nonce: 'previous' } },
    { ...event, data: { ...event.data, channel: 'other' } },
  ])
    assert.equal(
      acceptsParentMessage(changed, parent, ['https://customer.example'], 'bound'),
      false,
    )
})

test('framing policy rejects wildcards, paths, credentials and header injection', () => {
  assert.equal(
    widgetFrameAncestors(['https://site.example', 'https://site.example', 'http://localhost:5180']),
    'https://site.example http://localhost:5180',
  )
  assert.equal(
    widgetFrameAncestors([
      'https://site.example/route',
      '*',
      'https://site.example; script-src *',
      'http://public.example',
      'https://user:pass@site.example',
    ]),
    "'none'",
  )
})

test('widget transport never sends cookies and isolates capabilities across late cleanup', async (t) => {
  const server = await createServer({
    configFile: false,
    server: { middlewareMode: true },
    appType: 'custom',
  })
  try {
    const { createWidgetTransport } = (await server.ssrLoadModule(
      '/src/features/widget/transport.ts',
    )) as { createWidgetTransport: typeof Transport }
    const calls: { path: string; options: RequestInit }[] = []
    t.mock.method(globalThis, 'fetch', async (path: string, options: RequestInit) => {
      calls.push({ path, options })
      return new Response('{}', { status: 200 })
    })
    let next = 0
    const transport = createWidgetTransport(async () => ({
      id: `session-${++next}`,
      capability: `capability-${next}`,
      ticket: 'once',
      state: 'idle',
      configured: true,
      recognition: false,
    }))
    await transport.createSession({ resumeFrom: '', continueConversationID: '' })
    await transport.createSession({ resumeFrom: '', continueConversationID: '' })
    await transport.request('/api/v1/streams/session-1', 'DELETE')
    await transport.request('/api/v1/streams/session-2/ticket', 'POST')
    assert.equal(calls[0]!.options.credentials, 'omit')
    assert.equal(new Headers(calls[0]!.options.headers).get('Authorization'), 'Bearer capability-1')
    assert.equal(new Headers(calls[1]!.options.headers).get('Authorization'), 'Bearer capability-2')
    await assert.rejects(transport.request('/api/v1/streams/session-1/ticket', 'POST'), /has ended/)
    await assert.rejects(transport.request('/api/v1/workspaces/private'), /has ended/)
    assert.equal(calls.length, 2)
  } finally {
    await server.close()
  }
})
