import assert from 'node:assert/strict'
import { test, type TestContext } from 'node:test'
import { effectScope, shallowRef } from 'vue'
import { createServer } from 'vite'
import { defaultSettings } from '../../src/features/streaming/types.ts'
import type {
  HistoryItem,
  useConversationHistory as UseHistory,
  groupConversations as Group,
} from '../../src/features/history/useConversationHistory.ts'

function requests(t: TestContext) {
  const pending: { path: string; method: string; resolve: (response: Response) => void }[] = []
  t.mock.method(
    globalThis,
    'fetch',
    (path: string, options?: RequestInit) =>
      new Promise<Response>((resolve) => {
        pending.push({ path, method: options?.method ?? 'GET', resolve })
      }),
  )
  function take(path: string, method = 'GET') {
    const index = pending.findIndex((entry) => entry.path === path && entry.method === method)
    assert.notEqual(index, -1, `${method} ${path} was requested`)
    const entry = pending.splice(index, 1)[0]!
    return (body: unknown, status = 200) =>
      entry.resolve(new Response(JSON.stringify(body), { status }))
  }
  const list = (items: HistoryItem[], next = '', guests: HistoryItem[] = []) => {
    take('/api/v1/conversations')({ items, next })
    take('/api/v1/conversations/claimable')({ items: guests })
  }
  return { take, list, pending }
}
const settle = () => new Promise<void>((resolve) => setImmediate(resolve))
const item = (id: string, updated_at = '2026-11-05T12:00:00Z'): HistoryItem => ({
  id,
  title: id,
  language: 'en-US',
  updated_at,
  settings: defaultSettings(),
})
const turn = (id: string, sequence: number) => ({
  id,
  sequence,
  status: 'ready',
  user: id,
  reply: `Reply ${id}`,
  spoken: '',
})

test('sidebar history remains owner-bound and selection-safe across asynchronous operations', async (t) => {
  const server = await createServer({
    configFile: false,
    server: { middlewareMode: true },
    appType: 'custom',
  })
  try {
    const { useConversationHistory, groupConversations } = (await server.ssrLoadModule(
      '/src/features/history/useConversationHistory.ts',
    )) as { useConversationHistory: typeof UseHistory; groupConversations: typeof Group }
    function state(t: TestContext) {
      const owner = shallowRef<string | undefined>('owner-a')
      const scope = effectScope()
      t.after(() => scope.stop())
      const history = scope.run(() => useConversationHistory(() => owner.value))!
      return { owner, history }
    }
    await t.test(
      'late list and preview cannot restore another owner or clear current loading',
      async (t) => {
        const fixture = requests(t)
        const { owner, history } = state(t)
        const oldList = fixture.take('/api/v1/conversations')
        const oldGuests = fixture.take('/api/v1/conversations/claimable')
        const selected = history.select(item('private-a'))
        const oldPreview = fixture.take('/api/v1/conversations/private-a')
        owner.value = 'owner-b'
        oldList({ items: [item('private-a')] })
        oldGuests({ items: [item('guest-a')] })
        oldPreview({ conversation: item('private-a'), turns: [turn('a', 1)], before: 1 })
        assert.equal(await selected, undefined)
        await settle()
        assert.equal(history.loading.value, true)
        assert.equal(history.items.value.length, 0)
        assert.equal(history.claimable.value.length, 0)
        assert.equal(history.selectedId.value, '')
        fixture.list([item('private-b')])
        await settle()
        assert.deepEqual(
          history.items.value.map((entry) => entry.id),
          ['private-b'],
        )
        owner.value = undefined
        assert.deepEqual(history.items.value, [])
        assert.equal(history.loading.value, false)
      },
    )
    await t.test(
      'only the latest selected preview and older-message response may commit',
      async (t) => {
        const fixture = requests(t)
        const { history } = state(t)
        fixture.list([item('a'), item('b')])
        await settle()
        const a = history.select(item('a')),
          responseA = fixture.take('/api/v1/conversations/a')
        const b = history.select(item('b')),
          responseB = fixture.take('/api/v1/conversations/b')
        responseB({ conversation: item('b'), turns: [turn('b2', 2)], before: 2 })
        assert.equal((await b)?.item.id, 'b')
        responseA({ conversation: item('a'), turns: [turn('a1', 1)], before: 1 })
        assert.equal(await a, undefined)
        assert.equal(history.selectedId.value, 'b')
        const older = history.loadOlder(),
          responseOlder = fixture.take('/api/v1/conversations/b?before=2')
        history.clearSelection()
        responseOlder({ turns: [turn('b1', 1)], before: 0 })
        assert.equal(await older, undefined)
        assert.equal(history.selectedId.value, '')
        assert.equal(history.olderBefore.value, 0)
        assert.equal(history.loadingOlder.value, false)
      },
    )
    await t.test(
      'pagination deduplicates, deletion cannot resurrect, and failed removal retains the row',
      async (t) => {
        const fixture = requests(t)
        const { history } = state(t)
        fixture.list([item('a'), item('b')], 'cursor-1')
        await settle()
        const page = history.loadMore(),
          responsePage = fixture.take('/api/v1/conversations?before=cursor-1')
        const removal = history.remove(item('a'))
        fixture.take('/api/v1/conversations/a', 'DELETE')({})
        assert.equal(await removal, true)
        responsePage({ items: [item('a'), item('b'), item('c')], next: '' })
        await page
        assert.deepEqual(
          history.items.value.map((entry) => entry.id),
          ['b', 'c'],
        )
        const rejected = history.remove(item('b'))
        fixture.take('/api/v1/conversations/b', 'DELETE')({ error: 'Active conversation' }, 409)
        assert.equal(await rejected, false)
        assert.deepEqual(
          history.items.value.map((entry) => entry.id),
          ['b', 'c'],
        )
        assert.equal(history.error.value, 'This conversation could not be deleted.')
        assert.equal(history.hasMore.value, false)
      },
    )
    await t.test(
      'missing saved settings reports an actionable error without replacing the current selection',
      async (t) => {
        const fixture = requests(t)
        const { history } = state(t)
        fixture.list([item('current'), item('unavailable')])
        await settle()
        history.setSelection('current')
        const selection = history.select(item('unavailable'))
        fixture.take('/api/v1/conversations/unavailable')({
          conversation: { ...item('unavailable'), settings: undefined },
          turns: [],
        })
        assert.equal(await selection, undefined)
        assert.equal(history.selectedId.value, 'current')
        assert.equal(history.previewingId.value, '')
        assert.equal(
          history.error.value,
          'This conversation has no saved character settings and cannot be opened.',
        )
      },
    )
    await t.test('guest claim is serialized and refreshes only the current owner', async (t) => {
      const fixture = requests(t)
      const { owner, history } = state(t)
      fixture.list([], '', [item('guest')])
      await settle()
      const claim = history.claim(item('guest'))
      const response = fixture.take('/api/v1/conversations/guest/claim', 'POST')
      await history.claim(item('guest'))
      assert.equal(fixture.pending.length, 0)
      owner.value = 'owner-b'
      response({ saved: true })
      await claim
      assert.equal(history.loading.value, true)
      fixture.list([item('b')])
      await settle()
      assert.deepEqual(
        history.items.value.map((entry) => entry.id),
        ['b'],
      )
      assert.deepEqual(history.claimable.value, [])
    })
    await t.test('date groups use local calendar days in newest-first order', () => {
      const now = new Date(2026, 10, 5, 14)
      const dated = (id: string, day: number) => item(id, new Date(2026, 10, day, 12).toISOString())
      const groups = groupConversations(
        [dated('old', 1), dated('today', 5), dated('yesterday', 4), dated('older', -5)],
        now,
      )
      assert.deepEqual(
        groups.map((group) => group.label),
        ['Today', 'Yesterday', 'Previous 7 days', 'Older'],
      )
    })
  } finally {
    await server.close()
  }
})
