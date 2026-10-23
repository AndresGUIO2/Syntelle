import { computed, onMounted, onScopeDispose, readonly, shallowRef } from 'vue'
import { admitPerformance, directedPlan } from '../animation/performance'
import type { LaughterPackage } from '../types/laughterTypes'
import {
  decodeSpeechWav,
  mouthAt,
  speechTimeline,
  type MouthFrame,
  type SpeechResponse,
} from '../speech/speechTimeline'
import { speechBeats, type SpeechBeat } from '../animation/headMotion'
import {
  planSpeechBehavior,
  type BehaviorSettings,
  type SpeechBehaviorPlan,
} from '../animation/speechBehavior'

// The private render host routes the same PCM/viseme clock into a WebRTC track.
// Omitting this adapter preserves browser-speaker playback in historical trials.
export interface SpeechOutput {
  context: AudioContext
  destination: AudioNode
}

export function useSpeechPlayback(
  character: string,
  behavior?: BehaviorSettings,
  expressionLeadMs = 0,
  directed = false,
  laughter?: () => { package: LaughterPackage; assetUrl: (name: string) => string } | undefined,
  output?: SpeechOutput,
  referenceLaughter = false,
) {
  const spokenDuration = shallowRef(0)
  const articulating = computed(() => active.value && position.value < spokenDuration.value)
  const laughSegment = shallowRef<{ start: number; end: number; energy: number[] } | null>(null)
  const laughEnergy = computed(() => {
    const segment = laughSegment.value
    if (!segment || !active.value || position.value < segment.start || position.value > segment.end)
      return 0
    const at = Math.max(0, (position.value - segment.start) / 0.02)
    const lo = Math.min(segment.energy.length - 1, Math.floor(at)),
      hi = Math.min(segment.energy.length - 1, lo + 1)
    const fraction = Math.min(1, at - Math.floor(at))
    return (segment.energy[lo] ?? 0) * (1 - fraction) + (segment.energy[hi] ?? 0) * fraction
  })
  const laughAmount = computed(() => {
    const segment = laughSegment.value
    if (!segment || !active.value) return 0
    const local = position.value - segment.start,
      remaining = segment.end - position.value
    if (local < 0 || remaining < 0) return 0
    if (referenceLaughter) {
      const phase = local / (segment.end - segment.start)
      const ease = (value: number) => {
        const n = Math.max(0, Math.min(1, value))
        return n * n * (3 - 2 * n)
      }
      return ease(phase / 0.15) * (1 - ease((phase - 0.7) / 0.3))
    }
    const envelope = Math.min(1, local / 0.14, remaining / 0.24)
    return Math.max(0, envelope) * (0.65 + 0.35 * (segment.energy[Math.floor(local / 0.02)] ?? 0))
  })
  const configured = shallowRef(false),
    checking = shallowRef(true),
    loading = shallowRef(false)
  const active = shallowRef(false),
    paused = shallowRef(false),
    error = shallowRef('')
  const text = shallowRef(
    character === 'syntelle'
      ? 'Hello. My voice and facial movements share the same clock.'
      : 'Hello. I can speak, pause, and continue with natural movements.',
  )
  const language = shallowRef('en-US')
  const voice = shallowRef(
    character === 'syntelle'
      ? 'en-US-Ava:DragonHDLatestNeural'
      : 'en-US-Andrew:DragonHDLatestNeural',
  )
  const frame = shallowRef<MouthFrame>({ from: 'Rest', to: 'Rest', amount: 0 })
  const position = shallowRef(0),
    beats = shallowRef<SpeechBeat[]>([])
  const behaviorPlan = shallowRef<SpeechBehaviorPlan | null>(null),
    session = shallowRef(0)
  let context: AudioContext | null = output?.context ?? null,
    source: AudioBufferSourceNode | null = null
  let audio: AudioBuffer | null = null,
    timeline: ReturnType<typeof speechTimeline> = []
  let abort: AbortController | null = null,
    generation = 0,
    resumeToken = 0,
    handle = 0,
    offset = 0,
    startedAt = 0
  let mounted = true
  const cache = new Map<string, SpeechResponse>()
  function time() {
    if (!context || paused.value) return offset
    const stamp = output ? undefined : context.getOutputTimestamp?.()
    const clock = (stamp?.contextTime ?? 0) > 0 ? stamp!.contextTime! : context.currentTime
    return Math.max(0, Math.min(audio?.duration ?? 0, offset + clock - startedAt))
  }
  function detach() {
    resumeToken++
    if (source) {
      source.onended = null
      source.stop()
      source.disconnect()
      source = null
    }
    cancelAnimationFrame(handle)
    handle = 0
  }
  function stop() {
    generation++
    session.value = generation
    abort?.abort()
    abort = null
    detach()
    active.value = paused.value = loading.value = false
    offset = 0
    position.value = 0
    beats.value = []
    behaviorPlan.value = null
    laughSegment.value = null
    frame.value = { from: 'Rest', to: 'Rest', amount: 0 }
  }
  function tick() {
    if (!active.value || paused.value) return
    position.value = time()
    frame.value = mouthAt(timeline, position.value)
    handle = requestAnimationFrame(tick)
  }
  async function resume() {
    if (!context || !audio || !active.value || source) return
    const id = generation,
      token = ++resumeToken,
      currentContext = context,
      currentAudio = audio
    try {
      await currentContext.resume()
      if (id !== generation || token !== resumeToken || !mounted || document.hidden) return
      const node = currentContext.createBufferSource()
      source = node
      node.buffer = currentAudio
      node.connect(output?.destination ?? currentContext.destination)
      startedAt = currentContext.currentTime
      paused.value = false
      node.onended = () => {
        if (source !== node) return
        node.disconnect()
        source = null
        cancelAnimationFrame(handle)
        active.value = false
        position.value = currentAudio.duration
        frame.value = { from: 'Rest', to: 'Rest', amount: 0 }
      }
      node.start(0, offset)
      tick()
    } catch (e) {
      if (id !== generation || token !== resumeToken || !mounted) return
      stop()
      error.value = e instanceof Error ? e.message : 'Speech playback could not resume.'
    }
  }
  function pause() {
    if (!active.value || paused.value) return
    offset = time()
    paused.value = true
    detach()
  }
  async function speak() {
    stop()
    error.value = ''
    const id = generation
    // Resume during the click gesture before waiting for network/decode (Safari).
    try {
      loading.value = true
      const preparationStarted = performance.now()
      if (typeof AudioContext === 'undefined')
        throw Error('Audio playback is unavailable in this browser. Web Audio is required.')
      context ??= new AudioContext()
      await context.resume()
      if (id !== generation || !mounted) return
      const key = JSON.stringify([text.value, language.value, voice.value])
      let result = cache.get(key)
      if (!result) {
        abort = new AbortController()
        const response = await fetch('/api/v1/speech/synthesize', {
          method: 'POST',
          signal: abort.signal,
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            text: text.value,
            language: language.value,
            voice: voice.value,
            request_key: crypto.randomUUID(),
          }),
        })
        if (!response.ok) {
          const body = await response.json().catch(() => null)
          throw Error(body?.error?.message ?? 'Speech could not be prepared.')
        }
        result = (await response.json()) as SpeechResponse
      }
      if (id !== generation || !mounted) return
      cache.set(key, result)
      await loadPrepared(result, id, preparationStarted, text.value)
    } catch (e) {
      if (id !== generation || !mounted) return
      stop()
      error.value = e instanceof Error ? e.message : 'Speech playback failed.'
    }
  }
  async function activateAudio() {
    if (typeof AudioContext === 'undefined')
      throw Error('Audio playback is unavailable in this browser. Web Audio is required.')
    context ??= new AudioContext()
    await context.resume()
  }
  async function loadPrepared(
    result: SpeechResponse,
    id: number,
    preparationStarted: number,
    spokenText: string,
    settleExpression = true,
  ) {
    timeline = speechTimeline(result.visemes, result.duration_ms)
    const bytes = Uint8Array.from(atob(result.audio_base64), (c) => c.charCodeAt(0))
    const pcm = decodeSpeechWav(bytes)
    if (context) audio = await context.decodeAudioData(bytes.buffer)
    if (id !== generation || !mounted) return
    if (Math.abs(pcm.duration - result.duration_ms / 1000) > 0.1)
      throw Error('Speech duration and audio disagree.')
    spokenDuration.value = pcm.duration
    const companion = laughter?.()
    const clip = companion?.package.audio.find(
      (c) =>
        c.voice === result.voice && c.language === result.language && c.region === result.region,
    )
    let laughAudio: AudioBuffer | undefined,
      laughPcm: ReturnType<typeof decodeSpeechWav> | undefined
    if (directed && clip && result.performance?.resolved?.some((c) => c.kind === 'laugh')) {
      try {
        const response = await fetch(companion!.assetUrl(clip.file))
        if (!response.ok) throw Error('Laughter audio unavailable')
        const raw = await response.arrayBuffer()
        const hash = [...new Uint8Array(await crypto.subtle.digest('SHA-256', raw))]
          .map((b) => b.toString(16).padStart(2, '0'))
          .join('')
        if (hash !== clip.sha256) throw Error('Laughter audio checksum differs')
        laughPcm = decodeSpeechWav(new Uint8Array(raw))
        if (Math.abs(laughPcm.duration - clip.duration_ms / 1000) > 0.05)
          throw Error('Laughter duration differs')
        laughAudio = await context!.decodeAudioData(raw)
      } catch {
        laughAudio = undefined
        laughPcm = undefined
      }
    }
    if (id !== generation || !mounted) return
    const direction = directed
      ? admitPerformance(result.performance, spokenText, Date.now(), Boolean(laughAudio))
      : undefined
    if (
      direction?.resolved?.some((c) => c.kind === 'laugh') &&
      laughAudio &&
      laughPcm &&
      audio &&
      context
    ) {
      const start = audio.duration + 0.12,
        end = start + laughAudio.duration
      const sequence = context.createBuffer(
        1,
        Math.ceil((end + 0.1) * audio.sampleRate),
        audio.sampleRate,
      )
      sequence.copyToChannel(audio.getChannelData(0), 0)
      sequence.copyToChannel(laughAudio.getChannelData(0), 0, Math.round(start * audio.sampleRate))
      const energy: number[] = [],
        step = Math.round(laughPcm.sampleRate * 0.02)
      for (let i = 0; i < laughPcm.samples.length; i += step) {
        let sum = 0
        const count = Math.min(step, laughPcm.samples.length - i)
        for (let j = i; j < i + count; j++) sum += laughPcm.samples[j]! ** 2
        energy.push(Math.sqrt(sum / count))
      }
      const peak = Math.max(0.001, ...energy)
      let previous = 0
      laughSegment.value = {
        start,
        end,
        energy: energy.map((e) => {
          const target = e / peak
          if (!referenceLaughter) return target
          previous += (target - previous) * (target > previous ? 0.55 : 0.35)
          return previous
        }),
      }
      audio = sequence
    }
    if (direction?.resolved?.some((c) => c.kind === 'smile') && audio && context) {
      const withTail = context.createBuffer(
        audio.numberOfChannels,
        audio.length + Math.ceil(audio.sampleRate * 1.9),
        audio.sampleRate,
      )
      for (let channel = 0; channel < audio.numberOfChannels; channel++)
        withTail.copyToChannel(audio.getChannelData(channel), channel)
      audio = withTail
    }
    beats.value = speechBeats(pcm.samples, pcm.sampleRate)
    behaviorPlan.value = directed
      ? directedPlan(pcm.samples, pcm.sampleRate, direction)
      : behavior
        ? planSpeechBehavior(
            pcm.samples,
            pcm.sampleRate,
            [...(character + spokenText)].reduce(
              (seed, c) => (Math.imul(seed, 31) + c.charCodeAt(0)) >>> 0,
              17,
            ),
            behavior,
          )
        : null
    // Let the closed-mouth expression settle before audio starts, including
    // immediate cache hits. This delay is outside the audio/viseme clock.
    const remaining = settleExpression
      ? expressionLeadMs - (performance.now() - preparationStarted)
      : 0
    if (remaining > 0) await new Promise((resolve) => setTimeout(resolve, remaining))
    if (id !== generation || !mounted) return
    loading.value = false
    active.value = true
    offset = 0
    await resume()
  }
  async function playPreparedSpeech(result: SpeechResponse, spokenText: string, immediate = false) {
    stop()
    error.value = ''
    const id = generation
    try {
      loading.value = true
      await activateAudio()
      if (id !== generation || !mounted || document.hidden) return
      await loadPrepared(result, id, performance.now(), spokenText, !immediate)
    } catch (e) {
      if (id !== generation || !mounted) return
      stop()
      error.value = e instanceof Error ? e.message : 'Prepared speech playback failed.'
    }
  }
  function visibility() {
    if (document.hidden) pause()
  }
  const statusAbort = new AbortController()
  onMounted(async () => {
    document.addEventListener('visibilitychange', visibility)
    if (output) {
      configured.value = true
      checking.value = false
      return
    }
    try {
      const response = await fetch('/api/v1/speech/status', { signal: statusAbort.signal })
      if (response.ok && mounted) configured.value = Boolean((await response.json()).configured)
    } catch {
      /* The manual motion trial remains available without the API. */
    } finally {
      if (mounted) checking.value = false
    }
  })
  onScopeDispose(() => {
    mounted = false
    statusAbort.abort()
    stop()
    cache.clear()
    document.removeEventListener('visibilitychange', visibility)
    if (!output) void context?.close()
  })
  return {
    configured: readonly(configured),
    checking: readonly(checking),
    loading: readonly(loading),
    active: readonly(active),
    articulating,
    laughAmount,
    laughEnergy,
    paused: readonly(paused),
    error: readonly(error),
    frame: readonly(frame),
    position: readonly(position),
    beats: readonly(beats),
    behaviorPlan: readonly(behaviorPlan),
    session: readonly(session),
    text,
    language,
    voice,
    speak,
    activateAudio,
    playPreparedSpeech,
    pause,
    resume,
    stop,
  }
}
