import type { SpeechResponse } from '../character/speech/speechTimeline'
export type ConversationTurn = {
  id: string
  sequence: number
  status: string
  user: string
  reply: string
  spoken: string
}
export type ConversationEvent = {
  type: string
  conversation_id: string
  turn_id: string
  text?: string
  data?: ConversationTurn[] | SpeechResponse | SpeechPart | { spoken: string } | { parts: number }
}
export type SpeechPart = {
  speech: SpeechResponse
  text: string
  index: number
  final: boolean
  offset_ms: number
}
export const defaultInstructions =
  'Be a clear, friendly assistant. Match the depth and length to the question, and explain fully when asked. Do not invent company knowledge.'
