<script setup lang="ts">
import { useLocaleStore } from '../../stores/locale'
import { computed, onMounted, onScopeDispose, shallowRef, useTemplateRef, watch } from 'vue'
import ChatPanel from '../conversation/components/ChatPanel.vue'
import ConversationIcon from '../conversation/components/ConversationIcon.vue'
import RemotePlayer from './components/RemotePlayer.vue'
import AccountDialog from './components/AccountDialog.vue'
import StreamSettingsPanel from './components/StreamSettings.vue'
import { useWorkspaceViewport } from '../conversation/composables/useWorkspaceViewport'
import { useAccount } from './composables/useAccount'
import { useRemoteStream } from './composables/useRemoteStream'
import { AVA_VOICE, defaultSettings, type SavedConfiguration, type StreamSettings } from './types'

const locale = useLocaleStore()
const { t } = locale
const settings = shallowRef<StreamSettings>(defaultSettings())
const workspace = useTemplateRef<HTMLElement>('workspace')
const customizeButton = useTemplateRef<HTMLButtonElement>('customizeButton')
const { viewportStyle, keyboardOpen } = useWorkspaceViewport(workspace)
const accountOpen = shallowRef(false)
const emit = defineEmits<{ accountChange: [signedIn: boolean] }>()
function openAccount() {
  accountOpen.value = true
}
defineExpose({ openAccount })
const settingsOpen = shallowRef(false)
const settingsDialog = useTemplateRef<HTMLDialogElement>('settingsDialog')
const player = useTemplateRef<InstanceType<typeof RemotePlayer>>('player')
const mediaControls = useTemplateRef<HTMLElement>('mediaControls')
const query = new URLSearchParams(location.search)
const flow = query.get('flow_id') ?? undefined
const reducedMotion = matchMedia('(prefers-reduced-motion: reduce)')
try {
  const previous = JSON.parse(localStorage.getItem('syntelle.stream.settings') ?? 'null')
  if (previous?.character === 'syntelle')
    settings.value = { ...defaultSettings(), ...previous, voice: AVA_VOICE }
  else {
    // Carry the owner's existing appearance/instructions into the unified player.
    const appearance = JSON.parse(localStorage.getItem('syntelle.demo.appearance') ?? 'null')
    const background = JSON.parse(localStorage.getItem('syntelle.motion.background') ?? 'null')
    settings.value = {
      ...defaultSettings(),
      blinking: appearance?.blinking !== false,
      movement: appearance?.movement !== false,
      automatic_expressions: appearance?.automaticExpressions !== false,
      ...(['light-studio', 'dark-lounge', 'natural-space'].includes(background?.id)
        ? {
            background: background.id,
            blur: Math.max(0, Math.min(20, Number(background.blur) || 0)),
          }
        : {}),
      instructions:
        localStorage.getItem('syntelle.instructions.syntelle') ?? defaultSettings().instructions,
    }
  }
} catch {
  /* Local preferences are optional. */
}
settings.value = {
  ...settings.value,
  language: locale.speechLanguage,
  reduced_motion: reducedMotion.matches,
}
const auth = useAccount()
watch(auth.account, (account) => emit('accountChange', Boolean(account)), { immediate: true })
const remote = useRemoteStream(() => settings.value)
const microphoneStartupPending = shallowRef(false)
const canStartMicrophone = computed(() =>
  ['not-started', 'disconnected', 'paused', 'ended'].includes(remote.state.value),
)
watch(remote.ready, (ready) => {
  if (!ready || !microphoneStartupPending.value) return
  microphoneStartupPending.value = false
  void remote.toggleMicrophone()
})
watch([remote.connecting, remote.state, remote.error], () => {
  if (!microphoneStartupPending.value || remote.ready.value) return
  const failed =
    Boolean(remote.error.value) ||
    (!remote.connecting.value &&
      ['disconnected', 'paused', 'ended', 'exhausted'].includes(remote.state.value))
  if (failed) microphoneStartupPending.value = false
})
const controlsLocked = computed(
  () =>
    remote.connecting.value ||
    remote.speaking.value ||
    remote.pending.value ||
    remote.microphone.value ||
    remote.startingMicrophone.value,
)
let configureTimer: ReturnType<typeof setTimeout> | undefined
watch(
  () => locale.speechLanguage,
  (language) => {
    settings.value = { ...settings.value, language }
  },
  { flush: 'sync' },
)
watch(settings, (value, previous) => {
  try {
    localStorage.setItem('syntelle.stream.settings', JSON.stringify(value))
  } catch {
    /* Optional. */
  }
  if (previous.language !== value.language) {
    microphoneStartupPending.value = false
    clearTimeout(configureTimer)
    remote.stopMicrophone()
    void remote.stop()
    return
  }
  if (!remote.id.value) return
  const { background: _background, blur: _blur, ...character } = value
  const { background: _previousBackground, blur: _previousBlur, ...previousCharacter } = previous
  if (JSON.stringify(character) === JSON.stringify(previousCharacter)) return
  clearTimeout(configureTimer)
  configureTimer = setTimeout(() => remote.command('configure', '', value), 150)
})
watch([settingsOpen, settingsDialog], ([open, element]) => {
  if (!element) return
  if (open && !element.open) element.show()
  else if (!open && element.open) {
    element.close()
    customizeButton.value?.focus()
  }
})
function closeSettingsOnEscape(event: KeyboardEvent) {
  if (
    !settingsOpen.value ||
    event.key !== 'Escape' ||
    event.defaultPrevented ||
    event.target instanceof HTMLSelectElement
  )
    return
  event.preventDefault()
  settingsOpen.value = false
}
function preferenceChanged() {
  settings.value = { ...settings.value, reduced_motion: reducedMotion.matches }
}
reducedMotion.addEventListener('change', preferenceChanged)
async function beforeLogin() {
  microphoneStartupPending.value = false
  clearTimeout(configureTimer)
  await remote.stop()
}
async function reconnect() {
  if (remote.state.value === 'ended') remote.turns.value = []
  await auth.refresh()
  await remote.connect()
}
async function signedIn() {
  microphoneStartupPending.value = false
  await remote.stop()
  await auth.refresh()
}
async function load(configuration: SavedConfiguration) {
  microphoneStartupPending.value = false
  await remote.stop()
  settings.value = {
    ...defaultSettings(),
    ...configuration.settings,
    language: locale.speechLanguage,
    voice: AVA_VOICE,
    reduced_motion: reducedMotion.matches,
  }
}
async function previewExpression(kind: string, amount = 1, repetitions = 1) {
  if (kind === 'laugh') await player.value?.enableAudio()
  remote.command('preview', kind, undefined, amount, repetitions)
}
async function microphone() {
  await player.value?.enableAudio()
  if (remote.microphone.value || remote.startingMicrophone.value) {
    await remote.toggleMicrophone()
    return
  }
  if (!remote.ready.value) {
    if (remote.connecting.value || microphoneStartupPending.value) return
    microphoneStartupPending.value = true
    await reconnect()
    if (!remote.ready.value && !remote.connecting.value) microphoneStartupPending.value = false
    return
  }
  await remote.toggleMicrophone()
}
async function newChat() {
  microphoneStartupPending.value = false
  await remote.stop()
  remote.turns.value = []
  remote.partial.value = ''
}
onMounted(async () => {
  if (query.has('auth') || flow) accountOpen.value = true
  await auth.refresh()
  await remote.loadPolicy()
  if (query.has('auth_error')) {
    accountOpen.value = true
    auth.error.value = 'Sign-in could not finish. Try again or continue as a guest.'
  }
})
onScopeDispose(() => {
  clearTimeout(configureTimer)
  reducedMotion.removeEventListener('change', preferenceChanged)
})
</script>

<template>
  <div
    ref="workspace"
    class="stream-workspace"
    :class="{ 'keyboard-open': keyboardOpen }"
    :style="viewportStyle"
    data-testid="stream-workspace"
    :data-stream-id="remote.id.value"
    @keydown="closeSettingsOnEscape"
  >
    <div class="stream-panels">
      <div class="stage-column">
        <RemotePlayer
          ref="player"
          :media="remote.media.value"
          :ready="remote.ready.value"
          :suppressed="remote.audioSuppressed.value"
          :background="settings.background"
          :blur="settings.blur"
          :controls-target="mediaControls"
        />
        <div
          v-if="!remote.ready.value"
          class="connection-state"
          :class="{
            'connection-entry':
              ['not-started', 'disconnected', 'paused', 'ended'].includes(remote.state.value) ||
              remote.connecting.value,
            'connection-connecting': remote.connecting.value || microphoneStartupPending,
            'connection-paused': remote.state.value === 'paused',
          }"
        >
          <p v-if="remote.state.value !== 'not-started'" class="connection-status" role="status">
            {{ t(remote.status.value) }}
          </p>
          <p v-else-if="microphoneStartupPending" class="connection-status" role="status">
            {{ t('Starting microphone…') }}
          </p>
          <button
            v-if="
              ['disconnected', 'paused', 'ended'].includes(remote.state.value) &&
              !remote.connecting.value
            "
            type="button"
            class="entry-action"
            @click="reconnect"
          >
            <ConversationIcon name="play" />
            {{
              t(
                remote.state.value === 'paused'
                  ? 'Resume'
                  : remote.state.value === 'disconnected'
                    ? 'Reconnect'
                    : 'Start new session',
              )
            }}
          </button>
          <p v-if="remote.timeLimitsEnabled.value" class="session-note">
            {{ t('Up to 5 minutes per session. 15 minutes total.') }}
          </p>
        </div>
      </div>
      <ChatPanel
        class="conversation-panel"
        :turns="remote.turns.value"
        :partial="remote.partial.value"
        character="Syntelle"
        :configured="remote.configured.value"
        :can-start-microphone="canStartMicrophone"
        :not-started="remote.state.value === 'not-started'"
        :checking="remote.connecting.value || microphoneStartupPending"
        :ready="remote.ready.value"
        :pending="remote.pending.value"
        :speaking="remote.speaking.value"
        :microphone="remote.microphone.value"
        :input-level="remote.inputLevel.value"
        :starting-microphone="remote.startingMicrophone.value || microphoneStartupPending"
        :recognition-available="remote.recognition.value"
        :status="remote.status.value"
        :error="remote.error.value"
        :connection-message="
          t(
            remote.state.value === 'paused'
              ? 'Select Resume to continue your conversation.'
              : remote.state.value === 'not-started' || remote.state.value === 'ended'
                ? 'Turn on your microphone to start talking.'
                : remote.state.value === 'exhausted'
                  ? 'Your 15-minute demo allowance has been used.'
                  : 'Select Reconnect to restore video and voice controls.',
          )
        "
        @microphone="microphone"
        @stop="remote.interrupt"
        @new-chat="newChat"
      >
        <template #media-actions>
          <div ref="mediaControls" />
        </template>
        <template #actions>
          <button
            ref="customizeButton"
            class="quiet-action icon-action"
            type="button"
            :aria-label="t('Customize')"
            :title="t('Customize')"
            aria-haspopup="dialog"
            :aria-expanded="settingsOpen"
            @click="settingsOpen = true"
          >
            <ConversationIcon name="adjust" />
          </button>
        </template>
      </ChatPanel>
    </div>
    <AccountDialog
      v-model="accountOpen"
      :auth="auth"
      :settings="settings"
      :before-login="beforeLogin"
      :flow="flow"
      @load="load"
      @signed-in="signedIn"
      @signed-out="signedIn"
      @resume-stream="signedIn"
    />
    <dialog
      ref="settingsDialog"
      class="settings-dialog"
      aria-labelledby="stream-customize-title"
      @close="settingsOpen = false"
    >
      <header>
        <h2 id="stream-customize-title">{{ t('Customize Syntelle') }}</h2>
        <button
          class="quiet-action icon-action"
          type="button"
          :aria-label="t('Close')"
          :title="t('Close')"
          autofocus
          @click="settingsOpen = false"
        >
          <ConversationIcon name="close" />
        </button>
      </header>

      <StreamSettingsPanel
        v-model="settings"
        :disabled="controlsLocked"
        :preview-disabled="!remote.ready.value"
        @preview="previewExpression"
      />
    </dialog>
  </div>
</template>

<style scoped>
.stream-workspace {
  display: grid;
  grid-template-rows: minmax(0, 1fr);
  height: 100%;
  min-height: 0;
}
.workspace-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 44px;
  border: 0;
  padding: 8px 10px;
  background: transparent;
  color: var(--text);
  font-size: 13px;
}
.workspace-action svg {
  width: 18px;
  height: 18px;
}
.quiet-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 44px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
}
.icon-action {
  width: 44px;
  padding: 0;
}
.icon-action :deep(.conversation-icon) {
  width: 18px;
  height: 18px;
}
.quiet-action:disabled {
  opacity: 0.45;
  cursor: default;
}
@media (hover: hover) {
  .quiet-action:enabled:hover {
    color: var(--text);
    background: var(--subtle);
  }
}
@media (prefers-reduced-motion: no-preference) {
  .quiet-action {
    transition:
      color 150ms ease-out,
      background-color 150ms ease-out,
      transform 150ms ease-out;
  }
  .quiet-action:enabled:active {
    transform: scale(0.96);
  }
}
.stream-panels {
  display: grid;
  grid-template-columns: minmax(0, 2fr) minmax(0, 3fr);
  min-height: 0;
  gap: 32px;
}
.stage-column {
  position: relative;
  display: grid;
  min-height: 0;
}
.connection-state {
  position: absolute;
  z-index: 2;
  inset: auto 0 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 20px;
  background: #19181beb;
  border-radius: 0 0 16px 16px;
  pointer-events: none;
}
.connection-status {
  margin: 0;
  color: #d4d0da;
  font-size: 14px;
  text-align: center;
}
.connection-state .button {
  pointer-events: auto;
}
.connection-state.connection-entry {
  padding: 24px;
  background: transparent;
}
.connection-entry .connection-status {
  padding: 10px 14px;
  border-radius: 8px;
  background: #26232c;
  font-size: 13px;
}
.entry-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 48px;
  max-width: 100%;
  padding: 12px 18px 12px 16px;
  border: 1px solid #857b94;
  border-radius: 8px;
  color: #f3f0f7;
  background: #26232c;
  box-shadow: 0 2px 8px #0000001a;
  font: inherit;
  font-size: 14px;
  font-weight: 600;
  line-height: 20px;
  cursor: pointer;
  pointer-events: auto;
  -webkit-tap-highlight-color: transparent;
  transition: background-color 150ms ease-out;
}
.entry-action :deep(.conversation-icon) {
  color: #bba3f5;
}
.entry-action:focus-visible {
  outline: 2px solid #cbb8fa;
  outline-offset: 4px;
}
.entry-action:active {
  background: #3c3549;
}
@media (hover: hover) {
  .entry-action:hover {
    background: #35303f;
  }
}
.connection-entry .session-note {
  padding: 6px 10px;
  border-radius: 6px;
  color: #d4d0da;
  background: #26232c;
}
.session-note {
  margin: 0;
  color: #b7b2bf;
  font-size: 12px;
  text-align: center;
}
.conversation-panel {
  display: flex;
  flex-direction: column;
  min-height: 0;
  min-width: 0;
  background: var(--conversation-surface);
  border: 1px solid var(--divider);
  border-radius: 16px;
  box-shadow: var(--panel-shadow);
  overflow: hidden;
}
.settings-dialog {
  position: fixed;
  inset: 0 0 0 auto;
  margin: 0;
  width: min(440px, 100%);
  height: 100dvh;
  max-height: none;
  max-width: 100%;
  padding: 24px;
  color: var(--text);
  background: var(--surface);
  border: 0;
  border-left: 1px solid var(--divider);
  overflow-y: auto;
  overscroll-behavior: contain;
}
.settings-dialog header {
  position: sticky;
  top: -24px;
  background: var(--surface);
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin: -24px -24px 24px;
  padding: 16px 24px;
  border-bottom: 1px solid var(--divider);
}
.settings-dialog h2 {
  font-size: 20px;
  margin: 0;
}
@media (max-width: 899px) {
  .connection-state {
    inset: 0 0 0 auto;
    width: 50%;
    padding: 12px;
    border-radius: 0 16px 16px 0;
    background: #19181bb3;
  }
  .connection-state.connection-entry {
    padding: 12px;
  }
  .entry-action {
    padding: 10px 12px 10px 10px;
    font-size: 13px;
  }
  .settings-dialog {
    inset: auto 0 0;
    width: 100%;
    height: auto;
    max-height: 60dvh;
    border-left: 0;
    border-top: 1px solid var(--divider);
    border-radius: 12px 12px 0 0;
    padding-bottom: max(24px, env(safe-area-inset-bottom));
  }
  .stream-workspace {
    height: var(--demo-viewport-height, 100%);
  }
  .keyboard-open .stage-column {
    display: none;
  }
  .keyboard-open .stream-panels {
    grid-template-rows: minmax(0, 1fr);
  }
  .stream-panels {
    grid-template-columns: minmax(0, 1fr);
    grid-template-rows: clamp(160px, 26svh, 240px) minmax(0, 1fr);
    gap: 16px;
  }
  .conversation-panel :deep(.conversation-controls) {
    padding: 12px 14px max(10px, env(safe-area-inset-bottom));
  }
  .conversation-panel :deep(.transcript) {
    padding: 20px;
  }
  .conversation-panel :deep(.empty-title) {
    font-size: 20px;
  }
}
</style>
