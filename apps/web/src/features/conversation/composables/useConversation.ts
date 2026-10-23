import { computed, onMounted, onScopeDispose, shallowRef, watch, type Ref } from 'vue'
import type { useSpeechPlayback } from '../../character/composables/useSpeechPlayback'
import type { SpeechResponse } from '../../character/speech/speechTimeline'
import {
  defaultInstructions,
  type ConversationEvent,
  type ConversationTurn,
  type SpeechPart,
} from '../types'
import { useReplySpeech } from './useReplySpeech'
import { useConversationSocket } from './useConversationSocket'
import { useMicrophone } from './useMicrophone'

type Playback = ReturnType<typeof useSpeechPlayback>
type Options = {
  character: Ref<string>
  language: Ref<string>
  voice: Ref<string>
  playback: Ref<Playback | undefined>
}
export function useConversation(options: Options) {
  const turns = shallowRef<ConversationTurn[]>([]),
    draft = shallowRef(''),
    partial = shallowRef('')
  const instructions = shallowRef(defaultInstructions),
    pending = shallowRef(false),
    error = shallowRef('')
  const configured = shallowRef(false),
    recognitionAvailable = shallowRef(false),
    checking = shallowRef(true)
  const currentTurn = shallowRef(''),
    conversation = shallowRef('')
  const startingMicrophone = shallowRef(false)
  const status = computed(() =>
    options.playback.value?.active.value
      ? 'Speaking'
      : pending.value
        ? 'Preparing reply'
        : startingMicrophone.value
          ? 'Starting microphone…'
          : microphone.active.value
            ? 'Listening'
            : 'Ready',
  )
  let micID = '',
    connectionBinding = '',
    disposed = false
  let microphoneAttempt = 0
  const socket = useConversationSocket(handleEvent, () => {
    if (currentTurn.value) {
      replaceTurn(currentTurn.value, { status: 'interrupted' })
      error.value ||=
        'The reply was interrupted because the connection closed. Turn on the microphone to reconnect.'
      currentTurn.value = ''
    }
    replySpeech.reset()
    microphone.stop()
    micID = ''
    partial.value = ''
    pending.value = false
    options.playback.value?.stop()
  })
  const replySpeech = useReplySpeech(options.playback, {
    started: () => send('playback_started'),
    waiting: () => send('playback_waiting'),
    ended: () => {
      pending.value = false
      if (currentTurn.value) {
        replaceTurn(currentTurn.value, { status: 'ready' })
        send('playback_ended')
        currentTurn.value = ''
      }
    },
    failed: (message) => {
      error.value = message
      cancelReply('audio playback unavailable')
    },
  })
  const microphone = useMicrophone(
    (bytes) => {
      if (!micID) return
      const audio = btoa(String.fromCharCode(...new Uint8Array(bytes)))
      socket.send({
        type: 'audio',
        conversation_id: conversation.value,
        turn_id: micID,
        audio_base64: audio,
      })
    },
    () => {
      if (!currentTurn.value || !options.playback.value?.active.value) return
      // Energy only arms interruption detection. Noise or speaker leakage must
      // not destroy the reply before recognition confirms a new utterance.
      send('activity', currentTurn.value)
    },
    () => {
      if (micID) send('mic_stop', micID)
      micID = ''
      partial.value = ''
    },
  )
  function send(type: string, id = currentTurn.value, extra: object = {}) {
    return socket.send({ type, conversation_id: conversation.value, turn_id: id, ...extra })
  }
  function replaceTurn(id: string, change: Partial<ConversationTurn>) {
    turns.value = turns.value.map((turn) => (turn.id === id ? { ...turn, ...change } : turn))
  }
  function handleEvent(event: ConversationEvent) {
    if (event.conversation_id !== conversation.value || disposed) return
    if (event.type === 'persistence_warning') {
      // A cancelled turn may no longer be current; this warning owns no playback state.
      if (!error.value)
        error.value = event.text ?? 'Playback stopped, but this change could not be saved.'
      return
    }
    if (event.type === 'ready') {
      turns.value = (event.data ?? []) as ConversationTurn[]
      return
    }
    if (event.type === 'listening' || event.type === 'speech_detected') return
    if (event.type === 'transcript_partial') {
      if (event.turn_id === micID) partial.value = event.text ?? ''
      return
    }
    if (event.type === 'microphone_error') {
      if (event.turn_id === micID) {
        error.value = event.text ?? 'Recognition stopped.'
        stopMicrophone()
      }
      return
    }
    if (event.type === 'transcript_final') {
      if (turns.value.some((turn) => turn.id === event.turn_id)) return
      partial.value = ''
      cancelReply('recognized interruption')
      currentTurn.value = event.turn_id
      return
    }
    if (event.type === 'accepted') {
      if (event.turn_id !== currentTurn.value) return
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
    if (event.turn_id !== currentTurn.value) return
    if (event.type === 'reply_delta') {
      const turn = turns.value.find((item) => item.id === event.turn_id)
      replaceTurn(event.turn_id, { reply: (turn?.reply ?? '') + (event.text ?? '') })
    } else if (event.type === 'reply_complete') {
      replaceTurn(event.turn_id, {
        reply: event.text ?? '',
        spoken: (event.data as { spoken: string })?.spoken ?? '',
      })
    } else if (event.type === 'speech_part') {
      if (!document.hidden) replySpeech.push(event.data as SpeechPart)
    } else if (event.type === 'speech_complete') {
      replySpeech.close((event.data as { parts?: number } | undefined)?.parts)
    } else if (event.type === 'prepared_speech') {
      pending.value = false
      replaceTurn(event.turn_id, { status: 'ready' })
      const playback = options.playback.value
      if (playback && !document.hidden) {
        const id = event.turn_id
        void playback
          .playPreparedSpeech(event.data as SpeechResponse, event.text ?? '')
          .then(() => {
            if (id !== currentTurn.value) return
            if (playback.active.value) send('playback_started', id)
            else if (playback.error.value) {
              error.value = playback.error.value
              cancelReply('audio playback unavailable')
            }
          })
      }
    } else if (event.type === 'cancelled') {
      replySpeech.reset()
      pending.value = false
      currentTurn.value = ''
      options.playback.value?.stop()
      replaceTurn(event.turn_id, { status: 'interrupted' })
    } else if (event.type === 'error') {
      replySpeech.reset()
      options.playback.value?.stop()
      pending.value = false
      error.value = event.text ?? 'Conversation failed.'
      replaceTurn(event.turn_id, { status: 'failed' })
      currentTurn.value = ''
    }
  }
  async function connect() {
    const binding = JSON.stringify([
      conversation.value,
      options.language.value,
      options.voice.value,
      instructions.value,
    ])
    if (socket.connected.value && connectionBinding === binding) return true
    if (socket.connected.value) {
      connectionBinding = binding
      return send('configure', crypto.randomUUID(), {
        instructions: instructions.value,
        voice: options.voice.value,
      })
    }
    connectionBinding = binding
    return socket.open({
      type: 'hello',
      conversation_id: conversation.value,
      turn_id: crypto.randomUUID(),
      character: options.character.value,
      language: options.language.value,
      voice: options.voice.value,
      instructions: instructions.value,
    })
  }
  async function submit() {
    const text = draft.value.trim()
    if (!text || pending.value) return
    error.value = ''
    try {
      await options.playback.value?.activateAudio()
    } catch {
      error.value =
        'Audio playback is unavailable in this browser. Typed replies will still appear.'
    }
    try {
      if (!(await connect()) || document.hidden) return
      cancelReply('typed interruption')
      const id = crypto.randomUUID()
      currentTurn.value = id
      pending.value = true
      if (send('submit', id, { text })) draft.value = ''
      else pending.value = false
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Chat could not start.'
    }
  }
  function cancelReply(reason = 'owner stopped reply') {
    replySpeech.reset()
    if (currentTurn.value) {
      send('cancel', currentTurn.value, { reason })
      replaceTurn(currentTurn.value, { status: 'interrupted' })
    }
    currentTurn.value = ''
    pending.value = false
    options.playback.value?.stop()
  }
  function stopMicrophone() {
    microphoneAttempt++
    if (micID) send('mic_stop', micID)
    micID = ''
    microphone.stop()
    partial.value = ''
  }
  async function toggleMicrophone() {
    if (startingMicrophone.value) return
    if (microphone.active.value) {
      stopMicrophone()
      return
    }
    error.value = ''
    startingMicrophone.value = true
    const attempt = ++microphoneAttempt
    try {
      await options.playback.value?.activateAudio()
      if (disposed || document.hidden || attempt !== microphoneAttempt) return
      if (!(await connect()) || document.hidden || attempt !== microphoneAttempt) return
      if (await microphone.start()) {
        micID = crypto.randomUUID()
        send('mic_start', micID)
      }
    } catch (e) {
      error.value =
        e instanceof Error && e.message.includes('Web Audio')
          ? 'Voice conversation is not supported in this browser. Please use another browser.'
          : e instanceof Error
            ? e.message
            : 'Microphone could not start.'
    } finally {
      startingMicrophone.value = false
    }
  }
  function suspend() {
    cancelReply('workspace suspended')
    stopMicrophone()
    socket.close()
    connectionBinding = ''
  }
  function key() {
    return `syntelle.chat.${options.character.value}.${options.language.value}`
  }
  function newChat() {
    suspend()
    turns.value = []
    draft.value = ''
    error.value = ''
    conversation.value = crypto.randomUUID()
    try {
      localStorage.setItem(key(), conversation.value)
    } catch {
      /* In-memory chat remains available. */
    }
    if (configured.value) void connect()
  }
  watch(
    [options.character, options.language],
    () => {
      suspend()
      turns.value = []
      draft.value = ''
      error.value = ''
      try {
        conversation.value = localStorage.getItem(key()) ?? crypto.randomUUID()
        localStorage.setItem(key(), conversation.value)
        instructions.value =
          localStorage.getItem(`syntelle.instructions.${options.character.value}`) ??
          defaultInstructions
      } catch {
        conversation.value = crypto.randomUUID()
        instructions.value = defaultInstructions
      }
      if (configured.value) void connect()
    },
    { immediate: true, flush: 'sync' },
  )
  watch(instructions, (value) => {
    try {
      localStorage.setItem(`syntelle.instructions.${options.character.value}`, value)
    } catch {
      /* Optional preference. */
    }
  })
  watch(
    () => options.playback.value?.active.value,
    (active, previous) => {
      if (previous && !active && currentTurn.value && !pending.value && !replySpeech.active()) {
        send('playback_ended')
        currentTurn.value = ''
      }
    },
    // Finish the old playback before a recognized replacement turn can arrive.
    // A deferred stop watcher could otherwise clear the newer turn's identity.
    { flush: 'sync' },
  )
  const shownError = computed(() => error.value || microphone.error.value || socket.error.value)
  function visibility() {
    if (document.hidden) suspend()
  }
  const statusAbort = new AbortController()
  onMounted(async () => {
    document.addEventListener('visibilitychange', visibility)
    try {
      const response = await fetch('/api/v1/conversation/status', { signal: statusAbort.signal })
      if (response.ok) {
        const body = await response.json()
        configured.value = Boolean(body.configured)
        recognitionAvailable.value = Boolean(body.recognition)
        if (configured.value) await connect()
      }
    } catch {
      /* Character preview and customization remain available. */
    } finally {
      checking.value = false
    }
  })
  onScopeDispose(() => {
    disposed = true
    statusAbort.abort()
    suspend()
    document.removeEventListener('visibilitychange', visibility)
  })
  return {
    turns,
    draft,
    partial,
    instructions,
    pending,
    configured,
    recognitionAvailable,
    checking,
    status,
    error: shownError,
    microphone: microphone.active,
    inputLevel: microphone.inputLevel,
    startingMicrophone,
    submit,
    cancelReply,
    toggleMicrophone,
    newChat,
    suspend,
  }
}
