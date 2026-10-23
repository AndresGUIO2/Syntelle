import { onScopeDispose, readonly, shallowRef } from 'vue'
import workletURL from '../audio/capture.worklet.ts?worker&url'

export function useMicrophone(
  onPCM: (bytes: ArrayBuffer) => void,
  onActivity: () => void,
  onFailure: () => void,
) {
  const active = shallowRef(false),
    error = shallowRef(''),
    inputLevel = shallowRef(0)
  let context: AudioContext | null = null
  let stream: MediaStream | null = null
  let source: MediaStreamAudioSourceNode | null = null
  let node: AudioWorkletNode | null = null
  let generation = 0
  function stop() {
    generation++
    active.value = false
    inputLevel.value = 0
    if (node) {
      node.port.onmessage = null
      node.onprocessorerror = null
      node.disconnect()
      node.port.close()
    }
    source?.disconnect()
    stream?.getTracks().forEach((track) => track.stop())
    void context?.close()
    node = null
    source = null
    stream = null
    context = null
  }
  async function start() {
    stop()
    error.value = ''
    const id = generation
    let stage: 'audio' | 'microphone' | 'processing' = 'audio'
    try {
      if (
        !navigator.mediaDevices?.getUserMedia ||
        typeof AudioContext === 'undefined' ||
        typeof AudioWorkletNode === 'undefined'
      )
        throw Error(
          'Voice conversation requires microphone capture in a secure browser. Try another supported browser.',
        )
      const audioContext = new AudioContext()
      context = audioContext
      await audioContext.resume()
      stage = 'microphone'
      const input = await navigator.mediaDevices.getUserMedia({
        audio: {
          channelCount: 1,
          echoCancellation: true,
          noiseSuppression: true,
          autoGainControl: true,
        },
      })
      if (id !== generation) {
        input.getTracks().forEach((track) => track.stop())
        return false
      }
      stream = input
      stage = 'processing'
      await audioContext.audioWorklet.addModule(workletURL)
      if (id !== generation) return false
      source = audioContext.createMediaStreamSource(input)
      node = new AudioWorkletNode(audioContext, 'syntelle-conversation-capture', {
        channelCount: 1,
        channelCountMode: 'explicit',
        outputChannelCount: [1],
      })
      const failed = (message: string) => {
        if (id !== generation) return
        stop()
        error.value = message
        onFailure()
      }
      node.onprocessorerror = () =>
        failed('Microphone processing failed. Turn it on again to continue.')
      input.getTracks().forEach((track) => {
        track.onended = () => failed('Microphone disconnected. Reconnect it and turn it on again.')
      })
      node.port.onmessage = (
        event: MessageEvent<{ type: string; bytes: ArrayBuffer; level: number }>,
      ) => {
        if (id !== generation || !active.value) return
        if (event.data.type === 'pcm') {
          inputLevel.value = Number.isFinite(event.data.level)
            ? Math.min(1, Math.max(0, event.data.level))
            : 0
          onPCM(event.data.bytes)
          node?.port.postMessage('received')
        }
        if (event.data.type === 'activity') onActivity()
        if (event.data.type === 'overflow')
          failed('Microphone processing fell behind. Turn it on again to continue.')
      }
      source.connect(node)
      // The processor emits silence; connecting keeps Safari's graph running.
      node.connect(audioContext.destination)
      active.value = true
      return true
    } catch (e) {
      if (id !== generation) return false
      stop()
      error.value =
        e instanceof DOMException && e.name === 'NotAllowedError'
          ? stage === 'microphone'
            ? 'Microphone access is blocked by the browser or system. Check microphone permissions for this site and the app running it, then try again.'
            : 'Audio could not start in this browser. Try enabling the microphone again.'
          : e instanceof DOMException && e.name === 'NotFoundError'
            ? 'No microphone was found. Connect a microphone and try again.'
            : e instanceof DOMException && e.name === 'NotReadableError'
              ? 'The microphone could not be opened. Check that it is connected and available to this browser, then try again.'
              : e instanceof Error
                ? e.message
                : 'Microphone capture failed. Check your microphone and try again.'
      return false
    }
  }
  onScopeDispose(stop)
  return { active, error, inputLevel: readonly(inputLevel), start, stop }
}
