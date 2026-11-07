import { computed, nextTick, onScopeDispose, shallowRef } from 'vue'

const destinations = [
  '/',
  '/pricing',
  '/characters/new',
  '/?auth',
  '/?auth&return=characters',
  '/?view=workspaces',
  '/?view=admin',
] as const
type Destination = (typeof destinations)[number] | `/?character=char_${string}`
function isDestination(value: string | null): value is Destination {
  return (
    destinations.some((destination) => destination === value) ||
    /^\/\?character=char_[a-f0-9]{32}$/.test(value ?? '')
  )
}

export function usePageNavigation() {
  const path = shallowRef(location.pathname)
  const query = shallowRef(location.search)
  const selectedCharacter = computed(
    () => new URLSearchParams(query.value).get('character') ?? 'syntelle',
  )
  const pricing = computed(() => path.value.replace(/\/+$/, '') === '/pricing')
  const characterCreation = computed(() => path.value.replace(/\/+$/, '') === '/characters/new')
  const workspaces = computed(
    () => path.value === '/' && new URLSearchParams(query.value).get('view') === 'workspaces',
  )
  const admin = computed(
    () => path.value === '/' && new URLSearchParams(query.value).get('view') === 'admin',
  )
  const returningToCharacters = computed(() => {
    const params = new URLSearchParams(query.value)
    return path.value === '/' && params.has('auth') && params.get('return') === 'characters'
  })
  const chat = computed(
    () =>
      path.value === '/' &&
      !workspaces.value &&
      !admin.value &&
      !new URLSearchParams(query.value).has('auth'),
  )

  async function changed() {
    path.value = location.pathname
    query.value = location.search
    await nextTick()
    const main = document.getElementById('main-content')
    if (main) {
      main.scrollTop = 0
      main.focus({ preventScroll: true })
    }
  }
  function visit(destination: Destination, replace = false) {
    if (replace) history.replaceState(null, '', destination)
    else history.pushState(null, '', destination)
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
    if (!isDestination(href) || anchor.target === '_blank' || anchor.hasAttribute('download'))
      return
    event.preventDefault()
    if (location.pathname + location.search !== href) visit(href)
  }
  window.addEventListener('popstate', changed)
  onScopeDispose(() => window.removeEventListener('popstate', changed))
  return {
    pricing,
    characterCreation,
    workspaces,
    admin,
    chat,
    returningToCharacters,
    selectedCharacter,
    visit,
    follow,
  }
}
