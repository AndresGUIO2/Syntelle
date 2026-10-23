import { computed, onScopeDispose, shallowRef } from 'vue'
import { request, RequestError } from '../api'
import type { Envelope, StreamSettings } from '../types'
import type { ConversationEvent, ConversationTurn } from '../../conversation/types'
import { useMicrophone } from '../../conversation/composables/useMicrophone'

export function useRemoteStream(settings: () => StreamSettings) {
  const id = shallowRef('')
  const state = shallowRef('not-started')
  const media = shallowRef<MediaStream | null>(null)
  const error = shallowRef('')
  const ready = shallowRef(false)
  const connecting = shallowRef(false)
  const configured = shallowRef(false)
  const speechConfigured = shallowRef(false)
  const recognition = shallowRef(false)
  const timeLimitsEnabled = shallowRef<boolean | null>(null)
  const pending = shallowRef(false)
  const speaking = shallowRef(false)
  const audioSuppressed = shallowRef(false)
  const startingMicrophone = shallowRef(false)
  const partial = shallowRef('')
  const turns = shallowRef<ConversationTurn[]>([])
  const currentTurn = shallowRef('')
  let socket: WebSocket | null = null
  let peer: RTCPeerConnection | null = null
  let epoch = 0
  let micID = ''
  let interruptedMicrophone = ''
  let heartbeat: ReturnType<typeof setInterval> | undefined
  let signaling = Promise.resolve()
  let ice: RTCIceCandidateInit[] = []
  let disposed = false
  let retry = 0
  let recoveryTimer: ReturnType<typeof setTimeout> | undefined
  let connectionTimer: ReturnType<typeof setTimeout> | undefined
  let pausedID = ''
  let releasing = Promise.resolve()
  type Session = {
    id: string
    ticket: string
    state: string
    configured: boolean
    speech_configured?: boolean
    recognition: boolean
    time_limits_enabled?: boolean
  }

  async function loadPolicy() {
    try {
      const policy = await request<{ time_limits_enabled?: boolean }>('/api/v1/streams/status')
      if (!disposed) timeLimitsEnabled.value = policy.time_limits_enabled !== false
    } catch {
      // Starting the stream reports service failures; a policy read starts no work.
    }
  }
  const status = computed(() =>
    state.value === 'not-started'
      ? 'Turn on your microphone to start talking.'
      : state.value === 'ended'
        ? 'This session has ended.'
        : state.value === 'exhausted'
          ? 'Your 15-minute demo allowance has been used.'
          : state.value === 'paused'
            ? 'Syntelle is paused due to inactivity'
            : !ready.value
              ? connecting.value
                ? retry
                  ? 'Reconnecting to Syntelle…'
                  : 'Connecting to Syntelle…'
                : 'Syntelle is disconnected'
              : speaking.value
                ? 'Speaking'
                : pending.value
                  ? 'Preparing reply'
                  : microphone.active.value
                    ? 'Listening'
                    : 'Ready',
  )

  function connected() {
    ready.value = true
    connecting.value = false
    retry = 0
    error.value = ''
    clearTimeout(connectionTimer)
  }

  // Only transport and idle video recover automatically. User commands and
  // microphone capture are never replayed by connection recovery.
  function recover(binding: number) {
    if (binding !== epoch || disposed || state.value === 'paused') return
    detach()
    if (currentTurn.value) replaceTurn(currentTurn.value, { status: 'interrupted' })
    currentTurn.value = ''
    if (retry >= 3) {
      connecting.value = false
      state.value = 'disconnected'
      error.value = 'Syntelle is temporarily unavailable. Select Reconnect to try again.'
      return
    }
    const delay = [500, 1500, 3000][retry++]!
    connecting.value = true
    state.value = 'reconnecting'
    error.value = ''
    recoveryTimer = setTimeout(() => void connect(true), delay)
  }

  function send(type: string, data?: unknown, command_id?: string) {
    if (socket?.readyState !== WebSocket.OPEN || !id.value) return false
    socket.send(JSON.stringify({ type, session_id: id.value, command_id, data }))
    return true
  }

  function sendMicrophone(type: string, turn_id = micID, extra: object = {}) {
    return send('microphone', { type, turn_id, ...extra })
  }

  const microphone = useMicrophone(
    (bytes) => {
      if (micID)
        sendMicrophone('audio', micID, {
          audio_base64: btoa(String.fromCharCode(...new Uint8Array(bytes))),
        })
    },
    () => {
      if (currentTurn.value && speaking.value) sendMicrophone('activity', currentTurn.value)
    },
    () => stopMicrophone(),
  )

  function stopMicrophone() {
    if (micID) {
      interruptedMicrophone = micID
      sendMicrophone('mic_stop')
    }
    micID = ''
    microphone.stop()
    partial.value = ''
    startingMicrophone.value = false
  }

  async function toggleMicrophone() {
    if (microphone.active.value || startingMicrophone.value) {
      stopMicrophone()
      return
    }
    if (!ready.value || !recognition.value) return
    startingMicrophone.value = true
    error.value = ''
    micID = crypto.randomUUID()
    if (!sendMicrophone('mic_start')) {
      error.value = 'Reconnect the stream before using your microphone.'
      stopMicrophone()
      return
    }
    const active = await microphone.start()
    startingMicrophone.value = false
    if (!active) {
      error.value = microphone.error.value
      stopMicrophone()
    }
  }

  function replaceTurn(turn: string, patch: Partial<ConversationTurn>) {
    turns.value = turns.value.map((item) => (item.id === turn ? { ...item, ...patch } : item))
  }

  function conversation(event: ConversationEvent) {
    if (event.type === 'transcript_partial') {
      if (event.turn_id === micID) partial.value = event.text ?? ''
      return
    }
    if (event.type === 'microphone_error') {
      error.value = event.text || 'Recognition stopped.'
      stopMicrophone()
      return
    }
    if (event.type === 'transcript_final') {
      partial.value = ''
      audioSuppressed.value = true
    }
    if (event.type === 'accepted') {
      currentTurn.value = event.turn_id
      pending.value = true
      if (!turns.value.some((turn) => turn.id === event.turn_id))
        turns.value = [
          ...turns.value,
          {
            id: event.turn_id,
            sequence: turns.value.length + 1,
            status: 'preparing',
            user: event.text ?? '',
            reply: '',
            spoken: '',
          },
        ]
      return
    }
    if (event.type === 'persistence_warning') {
      error.value ||= event.text ?? 'The latest state could not be saved.'
      return
    }
    if (event.turn_id !== currentTurn.value) return
    if (event.type === 'reply_delta') {
      const previous = turns.value.find((turn) => turn.id === event.turn_id)
      replaceTurn(event.turn_id, { reply: (previous?.reply ?? '') + (event.text ?? '') })
    } else if (event.type === 'reply_complete') {
      replaceTurn(event.turn_id, {
        reply: event.text ?? '',
        spoken: (event.data as { spoken?: string })?.spoken ?? '',
      })
    } else if (event.type === 'cancelled') {
      audioSuppressed.value = true
      replaceTurn(event.turn_id, { status: 'interrupted' })
      currentTurn.value = ''
      pending.value = speaking.value = false
    } else if (event.type === 'error') {
      replaceTurn(event.turn_id, { status: 'failed' })
      pending.value = false
      error.value = event.text ?? 'The reply failed.'
    }
  }

  async function signal(data: {
    description?: RTCSessionDescriptionInit
    candidate?: RTCIceCandidateInit
  }) {
    const current = peer
    if (!current) return
    if (data.description) {
      await current.setRemoteDescription(data.description)
      for (const candidate of ice) await current.addIceCandidate(candidate)
      ice = []
      if (data.description.type === 'offer') {
        await current.setLocalDescription(await current.createAnswer())
        if (current === peer) send('signal', { description: current.localDescription })
      }
    }
    if (data.candidate) {
      if (current.remoteDescription) await current.addIceCandidate(data.candidate)
      else ice.push(data.candidate)
    }
  }

  function received(event: MessageEvent<string>, binding: number) {
    if (binding !== epoch) return
    let message: Envelope
    try {
      message = JSON.parse(event.data) as Envelope
    } catch {
      return
    }
    if (message.session_id && message.session_id !== id.value) return
    if (message.type === 'signal') {
      const target = peer
      signaling = signaling
        .then(() =>
          target === peer ? signal(message.data as Parameters<typeof signal>[0]) : undefined,
        )
        .catch(() => {
          if (binding === epoch)
            error.value = 'The video connection failed. Reconnect to try again.'
        })
    } else if (message.type === 'conversation') conversation(message.data as ConversationEvent)
    else if (message.type === 'state' || message.type === 'ready') {
      const data = message.data as
        { state?: string; configured?: boolean; recognition?: boolean } | undefined
      state.value = data?.state ?? (message.type === 'ready' ? 'idle' : state.value)
      if (data?.configured !== undefined) configured.value = data.configured
      if (data?.recognition !== undefined) recognition.value = data.recognition
      if (state.value === 'speaking') {
        speaking.value = true
        pending.value = false
      }
      if (state.value === 'idle') {
        if (currentTurn.value && speaking.value) replaceTurn(currentTurn.value, { status: 'ready' })
        speaking.value = pending.value = false
      }
    } else if (message.type === 'preview_audio') {
      currentTurn.value = (message.data as { turn_id: string }).turn_id
      pending.value = true
    } else if (message.type === 'playback') {
      const data = message.data as { event: string; turn_id: string }
      if (data.turn_id !== currentTurn.value) return
      speaking.value = data.event === 'playback_started'
      if (speaking.value) {
        audioSuppressed.value = false
        pending.value = false
      }
      if (data.event === 'playback_ended') {
        replaceTurn(data.turn_id, { status: 'ready' })
        pending.value = false
      }
    } else if (message.type === 'error') {
      const data = message.data as { message?: string; text?: string } | string | undefined
      error.value =
        typeof data === 'string'
          ? data
          : (data?.message ?? data?.text ?? 'The stream encountered an error.')
      pending.value = false
    } else if (message.type === 'paused' || message.type === 'ended') {
      const previousID = id.value
      if (currentTurn.value) replaceTurn(currentTurn.value, { status: 'interrupted' })
      detach()
      currentTurn.value = ''
      id.value = ''
      pausedID = message.type === 'paused' ? previousID : ''
      connecting.value = false
      retry = 0
      error.value = ''
      state.value = message.type === 'paused' ? 'paused' : 'ended'
    } else if (message.type === 'stopped') {
      error.value = 'This stream has ended.'
      void stop()
    }
  }

  async function connect(resume = false) {
    const resumeFrom = state.value === 'paused' ? pausedID : ''
    let released: Promise<void> | undefined
    if (resume) detach()
    else released = stop()
    const binding = epoch
    await released
    if (disposed || binding !== epoch) return
    connecting.value = true
    state.value = resume ? 'reconnecting' : 'connecting'
    error.value = ''
    try {
      let result: Session | undefined
      let restored = false
      if (resume && id.value) {
        const previousID = id.value
        // Failure requires explicit action: never allocate another renderer
        // automatically when the old session has closed or paused.
        const session = await request<Session>(`/api/v1/streams/${encodeURIComponent(previousID)}`)
        const ticket = await request<{ ticket: string }>(
          `/api/v1/streams/${encodeURIComponent(previousID)}/ticket`,
          'POST',
        )
        result = { ...session, ticket: ticket.ticket }
        restored = true
      }
      if (binding !== epoch || disposed) return
      result ??= await request<Session>('/api/v1/streams', 'POST', {
        settings: settings(),
        ...(resumeFrom ? { resume_from: resumeFrom } : {}),
      })
      if (binding !== epoch || disposed) {
        await request(`/api/v1/streams/${encodeURIComponent(result.id)}`, 'DELETE').catch(
          () => undefined,
        )
        return
      }
      id.value = result.id
      state.value = result.state
      configured.value = result.configured
      speechConfigured.value = result.speech_configured ?? result.configured
      recognition.value = result.recognition
      timeLimitsEnabled.value = result.time_limits_enabled !== false
      peer = new RTCPeerConnection({ iceServers: [] })
      const currentPeer = peer
      const stream = new MediaStream()
      media.value = stream
      peer.ontrack = ({ track }) => {
        if (binding !== epoch) return
        if (!stream.getTrackById(track.id)) {
          stream.addTrack(track)
          // addTrack() does not dispatch addtrack for manually assembled streams.
          stream.dispatchEvent(new Event('addtrack'))
        }
        track.onunmute = () => {
          if (binding === epoch && track.kind === 'video') connected()
        }
        if (track.kind === 'video' && !track.muted) connected()
      }
      peer.onicecandidate = ({ candidate }) => {
        if (candidate && binding === epoch) send('signal', { candidate: candidate.toJSON() })
      }
      peer.onconnectionstatechange = () => {
        if (binding !== epoch) return
        if (currentPeer.connectionState === 'connected') connected()
        if (['failed', 'disconnected'].includes(currentPeer.connectionState)) recover(binding)
      }
      const url = new URL(`/api/v1/streams/${result.id}/ws`, location.href)
      url.protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
      const connection = new WebSocket(url)
      socket = connection
      connection.onopen = () => {
        if (binding !== epoch) return connection.close()
        send('authenticate', { ticket: result.ticket })
        if (restored) {
          // Stop recognition/obsolete output on the surviving server channel;
          // reattachment does not submit any speech or model work.
          if (interruptedMicrophone) sendMicrophone('mic_stop', interruptedMicrophone)
          send('command', { type: 'interrupt', text: '' }, crypto.randomUUID())
        }
        interruptedMicrophone = ''
        heartbeat = setInterval(() => send('heartbeat'), 15_000)
      }
      connection.onmessage = (event) => received(event, binding)
      connection.onclose = () => {
        if (binding !== epoch) return
        recover(binding)
      }
      connectionTimer = setTimeout(() => recover(binding), 12_000)
    } catch (failure) {
      if (binding === epoch) {
        if (resumeFrom) {
          pausedID = resumeFrom
          state.value = 'paused'
          connecting.value = false
          error.value =
            failure instanceof Error ? failure.message : 'The stream encountered an error.'
        } else if (
          failure instanceof RequestError &&
          failure.status >= 400 &&
          failure.status < 500
        ) {
          error.value = failure.message
          connecting.value = false
          state.value =
            failure.message ===
            'Your 15-minute demo allowance is exhausted or in use by another session.'
              ? 'exhausted'
              : 'disconnected'
        } else recover(binding)
      }
    }
  }

  function command(
    type: string,
    text = '',
    value?: StreamSettings,
    amount?: number,
    repetitions?: number,
  ) {
    const commandID = crypto.randomUUID()
    if (
      !send(
        'command',
        {
          type,
          text,
          ...(value ? { settings: value } : {}),
          ...(amount !== undefined ? { amount } : {}),
          ...(repetitions !== undefined ? { repetitions } : {}),
        },
        commandID,
      )
    ) {
      error.value = 'Connect the stream before continuing.'
      return false
    }
    if (type === 'message' || type === 'speak') {
      if (speaking.value) audioSuppressed.value = true
      currentTurn.value = commandID
      pending.value = true
    }
    return true
  }

  function interrupt() {
    audioSuppressed.value = true
    command('interrupt')
    if (currentTurn.value) replaceTurn(currentTurn.value, { status: 'interrupted' })
    pending.value = speaking.value = false
  }

  function detach() {
    epoch++
    stopMicrophone()
    audioSuppressed.value = true
    clearTimeout(recoveryTimer)
    clearTimeout(connectionTimer)
    clearInterval(heartbeat)
    heartbeat = undefined
    socket?.close()
    socket = null
    peer?.close()
    peer = null
    media.value?.getTracks().forEach((track) => track.stop())
    media.value = null
    ice = []
    signaling = Promise.resolve()
    ready.value = speaking.value = pending.value = false
  }

  async function stop() {
    detach()
    retry = 0
    connecting.value = false
    const previousID = id.value
    id.value = ''
    pausedID = ''
    state.value = 'not-started'
    currentTurn.value = ''
    if (previousID) {
      const previous = releasing
      releasing = previous
        .then(() => request(`/api/v1/streams/${encodeURIComponent(previousID)}`, 'DELETE'))
        .then(
          () => undefined,
          () => undefined,
        )
    }
    await releasing
  }

  onScopeDispose(() => {
    disposed = true
    void stop()
  })
  return {
    id,
    state,
    media,
    ready,
    connecting,
    configured,
    speechConfigured,
    recognition,
    timeLimitsEnabled,
    loadPolicy,
    error,
    pending,
    speaking,
    audioSuppressed,
    partial,
    turns,
    status,
    currentTurn,
    startingMicrophone,
    microphone: microphone.active,
    inputLevel: microphone.inputLevel,
    connect,
    command,
    interrupt,
    stop,
    toggleMicrophone,
    stopMicrophone,
  }
}
