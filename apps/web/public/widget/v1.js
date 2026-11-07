/* Syntelle website widget v1. Public IDs only; authority stays inside the iframe. */
;(() => {
  'use strict'
  const script = document.currentScript
  if (!(script instanceof HTMLScriptElement)) return
  const base = new URL(script.src).origin
  const id = script.dataset.widget
  if (!id || !/^[a-zA-Z0-9_-]+$/.test(id)) return
  const mode = script.dataset.mode === 'floating' ? 'floating' : 'inline'
  const host = document.createElement('div')
  host.dataset.syntelleWidget = id
  script.insertAdjacentElement('afterend', host)
  const root = host.attachShadow({ mode: 'open' })
  // Mirror of theme.css for this standalone, dependency-free shadow host.
  const darkTheme = {
    surface: '#293f53',
    text: '#f3f0eb',
    border: '#4a6075',
    primary: '#eeeae2',
    primaryHover: '#ffffff',
    onPrimary: '#28262d',
    accent: '#bdc6e9',
  }
  function applyTheme(theme) {
    if (!['light', 'dark'].includes(theme)) return
    host.dataset.syntelleTheme = theme
    const colors =
      theme === 'dark'
        ? darkTheme
        : {
            surface: '#ffffff',
            text: '#24212b',
            border: '#35313e',
            primary: '#bba3f5',
            primaryHover: '#bba3f5',
            onPrimary: '#21172f',
            accent: '#6d4bd0',
          }
    for (const [name, color] of Object.entries(colors)) {
      host.style.setProperty(`--syntelle-widget-${name}`, color)
    }
  }
  const style = document.createElement('style')
  style.textContent = `:host{display:block;color-scheme:normal}*{box-sizing:border-box}.panel{background:#1c1b20;border:1px solid #35313e;border-radius:18px;overflow:hidden;width:100%;height:620px;min-height:360px}iframe{display:block;border:0;width:100%;height:100%;background:transparent}.floating{position:fixed;right:24px;bottom:92px;width:390px;height:min(680px,calc(100dvh - 120px));max-width:calc(100vw - 32px);z-index:2147483000;box-shadow:0 12px 48px #0003}.launcher{position:fixed;right:24px;bottom:24px;z-index:2147483001;display:flex;align-items:center;gap:10px;min-height:52px;border:1px solid transparent;border-radius:28px;padding:12px 20px;background:#bba3f5;color:#21172f;font:600 14px/1.4 system-ui,sans-serif;cursor:pointer;box-shadow:0 4px 20px #0002}.launcher:focus-visible{outline:3px solid #6d4bd0;outline-offset:3px}.launcher svg{width:20px;height:20px;flex:none}.close{position:absolute;right:12px;top:6px;background:#1c1b20;color:#f3f0f7;border:0;width:44px;height:44px;font:24px system-ui;cursor:pointer;border-radius:8px;z-index:1}.close:focus-visible{outline:2px solid #bba3f5;outline-offset:-3px}.floating iframe{padding-top:48px}[hidden]{display:none!important}@media(max-width:480px){.floating{right:12px;bottom:88px;max-width:calc(100vw - 24px)}.launcher{right:16px;bottom:16px}.panel:not(.floating){height:620px}}@media(prefers-reduced-motion:no-preference){.launcher{transition:background-color 120ms ease-out}}`
  const panel = document.createElement('div')
  panel.className = `panel${mode === 'floating' ? ' floating' : ''}`
  panel.id = `syntelle-${crypto.randomUUID()}`
  const iframe = document.createElement('iframe')
  iframe.src = `${base}/embed/${encodeURIComponent(id)}`
  iframe.title = 'Syntelle character'
  iframe.allow = 'microphone; autoplay; fullscreen; picture-in-picture'
  iframe.referrerPolicy = 'strict-origin'
  panel.append(iframe)
  root.append(style, panel)
  // Keep the close action in the player's heading instead of a second title bar.
  const sharedHeading = document.createElement('style')
  sharedHeading.textContent =
    '.panel{background:var(--syntelle-widget-surface,#1c1b20);border-color:var(--syntelle-widget-border,#35313e)}.close{background:transparent;color:var(--syntelle-widget-text,#f3f0f7)}.launcher{background:var(--syntelle-widget-primary,#bba3f5);color:var(--syntelle-widget-onPrimary,#21172f)}.launcher:focus-visible{outline-color:var(--syntelle-widget-accent,#6d4bd0)}.close:focus-visible{outline-color:var(--syntelle-widget-accent,#bba3f5)}.floating iframe{padding-top:0}@media(hover:hover){:host([data-syntelle-theme="dark"]) .launcher:hover{background:var(--syntelle-widget-primaryHover)}}'
  root.append(sharedHeading)
  let nonce = ''
  let destroyed = false
  let launcher
  let closeButton
  function send(type) {
    if (nonce)
      iframe.contentWindow?.postMessage(
        {
          channel: 'syntelle-widget-v1',
          nonce,
          type,
          ...(type === 'init' ? { placement: mode } : {}),
        },
        base,
      )
  }
  function close() {
    send('close')
    if (mode === 'floating') {
      panel.hidden = true
      launcher?.setAttribute('aria-expanded', 'false')
      launcher?.focus()
    }
  }
  if (mode === 'floating') {
    panel.hidden = true
    panel.setAttribute('role', 'dialog')
    panel.setAttribute('aria-label', 'Syntelle character')
    closeButton = document.createElement('button')
    closeButton.className = 'close'
    closeButton.type = 'button'
    closeButton.textContent = '\u00d7'
    closeButton.setAttribute('aria-label', 'Close character')
    closeButton.addEventListener('click', close)
    panel.prepend(closeButton)
    launcher = document.createElement('button')
    launcher.type = 'button'
    launcher.className = 'launcher'
    launcher.textContent = script.dataset.label || 'Start conversation'
    launcher.setAttribute('aria-controls', panel.id)
    launcher.setAttribute('aria-expanded', 'false')
    launcher.addEventListener('click', () => {
      if (!panel.hidden) {
        close()
        return
      }
      panel.hidden = false
      launcher.setAttribute('aria-expanded', 'true')
      closeButton.focus()
    })
    root.append(launcher)
  }
  function receive(event) {
    const data = event.data
    if (
      destroyed ||
      event.source !== iframe.contentWindow ||
      event.origin !== base ||
      data?.channel !== 'syntelle-widget-v1'
    )
      return
    if (data.type === 'ready' && data.widget_id === id && typeof data.nonce === 'string') {
      nonce = data.nonce
      applyTheme(data.theme)
      if (launcher && typeof data.launcher_label === 'string' && data.launcher_label)
        launcher.textContent = data.launcher_label
      if (launcher && data.position === 'bottom-left') {
        launcher.style.right = panel.style.right = 'auto'
        launcher.style.left = panel.style.left = '16px'
      }
      if (data.language === 'es-CO') {
        iframe.title = 'Personaje de Syntelle'
        panel.setAttribute('aria-label', 'Personaje de Syntelle')
        closeButton?.setAttribute('aria-label', 'Cerrar personaje')
      }
      send('init')
    } else if (nonce && data.nonce === nonce && data.type === 'theme-changed') {
      applyTheme(data.theme)
    } else if (nonce && data.nonce === nonce && data.type === 'close-request') {
      close()
    } else if (
      nonce &&
      data.nonce === nonce &&
      ['initialized', 'started', 'stopped', 'error'].includes(data.type)
    ) {
      host.dispatchEvent(
        new CustomEvent(`syntelle:${data.type}`, {
          bubbles: true,
          composed: true,
          detail: { widgetId: id },
        }),
      )
    }
  }
  function escape(event) {
    if (event.key === 'Escape' && !panel.hidden) close()
  }
  function destroy() {
    if (destroyed) return
    send('destroy')
    destroyed = true
    window.removeEventListener('message', receive)
    window.removeEventListener('pagehide', destroy)
    root.removeEventListener('keydown', escape)
    observer.disconnect()
    host.remove()
  }
  const observer = new MutationObserver(() => {
    if (!host.isConnected) destroy()
  })
  observer.observe(document.documentElement, { childList: true, subtree: true })
  host.addEventListener('syntelle:destroy', destroy, { once: true })
  window.addEventListener('message', receive)
  window.addEventListener('pagehide', destroy, { once: true })
  root.addEventListener('keydown', escape)
})()
