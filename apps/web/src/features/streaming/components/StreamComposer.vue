<script setup lang="ts">
import { computed, nextTick, useTemplateRef, watch } from 'vue'
import ConversationIcon from '../../conversation/components/ConversationIcon.vue'

const draft = defineModel<string>({ required: true })
const props = defineProps<{
  ready: boolean
  enabled: boolean
  exact: boolean
}>()
const emit = defineEmits<{ submit: []; returnToConversation: [] }>()
const input = useTemplateRef<HTMLTextAreaElement>('input')
const canSubmit = computed(() => props.ready && props.enabled && Boolean(draft.value.trim()))
const actionLabel = computed(() => (props.exact ? 'Speak text' : 'Send message'))

function submit() {
  if (canSubmit.value) emit('submit')
}
function keydown(event: KeyboardEvent) {
  if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) {
    event.preventDefault()
    submit()
  }
}
watch(
  [draft, input],
  async () => {
    await nextTick()
    const element = input.value
    if (!element) return
    element.style.height = 'auto'
    element.style.height = `${Math.min(144, Math.max(44, element.scrollHeight))}px`
  },
  { immediate: true, flush: 'post' },
)
</script>

<template>
  <div class="composer-area">
    <div v-if="exact" class="exact-notice">
      <span>Saying your words exactly</span>
      <button
        class="return-action"
        type="button"
        aria-label="Return to conversation"
        title="Return to conversation"
        @click="$emit('returnToConversation')"
      >
        <ConversationIcon name="close" />
      </button>
    </div>
    <form class="composer" @submit.prevent="submit">
      <label class="visually-hidden" for="stream-draft">{{
        exact ? 'Exact text for Syntelle to say' : 'Message Syntelle'
      }}</label>
      <textarea
        id="stream-draft"
        ref="input"
        v-model="draft"
        class="composer-input"
        rows="1"
        maxlength="16000"
        :placeholder="exact ? 'Write what you’d like Syntelle to say…' : 'Message Syntelle…'"
        aria-describedby="composer-shortcuts"
        @keydown="keydown"
      />
      <button
        class="send-action"
        type="submit"
        :disabled="!canSubmit"
        :aria-label="actionLabel"
        :title="actionLabel"
      >
        <ConversationIcon :name="exact ? 'play' : 'send'" />
      </button>
    </form>
    <span id="composer-shortcuts" class="visually-hidden"
      >Enter to send. Shift and Enter for a new line.</span
    >
  </div>
</template>

<style scoped>
.composer-area {
  padding: 16px 28px 20px;
  flex-shrink: 0;
}
.composer {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 44px;
  align-items: end;
  gap: 4px;
  padding: 7px;
  border: 1px solid var(--control-border);
  border-radius: 16px;
  background: var(--page);
}
.composer:focus-within {
  border-color: var(--accent);
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
.composer-input {
  display: block;
  width: 100%;
  min-width: 0;
  min-height: 44px;
  max-height: 144px;
  padding: 11px 9px;
  border: 0;
  border-radius: 0;
  background: transparent;
  color: var(--text);
  font: inherit;
  font-size: 14px;
  line-height: 22px;
  resize: none;
  overflow-y: auto;
}
.composer-input:focus-visible {
  outline: none;
}
.composer-input::placeholder {
  color: var(--muted);
  opacity: 1;
}
.send-action {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  padding: 0;
  border: 1px solid transparent;
  border-radius: 8px;
  color: var(--on-accent);
  background: var(--accent);
  cursor: pointer;
}
.send-action:disabled {
  color: var(--muted);
  background: var(--subtle);
  cursor: default;
}
.send-action :deep(.conversation-icon) {
  width: 20px;
  height: 20px;
}
.exact-notice {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 44px;
  margin-top: -8px;
  color: var(--muted);
  font-size: 12px;
}
.return-action {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  padding: 0;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--muted);
}
.return-action :deep(.conversation-icon) {
  width: 16px;
  height: 16px;
}
@media (hover: hover) {
  .send-action:enabled:hover {
    background: var(--accent-hover);
  }
  .return-action:hover {
    background: var(--subtle);
    color: var(--text);
  }
}
@media (prefers-reduced-motion: no-preference) {
  .send-action {
    transition:
      transform 150ms ease-out,
      background-color 150ms ease-out;
  }
  .send-action:enabled:active {
    transform: scale(0.96);
  }
}
@media (max-width: 899px) {
  .composer-area {
    padding: 12px 20px 16px;
  }
}
@media (forced-colors: active) {
  .send-action {
    border-color: ButtonText;
  }
}
</style>
