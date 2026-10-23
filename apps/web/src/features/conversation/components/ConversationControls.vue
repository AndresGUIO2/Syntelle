<script setup lang="ts">
import { useLocaleStore } from '../../../stores/locale'
import { computed, nextTick, useTemplateRef } from 'vue'
import ConversationIcon from './ConversationIcon.vue'
import MicrophoneControl from './MicrophoneControl.vue'

const { t } = useLocaleStore()
const props = defineProps<{
  ready: boolean
  canStartMicrophone?: boolean
  configured: boolean
  checking: boolean
  pending: boolean
  speaking: boolean
  microphone: boolean
  inputLevel: number
  error: boolean
  starting: boolean
  recognitionAvailable: boolean
  connectionMessage?: string
}>()
const emit = defineEmits<{ microphone: []; stop: [] }>()
defineSlots<{ 'media-actions'?: () => unknown }>()
const microphoneButton = useTemplateRef<InstanceType<typeof MicrophoneControl>>('microphoneButton')
async function stopReply() {
  emit('stop')
  await nextTick()
  microphoneButton.value?.focus()
}
const state = computed(() =>
  props.error
    ? 'error'
    : props.starting
      ? 'starting'
      : !props.ready
        ? 'off'
        : props.speaking
          ? 'speaking'
          : props.pending
            ? 'preparing'
            : props.microphone
              ? 'listening'
              : 'off',
)
const hint = computed(() => {
  if (props.checking) return 'Connecting video and voice controls…'
  if (!props.ready) return props.connectionMessage ?? 'Connect to continue using voice controls.'
  if (!props.configured || !props.recognitionAvailable)
    return 'Voice conversation is unavailable. Check the local server setup.'
  return ''
})
const microphoneStatus = computed(() =>
  props.starting
    ? 'Starting microphone…'
    : props.microphone
      ? 'Microphone on'
      : !props.ready && props.canStartMicrophone
        ? 'Start with microphone'
        : 'Microphone off',
)
</script>

<template>
  <div class="conversation-controls" role="group" :aria-label="t('Voice conversation controls')">
    <div class="voice-actions">
      <div v-if="$slots['media-actions']" class="media-actions">
        <slot name="media-actions" />
      </div>
      <MicrophoneControl
        ref="microphoneButton"
        class="microphone-presentation"
        :active="microphone"
        :starting="starting"
        :connects="!ready && canStartMicrophone"
        :state="state"
        :input-level="inputLevel"
        :disabled="
          starting ||
          (!microphone &&
            (checking ||
              (!ready && !canStartMicrophone) ||
              (ready && (!configured || !recognitionAvailable))))
        "
        @toggle="$emit('microphone')"
      />
      <button
        v-if="pending || speaking"
        class="workspace-action stop-reply"
        type="button"
        :aria-label="t('Stop reply')"
        @click="stopReply"
      >
        <ConversationIcon name="stop" /><span>{{ t('Stop reply') }}</span>
      </button>
      <p class="capture-status" :class="{ 'capture-status-on': microphone }" role="status">
        {{ t(microphoneStatus) }}
      </p>
    </div>
    <p v-if="hint" class="microphone-hint">{{ t(hint) }}</p>
  </div>
</template>

<style scoped>
.conversation-controls {
  flex-shrink: 0;
  padding: 16px 20px max(16px, env(safe-area-inset-bottom));
  border-top: 1px solid var(--divider);
  background:
    radial-gradient(ellipse at 50% 20%, var(--workspace-glow), transparent 70%), var(--surface);
}
.voice-actions {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 48px minmax(0, 1fr);
  align-items: center;
  gap: 8px 12px;
}
.microphone-presentation {
  grid-column: 2;
  grid-row: 1;
}
.media-actions {
  grid-column: 1;
  grid-row: 1;
  justify-self: start;
  min-width: 0;
}
.capture-status {
  grid-column: 1 / -1;
  grid-row: 2;
  margin: 0;
  color: var(--muted);
  font-size: 13px;
  font-weight: 500;
  line-height: 20px;
  min-height: 20px;
  text-align: center;
}
.capture-status-on {
  color: var(--text);
  font-weight: 700;
}
.stop-reply {
  grid-column: 3;
  grid-row: 1;
  justify-self: end;
  min-width: 136px;
  min-height: 48px;
  margin-left: 0;
  padding: 0 14px;
  gap: 8px;
  border: 1px solid color-mix(in srgb, var(--accent) 34%, var(--divider));
  border-radius: 10px;
  background: color-mix(in srgb, var(--accent) 16%, var(--surface));
  color: var(--text);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}
.stop-reply .conversation-icon {
  width: 20px;
  height: 20px;
}
.microphone-hint {
  margin: 8px auto 0;
  max-width: 42ch;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.5;
  text-align: center;
}
@media (max-width: 899px) {
  .conversation-controls {
    padding: 12px 16px max(12px, env(safe-area-inset-bottom));
  }
}
@media (max-width: 520px) {
  .stop-reply {
    grid-column: 1 / -1;
    grid-row: 3;
    justify-self: center;
  }
}
</style>
