import { RequestError } from '../streaming/api'
import type { RemoteSession, RemoteStreamTransport } from '../streaming/transport'

/** No account cookies, global guest identity or credentials in URLs. */
export async function widgetRequest<T>(
  path: string,
  method = 'GET',
  body?: unknown,
  capability = '',
): Promise<T> {
  const response = await fetch(path, {
    method,
    credentials: 'omit',
    cache: 'no-store',
    ...(method === 'DELETE' ? { keepalive: true } : {}),
    headers: {
      Accept: 'application/json',
      ...(method === 'GET'
        ? {}
        : { 'Content-Type': 'application/json', 'X-Syntelle-Request': '1' }),
      ...(capability ? { Authorization: `Bearer ${capability}` } : {}),
    },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
  })
  const value = await response.json().catch(() => null)
  if (!response.ok)
    throw new RequestError(
      value?.error?.message ?? value?.error ?? 'This character is currently unavailable.',
      response.status,
    )
  return value as T
}

export function createWidgetTransport(
  create: () => Promise<RemoteSession & { capability: string }>,
): RemoteStreamTransport {
  const capabilities = new Map<string, string>()
  return {
    loadPolicy: false,
    async createSession() {
      const session = await create()
      if (!session.capability) throw new Error('The session could not be authorized.')
      capabilities.set(session.id, session.capability)
      return session
    },
    async request<T>(path: string, method = 'GET', body?: unknown) {
      const match = /^\/api\/v1\/streams\/([^/]+)(?:\/ticket)?$/.exec(path)
      const id = match ? decodeURIComponent(match[1]!) : ''
      const capability = capabilities.get(id)
      if (!capability) throw new Error('This session has ended. Start a new conversation.')
      try {
        return await widgetRequest<T>(path, method, body, capability)
      } finally {
        if (method === 'DELETE') capabilities.delete(id)
      }
    },
  }
}
