import { onScopeDispose, shallowRef } from 'vue'
import { request } from '../streaming/api'

/** A route instance is keyed by owner + workspace. Never commit after it is removed. */
export function useConsoleResource<T>(path: string) {
  const data = shallowRef<T | null>(null)
  const loading = shallowRef(false)
  const error = shallowRef('')
  let epoch = 0
  let disposed = false
  async function refresh() {
    const revision = ++epoch
    loading.value = true
    error.value = ''
    try {
      const result = await request<T>(path)
      if (!disposed && revision === epoch) data.value = result
    } catch (failure) {
      if (!disposed && revision === epoch)
        error.value =
          failure instanceof Error
            ? failure.message
            : 'This request could not be completed. Please try again.'
    } finally {
      if (!disposed && revision === epoch) loading.value = false
    }
  }
  onScopeDispose(() => {
    disposed = true
    epoch++
  })
  return { data, loading, error, refresh }
}
