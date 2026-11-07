interface Turnstile {
  render(element: HTMLElement, options: Record<string, unknown>): string
  remove(id: string): void
}
declare global {
  interface Window {
    turnstile?: Turnstile
  }
}
let scriptPromise: Promise<Turnstile> | undefined
function load(): Promise<Turnstile> {
  if (window.turnstile) return Promise.resolve(window.turnstile)
  scriptPromise ??= new Promise<Turnstile>((resolve, reject) => {
    const script = document.createElement('script')
    const timer = setTimeout(() => {
      script.remove()
      reject(Error('Verification could not load. Please try again.'))
    }, 15_000)
    script.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit'
    script.async = true
    script.onload = () => {
      clearTimeout(timer)
      if (window.turnstile) resolve(window.turnstile)
      else reject(Error('Verification is unavailable.'))
    }
    script.onerror = () => {
      clearTimeout(timer)
      script.remove()
      reject(Error('Verification could not load. Please try again.'))
    }
    document.head.append(script)
  }).catch((failure: unknown) => {
    scriptPromise = undefined
    throw failure
  })
  return scriptPromise
}

export async function verifyVisitor(
  element: HTMLElement,
  sitekey: string,
  nonce: string,
  signal: AbortSignal,
): Promise<string> {
  if (!sitekey) throw Error('Website access has not been configured yet.')
  const turnstile = await load()
  if (signal.aborted) throw new DOMException('Cancelled', 'AbortError')
  return new Promise((resolve, reject) => {
    let widget = ''
    let settled = false
    const finish = (error?: Error, token?: string) => {
      if (settled) return
      settled = true
      clearTimeout(timer)
      signal.removeEventListener('abort', abort)
      // A fixture may complete synchronously during render.
      queueMicrotask(() => {
        if (widget) turnstile.remove(widget)
      })
      if (error) reject(error)
      else resolve(token!)
    }
    const abort = () => finish(new DOMException('Cancelled', 'AbortError'))
    const timer = setTimeout(
      () => finish(Error('Verification expired. Please try again.')),
      120_000,
    )
    signal.addEventListener('abort', abort, { once: true })
    widget = turnstile.render(element, {
      sitekey,
      action: 'widget_start',
      cData: nonce,
      theme: 'auto',
      callback: (token: string) => finish(undefined, token),
      'error-callback': () => finish(Error('Verification could not complete. Please try again.')),
      'expired-callback': () => finish(Error('Verification expired. Please try again.')),
    })
  })
}
