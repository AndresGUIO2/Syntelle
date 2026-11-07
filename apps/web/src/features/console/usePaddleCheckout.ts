import { onScopeDispose } from 'vue'

type PaddleEvent = { name: string }
interface PaddleClient {
  Environment: { set: (environment: 'sandbox') => void }
  Initialize: (input: { token: string; eventCallback: (event: PaddleEvent) => void }) => void
  Checkout: {
    open: (input: {
      transactionId: string
      settings: { displayMode: 'overlay'; locale: 'en' | 'es'; theme: 'light' | 'dark' }
    }) => void
    close: () => void
  }
}
declare global {
  interface Window {
    Paddle?: PaddleClient
  }
}
let script: Promise<PaddleClient> | undefined
let initializedToken = ''
let active: { owner: symbol; event: (event: PaddleEvent) => void } | undefined

function loadPaddle(): Promise<PaddleClient> {
  if (window.Paddle) return Promise.resolve(window.Paddle)
  if (script) return script
  script = new Promise((resolve, reject) => {
    const element = document.createElement('script')
    element.src = 'https://cdn.paddle.com/paddle/v2/paddle.js'
    element.async = true
    const timeout = window.setTimeout(
      () => finish(new Error('The secure checkout could not load. Try opening it again.')),
      15000,
    )
    function finish(error?: Error) {
      window.clearTimeout(timeout)
      element.onload = null
      element.onerror = null
      if (error || !window.Paddle) {
        element.remove()
        script = undefined
        reject(error ?? new Error('The secure checkout is unavailable.'))
        return
      }
      resolve(window.Paddle)
    }
    element.onload = () => finish()
    element.onerror = () =>
      finish(new Error('The secure checkout could not load. Try opening it again.'))
    document.head.append(element)
  })
  return script
}

/** Load the official browser SDK only after an explicit billing action. No browser event grants access. */
export function usePaddleCheckout(onResult: (event: 'closed' | 'completed' | 'error') => void) {
  const owner = Symbol('workspace-checkout')
  let disposed = false
  function close() {
    if (active?.owner !== owner) return
    active = undefined
    window.Paddle?.Checkout.close()
  }
  async function open(transactionId: string, token: string, locale: 'en' | 'es') {
    if (!/^test_[a-zA-Z0-9]+$/.test(token) || !/^txn_[a-z0-9]{26}$/.test(transactionId))
      throw new Error('The test payment configuration is not valid.')
    const paddle = await loadPaddle()
    if (disposed) return
    if (initializedToken && initializedToken !== token)
      throw new Error('The payment configuration changed. Reload this page before continuing.')
    if (!initializedToken) {
      paddle.Environment.set('sandbox')
      paddle.Initialize({ token, eventCallback: (event) => active?.event(event) })
      initializedToken = token
    }
    if (active) paddle.Checkout.close()
    active = {
      owner,
      event(event) {
        if (disposed || active?.owner !== owner) return
        if (event.name === 'checkout.completed') onResult('completed')
        else if (event.name === 'checkout.closed') onResult('closed')
        else if (event.name === 'checkout.error') onResult('error')
      },
    }
    paddle.Checkout.open({
      transactionId,
      settings: {
        displayMode: 'overlay',
        locale,
        theme: document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light',
      },
    })
  }
  onScopeDispose(() => {
    disposed = true
    close()
  })
  return { open, close }
}
