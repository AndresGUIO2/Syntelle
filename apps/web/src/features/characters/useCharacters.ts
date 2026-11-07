import { computed, onMounted, onScopeDispose, shallowRef } from 'vue'
import { request } from '../streaming/api'
import { createCharacter, type PersonalCharacter } from './api'

export function useCharacters() {
  const items = shallowRef<PersonalCharacter[]>([])
  const loading = shallowRef(true)
  const submitting = shallowRef(false)
  const error = shallowRef('')
  const active = computed(() =>
    items.value.some((item) => item.status === 'queued' || item.status === 'processing'),
  )
  let disposed = false
  let timer: ReturnType<typeof setTimeout> | undefined
  let refreshGeneration = 0
  // Keep the key after an uncertain network result so resubmission cannot duplicate work.
  let lastSubmission: { name: string; image: File; key: string } | undefined
  async function refresh() {
    clearTimeout(timer)
    const generation = ++refreshGeneration
    try {
      const result = await request<{ items: PersonalCharacter[] }>('/api/v1/characters')
      if (disposed || generation !== refreshGeneration) return
      items.value = result.items
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
    if (submitting.value || active.value) return false
    submitting.value = true
    error.value = ''
    if (!lastSubmission || lastSubmission.name !== name || lastSubmission.image !== image)
      lastSubmission = { name, image, key: crypto.randomUUID() }
    try {
      const item = await createCharacter(name, image, lastSubmission.key)
      if (disposed) return true
      items.value = [item, ...items.value.filter((existing) => existing.id !== item.id)]
      lastSubmission = undefined
      await refresh()
      return true
    } catch (failure) {
      if (!disposed)
        error.value =
          failure instanceof Error
            ? failure.message
            : 'The portrait could not be uploaded. Try again.'
      return false
    } finally {
      if (!disposed) submitting.value = false
    }
  }
  async function remove(id: string) {
    try {
      await request(`/api/v1/characters/${encodeURIComponent(id)}`, 'DELETE')
      await refresh()
    } catch (failure) {
      error.value =
        failure instanceof Error ? failure.message : 'The character could not be removed.'
    }
  }
  onMounted(() => void refresh())
  onScopeDispose(() => {
    disposed = true
    clearTimeout(timer)
    refreshGeneration++
  })
  return { items, loading, submitting, error, active, refresh, submit, remove }
}
