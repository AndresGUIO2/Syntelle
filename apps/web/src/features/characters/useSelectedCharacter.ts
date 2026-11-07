import { computed, onScopeDispose, shallowRef, watch } from 'vue'
import { characterPortrait, getCharacter, isCharacterID, type PersonalCharacter } from './api'
import type { StreamSettings } from '../streaming/types'

export function useSelectedCharacter(character: () => StreamSettings['character']) {
  const item = shallowRef<PersonalCharacter | null>(null)
  const loading = shallowRef(character() !== 'syntelle')
  const error = shallowRef('')
  let generation = 0
  const custom = computed(() => character() !== 'syntelle')
  const ready = computed(
    () => !custom.value || (!loading.value && item.value?.status === 'ready' && !error.value),
  )
  const name = computed(() => (custom.value ? (item.value?.name ?? 'Your character') : 'Syntelle'))
  const poster = computed(() =>
    custom.value
      ? item.value?.portrait_available
        ? characterPortrait(item.value.id)
        : null
      : undefined,
  )
  async function refresh() {
    const id = character()
    const current = ++generation
    item.value = null
    error.value = ''
    loading.value = id !== 'syntelle'
    if (id === 'syntelle') return true
    try {
      if (!isCharacterID(id))
        throw Error('This character is unavailable. Open a ready character from your collection.')
      const result = await getCharacter(id)
      if (current !== generation) return false
      item.value = result
      if (result.status !== 'ready')
        error.value = 'This character is not ready yet. Check its preparation in Characters.'
      return result.status === 'ready'
    } catch {
      if (current === generation)
        error.value =
          'This character is unavailable. Sign in to its account and open it from Characters.'
      return false
    } finally {
      if (current === generation) loading.value = false
    }
  }
  watch(character, () => void refresh(), { immediate: true })
  onScopeDispose(() => {
    generation++
  })
  return { custom, ready, name, poster, loading, error, refresh }
}
