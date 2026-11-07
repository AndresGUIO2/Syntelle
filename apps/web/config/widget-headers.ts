import type { Plugin, Connect } from 'vite'

export function widgetFrameAncestors(values: unknown): string {
  if (!Array.isArray(values)) return "'none'"
  const origins = values.filter((value): value is string => {
    if (typeof value !== 'string') return false
    try {
      const url = new URL(value)
      return (
        url.origin === value &&
        !url.username &&
        !url.password &&
        (url.protocol === 'https:' ||
          (url.protocol === 'http:' && ['localhost', '127.0.0.1'].includes(url.hostname)))
      )
    } catch {
      return false
    }
  })
  return [...new Set(origins)].join(' ') || "'none'"
}

/** Local Vite equivalent of the production edge's per-widget framing policy. */
export function widgetHeaders(): Plugin {
  const middleware: Connect.NextHandleFunction = async (req, res, next) => {
    const path = (req.url ?? '/').split('?')[0]!
    if (path.startsWith('/api/')) return next()
    const embedded = /^\/embed\/([a-zA-Z0-9_-]+)\/?$/.exec(path)
    if (!embedded) {
      res.setHeader('Content-Security-Policy', "frame-ancestors 'none'")
      res.setHeader('X-Frame-Options', 'DENY')
      return next()
    }
    res.setHeader('Cache-Control', 'no-store')
    res.setHeader('Referrer-Policy', 'strict-origin')
    res.setHeader('Content-Security-Policy', "frame-ancestors 'none'")
    try {
      const target = new URL(
        `/api/v1/widgets/${embedded[1]}`,
        process.env.API_PROXY_TARGET ?? 'http://127.0.0.1:8080',
      )
      const response = await fetch(target, { signal: AbortSignal.timeout(3000) })
      if (!response.ok) {
        res.statusCode = response.status
        res.end('This character is unavailable.')
        return
      }
      const metadata = (await response.json()) as { allowed_origins?: unknown }
      res.setHeader(
        'Content-Security-Policy',
        `frame-ancestors ${widgetFrameAncestors(metadata.allowed_origins)}; object-src 'none'; base-uri 'self'`,
      )
      next()
    } catch {
      res.statusCode = 503
      res.end('This character is temporarily unavailable.')
    }
  }
  return {
    name: 'widget-framing-policy',
    configureServer(server) {
      server.middlewares.use(middleware)
    },
    configurePreviewServer(server) {
      server.middlewares.use(middleware)
    },
  }
}
