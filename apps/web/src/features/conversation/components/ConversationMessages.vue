<script setup lang="ts">
import { useLocaleStore } from '../../../stores/locale'
import { nextTick, useTemplateRef, watch } from 'vue'
import type { ConversationTurn } from '../types'
const { t } = useLocaleStore()
const props = defineProps<{
  turns: ConversationTurn[]
  partial: string
  character: string
  emptyDescription?: string
  emptyTitle?: string
}>()
const transcript = useTemplateRef<HTMLElement>('transcript')
watch(
  () => [props.turns, props.partial],
  async () => {
    const element = transcript.value
    if (!props.turns.length && !props.partial) {
      if (element) element.scrollTop = 0
      return
    }
    const follow = element && element.scrollHeight - element.scrollTop - element.clientHeight < 100
    await nextTick()
    if (follow && element) element.scrollTop = element.scrollHeight
  },
)
</script>

<template>
  <div
    ref="transcript"
    class="transcript"
    role="log"
    :aria-label="t('Conversation')"
    aria-live="polite"
    aria-relevant="additions text"
  >
    <div v-if="!turns.length && !partial" class="empty-conversation">
      <h2 class="empty-title">{{ t(emptyTitle ?? 'Start a conversation') }}</h2>
      <p class="empty-description">
        {{
          t(
            emptyDescription ??
              'Turn on your microphone and talk with {character}. Your conversation will appear here.',
            { character },
          )
        }}
      </p>
    </div>
    <article v-for="turn in turns" :key="turn.id" class="exchange">
      <div class="user-message">
        <p class="message-speaker">{{ t('You') }}</p>
        <p class="message-text message-body">{{ turn.user }}</p>
      </div>
      <div v-if="turn.reply" class="character-message">
        <p class="message-speaker">{{ character }}</p>
        <p class="message-text message-body">{{ turn.reply }}</p>
      </div>
      <p v-if="turn.status === 'interrupted'" class="message-note">{{ t('Reply interrupted') }}</p>
      <p v-else-if="turn.status === 'failed'" class="message-note">
        {{ t('Reply could not finish') }}
      </p>
    </article>
    <p v-if="partial" class="partial-transcript">{{ partial }}…</p>
  </div>
</template>

<style scoped>
.transcript {
  flex: 1 1 0;
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
  scrollbar-gutter: stable;
  padding: 24px 28px;
  scrollbar-width: thin;
  scrollbar-color: var(--control-border) transparent;
}
.empty-conversation {
  display: flex;
  flex-direction: column;
  justify-content: center;
  min-height: 100%;
  max-width: 380px;
  margin: 0 auto;
  padding: 0;
}
.empty-title {
  font-size: 26px;
  line-height: 1.3;
  font-weight: 500;
  letter-spacing: -0.5px;
  margin: 0 0 10px;
}
.empty-description {
  color: var(--muted);
  font-size: 14px;
  line-height: 1.6;
  margin: 0;
  max-width: 36ch;
}
.exchange + .exchange {
  margin-top: 20px;
}
.user-message {
  width: fit-content;
  max-width: 88%;
  margin-left: auto;
}
.user-message .message-speaker {
  text-align: right;
  margin-right: 4px;
}
.user-message .message-body {
  padding: 12px 16px;
  border: 1px solid color-mix(in srgb, var(--accent) 28%, transparent);
  border-radius: 16px 4px 16px 16px;
  background: var(--user-surface);
  box-shadow: var(--message-shadow);
}
.character-message {
  margin-top: 16px;
}
.character-message .message-body {
  max-width: 72ch;
  padding: 14px 16px;
  border: 1px solid var(--divider);
  border-radius: 4px 16px 16px 16px;
  background: var(--surface-raised);
  box-shadow: var(--message-shadow);
}
.message-speaker {
  margin: 0 0 6px;
  font-size: 12px;
  font-weight: 600;
  color: var(--muted);
}
.character-message .message-speaker {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: 4px;
  margin-bottom: 10px;
  color: var(--text);
}
.character-message .message-speaker::before {
  content: '';
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: linear-gradient(135deg, var(--accent), #d2b8f5);
}
.message-text {
  margin: 0;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  line-height: 1.65;
  font-size: 15px;
}
.message-note,
.partial-transcript {
  font-size: 12px;
  color: var(--muted);
  line-height: 1.5;
}
@media (max-width: 899px) {
  .transcript {
    padding: 20px;
  }
  .empty-title {
    font-size: 20px;
  }
  .empty-conversation {
    padding: 0;
  }
}
</style>
