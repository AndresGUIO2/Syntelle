import { shallowRef } from 'vue'
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

  async function run<T>(work: () => Promise<T>): Promise<T | undefined> {
    if (loading.value) return
    loading.value = true
    error.value = ''
    try {
      return await work()
    } catch (failure) {
      error.value = failure instanceof Error ? failure.message : 'The account request failed.'
    } finally {
      loading.value = false
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
    await run(async () => {
      const [saved, access] = await Promise.all([
        request<{ items: SavedConfiguration[] }>('/api/v1/configurations'),
        request<{ items: IntegrationKey[] }>('/api/v1/keys'),
      ])
      configurations.value = saved.items
      keys.value = access.items
    })
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
    const result = await run(() =>
      request<{ account: Account }>('/api/v1/auth/email/verify', 'POST', {
        challenge_id,
        code,
        confirm_link: confirmLink,
      }),
    )
    if (result) {
      account.value = result.account
      await loadLibrary()
      return true
    }
    return false
  }

  async function save(name: string, settings: StreamSettings) {
    const result = await run(() =>
      request<SavedConfiguration>('/api/v1/configurations', 'POST', { name, settings }),
    )
    if (result) await loadLibrary()
    return Boolean(result)
  }

  async function remove(id: string) {
    const result = await run(async () => {
      await request(`/api/v1/configurations/${encodeURIComponent(id)}`, 'DELETE')
      return true
    })
    if (result) configurations.value = configurations.value.filter((item) => item.id !== id)
  }

  async function update(id: string, name: string, settings: StreamSettings) {
    const result = await run(() =>
      request<SavedConfiguration>(`/api/v1/configurations/${encodeURIComponent(id)}`, 'PATCH', {
        name,
        settings,
      }),
    )
    if (result) await loadLibrary()
  }

  async function createKey(name: string) {
    secret.value = ''
    const result = await run(() =>
      request<IntegrationKey & { secret: string }>('/api/v1/keys', 'POST', { name }),
    )
    if (result) {
      secret.value = result.secret
      keys.value = [
        ...keys.value,
        { id: result.id, name: result.name, created_at: result.created_at },
      ]
    }
  }

  async function revokeKey(id: string) {
    const result = await run(async () => {
      await request(`/api/v1/keys/${encodeURIComponent(id)}`, 'DELETE')
      return true
    })
    if (result) keys.value = keys.value.filter((item) => item.id !== id)
  }

  async function logout() {
    await run(async () => {
      await request('/api/v1/auth/logout', 'POST', {})
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
