<script setup lang="ts">
import { useLocaleStore } from '../../stores/locale'
import {
  computed,
  nextTick,
  onMounted,
  onActivated,
  onDeactivated,
  onScopeDispose,
  shallowRef,
  useTemplateRef,
  watch,
} from 'vue'
import ChatPanel from '../conversation/components/ChatPanel.vue'
import ConversationIcon from '../conversation/components/ConversationIcon.vue'
import RemotePlayer from './components/RemotePlayer.vue'
import StreamSettingsPanel from './components/StreamSettings.vue'
import { useWorkspaceViewport } from '../conversation/composables/useWorkspaceViewport'
import type { useAccount } from './composables/useAccount'
import { useRemoteStream } from './composables/useRemoteStream'
import {
  DEFAULT_VOICE,
  defaultSettings,
  type SavedConfiguration,
  type StreamSettings,
} from './types'
import type { ConversationTurn } from '../conversation/types'
import { isCharacterID } from '../characters/api'
import { useSelectedCharacter } from '../characters/useSelectedCharacter'
import type { HistoryItem } from '../history/useConversationHistory'

const props = withDefaults(
  defineProps<{ characterId?: string; auth: ReturnType<typeof useAccount> }>(),
  { characterId: 'syntelle' },
)
const auth = props.auth
const settings = defineModel<StreamSettings>({ required: true })
const emit = defineEmits<{
  navigate: [event: MouseEvent]
  conversationChanged: [id: string | null]
  conversationUpdated: [id: string]
  newConversation: []
}>()
const active = shallowRef(true)
let actionEpoch = 0
const locale = useLocaleStore()
const { t } = locale
const workspace = useTemplateRef<HTMLElement>('workspace')
const customizeButton = useTemplateRef<HTMLButtonElement>('customizeButton')
const { viewportStyle, keyboardOpen } = useWorkspaceViewport(workspace)
const settingsOpen = shallowRef(false)
const settingsDialog = useTemplateRef<HTMLDialogElement>('settingsDialog')
const customizeHeading = useTemplateRef<HTMLElement>('customizeHeading')
const player = useTemplateRef<InstanceType<typeof RemotePlayer>>('player')
const mediaControls = useTemplateRef<HTMLElement>('mediaControls')
const reducedMotion = matchMedia('(prefers-reduced-motion: reduce)')
const invalidCharacter = props.characterId !== 'syntelle' && !isCharacterID(props.characterId)
const selectedCharacter = useSelectedCharacter(() => settings.value.character)
const remote = useRemoteStream(() => settings.value)
const historyPreview = shallowRef<Pick<HistoryItem, 'id' | 'language' | 'settings'> | null>(null)
watch(remote.conversationId, (id) => emit('conversationChanged', id || null))
watch(remote.conversationRevision, () => {
  if (remote.conversationId.value) emit('conversationUpdated', remote.conversationId.value)
})
const microphoneStartupPending = shallowRef(false)
const queuedPreview = shallowRef<{ kind: string; amount: number; repetitions: number } | null>(null)
const canStartMicrophone = computed(
  () =>
    !invalidCharacter &&
    selectedCharacter.ready.value &&
    (!historyPreview.value || Boolean(historyPreview.value.settings)) &&
    !remote.suspending.value &&
    ['not-started', 'disconnected', 'paused', 'ended'].includes(remote.state.value),
)
watch(remote.ready, (ready) => {
  if (!active.value || !ready || !microphoneStartupPending.value) return
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
const previewDisabled = computed(
  () =>
    !selectedCharacter.ready.value ||
    invalidCharacter ||
    Boolean(queuedPreview.value) ||
    remote.pending.value ||
    remote.speaking.value ||
    remote.startingMicrophone.value ||
    remote.state.value === 'exhausted',
)
watch([remote.ready, remote.connecting, remote.error, remote.state], () => {
  const preview = queuedPreview.value
  if (!active.value || !preview) return
  if (remote.ready.value) {
    queuedPreview.value = null
    remote.command('preview', preview.kind, undefined, preview.amount, preview.repetitions)
  } else if (
    remote.error.value ||
    (!remote.connecting.value &&
      ['disconnected', 'paused', 'ended', 'exhausted'].includes(remote.state.value))
  ) {
    queuedPreview.value = null
  }
})
let configureTimer: ReturnType<typeof setTimeout> | undefined
watch(settings, (value, previous) => {
  if (previous.language !== value.language) {
    microphoneStartupPending.value = false
    clearTimeout(configureTimer)
    remote.stopMicrophone()
    // Preserve a resumable handle: the server will require the original
    // language or an explicit new conversation, rather than losing context.
    if (remote.state.value === 'paused' || remote.suspending.value) return
    void remote.stop()
    return
  }
  if (!active.value || !remote.id.value || remote.suspending.value) return
  const { background: _background, blur: _blur, ...character } = value
  const { background: _previousBackground, blur: _previousBlur, ...previousCharacter } = previous
  if (JSON.stringify(character) === JSON.stringify(previousCharacter)) return
  clearTimeout(configureTimer)
  configureTimer = setTimeout(() => {
    if (active.value && !remote.suspending.value) remote.command('configure', '', value)
  }, 150)
})
watch([settingsOpen, settingsDialog], async ([open, element], [wasOpen]) => {
  if (!element) return
  if (open) {
    if (!element.open) element.show()
    await nextTick()
    if (settingsOpen.value) customizeHeading.value?.focus()
  } else if (wasOpen) {
    if (element.open) element.close()
    await nextTick()
    if (active.value) customizeButton.value?.focus()
  }
})
function closeSettingsOnEscape(event: KeyboardEvent) {
  const nativeMenuOpen =
    event.target instanceof HTMLSelectElement &&
    CSS.supports('selector(select:open)') &&
    event.target.matches(':open')
  if (
    !settingsOpen.value ||
    event.key !== 'Escape' ||
    event.defaultPrevented ||
    [...document.querySelectorAll('dialog[open]')].some((el) => el !== settingsDialog.value) ||
    nativeMenuOpen
  )
    return
  event.preventDefault()
  settingsOpen.value = false
}
async function stopForAuth() {
  actionEpoch++
  microphoneStartupPending.value = false
  queuedPreview.value = null
  clearTimeout(configureTimer)
  await remote.stop()
}
async function prepareHistorySelection() {
  actionEpoch++
  microphoneStartupPending.value = false
  queuedPreview.value = null
  clearTimeout(configureTimer)
  settingsOpen.value = false
  settingsDialog.value?.close()
  player.value?.suspendPresentation()
  const paused = await remote.suspendNavigation()
  // This action remains in the same mounted chat. Restore its ability to
  // respond to a later explicit Continue, with the paused media detached.
  await nextTick()
  if (active.value && !remote.suspending.value) {
    remote.activatePresentation()
    player.value?.activatePresentation()
  }
  return paused
}
async function reconnect() {
  if (!active.value || invalidCharacter || remote.suspending.value) return
  if (historyPreview.value && !historyPreview.value.settings) return
  const action = ++actionEpoch
  if (remote.state.value === 'ended') remote.turns.value = []
  await auth.refresh()
  if (!active.value || action !== actionEpoch) return
  if (!(await selectedCharacter.refresh()) || !active.value || action !== actionEpoch) return
  await remote.connect(false, historyPreview.value?.id ?? '')
  if (active.value && action === actionEpoch && remote.id.value) historyPreview.value = null
}
async function previewHistory(
  item: { id: string; language: string; settings?: StreamSettings },
  turns: ConversationTurn[],
) {
  if (!active.value) return false
  const action = ++actionEpoch
  microphoneStartupPending.value = false
  queuedPreview.value = null
  clearTimeout(configureTimer)
  settingsOpen.value = false
  await remote.stop()
  if (!active.value || action !== actionEpoch) return false
  if (item.settings) {
    locale.locale = item.language === 'es-CO' ? 'es' : 'en'
    settings.value = {
      ...defaultSettings(),
      ...item.settings,
      language: locale.speechLanguage,
      voice: DEFAULT_VOICE,
      reduced_motion: reducedMotion.matches,
    }
  }
  await nextTick()
  if (!active.value || action !== actionEpoch) return false
  await remote.stop()
  if (!active.value || action !== actionEpoch) return false
  remote.turns.value = turns
  remote.partial.value = ''
  remote.error.value = ''
  historyPreview.value = item
  remote.conversationId.value = item.id
  return true
}
async function continueHistory(
  item: { id: string; language: string; settings?: StreamSettings },
  turns: ConversationTurn[],
) {
  if (!(await previewHistory(item, turns)) || !item.settings) return
  const action = actionEpoch
  await player.value?.enableAudio()
  if (!active.value || action !== actionEpoch) return
  await reconnect()
}
function updateHistoryTurns(id: string, turns: ConversationTurn[]) {
  if (!active.value || remote.conversationId.value !== id) return
  const current = new Map(remote.turns.value.map((turn) => [turn.id, turn]))
  for (const turn of turns) if (!current.has(turn.id)) current.set(turn.id, turn)
  remote.turns.value = [...current.values()].sort((a, b) => a.sequence - b.sequence)
}
async function load(configuration: SavedConfiguration) {
  const action = ++actionEpoch
  microphoneStartupPending.value = false
  historyPreview.value = null
  remote.conversationId.value = ''
  await remote.stop()
  if (!active.value || action !== actionEpoch) return
  settings.value = {
    ...defaultSettings(),
    ...configuration.settings,
    language: locale.speechLanguage,
    voice: DEFAULT_VOICE,
    reduced_motion: reducedMotion.matches,
  }
}
async function previewExpression(kind: string, amount = 1, repetitions = 1) {
  if (!active.value || remote.suspending.value) return
  const action = actionEpoch
  if (kind === 'laugh') await player.value?.enableAudio()
  if (!active.value || action !== actionEpoch) return
  microphoneStartupPending.value = false
  if (remote.microphone.value) remote.stopMicrophone()
  if (remote.ready.value) {
    remote.command('preview', kind, undefined, amount, repetitions)
    return
  }
  queuedPreview.value = { kind, amount, repetitions }
  if (!remote.connecting.value) await reconnect()
}
async function microphone() {
  if (
    !active.value ||
    remote.suspending.value ||
    invalidCharacter ||
    !selectedCharacter.ready.value
  )
    return
  const action = actionEpoch
  await player.value?.enableAudio()
  if (!active.value || action !== actionEpoch) return
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
async function entryAction() {
  if (!active.value || remote.suspending.value) return
  if (historyPreview.value || remote.state.value === 'paused') {
    const action = actionEpoch
    await player.value?.enableAudio()
    if (active.value && action === actionEpoch) await reconnect()
    return
  }
  if (remote.state.value === 'not-started' || remote.state.value === 'ended') {
    await microphone()
    return
  }
  microphoneStartupPending.value = false
  await reconnect()
}
async function newChat() {
  emit('newConversation')
  const action = ++actionEpoch
  historyPreview.value = null
  remote.conversationId.value = ''
  queuedPreview.value = null
  microphoneStartupPending.value = false
  await remote.stop()
  if (!active.value || action !== actionEpoch) return
  remote.turns.value = []
  remote.partial.value = ''
}
watch(auth.account, (account, previous) => {
  if (previous && account?.id !== previous.id) {
    historyPreview.value = null
    remote.conversationId.value = ''
    void stopForAuth()
    remote.turns.value = []
    remote.partial.value = ''
  }
})
function suspendNavigation() {
  active.value = false
  actionEpoch++
  microphoneStartupPending.value = false
  queuedPreview.value = null
  clearTimeout(configureTimer)
  settingsOpen.value = false
  settingsDialog.value?.close()
  player.value?.suspendPresentation()
  return remote.suspendNavigation()
}
onActivated(() => {
  active.value = true
  remote.activatePresentation()
  // A return may reveal a deleted character or an expired account, but it
  // must never allocate a renderer or capture a microphone by itself.
  void auth.refresh()
  void selectedCharacter.refresh()
})
onDeactivated(() => {
  void suspendNavigation()
})
onMounted(() => {
  window.addEventListener('keydown', closeSettingsOnEscape)
  void remote.loadPolicy()
})
onScopeDispose(() => {
  actionEpoch++
  active.value = false
  window.removeEventListener('keydown', closeSettingsOnEscape)
  clearTimeout(configureTimer)
})
defineExpose({
  load,
  previewHistory,
  continueHistory,
  updateHistoryTurns,
  prepareHistorySelection,
  stopForAuth,
  newChat,
  suspendNavigation,
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
          :media="remote.suspending.value ? null : remote.media.value"
          :ready="remote.ready.value"
          :suppressed="remote.audioSuppressed.value"
          :still="['not-started', 'disconnected', 'paused', 'ended'].includes(remote.state.value)"
          :background="settings.background"
          :blur="settings.blur"
          :controls-target="mediaControls"
          :poster="invalidCharacter ? null : selectedCharacter.poster.value"
          :character-name="selectedCharacter.name.value"
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
          <p
            v-if="invalidCharacter || selectedCharacter.error.value"
            class="connection-status character-error"
            role="alert"
          >
            {{
              t(
                invalidCharacter
                  ? 'This character is unavailable. Open a ready character from your collection.'
                  : selectedCharacter.error.value,
              )
            }}
            <a href="/characters" @click="emit('navigate', $event)">{{ t('Characters') }}</a>
          </p>
          <p v-else-if="selectedCharacter.loading.value" class="connection-status" role="status">
            {{ t('Loading your character…') }}
          </p>
          <p
            v-if="
              (historyPreview && !historyPreview.settings) || remote.state.value !== 'not-started'
            "
            class="connection-status"
            role="status"
          >
            {{
              t(
                historyPreview && !historyPreview.settings
                  ? 'This saved conversation is unavailable. Start a new conversation to continue.'
                  : remote.status.value,
              )
            }}
          </p>
          <button
            v-if="
              ['not-started', 'disconnected', 'paused', 'ended'].includes(remote.state.value) &&
              !remote.connecting.value
            "
            type="button"
            class="entry-action"
            :disabled="
              invalidCharacter ||
              !selectedCharacter.ready.value ||
              remote.suspending.value ||
              (Boolean(historyPreview) && !historyPreview?.settings)
            "
            @click="entryAction"
          >
            <svg
              class="entry-action-wave"
              viewBox="0 0 32 32"
              fill="none"
              stroke="currentColor"
              stroke-width="2.5"
              stroke-linecap="round"
              aria-hidden="true"
            >
              <path d="M4 14v4M10 9v14M16 4v24M22 10v12M28 14v4" />
            </svg>
            <span class="entry-action-label">
              {{
                t(
                  historyPreview || remote.state.value === 'paused'
                    ? 'Continue conversation'
                    : remote.state.value === 'disconnected'
                      ? 'Reconnect'
                      : remote.state.value === 'ended'
                        ? 'Start new session'
                        : 'Start conversation',
                )
              }}
            </span>
            <svg
              class="entry-action-arrow"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"
            >
              <path d="M5 12h14m-5-5 5 5-5 5" />
            </svg>
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
        :character="selectedCharacter.name.value"
        :configured="remote.configured.value"
        :can-start-microphone="canStartMicrophone"
        :not-started="!historyPreview && remote.state.value === 'not-started'"
        :paused="Boolean(historyPreview) || remote.state.value === 'paused'"
        :checking="remote.connecting.value || remote.suspending.value || microphoneStartupPending"
        :ready="remote.ready.value"
        :pending="remote.pending.value"
        :speaking="remote.speaking.value"
        :microphone="remote.microphone.value"
        :input-level="remote.inputLevel.value"
        :starting-microphone="remote.startingMicrophone.value || microphoneStartupPending"
        :recognition-available="remote.recognition.value"
        :status="
          historyPreview ? 'Saved conversation. Continue when you are ready.' : remote.status.value
        "
        :error="remote.error.value"
        :connection-message="
          t(
            historyPreview || remote.state.value === 'paused'
              ? 'Select Continue conversation when you are ready.'
              : remote.state.value === 'not-started'
                ? ''
                : remote.state.value === 'ended'
                  ? 'Select Start new session to continue.'
                  : remote.state.value === 'exhausted'
                    ? 'Your 15-minute demo allowance has been used.'
                    : 'Select Reconnect to restore video and voice controls.',
          )
        "
        @microphone="microphone"
        @stop="remote.interrupt"
        @new-chat="newChat"
      >
        <template #before-messages><slot name="history-tools" /></template>
        <template #media-actions>
          <div ref="mediaControls" />
        </template>
        <template #actions>
          <slot name="conversation-actions" />
          <button
            ref="customizeButton"
            class="quiet-action customize-action"
            type="button"
            :aria-label="t('Customize')"
            :title="t('Customize')"
            aria-haspopup="dialog"
            aria-controls="stream-customization"
            :aria-expanded="settingsOpen"
            @click="settingsOpen = true"
          >
            <ConversationIcon name="adjust" /><span class="customize-label">{{
              t('Customize')
            }}</span>
          </button>
        </template>
      </ChatPanel>
    </div>
    <dialog
      id="stream-customization"
      ref="settingsDialog"
      class="settings-dialog"
      aria-labelledby="stream-customize-title"
      @close="settingsOpen = false"
    >
      <header class="customize-header">
        <h2 id="stream-customize-title" ref="customizeHeading" tabindex="-1">
          {{ selectedCharacter.custom.value ? t('Customize character') : t('Customize Syntelle') }}
        </h2>
        <button
          class="quiet-action icon-action"
          type="button"
          :aria-label="t('Close')"
          :title="t('Close')"
          @click="settingsOpen = false"
        >
          <ConversationIcon name="close" />
        </button>
      </header>
      <StreamSettingsPanel
        v-model="settings"
        :active="settingsOpen"
        :disabled="controlsLocked"
        :preview-disabled="previewDisabled"
        :laughter-busy="remote.laughterBusy.value"
        :laughter-available="remote.laughterAvailable.value"
        :emotions-available="!selectedCharacter.custom.value && remote.emotionsAvailable.value"
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
.customize-action {
  gap: 8px;
  padding: 0 12px;
  color: var(--text);
  font: inherit;
  font-size: 13px;
  font-weight: 600;
}
@media (max-width: 760px) {
  .customize-label {
    display: none;
  }
  .customize-action {
    width: 44px;
    padding: 0;
  }
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
.connection-state .entry-action {
  pointer-events: auto;
}
.character-error {
  pointer-events: auto;
}
.character-error a {
  color: #cbb8fa;
}
.connection-state.connection-entry {
  inset: 0;
  padding: 24px;
  background: transparent;
}
.connection-entry .connection-status {
  position: absolute;
  top: 16px;
  left: 50%;
  width: max-content;
  max-width: calc(100% - 32px);
  transform: translateX(-50%);
  padding: 10px 14px;
  border-radius: 8px;
  background: #26232c;
  font-size: 13px;
}
.connection-paused .connection-status {
  color: #f3f0f7;
  border: 1px solid #857b94;
  background: #26232ce8;
}
.entry-action {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 16px;
  min-height: 72px;
  max-width: 100%;
  padding: 18px 22px;
  border: 1px solid var(--control-border);
  border-radius: 16px 16px 16px 4px;
  color: var(--text);
  background: color-mix(in srgb, var(--surface-raised) 78%, transparent);
  box-shadow: 0 2px 8px #14131633;
  backdrop-filter: blur(3px);
  font: inherit;
  cursor: pointer;
  pointer-events: auto;
  -webkit-tap-highlight-color: transparent;
}
.entry-action-wave {
  flex-shrink: 0;
  width: 32px;
  height: 32px;
  color: var(--accent);
}
.entry-action-arrow {
  flex-shrink: 0;
  width: 20px;
  height: 20px;
  color: var(--muted);
}
.entry-action-label {
  font-size: 15px;
  font-weight: 600;
  line-height: 20px;
  text-align: center;
}
.entry-action:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 4px;
}
@media (hover: hover) {
  .entry-action:hover {
    background: color-mix(in srgb, var(--subtle) 84%, transparent);
    border-color: color-mix(in srgb, var(--control-border) 72%, transparent);
  }
  .entry-action:hover .entry-action-arrow {
    color: var(--accent);
  }
}
@media (prefers-reduced-motion: no-preference) {
  .entry-action {
    transition:
      background-color 150ms ease-out,
      border-color 150ms ease-out,
      transform 150ms ease-out;
  }
  .entry-action:active {
    transform: scale(0.96);
  }
}
.connection-entry .session-note {
  position: absolute;
  right: 16px;
  bottom: 16px;
  left: 16px;
  width: fit-content;
  max-width: calc(100% - 32px);
  margin: 0 auto;
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
  inset: 48px 0 0 auto;
  z-index: 3;
  margin: 0;
  width: min(460px, 100%);
  max-width: 100%;
  height: calc(100dvh - 48px);
  max-height: none;
  padding: 0;
  color: var(--text);
  background: var(--surface);
  border: 0;
  border-left: 1px solid var(--divider);
  overflow: hidden;
}
.settings-dialog[open] {
  display: flex;
  flex-direction: column;
}
.customize-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 20px;
  flex-shrink: 0;
  background: var(--surface);
  border-bottom: 1px solid var(--divider);
}
.customize-header h2 {
  margin: 0;
  font-size: 18px;
  line-height: 1.35;
  font-weight: 600;
}
.customize-header :is(h2, button):focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 3px;
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
    inset: 0;
    width: 100%;
    padding: 12px;
    background: transparent;
  }
  .entry-action {
    min-height: 64px;
    gap: 12px;
    padding: 14px 16px;
  }
  .entry-action-label {
    font-size: 14px;
  }
  .customize-header {
    padding: 12px 16px;
    gap: 8px;
  }
  .customize-header h2 {
    font-size: 16px;
  }
  .settings-dialog {
    inset: auto 0 0;
    width: 100%;
    height: min(70dvh, var(--demo-viewport-height, 70dvh));
    border-left: 0;
    border-top: 1px solid var(--divider);
    border-radius: 12px 12px 0 0;
    padding-bottom: env(safe-area-inset-bottom);
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
    grid-template-rows: clamp(190px, 30svh, 216px) minmax(0, 1fr);
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
@media (max-width: 600px), (max-width: 899px) and (max-height: 520px) {
  .stream-panels {
    gap: 12px;
  }
  .stage-column :deep(.remote-stage) {
    background: var(--subtle);
  }
  .stage-column .connection-entry {
    inset: 0;
    width: 100%;
    padding: 8px;
    background: transparent;
    justify-content: flex-end;
  }
  .stage-column .connection-entry .connection-status {
    display: none;
  }
  .stage-column .connection-entry .character-error {
    display: block;
    top: 8px;
    right: 8px;
    left: 8px;
    width: auto;
    transform: none;
    font-size: 12px;
  }
  .stage-column .connection-entry .entry-action {
    min-height: 48px;
    gap: 8px;
    padding: 8px 12px;
  }
  .stage-column .connection-entry .session-note {
    top: 8px;
    right: 8px;
    bottom: auto;
    left: 8px;
    max-width: calc(100% - 16px);
    margin: 0 auto;
  }
  .conversation-panel :deep(.chat-header) {
    padding: 7px 10px;
    gap: 4px;
  }
  .conversation-panel :deep(.chat-title) {
    font-size: 14px;
  }
  .conversation-panel :deep(.chat-status) {
    font-size: 11px;
  }
  .conversation-panel :deep(.transcript) {
    padding: 12px;
  }
  .conversation-panel :deep(.empty-title) {
    margin-bottom: 6px;
    font-size: 17px;
  }
  .conversation-panel :deep(.empty-description),
  .conversation-panel :deep(.message-text) {
    font-size: 13px;
  }
  .conversation-panel :deep(.message-speaker) {
    font-size: 11px;
  }
  .conversation-panel :deep(.conversation-controls) {
    padding: 6px 10px max(6px, env(safe-area-inset-bottom));
  }
  .conversation-panel :deep(.voice-actions) {
    grid-template-columns: minmax(0, 1fr) 44px minmax(0, 1fr);
    gap: 4px 8px;
  }
  .conversation-panel :deep(.microphone-control),
  .conversation-panel :deep(.microphone-button) {
    width: 44px;
    height: 44px;
  }
  .conversation-panel :deep(.microphone-button .conversation-icon) {
    width: 20px;
    height: 20px;
  }
  .conversation-panel :deep(.microphone-halo) {
    inset: -6px;
    opacity: 0.22;
  }
  .conversation-panel :deep(.microphone-orbit),
  .conversation-panel :deep(.microphone-progress) {
    inset: -2px;
    box-shadow: none;
  }
  .conversation-panel :deep(.capture-status) {
    min-height: 14px;
    font-size: 11px;
    line-height: 14px;
  }
  .conversation-panel :deep(.microphone-hint) {
    margin-top: 2px;
    font-size: 11px;
  }
}
@media (max-width: 899px) and (max-height: 520px) {
  .stage-column {
    display: none;
  }
  .stream-panels {
    grid-template-rows: minmax(0, 1fr);
    gap: 0;
  }
}
@media (max-width: 480px) and (orientation: portrait) {
  .stream-panels {
    grid-template-rows: min(calc(133.333vw - 24px), 53svh, 480px) minmax(0, 1fr);
  }
}
@media (max-width: 480px) and (orientation: portrait) and (max-height: 700px) {
  .stream-panels {
    grid-template-rows: min(calc(133.333vw - 24px), 38svh) minmax(0, 1fr);
  }
}
@media (max-width: 480px) and (orientation: portrait) and (max-height: 520px) {
  .stream-panels {
    grid-template-rows: minmax(0, 1fr);
  }
}
</style>
