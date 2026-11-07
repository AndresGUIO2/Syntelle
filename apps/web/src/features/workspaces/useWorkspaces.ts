import { computed, shallowRef } from 'vue'
import { request } from '../streaming/api'
import type { SavedConfiguration } from '../streaming/types'

export type WorkspaceRole = 'owner' | 'administrator' | 'editor' | 'billing'
export interface Workspace {
  id: string
  name: string
  role: WorkspaceRole
  suspended: boolean
}
export interface Member {
  account_id: string
  email: string
  name: string
  role: WorkspaceRole
}
export interface WorkspaceKey {
  id: string
  name: string
  scope: string
  status: string
  revoked_at: string | null
}
export interface Activity {
  actor_email: string
  action: string
  target_id: string
  reason: string
  created_at: string
}
export type Section =
  'overview' | 'characters' | 'integrations' | 'usage' | 'team' | 'billing' | 'activity'

export function useWorkspaces() {
  const items = shallowRef<Workspace[]>([])
  const selectedId = shallowRef('')
  const section = shallowRef<Section>('overview')
  const members = shallowRef<Member[]>([])
  const configurations = shallowRef<SavedConfiguration[]>([])
  const personal = shallowRef<SavedConfiguration[]>([])
  const keys = shallowRef<WorkspaceKey[]>([])
  const activity = shallowRef<Activity[]>([])
  const error = shallowRef('')
  const loading = shallowRef(false)
  const secret = shallowRef('')
  const invitation = shallowRef('')
  const transfer = shallowRef('')
  const commercial = shallowRef<{
    status: string
    included_minutes: number | null
    prepaid_balance: number | null
    concurrency: number | null
  } | null>(null)
  const selected = computed(() => items.value.find((item) => item.id === selectedId.value) ?? null)
  const manage = computed(
    () => selected.value?.role === 'owner' || selected.value?.role === 'administrator',
  )
  const operate = computed(() => manage.value || selected.value?.role === 'editor')
  let epoch = 0

  function sectionAllowed(role: WorkspaceRole, target: Section) {
    if (target === 'overview' || target === 'usage' || target === 'billing') return true
    if (target === 'characters') return role !== 'billing'
    return role === 'owner' || role === 'administrator'
  }
  function clearDetails() {
    members.value = []
    configurations.value = []
    personal.value = []
    keys.value = []
    activity.value = []
    commercial.value = null
    secret.value = ''
    invitation.value = ''
    transfer.value = ''
  }

  async function run(work: () => Promise<void>) {
    error.value = ''
    loading.value = true
    try {
      await work()
      return true
    } catch (cause) {
      error.value = cause instanceof Error ? cause.message : 'The request failed.'
      return false
    } finally {
      loading.value = false
    }
  }
  async function refresh() {
    await run(async () => {
      const result = await request<{ items: Workspace[] }>('/api/v1/workspaces')
      const previous = selected.value
      items.value = result.items
      if (!result.items.some((item) => item.id === selectedId.value))
        selectedId.value = result.items[0]?.id ?? ''
      if (previous?.id !== selectedId.value || previous?.role !== selected.value?.role)
        clearDetails()
      if (selected.value && !sectionAllowed(selected.value.role, section.value))
        section.value = 'overview'
      if (selectedId.value) await loadSection()
    })
  }
  async function select(id: string) {
    if (!items.value.some((item) => item.id === id)) return
    selectedId.value = id
    clearDetails()
    if (selected.value && !sectionAllowed(selected.value.role, section.value))
      section.value = 'overview'
    await loadSection()
  }
  async function loadSection() {
    const id = selectedId.value
    if (!id) return
    const current = ++epoch
    const base = `/api/v1/workspaces/${encodeURIComponent(id)}`
    try {
      const overview = await request<{ commercial: typeof commercial.value }>(base)
      if (current !== epoch) return
      commercial.value = overview.commercial
      if (manage.value) {
        const results = await Promise.all([
          request<{ items: Member[] }>(base + '/members'),
          request<{ items: Activity[] }>(base + '/activity'),
          request<{ items: WorkspaceKey[] }>(base + '/keys'),
        ])
        if (current !== epoch) return
        members.value = results[0].items
        activity.value = results[1].items
        keys.value = results[2].items
      }
      if (operate.value) {
        const [shared, own] = await Promise.all([
          request<{ items: SavedConfiguration[] }>(base + '/configurations'),
          request<{ items: SavedConfiguration[] }>('/api/v1/configurations'),
        ])
        if (current !== epoch) return
        configurations.value = shared.items
        personal.value = own.items
      }
    } catch (cause) {
      if (current === epoch)
        error.value = cause instanceof Error ? cause.message : 'Workspace could not be loaded.'
    }
  }
  async function create(name: string) {
    await run(async () => {
      const result = await request<Workspace>('/api/v1/workspaces', 'POST', { name })
      await refresh()
      await select(result.id)
    })
  }
  async function invite(email: string, role: WorkspaceRole) {
    await run(async () => {
      const result = await request<{ token: string }>(
        `/api/v1/workspaces/${selectedId.value}/invitations`,
        'POST',
        { email, role },
      )
      invitation.value = `${location.origin}/?view=workspaces&invite=${encodeURIComponent(result.token)}`
      await loadSection()
    })
  }
  async function acceptInvitation(token: string) {
    return await run(async () => {
      await request('/api/v1/workspaces/invitations/accept', 'POST', { token })
      await refresh()
    })
  }
  async function memberChange(accountId: string, role?: WorkspaceRole) {
    await run(async () => {
      await request(
        `/api/v1/workspaces/${selectedId.value}/members/${encodeURIComponent(accountId)}`,
        role ? 'PATCH' : 'DELETE',
        role ? { role } : undefined,
      )
      await loadSection()
    })
  }
  async function copyConfiguration(id: string) {
    await run(async () => {
      await request(`/api/v1/workspaces/${selectedId.value}/configurations/copy`, 'POST', {
        configuration_id: id,
      })
      await loadSection()
    })
  }
  async function renameConfiguration(item: SavedConfiguration, name: string) {
    await run(async () => {
      await request(
        `/api/v1/workspaces/${selectedId.value}/configurations/${encodeURIComponent(item.id)}`,
        'PATCH',
        { name, settings: item.settings },
      )
      await loadSection()
    })
  }
  async function removeConfiguration(id: string) {
    await run(async () => {
      await request(
        `/api/v1/workspaces/${selectedId.value}/configurations/${encodeURIComponent(id)}`,
        'DELETE',
      )
      await loadSection()
    })
  }
  async function createKey(name: string) {
    await run(async () => {
      const key = await request<{ secret: string }>(
        `/api/v1/workspaces/${selectedId.value}/keys`,
        'POST',
        { name },
      )
      secret.value = key.secret
      await loadSection()
    })
  }
  async function revokeKey(id: string) {
    await run(async () => {
      await request(
        `/api/v1/workspaces/${selectedId.value}/keys/${encodeURIComponent(id)}`,
        'DELETE',
      )
      secret.value = ''
      await loadSection()
    })
  }
  async function startOwnershipEmail() {
    let id = ''
    await run(async () => {
      const result = await request<{ challenge_id: string }>(
        `/api/v1/workspaces/${selectedId.value}/ownership-email/start`,
        'POST',
      )
      id = result.challenge_id
    })
    return id
  }
  async function transferOwnership(accountId: string, challengeId: string, code: string) {
    await run(async () => {
      const base = `/api/v1/workspaces/${selectedId.value}`
      const proof = await request<{ proof_id: string }>(base + '/ownership-email/verify', 'POST', {
        challenge_id: challengeId,
        code,
      })
      const result = await request<{ token: string }>(base + '/ownership-transfer', 'POST', {
        account_id: accountId,
        proof_id: proof.proof_id,
      })
      transfer.value = `${location.origin}/?view=workspaces&transfer=${encodeURIComponent(result.token)}`
    })
  }
  async function acceptTransfer(token: string) {
    return await run(async () => {
      await request('/api/v1/workspaces/ownership/accept', 'POST', { token })
      await refresh()
    })
  }
  return {
    items,
    selectedId,
    selected,
    section,
    members,
    configurations,
    personal,
    keys,
    activity,
    error,
    loading,
    secret,
    invitation,
    transfer,
    commercial,
    manage,
    operate,
    refresh,
    select,
    create,
    invite,
    acceptInvitation,
    memberChange,
    copyConfiguration,
    renameConfiguration,
    removeConfiguration,
    createKey,
    revokeKey,
    startOwnershipEmail,
    transferOwnership,
    acceptTransfer,
  }
}
