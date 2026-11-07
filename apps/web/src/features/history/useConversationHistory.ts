import { computed, onScopeDispose, shallowReadonly, shallowRef, watch } from 'vue'
import { request } from '../streaming/api'
import type { StreamSettings } from '../streaming/types'
import type { ConversationTurn } from '../conversation/types'

export interface HistoryItem {
  id: string
  title: string
  language: string
  updated_at: string
  expires_at?: string
  settings?: StreamSettings
}
export interface HistoryPreview {
  item: HistoryItem
  turns: ConversationTurn[]
}
type GroupLabel = 'Today' | 'Yesterday' | 'Previous 7 days' | 'Older'

export function groupConversations(items: readonly HistoryItem[], now = new Date()) {
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime()
  const yesterday = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1).getTime()
  const week = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 7).getTime()
  const groups = new Map<GroupLabel, HistoryItem[]>()
  for (const item of [...items].sort(
    (a, b) => Date.parse(b.updated_at) - Date.parse(a.updated_at),
  )) {
    const time = Date.parse(item.updated_at)
    const label: GroupLabel =
      time >= today
        ? 'Today'
        : time >= yesterday
          ? 'Yesterday'
          : time >= week
            ? 'Previous 7 days'
            : 'Older'
    const group = groups.get(label) ?? []
    group.push(item)
    groups.set(label, group)
  }
  return [...groups].map(([label, items]) => ({ label, items }))
}

export function useConversationHistory(owner: () => string | null | undefined) {
  const items = shallowRef<HistoryItem[]>([])
  const claimable = shallowRef<HistoryItem[]>([])
  const selectedId = shallowRef('')
  const loading = shallowRef(false)
  const loadingMore = shallowRef(false)
  const previewingId = shallowRef('')
  const mutatingId = shallowRef('')
  const olderBefore = shallowRef(0)
  const loadingOlder = shallowRef(false)
  let preview: HistoryPreview | undefined
  const error = shallowRef('')
  const next = shallowRef('')
  const groups = computed(() => groupConversations(items.value))
  const hasMore = computed(() => Boolean(next.value))
  let epoch = 0,
    listRevision = 0,
    selectionRevision = 0
  const removed = new Set<string>()
  const current = (binding: number, account: ReturnType<typeof owner>) =>
    binding === epoch && Boolean(account) && account === owner()

  async function refresh() {
    const account = owner(),
      binding = epoch,
      revision = ++listRevision
    if (!account) return
    loading.value = true
    loadingMore.value = false
    error.value = ''
    try {
      const [page, guests] = await Promise.all([
        request<{ items: HistoryItem[]; next?: string }>('/api/v1/conversations'),
        request<{ items: HistoryItem[] }>('/api/v1/conversations/claimable'),
      ])
      if (!current(binding, account) || revision !== listRevision) return
      items.value = page.items.filter((item) => !removed.has(item.id))
      next.value = page.next ?? ''
      claimable.value = guests.items
    } catch {
      if (current(binding, account) && revision === listRevision)
        error.value = 'Conversations could not be loaded.'
    } finally {
      if (current(binding, account) && revision === listRevision) loading.value = false
    }
  }
  async function loadMore() {
    const account = owner(),
      binding = epoch,
      revision = listRevision,
      cursor = next.value
    if (!account || !cursor || loading.value || loadingMore.value) return
    loadingMore.value = true
    error.value = ''
    try {
      const page = await request<{ items: HistoryItem[]; next?: string }>(
        `/api/v1/conversations?before=${encodeURIComponent(cursor)}`,
      )
      if (!current(binding, account) || revision !== listRevision) return
      const known = new Set(items.value.map((item) => item.id))
      items.value = [
        ...items.value,
        ...page.items.filter((item) => !known.has(item.id) && !removed.has(item.id)),
      ]
      next.value = page.next ?? ''
    } catch {
      if (current(binding, account) && revision === listRevision)
        error.value = 'More conversations could not be loaded.'
    } finally {
      if (current(binding, account) && revision === listRevision) loadingMore.value = false
    }
  }
  async function select(item: HistoryItem): Promise<HistoryPreview | undefined> {
    if (!owner() || removed.has(item.id) || mutatingId.value) return
    const account = owner(),
      binding = epoch,
      revision = ++selectionRevision
    loadingOlder.value = false
    previewingId.value = item.id
    error.value = ''
    try {
      const page = await request<{
        conversation: HistoryItem
        turns: ConversationTurn[]
        before?: number
      }>(`/api/v1/conversations/${encodeURIComponent(item.id)}`)
      if (!current(binding, account) || revision !== selectionRevision || removed.has(item.id))
        return
      if (!page.conversation.settings) {
        error.value = 'This conversation has no saved character settings and cannot be opened.'
        return
      }
      selectedId.value = page.conversation.id
      olderBefore.value = page.before ?? 0
      loadingOlder.value = false
      preview = { item: page.conversation, turns: page.turns }
      return preview
    } catch {
      if (current(binding, account) && revision === selectionRevision)
        error.value =
          'This conversation is unavailable. Choose another conversation or start a new one.'
    } finally {
      if (current(binding, account) && revision === selectionRevision) previewingId.value = ''
    }
  }
  async function loadOlder(): Promise<HistoryPreview | undefined> {
    const account = owner(),
      binding = epoch,
      revision = selectionRevision,
      selected = preview,
      before = olderBefore.value
    if (
      !account ||
      !selected ||
      !before ||
      loadingOlder.value ||
      previewingId.value ||
      mutatingId.value
    )
      return
    loadingOlder.value = true
    error.value = ''
    try {
      const page = await request<{ turns: ConversationTurn[]; before?: number }>(
        `/api/v1/conversations/${encodeURIComponent(selected.item.id)}?before=${before}`,
      )
      if (
        !current(binding, account) ||
        revision !== selectionRevision ||
        removed.has(selected.item.id)
      )
        return
      const known = new Set(selected.turns.map((turn) => turn.id))
      preview = {
        item: selected.item,
        turns: [...page.turns.filter((turn) => !known.has(turn.id)), ...selected.turns],
      }
      olderBefore.value = page.before ?? 0
      return preview
    } catch {
      if (current(binding, account) && revision === selectionRevision)
        error.value = 'Older messages could not be loaded.'
    } finally {
      if (current(binding, account) && revision === selectionRevision) loadingOlder.value = false
    }
  }
  async function remove(item: HistoryItem) {
    const account = owner(),
      binding = epoch
    if (!account || mutatingId.value || previewingId.value || loadingOlder.value) return false
    mutatingId.value = item.id
    error.value = ''
    try {
      await request(`/api/v1/conversations/${encodeURIComponent(item.id)}`, 'DELETE')
      if (!current(binding, account)) return false
      removed.add(item.id)
      items.value = items.value.filter((entry) => entry.id !== item.id)
      if (selectedId.value === item.id) clearSelection()
      if (previewingId.value === item.id) {
        selectionRevision++
        previewingId.value = ''
      }
      return true
    } catch {
      if (current(binding, account)) error.value = 'This conversation could not be deleted.'
      return false
    } finally {
      if (current(binding, account)) mutatingId.value = ''
    }
  }
  async function claim(item: HistoryItem) {
    const account = owner(),
      binding = epoch
    if (!account || mutatingId.value || previewingId.value || loadingOlder.value) return
    mutatingId.value = item.id
    error.value = ''
    try {
      await request(`/api/v1/conversations/${encodeURIComponent(item.id)}/claim`, 'POST', {})
      if (current(binding, account)) await refresh()
    } catch {
      if (current(binding, account)) error.value = 'This conversation could not be saved.'
    } finally {
      if (current(binding, account)) mutatingId.value = ''
    }
  }
  function setSelection(id: string | null) {
    if (selectedId.value === (id ?? '') && !previewingId.value) return
    selectionRevision++
    selectedId.value = id ?? ''
    previewingId.value = ''
    loadingOlder.value = false
    if (preview?.item.id !== id) {
      preview = undefined
      olderBefore.value = 0
    }
  }
  function clearSelection() {
    setSelection(null)
  }
  watch(
    owner,
    () => {
      epoch++
      listRevision++
      selectionRevision++
      items.value = []
      claimable.value = []
      selectedId.value = previewingId.value = mutatingId.value = next.value = error.value = ''
      loading.value = loadingMore.value = false
      loadingOlder.value = false
      olderBefore.value = 0
      preview = undefined
      removed.clear()
      if (owner()) void refresh()
    },
    { immediate: true, flush: 'sync' },
  )
  onScopeDispose(() => {
    epoch++
  })
  return {
    items: shallowReadonly(items),
    claimable: shallowReadonly(claimable),
    selectedId: shallowReadonly(selectedId),
    loading: shallowReadonly(loading),
    loadingMore: shallowReadonly(loadingMore),
    previewingId: shallowReadonly(previewingId),
    mutatingId: shallowReadonly(mutatingId),
    error: shallowReadonly(error),
    groups,
    hasMore,
    olderBefore: shallowReadonly(olderBefore),
    loadingOlder: shallowReadonly(loadingOlder),
    refresh,
    loadMore,
    loadOlder,
    select,
    remove,
    claim,
    setSelection,
    clearSelection,
  }
}
