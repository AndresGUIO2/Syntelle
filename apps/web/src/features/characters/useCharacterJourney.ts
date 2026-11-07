import { shallowReactive } from 'vue'
import type { PersonalCharacter } from './api'

export interface CharacterConnection {
  characterId: string
  publicationId?: string
  workspaceId?: string
}

export interface CharacterJourney {
  name: string
  image: File | null
  preview: string
  width: number
  height: number
  step: 'portrait' | 'review'
  checking: boolean
  error: string
  submitting: boolean
  submitError: string
  receipt: PersonalCharacter | null
  pending?: { name: string; image: File; key: string }
  valid: boolean
}

const journeys = new Map<string, CharacterJourney>()

/** In-memory only: keep an unfinished upload through navigation, never across accounts. */
export function useCharacterJourney(ownerId: string): CharacterJourney {
  let journey = journeys.get(ownerId)
  if (!journey) {
    journey = shallowReactive<CharacterJourney>({
      name: '',
      image: null,
      preview: '',
      width: 0,
      height: 0,
      step: 'portrait',
      checking: false,
      error: '',
      submitting: false,
      submitError: '',
      receipt: null,
      valid: true,
    })
    journeys.set(ownerId, journey)
  }
  return journey
}

export function clearCharacterDraft(journey: CharacterJourney) {
  if (journey.preview) URL.revokeObjectURL(journey.preview)
  journey.name = ''
  journey.image = null
  journey.preview = ''
  journey.width = 0
  journey.height = 0
  journey.step = 'portrait'
  journey.error = ''
  journey.submitError = ''
  journey.pending = undefined
}

/** Clear all accounts on sign-out/identity replacement, or one account explicitly. */
export function resetCharacterJourney(ownerId?: string) {
  for (const [id, journey] of journeys) {
    if (ownerId !== undefined && ownerId !== id) continue
    journey.valid = false
    clearCharacterDraft(journey)
    journey.receipt = null
    journeys.delete(id)
  }
}

export async function inspectPortrait(file: File): Promise<{ width: number; height: number }> {
  if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type))
    throw new Error('Choose a JPEG, PNG, or WebP portrait.')
  if (file.size > 5_000_000) throw new Error('Choose a portrait no larger than 5 MB.')
  const url = URL.createObjectURL(file)
  try {
    const image = new Image()
    image.src = url
    await image.decode()
    const width = image.naturalWidth,
      height = image.naturalHeight
    if (!width || !height)
      throw new Error('The portrait could not be opened. Choose another image.')
    if (width * height > 4_194_304 || width > 4096 || height > 4096)
      throw new Error('Use a portrait up to 4 megapixels, with neither side above 4096 pixels.')
    return { width, height }
  } catch (failure) {
    if (failure instanceof Error && failure.message.startsWith('Use a portrait')) throw failure
    throw new Error('The portrait could not be opened. Choose another image.', { cause: failure })
  } finally {
    URL.revokeObjectURL(url)
  }
}
