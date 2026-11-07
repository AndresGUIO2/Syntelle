import { RequestError, request } from '../streaming/api'

export type CharacterID = `char_${string}`
export interface PersonalCharacter {
  id: CharacterID
  name: string
  status: 'queued' | 'processing' | 'ready' | 'failed'
  failure_code?: string
  portrait_available: boolean
  created_at: string
}
export function isCharacterID(value: unknown): value is CharacterID {
  return typeof value === 'string' && /^char_[a-f0-9]{32}$/.test(value)
}
export function characterPortrait(id: CharacterID): string {
  return `/api/v1/characters/${encodeURIComponent(id)}/portrait`
}
export function getCharacter(id: CharacterID) {
  return request<PersonalCharacter>(`/api/v1/characters/${encodeURIComponent(id)}`)
}
export async function createCharacter(name: string, image: File, requestKey: string) {
  const body = new FormData()
  body.set('name', name.trim())
  body.set('image', image)
  const response = await fetch('/api/v1/characters', {
    method: 'POST',
    credentials: 'same-origin',
    body,
    headers: {
      Accept: 'application/json',
      'X-Syntelle-Request': '1',
      'Idempotency-Key': requestKey,
    },
  })
  const data = await response.json().catch(() => null)
  if (!response.ok)
    throw new RequestError(
      data?.error?.message ?? data?.message ?? 'The portrait could not be uploaded. Try again.',
      response.status,
    )
  return data as PersonalCharacter
}
export function characterFailure(code?: string) {
  switch (code) {
    case 'generation_not_enabled':
      return 'New portrait preparation is not enabled yet. Your image was received, but no character was generated.'
    case 'generation_budget_unavailable':
      return 'Character preparation is temporarily at capacity. Your image has not been generated.'
    case 'provider_failed':
      return 'The image service could not finish preparation. No character was generated. Please try again later.'
    case 'models_unavailable':
    case 'native_checker_unavailable':
      return 'Character preparation is temporarily unavailable. Your portrait is not the cause. Please try again later.'
    case 'portrait_face_unsuitable':
      return 'Choose a clear, front-facing portrait with one face and an unobstructed mouth.'
    case 'portrait_aspect_unsupported':
      return 'Use a standard portrait, landscape or square image with the full head and shoulders visible.'
    case 'portrait_resolution_unsupported':
      return 'Reduce the image size and upload it again: up to 4 megapixels, with neither side above 4096 pixels.'
    case 'source_expired':
      return 'Preparation did not finish before the upload expired. Upload your portrait again.'
    case 'cancelled':
      return 'Preparation was cancelled.'
    default:
      return 'This portrait could not be prepared safely. Try a different portrait.'
  }
}
