import { computed, onMounted, onScopeDispose, shallowRef } from 'vue'
import { request } from '../streaming/api'
import { createCharacter, type PersonalCharacter } from './api'
import { clearCharacterDraft, useCharacterJourney } from './useCharacterJourney'

export function useCharacters(ownerId = '') {
  const journey = useCharacterJourney(ownerId)
  const items = shallowRef<PersonalCharacter[]>([])
  const loading = shallowRef(true)
  const submitting = computed(() => journey.submitting)
  const error = shallowRef('')
  const removingId = shallowRef('')
  const active = computed(() =>
    items.value.some((item) => item.status === 'queued' || item.status === 'processing'),
  )
  const resubmittingPending = computed(
    () => journey.pending?.name === journey.name.trim() && journey.pending?.image === journey.image,
  )
  let disposed = false
  let timer: ReturnType<typeof setTimeout> | undefined
  let refreshGeneration = 0
  async function refresh() {
    clearTimeout(timer)
    const generation = ++refreshGeneration
    try {
      const result = await request<{ items: PersonalCharacter[] }>('/api/v1/characters')
      if (disposed || !journey.valid || generation !== refreshGeneration) return
      items.value = result.items
      if (journey.receipt) {
        const updated = result.items.find((item) => item.id === journey.receipt?.id)
        if (updated) journey.receipt = updated
      }
      error.value = ''
    } catch (failure) {
      if (!disposed && generation === refreshGeneration)
        error.value =
          failure instanceof Error ? failure.message : 'Your characters could not be loaded.'
    } finally {
      if (!disposed && generation === refreshGeneration) {
        loading.value = false
        if (active.value) timer = setTimeout(() => void refresh(), 2500)
      }
    }
  }
  async function submit(name: string, image: File) {
    if (submitting.value || (active.value && !resubmittingPending.value) || !journey.valid)
      return null
    journey.submitting = true
    journey.submitError = ''
    error.value = ''
    // The key and exact original File survive route unmounts after uncertain responses.
    if (!journey.pending || journey.pending.name !== name || journey.pending.image !== image)
      journey.pending = { name, image, key: crypto.randomUUID() }
    try {
      const item = await createCharacter(name, image, journey.pending.key)
      if (!journey.valid) return null
      journey.receipt = item
      clearCharacterDraft(journey)
      if (!disposed) {
        items.value = [item, ...items.value.filter((existing) => existing.id !== item.id)]
        await refresh()
      }
      return item
    } catch (failure) {
      if (journey.valid) {
        // Refresh admission state after an uncertain response. A deliberate repeat
        // with the same original File/key retrieves that request, never a new job.
        if (!disposed) await refresh()
        journey.submitError =
          failure instanceof Error
            ? failure.message
            : 'The portrait could not be uploaded. Try again.'
        if (!disposed) error.value = journey.submitError
      }
      return null
    } finally {
      if (journey.valid) journey.submitting = false
    }
  }
  async function remove(id: string) {
    if (removingId.value || !journey.valid) return
    removingId.value = id
    try {
      await request(`/api/v1/characters/${encodeURIComponent(id)}`, 'DELETE')
      if (disposed || !journey.valid) return
      if (journey.receipt?.id === id) journey.receipt = null
      await refresh()
    } catch (failure) {
      if (!disposed && journey.valid)
        error.value =
          failure instanceof Error ? failure.message : 'The character could not be removed.'
    } finally {
      if (!disposed) removingId.value = ''
    }
  }
  onMounted(() => void refresh())
  onScopeDispose(() => {
    disposed = true
    clearTimeout(timer)
    refreshGeneration++
  })
  return {
    items,
    loading,
    submitting,
    error,
    active,
    resubmittingPending,
    removingId,
    refresh,
    submit,
    remove,
  }
}
