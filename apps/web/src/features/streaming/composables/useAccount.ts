import { shallowRef, watch } from 'vue'
import { request } from '../api'
import type {
  Account,
  AuthStatus,
  IntegrationKey,
  SavedConfiguration,
  StreamSettings,
} from '../types'

export function useAccount() {
  const account = shallowRef<Account | null>(null)
  const status = shallowRef<AuthStatus | null>(null)
  const loading = shallowRef(false)
  const error = shallowRef('')
  const configurations = shallowRef<SavedConfiguration[]>([])
  const keys = shallowRef<IntegrationKey[]>([])
  const secret = shallowRef('')
  let requestEpoch = 0
  let ownerEpoch = 0
  let operationEpoch = 0

  watch(
    account,
    (value, previous) => {
      if (value?.id === previous?.id) return
      // A globally shared account must never publish another owner's late data.
      ownerEpoch++
      operationEpoch++
      requestEpoch++
      configurations.value = []
      keys.value = []
      secret.value = ''
      error.value = ''
      loading.value = false
    },
    { flush: 'sync' },
  )

  async function run<T>(work: () => Promise<T>): Promise<T | undefined> {
    if (loading.value) return
    const owner = ownerEpoch
    const operation = ++operationEpoch
    loading.value = true
    error.value = ''
    try {
      const result = await work()
      return owner === ownerEpoch ? result : undefined
    } catch (failure) {
      if (owner === ownerEpoch && operation === operationEpoch)
        error.value = failure instanceof Error ? failure.message : 'The account request failed.'
    } finally {
      if (operation === operationEpoch) loading.value = false
    }
  }

  async function refresh() {
    const epoch = ++requestEpoch
    try {
      const [auth, session] = await Promise.all([
        request<AuthStatus>('/api/v1/auth/status'),
        request<{ account: Account | null }>('/api/v1/auth/me'),
      ])
      if (epoch !== requestEpoch) return
      status.value = auth
      account.value = session.account
    } catch (failure) {
      if (epoch === requestEpoch)
        error.value = failure instanceof Error ? failure.message : 'Account access is unavailable.'
    }
  }

  async function loadLibrary() {
    if (!account.value) return
    const owner = ownerEpoch
    const result = await run(() =>
      Promise.all([
        request<{ items: SavedConfiguration[] }>('/api/v1/configurations'),
        request<{ items: IntegrationKey[] }>('/api/v1/keys'),
      ]),
    )
    if (result && owner === ownerEpoch) {
      const [saved, access] = result
      configurations.value = saved.items
      keys.value = access.items
    }
  }

  function sendCode(email: string, flow?: string) {
    return run(() =>
      request<{
        challenge_id: string
        expires_in: number
        resend_after: number
        purpose?: string
        provider?: string
        requires_confirmation?: boolean
      }>('/api/v1/auth/email/start', 'POST', { email, ...(flow ? { flow_id: flow } : {}) }),
    )
  }

  async function verify(challenge_id: string, code: string, confirmLink = false) {
    if (loading.value) return false
    const owner = ownerEpoch
    requestEpoch++ // A pre-verification account read cannot undo sign-in.
    const result = await run(() =>
      request<{ account: Account }>('/api/v1/auth/email/verify', 'POST', {
        challenge_id,
        code,
        confirm_link: confirmLink,
      }),
    )
    if (result && owner === ownerEpoch) {
      account.value = result.account
      const verifiedOwner = ownerEpoch
      await loadLibrary()
      return verifiedOwner === ownerEpoch
    }
    return false
  }

  async function save(name: string, settings: StreamSettings) {
    const owner = ownerEpoch
    const result = await run(() =>
      request<SavedConfiguration>('/api/v1/configurations', 'POST', { name, settings }),
    )
    if (result && owner === ownerEpoch) await loadLibrary()
    return Boolean(result) && owner === ownerEpoch
  }

  async function remove(id: string) {
    const owner = ownerEpoch
    const result = await run(async () => {
      await request(`/api/v1/configurations/${encodeURIComponent(id)}`, 'DELETE')
      return true
    })
    if (result && owner === ownerEpoch)
      configurations.value = configurations.value.filter((item) => item.id !== id)
  }

  async function update(id: string, name: string, settings: StreamSettings) {
    const owner = ownerEpoch
    const result = await run(() =>
      request<SavedConfiguration>(`/api/v1/configurations/${encodeURIComponent(id)}`, 'PATCH', {
        name,
        settings,
      }),
    )
    if (result && owner === ownerEpoch) await loadLibrary()
  }

  async function createKey(name: string) {
    const owner = ownerEpoch
    secret.value = ''
    const result = await run(() =>
      request<IntegrationKey & { secret: string }>('/api/v1/keys', 'POST', { name }),
    )
    if (result && owner === ownerEpoch) {
      secret.value = result.secret
      keys.value = [
        ...keys.value,
        { id: result.id, name: result.name, created_at: result.created_at },
      ]
    }
  }

  async function revokeKey(id: string) {
    const owner = ownerEpoch
    const result = await run(async () => {
      await request(`/api/v1/keys/${encodeURIComponent(id)}`, 'DELETE')
      return true
    })
    if (result && owner === ownerEpoch) keys.value = keys.value.filter((item) => item.id !== id)
  }

  async function logout() {
    // Sign-out supersedes an in-flight library/read without letting its late
    // completion reset loading or restore private resources.
    const owner = ++ownerEpoch
    operationEpoch++
    requestEpoch++
    loading.value = false
    secret.value = ''
    await run(async () => {
      await request('/api/v1/auth/logout', 'POST', {})
      if (owner !== ownerEpoch) return
      account.value = null
      configurations.value = []
      keys.value = []
      secret.value = ''
      await refresh()
    })
  }

  function clearError() {
    error.value = ''
  }
  function forgetSecret() {
    secret.value = ''
  }
  function reportError(message: string) {
    error.value = message
  }

  return {
    account,
    status,
    loading,
    error,
    configurations,
    keys,
    secret,
    refresh,
    loadLibrary,
    sendCode,
    verify,
    save,
    update,
    remove,
    createKey,
    revokeKey,
    logout,
    clearError,
    forgetSecret,
    reportError,
  }
}
