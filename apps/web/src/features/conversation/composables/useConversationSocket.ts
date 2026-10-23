import { onScopeDispose, shallowRef } from 'vue'
import type { ConversationEvent } from '../types'

export function useConversationSocket(
  onEvent: (event: ConversationEvent) => void,
  onDisconnect: () => void,
) {
  const connected = shallowRef(false),
    error = shallowRef('')
  let socket: WebSocket | null = null,
    generation = 0
  let settle: ((value: boolean) => void) | null = null,
    timer = 0
  function close() {
    settle?.(false)
    settle = null
    clearTimeout(timer)
    generation++
    connected.value = false
    if (socket) {
      socket.onopen = socket.onmessage = socket.onclose = socket.onerror = null
      socket.close()
    }
    socket = null
  }
  function send(event: object) {
    if (socket?.readyState !== WebSocket.OPEN) return false
    if (socket.bufferedAmount > 64000) {
      error.value = 'Connection fell behind. Reconnect before continuing.'
      close()
      onDisconnect()
      return false
    }
    socket.send(JSON.stringify(event))
    return true
  }
  async function open(hello: object) {
    close()
    error.value = ''
    const id = generation
    return new Promise<boolean>((resolve) => {
      settle = resolve
      const ws = new WebSocket(
        `${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}/api/v1/conversation/ws`,
      )
      socket = ws
      timer = window.setTimeout(() => {
        if (id === generation) {
          close()
          error.value = 'Conversation connection timed out.'
          resolve(false)
        }
      }, 8000)
      ws.onopen = () => {
        if (id !== generation) return
        connected.value = true
        send(hello)
      }
      ws.onmessage = (message) => {
        if (id !== generation) return
        try {
          const event = JSON.parse(message.data) as ConversationEvent
          if (event.type === 'error') {
            error.value = event.text ?? 'Conversation connection failed.'
            clearTimeout(timer)
            resolve(false)
          }
          if (event.type === 'ready') {
            clearTimeout(timer)
            settle = null
            resolve(true)
          }
          onEvent(event)
        } catch {
          error.value = 'Invalid conversation event.'
          close()
          onDisconnect()
        }
      }
      ws.onerror = () => {
        if (id === generation) error.value = 'Conversation service is unavailable.'
      }
      ws.onclose = () => {
        clearTimeout(timer)
        resolve(false)
        if (id !== generation) return
        if (!error.value)
          error.value = 'Conversation connection closed. Turn on the microphone to reconnect.'
        connected.value = false
        onDisconnect()
      }
    })
  }
  onScopeDispose(close)
  return { connected, error, open, close, send }
}
