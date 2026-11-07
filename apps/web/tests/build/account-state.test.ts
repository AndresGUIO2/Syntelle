import assert from 'node:assert/strict'
import { test, type TestContext } from 'node:test'
import { createServer } from 'vite'
import type { useAccount as UseAccount } from '../../src/features/streaming/composables/useAccount.ts'

function requests(t: TestContext) {
  const pending: {
    path: string
    method: string
    resolve: (response: Response) => void
  }[] = []
  t.mock.method(
    globalThis,
    'fetch',
    (path: string, options?: RequestInit) =>
      new Promise<Response>((resolve) => {
        pending.push({ path, method: options?.method ?? 'GET', resolve })
      }),
  )
  function take(path: string, method = 'GET') {
    const index = pending.findIndex((item) => item.path === path && item.method === method)
    assert.notEqual(index, -1, `${method} ${path} was requested`)
    const request = pending.splice(index, 1)[0]!
    return (body: unknown, status = 200) =>
      request.resolve(new Response(JSON.stringify(body), { status }))
  }
  function session(owner: string | null) {
    take('/api/v1/auth/status')({
      email: { configured: true },
      google: { configured: false },
      github: { configured: false },
    })
    take('/api/v1/auth/me')({
      account: owner ? { id: owner, name: owner, email: `${owner}@example.invalid` } : null,
    })
  }
  return { take, session }
}

test('shell account resources remain bound to their owner across asynchronous work', async (t) => {
  const server = await createServer({
    configFile: false,
    server: { middlewareMode: true },
    appType: 'custom',
  })
  try {
    const { useAccount } = (await server.ssrLoadModule(
      '/src/features/streaming/composables/useAccount.ts',
    )) as { useAccount: typeof UseAccount }

    await t.test(
      'late library results cannot restore a previous owner or clear new loading',
      async (t) => {
        const fixture = requests(t)
        const state = useAccount()
        const initial = state.refresh()
        fixture.session('owner-a')
        await initial
        const oldLibrary = state.loadLibrary()
        const oldConfigurations = fixture.take('/api/v1/configurations')
        const oldKeys = fixture.take('/api/v1/keys')
        state.configurations.value = [{ id: 'old' }] as typeof state.configurations.value
        state.keys.value = [{ id: 'old' }] as typeof state.keys.value
        state.secret.value = 'old secret'

        const switched = state.refresh()
        fixture.session('owner-b')
        await switched
        assert.deepEqual(state.configurations.value, [])
        assert.deepEqual(state.keys.value, [])
        assert.equal(state.secret.value, '')
        assert.equal(state.loading.value, false)

        const newLibrary = state.loadLibrary()
        oldConfigurations({ items: [{ id: 'a-private' }] })
        oldKeys({ items: [{ id: 'a-key' }] })
        await oldLibrary
        assert.equal(state.loading.value, true)
        assert.deepEqual(state.configurations.value, [])
        assert.deepEqual(state.keys.value, [])
        fixture.take('/api/v1/configurations')({ items: [{ id: 'b-private' }] })
        fixture.take('/api/v1/keys')({ items: [{ id: 'b-key' }] })
        await newLibrary
        assert.deepEqual(state.configurations.value, [{ id: 'b-private' }])
        assert.deepEqual(state.keys.value, [{ id: 'b-key' }])
        assert.equal(state.loading.value, false)
      },
    )

    await t.test('a late integration secret is not published after an owner change', async (t) => {
      const fixture = requests(t)
      const state = useAccount()
      const initial = state.refresh()
      fixture.session('owner-a')
      await initial
      const creation = state.createKey('A integration')
      const oldKey = fixture.take('/api/v1/keys', 'POST')
      const switched = state.refresh()
      fixture.session('owner-b')
      await switched
      oldKey({ id: 'a-key', name: 'A integration', secret: 'a-private-secret', created_at: 'now' })
      await creation
      assert.equal(state.secret.value, '')
      assert.deepEqual(state.keys.value, [])
      assert.equal(state.account.value?.id, 'owner-b')
    })

    await t.test(
      'a stale failure cannot replace the current owner error or loading state',
      async (t) => {
        const fixture = requests(t)
        const state = useAccount()
        const initial = state.refresh()
        fixture.session('owner-a')
        await initial
        const oldRemoval = state.remove('a-private')
        const failure = fixture.take('/api/v1/configurations/a-private', 'DELETE')
        const switched = state.refresh()
        fixture.session('owner-b')
        await switched
        const current = state.loadLibrary()
        failure({ error: 'Previous owner failure' }, 403)
        await oldRemoval
        assert.equal(state.error.value, '')
        assert.equal(state.loading.value, true)
        fixture.take('/api/v1/configurations')({ items: [] })
        fixture.take('/api/v1/keys')({ items: [] })
        await current
      },
    )

    await t.test(
      'sign-out invalidates an in-flight account refresh and library read',
      async (t) => {
        const fixture = requests(t)
        const state = useAccount()
        const initial = state.refresh()
        fixture.session('owner-a')
        await initial
        const oldLibrary = state.loadLibrary()
        const oldConfigurations = fixture.take('/api/v1/configurations')
        const oldKeys = fixture.take('/api/v1/keys')
        const oldRefresh = state.refresh()
        const oldStatus = fixture.take('/api/v1/auth/status')
        const oldSession = fixture.take('/api/v1/auth/me')

        const logout = state.logout()
        fixture.take('/api/v1/auth/logout', 'POST')({})
        // The logout response is parsed before its fresh session read begins.
        await new Promise<void>((resolve) => setImmediate(resolve))
        fixture.session(null)
        await logout
        oldStatus({ email: { configured: true } })
        oldSession({ account: { id: 'owner-a', email: 'owner-a@example.invalid' } })
        oldConfigurations({ items: [{ id: 'a-private' }] })
        oldKeys({ items: [{ id: 'a-key' }] })
        await Promise.all([oldRefresh, oldLibrary])
        assert.equal(state.account.value, null)
        assert.deepEqual(state.configurations.value, [])
        assert.deepEqual(state.keys.value, [])
        assert.equal(state.secret.value, '')
        assert.equal(state.loading.value, false)
      },
    )
    await t.test(
      'sign-in cannot complete a stale owner transition while loading saved setups',
      async (t) => {
        const fixture = requests(t)
        const state = useAccount()
        const verification = state.verify('fixture-challenge', '123456')
        fixture.take(
          '/api/v1/auth/email/verify',
          'POST',
        )({
          account: { id: 'owner-a', email: 'owner-a@example.invalid' },
        })
        await new Promise<void>((resolve) => setImmediate(resolve))
        const oldConfigurations = fixture.take('/api/v1/configurations')
        const oldKeys = fixture.take('/api/v1/keys')
        const switched = state.refresh()
        fixture.session('owner-b')
        await switched
        oldConfigurations({ items: [{ id: 'a-private' }] })
        oldKeys({ items: [{ id: 'a-key' }] })
        assert.equal(await verification, false)
        assert.equal(state.account.value?.id, 'owner-b')
        assert.deepEqual(state.configurations.value, [])
        assert.deepEqual(state.keys.value, [])
      },
    )
  } finally {
    await server.close()
  }
})
