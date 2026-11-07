<script setup lang="ts">
import { computed, onMounted, onScopeDispose, shallowRef, useTemplateRef, watch } from 'vue'
import { useLocaleStore } from '../../stores/locale'
import RemotePlayer from '../streaming/components/RemotePlayer.vue'
import { useRemoteStream } from '../streaming/composables/useRemoteStream'
import { defaultSettings } from '../streaming/types'
import type { RemoteSession } from '../streaming/transport'
import { verifyVisitor } from './challenge'
import { createWidgetTransport, widgetRequest } from './transport'
import {
  acceptsParentMessage,
  WIDGET_CHANNEL,
  type WidgetBootstrap,
  type WidgetMetadata,
} from './types'

const props = defineProps<{ widgetId: string }>()
const locale = useLocaleStore()
const { t } = locale
const metadata = shallowRef<WidgetMetadata | null>(null)
const failure = shallowRef('')
const parentOrigin = shallowRef('')
const checking = shallowRef(false)
const floating = shallowRef(false)
const draft = shallowRef('')
const controls = useTemplateRef<HTMLElement>('controls')
const challenge = useTemplateRef<HTMLElement>('challenge')
const player = useTemplateRef<InstanceType<typeof RemotePlayer>>('player')
const handshakeNonce = crypto.randomUUID()
let disposed = false
let startup: AbortController | undefined
let readyTimer: ReturnType<typeof setInterval> | undefined
const path = `/api/v1/widgets/${encodeURIComponent(props.widgetId)}`
const settings = computed(() => ({
  ...defaultSettings(),
  instructions: '',
  language: metadata.value?.appearance.language ?? 'en-US',
  background: metadata.value?.player.background ?? 'natural-space',
  blur: metadata.value?.player.blur ?? 0,
}))
function tellParent(type: string, extra: Record<string, unknown> = {}) {
  if (parentOrigin.value)
    window.parent.postMessage(
      { channel: WIDGET_CHANNEL, type, nonce: handshakeNonce, ...extra },
      parentOrigin.value,
    )
}
const transport = createWidgetTransport(async () => {
  if (!parentOrigin.value || !metadata.value?.available || !challenge.value || !startup)
    throw Error('This character is currently unavailable.')
  const signal = startup.signal
  const bootstrap = await widgetRequest<WidgetBootstrap>(`${path}/bootstrap`, 'POST', {
    parent_origin: parentOrigin.value,
  })
  if (signal.aborted) throw new DOMException('Cancelled', 'AbortError')
  const token = await verifyVisitor(
    challenge.value,
    metadata.value.turnstile_site_key,
    bootstrap.nonce,
    signal,
  )
  if (signal.aborted) throw new DOMException('Cancelled', 'AbortError')
  return widgetRequest<RemoteSession & { capability: string }>(`${path}/sessions`, 'POST', {
    bootstrap_id: bootstrap.bootstrap_id,
    nonce: bootstrap.nonce,
    parent_origin: parentOrigin.value,
    turnstile_token: token,
    request_id: crypto.randomUUID(),
  })
})
const remote = useRemoteStream(() => settings.value, transport)
const busy = computed(() => checking.value || remote.connecting.value)
const canStart = computed(() =>
  Boolean(metadata.value?.available && parentOrigin.value && !busy.value),
)
const live = computed(() => remote.ready.value || remote.connecting.value)
const systemTheme = matchMedia('(prefers-color-scheme: dark)')
function applyTheme() {
  const choice = metadata.value?.appearance.theme ?? 'system'
  document.documentElement.dataset.theme =
    choice === 'system' ? (systemTheme.matches ? 'dark' : 'light') : choice
}
function stop() {
  startup?.abort()
  startup = undefined
  checking.value = false
  player.value?.suspendPresentation()
  void remote.stop()
  tellParent('stopped')
}
async function start() {
  if (!canStart.value) return
  failure.value = ''
  startup?.abort()
  const attempt = new AbortController()
  startup = attempt
  checking.value = true
  player.value?.activatePresentation()
  remote.activatePresentation()
  // Capture the user's sound activation before awaiting verification/network.
  void player.value?.enableAudio()
  try {
    await remote.connect()
  } finally {
    if (startup === attempt) checking.value = false
  }
}
function receive(event: MessageEvent) {
  const data = metadata.value
  if (!data || !acceptsParentMessage(event, window.parent, data.allowed_origins, handshakeNonce))
    return
  if (parentOrigin.value && parentOrigin.value !== event.origin) return
  if (event.data.type === 'init') {
    parentOrigin.value = event.origin
    floating.value = event.data.placement === 'floating'
    clearInterval(readyTimer)
    tellParent('initialized')
  } else if (parentOrigin.value && (event.data.type === 'close' || event.data.type === 'destroy'))
    stop()
}
function announce() {
  if (!disposed && metadata.value && !parentOrigin.value)
    window.parent.postMessage(
      {
        channel: WIDGET_CHANNEL,
        type: 'ready',
        nonce: handshakeNonce,
        widget_id: props.widgetId,
        launcher_label: metadata.value.appearance.launcher_label,
        position: metadata.value.appearance.position,
        language: metadata.value.appearance.language,
        theme: document.documentElement.dataset.theme,
      },
      '*',
    ) // Public handshake only; never credentials, transcript or private configuration.
}
function escape(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    stop()
    tellParent('close-request')
  }
}
function submit() {
  const text = draft.value.trim()
  if (text && remote.ready.value && remote.command('message', text)) draft.value = ''
}
watch(remote.ready, (ready) => {
  if (ready) tellParent('started')
})
watch(remote.error, (error) => {
  if (error) tellParent('error', { message: 'The conversation needs attention.' })
})
onMounted(async () => {
  window.addEventListener('message', receive)
  window.addEventListener('pagehide', stop)
  window.addEventListener('keydown', escape)
  systemTheme.addEventListener('change', applyTheme)
  applyTheme()
  if (window.parent === window) {
    failure.value = 'Install this widget on an allowed website to use it.'
    return
  }
  try {
    const data = await widgetRequest<WidgetMetadata>(path)
    if (disposed) return
    metadata.value = data
    locale.locale = data.appearance.language === 'es-CO' ? 'es' : 'en'
    applyTheme()
    announce()
    readyTimer = setInterval(announce, 1000)
  } catch (error) {
    if (!disposed)
      failure.value =
        error instanceof Error ? error.message : 'This character is currently unavailable.'
  }
})
onScopeDispose(() => {
  disposed = true
  clearInterval(readyTimer)
  window.removeEventListener('message', receive)
  window.removeEventListener('pagehide', stop)
  window.removeEventListener('keydown', escape)
  systemTheme.removeEventListener('change', applyTheme)
  stop()
})
</script>

<template>
  <main class="widget-host">
    <header class="widget-heading" :class="{ floating }">
      <div>
        <span class="live-dot" :class="{ active: remote.ready.value }" />{{
          metadata?.player.character_name || metadata?.name || 'Syntelle'
        }}
      </div>
      <button v-if="live && !floating" type="button" class="end-action" @click="stop">
        {{ t('End conversation') }}
      </button>
    </header>
    <div class="widget-stage">
      <RemotePlayer
        ref="player"
        :media="remote.media.value"
        :ready="remote.ready.value"
        :suppressed="remote.audioSuppressed.value"
        :still="false"
        :background="settings.background"
        :blur="settings.blur"
        :controls-target="controls"
        :poster="metadata?.player.poster || null"
        :character-name="metadata?.player.character_name"
      />
      <div v-if="!remote.ready.value" class="start-panel">
        <template v-if="metadata && !failure">
          <h1>{{ metadata.name }}</h1>
          <p>
            {{
              t(
                metadata.available
                  ? 'A conversation, right here.'
                  : metadata.reason || 'This character is currently unavailable.',
              )
            }}
          </p>
          <button
            v-if="metadata.available"
            class="start-action"
            type="button"
            :disabled="!canStart"
            @click="start"
          >
            {{
              t(busy ? 'Connecting…' : metadata.appearance.launcher_label || 'Start conversation')
            }}
          </button>
          <p v-if="!parentOrigin" class="small-note">{{ t('Checking this website…') }}</p>
        </template>
        <p v-else role="status">{{ t(failure || 'Loading character…') }}</p>
      </div>
    </div>
    <div ref="challenge" class="challenge" :class="{ visible: checking }" />
    <p v-if="remote.error.value" class="widget-error" role="alert">{{ t(remote.error.value) }}</p>
    <section v-if="remote.ready.value" class="conversation-tools">
      <p class="status" role="status">{{ t(remote.status.value) }}</p>
      <p v-if="remote.partial.value" class="partial">{{ remote.partial.value }}</p>
      <div class="tool-row">
        <button
          v-if="remote.recognition.value"
          type="button"
          class="mic-action"
          :aria-pressed="remote.microphone.value"
          :disabled="remote.startingMicrophone.value"
          @click="remote.toggleMicrophone"
        >
          {{ t(remote.microphone.value ? 'Turn off microphone' : 'Turn on microphone') }}
        </button>
        <button
          v-if="remote.speaking.value || remote.pending.value"
          type="button"
          class="end-action"
          @click="remote.interrupt"
        >
          {{ t('Interrupt') }}
        </button>
      </div>
      <form class="message-form" @submit.prevent="submit">
        <input
          v-model="draft"
          :aria-label="t('Message')"
          :placeholder="t('Type a message…')"
          maxlength="2000"
        />
        <button type="submit" :disabled="!draft.trim()">{{ t('Send') }}</button>
      </form>
      <details v-if="remote.turns.value.length" class="transcript">
        <summary>{{ t('Transcript') }}</summary>
        <div class="turns" aria-live="polite">
          <div v-for="turn in remote.turns.value" :key="turn.id">
            <p>
              <strong>{{ t('You') }}:</strong> {{ turn.user }}
            </p>
            <p>{{ turn.reply }}</p>
          </div>
        </div>
      </details>
    </section>
    <footer class="widget-footer">
      <span>Powered by <strong>Syntelle</strong></span>
      <div ref="controls" class="player-controls" />
    </footer>
  </main>
</template>

<style scoped>
.widget-host {
  height: 100dvh;
  min-height: 320px;
  display: flex;
  flex-direction: column;
  background: var(--surface);
  color: var(--text);
  overflow: auto;
}
.widget-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 56px;
  padding: 8px 20px;
  font-size: 14px;
  font-weight: 600;
  border-bottom: 1px solid var(--divider);
}
.widget-heading > div {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.widget-heading.floating {
  padding-right: 60px;
}
.live-dot {
  width: 7px;
  height: 7px;
  flex: none;
  border-radius: 50%;
  background: var(--muted);
}
.live-dot.active {
  background: var(--accent);
}
.widget-stage {
  position: relative;
  flex: 1;
  min-height: 240px;
  display: grid;
  overflow: hidden;
}
.widget-stage :deep(.remote-stage) {
  height: 100%;
  min-height: 240px;
  border: 0;
  border-radius: 0;
}
.start-panel {
  position: absolute;
  z-index: 3;
  inset: auto 16px 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  text-align: center;
  padding: 18px;
  gap: 10px;
  border-radius: 14px;
  background: color-mix(in srgb, var(--surface) 96%, transparent);
  box-shadow: 0 4px 20px #00000016;
}
.start-panel h1 {
  font-size: 24px;
  letter-spacing: -0.03em;
  margin: 0;
}
.start-panel p {
  margin: 0;
  max-width: 28ch;
  color: var(--muted);
  font-size: 14px;
}
.start-action,
.mic-action,
.message-form button {
  border: 1px solid transparent;
  border-radius: 10px;
  background: var(--accent);
  color: var(--on-accent);
  min-height: 44px;
  padding: 10px 18px;
  font-size: 14px;
  font-weight: 600;
}
.start-action:disabled {
  opacity: 0.5;
}
.small-note {
  font-size: 12px !important;
}
.end-action {
  background: none;
  border: 0;
  color: var(--muted);
  font-size: 13px;
  min-height: 44px;
  padding: 8px;
}
.widget-error {
  padding: 12px 20px;
  margin: 0;
  color: var(--error);
  font-size: 13px;
}
.challenge {
  display: none;
}
.challenge.visible {
  display: grid;
  place-items: center;
  padding: 8px;
  background: var(--surface);
}
.conversation-tools {
  padding: 14px 20px 4px;
}
.status {
  font-size: 12px;
  color: var(--muted);
  margin: 0 0 8px;
}
.partial {
  font-size: 13px;
}
.tool-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}
.message-form {
  display: flex;
  gap: 8px;
}
.message-form input {
  flex: 1;
  min-width: 0;
  background: var(--page);
  color: var(--text);
  border: 1px solid var(--control-border);
  border-radius: 10px;
  padding: 10px 12px;
  font-size: 14px;
}
.message-form button {
  padding: 10px 14px;
}
.message-form button:disabled {
  opacity: 0.5;
}
.transcript {
  font-size: 13px;
  color: var(--muted);
  margin-top: 8px;
}
.transcript summary {
  min-height: 40px;
  align-content: center;
  cursor: pointer;
}
.turns {
  max-height: 180px;
  overflow: auto;
  overscroll-behavior: contain;
}
.turns p {
  margin: 0 0 10px;
  white-space: pre-wrap;
}
.widget-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  min-height: 48px;
  padding: 4px 20px;
  color: var(--muted);
  font-size: 11px;
}
.widget-footer strong {
  font-weight: 600;
  color: var(--text);
}
.player-controls {
  display: flex;
  align-items: center;
}
.player-controls :deep(button) {
  min-height: 40px;
  min-width: 40px;
}
button:focus-visible,
input:focus-visible,
summary:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 3px;
}
@media (max-height: 540px) {
  .widget-stage,
  .widget-stage :deep(.remote-stage) {
    min-height: 180px;
  }
  .conversation-tools {
    padding-top: 8px;
  }
  .widget-host {
    min-height: 0;
  }
}
</style>
