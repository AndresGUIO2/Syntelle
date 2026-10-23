import { defineStore } from 'pinia'
import { onScopeDispose, shallowRef } from 'vue'

type Theme = 'light' | 'dark'

export const useThemeStore = defineStore('theme', () => {
  const media = matchMedia('(prefers-color-scheme: dark)')
  const theme = shallowRef<Theme>(
    document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light',
  )
  let manual = false
  try {
    manual = ['light', 'dark'].includes(localStorage.getItem('syntelle-theme') ?? '')
  } catch {
    /* Storage is optional. */
  }

  function apply(next: Theme) {
    const root = document.documentElement
    root.classList.add('theme-changing')
    theme.value = next
    root.dataset.theme = next
    void root.offsetHeight
    requestAnimationFrame(() => root.classList.remove('theme-changing'))
  }

  function toggle() {
    manual = true
    apply(theme.value === 'light' ? 'dark' : 'light')
    try {
      localStorage.setItem('syntelle-theme', theme.value)
    } catch {
      /* Continue for this session. */
    }
  }

  function followSystem(event: MediaQueryListEvent) {
    if (!manual) apply(event.matches ? 'dark' : 'light')
  }
  media.addEventListener('change', followSystem)
  onScopeDispose(() => media.removeEventListener('change', followSystem))
  return { theme, toggle }
})
