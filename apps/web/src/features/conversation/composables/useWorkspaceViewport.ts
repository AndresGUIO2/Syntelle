import { computed, onMounted, onScopeDispose, shallowRef, type Ref } from 'vue'

// Safari's keyboard resizes the visual viewport without resizing the page layout.
export function useWorkspaceViewport(host: Ref<HTMLElement | null>) {
  const height = shallowRef<number | null>(null)
  const keyboardOpen = shallowRef(false)
  let restingHeight = 0
  let width = 0
  let frame = 0
  function measure() {
    const viewport = window.visualViewport
    const visibleHeight = viewport?.height ?? window.innerHeight
    const editable = document.activeElement?.matches(
      'textarea, input:not([type="checkbox"]):not([type="range"])',
    )
    if (width !== window.innerWidth || !editable) restingHeight = visibleHeight
    width = window.innerWidth
    keyboardOpen.value = Boolean(editable && restingHeight - visibleHeight > 100)
    const top = Math.max(
      0,
      (host.value?.getBoundingClientRect().top ?? 0) - (viewport?.offsetTop ?? 0),
    )
    height.value = Math.floor(visibleHeight - top - 12)
  }
  function schedule() {
    cancelAnimationFrame(frame)
    frame = requestAnimationFrame(measure)
  }
  onMounted(() => {
    measure()
    window.addEventListener('resize', schedule)
    window.visualViewport?.addEventListener('resize', schedule)
    window.visualViewport?.addEventListener('scroll', schedule)
    document.addEventListener('focusin', schedule)
    document.addEventListener('focusout', schedule)
  })
  onScopeDispose(() => {
    cancelAnimationFrame(frame)
    window.removeEventListener('resize', schedule)
    window.visualViewport?.removeEventListener('resize', schedule)
    window.visualViewport?.removeEventListener('scroll', schedule)
    document.removeEventListener('focusin', schedule)
    document.removeEventListener('focusout', schedule)
  })
  return {
    viewportStyle: computed(() =>
      height.value === null ? {} : { '--demo-viewport-height': `${height.value}px` },
    ),
    keyboardOpen,
  }
}
