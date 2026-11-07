import { onScopeDispose, shallowRef, watch } from 'vue'

export interface ConnectionSelection {
  characterId?: string
  publicationId?: string
  workspaceId?: string
}
export interface ConnectionIntent extends ConnectionSelection {
  accountId: string
  destination: '/console/integrations'
}
const storageKey = 'syntelle.console.connection-intent'
const opaqueId = /^[A-Za-z0-9_-]{1,128}$/
export function validateConnectionSelection(value: ConnectionSelection): boolean {
  const validId = (id: unknown) => id === undefined || (typeof id === 'string' && opaqueId.test(id))
  return (
    (value.characterId === undefined ||
      (typeof value.characterId === 'string' &&
        (value.characterId === 'syntelle' || /^char_[a-f0-9]{32}$/.test(value.characterId)))) &&
    validId(value.publicationId) &&
    validId(value.workspaceId) &&
    (!value.publicationId || Boolean(value.workspaceId))
  )
}

/** Only route intent and opaque IDs survive reload; files and credentials never do. */
export function useConnectionIntent(owner: () => string | undefined) {
  const intent = shallowRef<ConnectionIntent | null>(null)
  const error = shallowRef('')
  function clear() {
    intent.value = null
    error.value = ''
    try {
      sessionStorage.removeItem(storageKey)
    } catch {
      /* Optional restoration. */
    }
  }
  function unavailable(message: string) {
    clear()
    error.value = message
  }
  function begin(selection: ConnectionSelection = {}) {
    const accountId = owner()
    if (!accountId || !validateConnectionSelection(selection)) return false
    error.value = ''
    intent.value = {
      characterId: selection.characterId,
      publicationId: selection.publicationId,
      workspaceId: selection.workspaceId,
      accountId,
      destination: '/console/integrations',
    }
    try {
      sessionStorage.setItem(storageKey, JSON.stringify(intent.value))
    } catch {
      /* In-memory works. */
    }
    return true
  }
  watch(
    owner,
    (accountId, previous) => {
      if (previous && previous !== accountId) clear()
      if (!accountId) {
        intent.value = null
        return
      }
      try {
        const saved = JSON.parse(
          sessionStorage.getItem(storageKey) ?? 'null',
        ) as ConnectionIntent | null
        if (
          saved?.accountId === accountId &&
          saved.destination === '/console/integrations' &&
          validateConnectionSelection(saved)
        ) {
          // Reconstruct the allowlist instead of forwarding arbitrary stored fields.
          intent.value = {
            accountId,
            destination: '/console/integrations',
            characterId: saved.characterId,
            publicationId: saved.publicationId,
            workspaceId: saved.workspaceId,
          }
        } else clear()
      } catch {
        clear()
      }
    },
    { immediate: true },
  )
  onScopeDispose(() => {
    intent.value = null
  })
  return { intent, error, begin, clear, unavailable }
}
