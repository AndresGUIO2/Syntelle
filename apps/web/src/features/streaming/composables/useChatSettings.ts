import { onScopeDispose, shallowRef, watch } from 'vue'
import { useLocaleStore } from '../../../stores/locale'
import { DEFAULT_VOICE, defaultSettings, type StreamSettings } from '../types'
import { isCharacterID } from '../../characters/api'

// The shell owns one current setup, so account actions and a cached chat read
// the same normalized settings without creating a hidden rendering session.
export function useChatSettings() {
  const locale = useLocaleStore()
  const reducedMotion = matchMedia('(prefers-reduced-motion: reduce)')
  function sampleSettings(): StreamSettings {
    let selected = defaultSettings()
    try {
      const previous = JSON.parse(localStorage.getItem('syntelle.stream.settings') ?? 'null')
      if (previous?.character === 'syntelle')
        selected = { ...selected, ...previous, voice: DEFAULT_VOICE }
      else {
        const appearance = JSON.parse(localStorage.getItem('syntelle.demo.appearance') ?? 'null')
        const background = JSON.parse(localStorage.getItem('syntelle.motion.background') ?? 'null')
        selected = {
          ...selected,
          blinking: appearance?.blinking !== false,
          movement: appearance?.movement !== false,
          automatic_expressions: appearance?.automaticExpressions !== false,
          ...(['light-studio', 'dark-lounge', 'natural-space'].includes(background?.id)
            ? {
                background: background.id,
                blur: Math.max(0, Math.min(20, Number(background.blur) || 0)),
              }
            : {}),
          instructions:
            localStorage.getItem('syntelle.instructions.syntelle') ?? selected.instructions,
        }
      }
    } catch {
      /* Optional local preferences. */
    }
    return {
      ...selected,
      voice: DEFAULT_VOICE,
      language: locale.speechLanguage,
      reduced_motion: reducedMotion.matches,
    }
  }
  const settings = shallowRef<StreamSettings>(sampleSettings())
  function selectCharacter(character: string) {
    if (character !== 'syntelle' && !isCharacterID(character)) return
    if (settings.value.character === character) return
    settings.value =
      character === 'syntelle'
        ? sampleSettings()
        : {
            ...defaultSettings(),
            character,
            language: locale.speechLanguage,
            voice: DEFAULT_VOICE,
            reduced_motion: reducedMotion.matches,
            emotion_regulation: { enabled: false, temperament: 'calm' },
          }
  }
  function preferenceChanged() {
    settings.value = { ...settings.value, reduced_motion: reducedMotion.matches }
  }
  reducedMotion.addEventListener('change', preferenceChanged)
  onScopeDispose(() => reducedMotion.removeEventListener('change', preferenceChanged))
  watch(
    () => locale.speechLanguage,
    (language) => {
      settings.value = { ...settings.value, language }
    },
    { flush: 'sync' },
  )
  watch(settings, (value) => {
    // Personal character settings must not replace the sample's saved setup.
    if (value.character !== 'syntelle') return
    try {
      localStorage.setItem('syntelle.stream.settings', JSON.stringify(value))
    } catch {
      /* Optional. */
    }
  })
  return { settings, selectCharacter }
}
