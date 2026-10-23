import { computed, onMounted, onScopeDispose, shallowRef } from 'vue'

// Images are external resources, not reactive trees. Late decodes may settle
// after unmount; they must never publish into a disposed component scope.
export function usePreparedImages<T>(
  prepare: (load: (url: string) => Promise<HTMLImageElement>) => Promise<T>,
  message: string | ((failure: unknown) => string),
) {
  const state = shallowRef<T | null>(null)
  const error = shallowRef('')
  const pending = new Set<HTMLImageElement>()
  let disposed = false

  async function load(url: string) {
    if (disposed) throw new Error('Image preparation was disposed.')
    const image = new Image()
    pending.add(image)
    image.src = url
    try {
      await image.decode()
      return image
    } finally {
      pending.delete(image)
    }
  }

  onMounted(async () => {
    try {
      const result = await prepare(load)
      if (!disposed) state.value = result
    } catch (failure) {
      if (!disposed) error.value = typeof message === 'function' ? message(failure) : message
    }
  })
  onScopeDispose(() => {
    disposed = true
    for (const image of pending) image.src = ''
    pending.clear()
    state.value = null
  })
  return { images: computed(() => state.value), error: computed(() => error.value) }
}
