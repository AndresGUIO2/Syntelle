/** Alternate admission/authentication only; media and articulation stay shared. */
export interface RemoteSession {
  ice_servers?: RTCIceServer[]
  startup_timeout_ms?: number
  id: string
  ticket: string
  state: string
  configured: boolean
  speech_configured?: boolean
  recognition: boolean
  laughter_available?: boolean
  emotions_available?: boolean
  time_limits_enabled?: boolean
}

export interface RemoteStreamTransport {
  request<T>(path: string, method?: string, body?: unknown): Promise<T>
  createSession(context: {
    resumeFrom: string
    continueConversationID: string
  }): Promise<RemoteSession>
  /** Public widgets do not inherit guest/account demo policy. */
  loadPolicy?: boolean
}
