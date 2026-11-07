import { computed, onScopeDispose, shallowRef, watch } from 'vue'
import { request, RequestError } from '../streaming/api'
import type { Workspace } from '../workspaces/useWorkspaces'

/** Shell-owned selection. Responses and storage are scoped to the signed-in owner. */
export function useConsoleWorkspace(owner: () => string | undefined) {
  const items = shallowRef<Workspace[]>([])
  const selectedId = shallowRef('')
  const loading = shallowRef(false)
  const error = shallowRef('')
  const creating = shallowRef(false)
  const uncertainCreation = shallowRef(false)
  let epoch = 0
  let disposed = false
  const selected = computed(() => items.value.find((item) => item.id === selectedId.value) ?? null)
  function select(id: string) {
    if (!items.value.some((item) => item.id === id)) return
    selectedId.value = id
    uncertainCreation.value = false
    try {
      sessionStorage.setItem(`syntelle.console.workspace.${owner()}`, id)
    } catch {
      /* Optional preference. */
    }
  }
  async function refresh() {
    const account = owner()
    const revision = ++epoch
    if (!account) {
      items.value = []
      selectedId.value = ''
      error.value = ''
      loading.value = false
      return
    }
    loading.value = true
    error.value = ''
    try {
      const result = await request<{ items: Workspace[] }>('/api/v1/workspaces')
      if (disposed || revision !== epoch || account !== owner()) return
      items.value = result.items
      let preferred = selectedId.value
      try {
        preferred ||= sessionStorage.getItem(`syntelle.console.workspace.${account}`) ?? ''
      } catch {
        /* Optional preference. */
      }
      selectedId.value =
        result.items.find((item) => item.id === preferred)?.id ?? result.items[0]?.id ?? ''
    } catch (failure) {
      if (!disposed && revision === epoch)
        error.value = failure instanceof Error ? failure.message : 'Workspaces could not be loaded.'
    } finally {
      if (!disposed && revision === epoch) loading.value = false
    }
  }
  async function create(name: string) {
    if (creating.value || !owner()) return false
    if (uncertainCreation.value) {
      await refresh()
      error.value =
        'Workspace creation could not be confirmed. Check the workspace selector before creating another.'
      return false
    }
    const account = owner()
    creating.value = true
    error.value = ''
    try {
      const result = await request<Workspace>('/api/v1/workspaces', 'POST', { name })
      if (disposed || account !== owner()) return false
      // Keep a confirmed creation even if a later list request is unavailable.
      // Invalidate older reads so they cannot erase the newly created workspace.
      epoch++
      loading.value = false
      items.value = [...items.value.filter((item) => item.id !== result.id), result]
      select(result.id)
      return true
    } catch (failure) {
      if (!disposed && account === owner()) {
        if (!(failure instanceof RequestError) || failure.status >= 500) {
          uncertainCreation.value = true
          await refresh()
          if (disposed || account !== owner()) return false
          error.value =
            'Workspace creation could not be confirmed. Check the workspace selector before creating another.'
          return false
        }
        error.value = failure instanceof Error ? failure.message : 'Workspace could not be created.'
      }
      return false
    } finally {
      if (!disposed && account === owner()) creating.value = false
    }
  }
  watch(
    owner,
    () => {
      items.value = []
      selectedId.value = ''
      creating.value = false
      uncertainCreation.value = false
      void refresh()
    },
    { immediate: true },
  )
  onScopeDispose(() => {
    disposed = true
    epoch++
  })
  return {
    items,
    selectedId,
    selected,
    loading,
    error,
    creating,
    uncertainCreation,
    select,
    refresh,
    create,
  }
}
