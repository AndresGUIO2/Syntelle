import { computed, nextTick, onScopeDispose, shallowRef } from 'vue'

export type ConsoleSection = 'overview' | 'integrations' | 'usage' | 'billing' | 'team' | 'settings'
export type NavigationSection =
  ConsoleSection | 'chat' | 'characters' | 'workspaces' | 'admin' | 'pricing'

const destinations = [
  '/',
  '/pricing',
  '/characters',
  '/characters/new',
  '/console',
  '/console/integrations',
  '/console/usage',
  '/console/billing',
  '/console/team',
  '/console/settings',
  '/?auth',
  '/?auth&return=characters',
  '/?view=workspaces',
  '/?view=admin',
] as const
export type Destination = (typeof destinations)[number] | `/?character=char_${string}`
function isDestination(value: string | null): value is Destination {
  return (
    destinations.some((destination) => destination === value) ||
    /^\/\?character=char_[a-f0-9]{32}$/.test(value ?? '')
  )
}
export function usePageNavigation() {
  const path = shallowRef(location.pathname)
  const query = shallowRef(location.search)
  const selectedCharacter = shallowRef(
    new URLSearchParams(location.search).get('character') ?? 'syntelle',
  )
  const conversationHref = computed<Destination>(() =>
    selectedCharacter.value === 'syntelle'
      ? '/'
      : (`/?character=${selectedCharacter.value}` as Destination),
  )
  const pricing = computed(() => path.value.replace(/\/+$/, '') === '/pricing')
  const characterCreation = computed(() => path.value.replace(/\/+$/, '') === '/characters/new')
  const characters = computed(
    () => characterCreation.value || path.value.replace(/\/+$/, '') === '/characters',
  )
  const workspaces = computed(
    () => path.value === '/' && new URLSearchParams(query.value).get('view') === 'workspaces',
  )
  const admin = computed(
    () => path.value === '/' && new URLSearchParams(query.value).get('view') === 'admin',
  )
  const chat = computed(() => path.value === '/' && !workspaces.value && !admin.value)
  const authQuery = computed(() => new URLSearchParams(query.value))
  const consolePage = computed(
    () => path.value === '/console' || path.value.startsWith('/console/'),
  )
  const consoleSection = computed<ConsoleSection>(() => {
    const section = path.value.split('/')[2]
    return ['integrations', 'usage', 'billing', 'team', 'settings'].includes(section ?? '')
      ? (section as ConsoleSection)
      : 'overview'
  })
  const active = computed<NavigationSection>(() =>
    characters.value
      ? 'characters'
      : pricing.value
        ? 'pricing'
        : workspaces.value
          ? 'team'
          : admin.value
            ? 'admin'
            : consolePage.value
              ? consoleSection.value
              : 'chat',
  )
  async function changed() {
    path.value = location.pathname
    query.value = location.search
    if (
      chat.value &&
      !authQuery.value.has('auth') &&
      !authQuery.value.has('flow_id') &&
      !authQuery.value.has('auth_error')
    )
      selectedCharacter.value = authQuery.value.get('character') ?? 'syntelle'
    await nextTick()
    const main = document.getElementById('main-content')
    if (main) {
      if (!chat.value) main.scrollTop = 0
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
    characters,
    characterCreation,
    workspaces,
    admin,
    consolePage,
    consoleSection,
    chat,
    active,
    authQuery,
    selectedCharacter,
    conversationHref,
    visit,
    follow,
  }
}
