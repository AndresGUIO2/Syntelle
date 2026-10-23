export interface NativeViseme {
  id: number
  offset_ms: number
}
export interface SpeechResponse {
  region?: string
  performance?: import('../types/performanceTypes').ResolvedPerformance
  audio_base64: string
  mime: string
  duration_ms: number
  visemes: NativeViseme[]
  voice: string
  language: string
  sdk_version: string
  cached: boolean
}
export interface MouthFrame {
  from: string
  to: string
  amount: number
}
// L/D/T/N have no dedicated texture in the nine-pose package; EI is an explicit approximation.
const map = [
  'Rest',
  'A',
  'A',
  'O',
  'EI',
  'EI',
  'EI',
  'U',
  'O',
  'A',
  'O',
  'A',
  'Rest',
  'CHSHR',
  'EI',
  'SZ',
  'CHSHR',
  'CHSHR',
  'FV',
  'EI',
  'Rest',
  'MBP',
]
export const AZURE_MAPPING_VERSION = 'azure-nine-pose/1'
export const VISEME_PACING_VERSION = 'bounded-articulation/2'
export function decodeSpeechWav(bytes: Uint8Array) {
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength)
  const tag = (at: number) => String.fromCharCode(...bytes.subarray(at, at + 4))
  if (bytes.length < 44 || tag(0) !== 'RIFF' || tag(8) !== 'WAVE') throw Error('Invalid speech WAV')
  let rate = 0,
    start = 0,
    length = 0
  for (let at = 12; at + 8 <= bytes.length;) {
    const size = view.getUint32(at + 4, true),
      end = at + 8 + size
    if (end > bytes.length) throw Error('Truncated speech WAV')
    if (tag(at) === 'fmt ') {
      if (
        size < 16 ||
        view.getUint16(at + 8, true) !== 1 ||
        view.getUint16(at + 10, true) !== 1 ||
        view.getUint16(at + 22, true) !== 16
      )
        throw Error('Expected mono PCM16 speech')
      rate = view.getUint32(at + 12, true)
    }
    if (tag(at) === 'data') {
      start = at + 8
      length = size
    }
    at = end + (size % 2)
  }
  if (!rate || !length || length % 2) throw Error('Missing speech PCM data')
  const samples = new Float32Array(length / 2)
  for (let i = 0; i < samples.length; i++) samples[i] = view.getInt16(start + i * 2, true) / 32768
  return { samples, sampleRate: rate, duration: samples.length / rate }
}
export function speechTimeline(events: NativeViseme[], duration: number) {
  let previous = -1
  const timeline: { time: number; pose: string }[] = []
  for (const event of events) {
    if (
      !Number.isInteger(event.id) ||
      !map[event.id] ||
      !Number.isFinite(event.offset_ms) ||
      event.offset_ms < previous ||
      event.offset_ms > duration + 100
    )
      throw Error('Invalid native Azure viseme timeline')
    previous = event.offset_ms
    const pose = map[event.id]!
    if (timeline.at(-1)?.pose !== pose) timeline.push({ time: event.offset_ms / 1000, pose })
  }
  if (!timeline.some((event) => event.pose !== 'Rest'))
    throw Error('Dragon HD returned no spoken visemes')
  // Keep contact gestures, but do not chase every brief intermediate shape.
  // Retained event times stay on the original audio clock (no accumulated delay).
  const end = duration / 1000
  const paced = timeline.filter((event, index) => {
    const span = Math.min(end, timeline[index + 1]?.time ?? end) - event.time
    const contact = event.pose === 'MBP' || event.pose === 'FV'
    return index === 0 || span + 1e-9 >= (contact ? 0.035 : 0.06)
  })
  const merged = paced.filter(
    (event, index) => index === 0 || paced[index - 1]!.pose !== event.pose,
  )
  merged.push({ time: end, pose: 'Rest' })
  return merged
}
export function mouthAt(timeline: ReturnType<typeof speechTimeline>, seconds: number): MouthFrame {
  // A small visual lead centers the gesture nearer its audio event instead of
  // starting a long transition only after the sound has already arrived.
  const articulationTime = seconds + 0.02
  let index = -1
  for (let i = 0; i < timeline.length && timeline[i]!.time <= articulationTime; i++) index = i
  if (index < 0) return { from: 'Rest', to: 'Rest', amount: 0 }
  const current = timeline[index]!,
    previous = timeline[index - 1]?.pose ?? 'Rest'
  const next = timeline[index + 1]?.time ?? current.time + 0.2
  const duration = Math.max(
    0.001,
    Math.min(current.pose === 'MBP' || current.pose === 'FV' ? 0.045 : 0.08, next - current.time),
  )
  const t = Math.max(0, Math.min(1, (articulationTime - current.time) / duration))
  return { from: previous, to: current.pose, amount: t * t * (3 - 2 * t) }
}
