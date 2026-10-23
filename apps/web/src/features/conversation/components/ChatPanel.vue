<script setup lang="ts">
import { useLocaleStore } from '../../../stores/locale'
import { computed } from 'vue'
import ConversationIcon from './ConversationIcon.vue'
import ConversationMessages from './ConversationMessages.vue'
import ConversationControls from './ConversationControls.vue'
import type { ConversationTurn } from '../types'
const { t, errorMessage } = useLocaleStore()
const props = defineProps<{
  turns: ConversationTurn[]
  partial: string
  character: string
  configured: boolean
  canStartMicrophone?: boolean
  notStarted?: boolean
  paused?: boolean
  checking: boolean
  ready: boolean
  pending: boolean
  speaking: boolean
  microphone: boolean
  inputLevel: number
  startingMicrophone: boolean
  recognitionAvailable: boolean
  status: string
  error: string
  connectionMessage?: string
}>()
defineEmits<{ microphone: []; stop: []; newChat: [] }>()
defineSlots<{
  actions?: () => unknown
  'media-actions'?: () => unknown
}>()
const displayStatus = computed(() => {
  if (props.error) return 'Conversation needs attention'
  if (props.checking) return 'Connecting…'
  if (props.notStarted) return 'Ready when you are.'
  if (props.paused) return props.status
  if (!props.configured) return 'Chat setup needed'
  if (!props.ready) return 'Loading character…'
  if (!props.recognitionAvailable) return 'Voice setup needed'
  return props.status
})
const emptyDescription = computed(() =>
  props.paused
    ? 'Select Resume to continue your conversation.'
    : props.notStarted
      ? 'Your conversation will appear here.'
      : undefined,
)
</script>

<template>
  <section class="chat-panel" :aria-label="t('Chat workspace')">
    <header class="chat-header">
      <div class="chat-heading">
        <h1 class="chat-title">{{ t('Chat with {character}', { character }) }}</h1>
        <p class="chat-status" :class="{ 'needs-attention': error }" role="status">
          {{ t(displayStatus) }}
        </p>
      </div>
      <div class="chat-actions">
        <button
          class="new-chat"
          type="button"
          :aria-label="t('New chat')"
          :title="t('New chat')"
          @click="$emit('newChat')"
        >
          <ConversationIcon name="new-chat" />
          <span class="new-chat-label">{{ t('New chat') }}</span>
        </button>
        <slot name="actions" />
      </div>
    </header>
    <ConversationMessages
      :turns="turns"
      :partial="partial"
      :character="character"
      :empty-description="emptyDescription"
    />
    <p v-if="!checking && !configured && !notStarted" class="setup-note">
      {{ t('Voice conversation is not configured on the local server yet.') }}
    </p>
    <p v-if="error" class="chat-error" role="alert">{{ errorMessage(error) }}</p>
    <ConversationControls
      :ready="ready"
      :can-start-microphone="canStartMicrophone ?? false"
      :configured="configured"
      :checking="checking"
      :connection-message="connectionMessage"
      :pending="pending"
      :speaking="speaking"
      :microphone="microphone"
      :input-level="inputLevel"
      :error="Boolean(error)"
      :recognition-available="recognitionAvailable"
      :starting="startingMicrophone"
      @microphone="$emit('microphone')"
      @stop="$emit('stop')"
    >
      <template v-if="$slots['media-actions']" #media-actions>
        <slot name="media-actions" />
      </template>
    </ConversationControls>
  </section>
</template>

<style scoped>
.chat-panel {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}
.chat-header {
  display: flex;
  flex-shrink: 0;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 18px 20px 18px 28px;
  background: var(--surface);
  border-bottom: 1px solid var(--divider);
}
.chat-heading {
  min-width: 0;
}
.chat-title {
  font-size: 18px;
  line-height: 1.35;
  letter-spacing: -0.35px;
  font-weight: 600;
  margin: 0;
}
.chat-status {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--muted);
  margin: 4px 0 0;
}
.chat-status::before {
  content: '';
  width: 5px;
  height: 5px;
  flex-shrink: 0;
  border-radius: 50%;
  background: var(--accent);
}
.needs-attention::before {
  background: var(--error);
}
.chat-actions {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  gap: 2px;
}
.new-chat {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-width: 44px;
  min-height: 44px;
  padding: 0 11px;
  border: 1px solid color-mix(in srgb, var(--accent) 18%, var(--divider));
  border-radius: 9px;
  background: color-mix(in srgb, var(--accent) 5%, var(--surface));
  color: var(--text);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}
.new-chat .conversation-icon {
  width: 20px;
  height: 20px;
}
.new-chat-label {
  white-space: nowrap;
}
@media (hover: hover) {
  .new-chat:hover {
    color: var(--text);
    background: var(--subtle);
  }
}
@media (max-width: 520px) {
  .new-chat {
    width: 44px;
    padding: 0;
  }
  .new-chat-label {
    display: none;
  }
}
.setup-note,
.chat-error {
  flex-shrink: 0;
  margin: 0 20px;
  padding: 10px 12px;
  border-radius: 6px;
  background: var(--subtle);
  color: var(--muted);
  font-size: 13px;
  line-height: 1.5;
  max-height: 120px;
  overflow-y: auto;
}
.chat-error {
  color: var(--error);
}
@media (max-width: 899px) {
  .chat-header {
    padding: 12px 16px;
    gap: 8px;
  }
  .chat-title {
    font-size: 16px;
  }
}
</style>
