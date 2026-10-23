import { computed, shallowRef, watch } from 'vue'
import { defineStore } from 'pinia'
import { spanish } from '../locales/es'

export type PlatformLocale = 'en' | 'es'

export const useLocaleStore = defineStore('locale', () => {
  let initial: PlatformLocale = 'en'
  try {
    const saved = localStorage.getItem('syntelle.locale')
    if (saved === 'en' || saved === 'es') initial = saved
    else if (
      JSON.parse(localStorage.getItem('syntelle.stream.settings') ?? 'null')?.language === 'es-CO'
    )
      initial = 'es'
  } catch {
    // Storage is optional; retain the platform's English default.
  }
  const locale = shallowRef<PlatformLocale>(initial)
  const speechLanguage = computed(() => (locale.value === 'es' ? 'es-CO' : 'en-US'))

  function t(message: string, values: Record<string, string | number> = {}): string {
    const normalized = message.replace(/\s+/g, ' ').trim()
    const status = normalized.match(
      /^The local service could not complete this request \((\d+)\)\.$/,
    )
    if (status)
      return t('The local service could not complete this request ({status}).', {
        status: status[1]!,
      })
    const translated = locale.value === 'es' ? (spanish[normalized] ?? normalized) : normalized
    return translated.replace(/\{(\w+)\}/g, (match, key: string) => String(values[key] ?? match))
  }

  function errorMessage(message: string) {
    const normalized = message.replace(/\s+/g, ' ').trim()
    if (
      locale.value === 'en' ||
      spanish[normalized] ||
      normalized.startsWith('The local service could not complete this request (')
    )
      return t(message)
    return t('This request could not be completed. Please try again.')
  }

  watch(
    locale,
    (value) => {
      document.documentElement.lang = value
      document
        .querySelector('meta[name="description"]')
        ?.setAttribute(
          'content',
          t('Create expressive characters with Syntelle, by Colors Studio.'),
        )
      try {
        localStorage.setItem('syntelle.locale', value)
      } catch {
        // Keep the selection for this session when storage is unavailable.
      }
    },
    { immediate: true, flush: 'sync' },
  )

  return { locale, speechLanguage, t, errorMessage }
})
