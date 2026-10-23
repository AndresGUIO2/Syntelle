import { computed, nextTick, onScopeDispose, shallowRef } from 'vue'

type Destination = '/' | '/pricing' | '/characters/new' | '/?auth'

export function usePageNavigation() {
  const path = shallowRef(location.pathname)
  const pricing = computed(() => path.value.replace(/\/+$/, '') === '/pricing')
  const characterCreation = computed(
    () => path.value.replace(/\/+$/, '') === '/characters/new',
  )

  async function changed() {
    path.value = location.pathname
    await nextTick()
    const main = document.getElementById('main-content')
    if (main) {
      main.scrollTop = 0
      main.focus({ preventScroll: true })
    }
  }
  function visit(destination: Destination) {
    history.pushState(null, '', destination)
    void changed()
  }
  function follow(event: MouseEvent) {
    if (
      event.defaultPrevented ||
      event.button !== 0 ||
      event.metaKey ||
      event.ctrlKey ||
      event.shiftKey ||
      event.altKey
    )
      return
    const anchor = event.currentTarget as HTMLAnchorElement
    const href = anchor.getAttribute('href')
    if (
      (href !== '/' && href !== '/pricing' && href !== '/characters/new') ||
      anchor.target === '_blank' ||
      anchor.hasAttribute('download')
    )
      return
    event.preventDefault()
    if (location.pathname !== href || location.search) visit(href)
  }
  window.addEventListener('popstate', changed)
  onScopeDispose(() => window.removeEventListener('popstate', changed))
  return { pricing, characterCreation, visit, follow }
}
