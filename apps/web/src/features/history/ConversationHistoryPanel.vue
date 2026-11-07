<script setup lang="ts">
import { nextTick, onScopeDispose, useTemplateRef, watch } from 'vue'
import AppIcon from '../../components/AppIcon.vue'
import { useLocaleStore } from '../../stores/locale'
import ConversationList from './ConversationList.vue'
import type { HistoryItem, useConversationHistory } from './useConversationHistory'

const open = defineModel<boolean>({ required: true })
defineProps<{ history: ReturnType<typeof useConversationHistory> }>()
const emit = defineEmits<{ open: [item: HistoryItem]; deleted: [id: string] }>()
const { t } = useLocaleStore()
const dialog = useTemplateRef<HTMLDialogElement>('dialog')
const heading = useTemplateRef<HTMLElement>('heading')
let previousFocus: HTMLElement | null = null
let previousOverflow: string | undefined
let disposed = false

function unlockScroll() {
  if (previousOverflow === undefined) return
  document.body.style.overflow = previousOverflow
  previousOverflow = undefined
}
function closed() {
  if (dialog.value?.open) return
  open.value = false
  unlockScroll()
  const target = previousFocus
  previousFocus = null
  if (!disposed && target?.isConnected) void nextTick(() => target.focus())
}
function backdrop(event: MouseEvent) {
  const element = dialog.value
  if (!element || event.target !== element) return
  const bounds = element.getBoundingClientRect()
  if (
    event.clientX < bounds.left ||
    event.clientX > bounds.right ||
    event.clientY < bounds.top ||
    event.clientY > bounds.bottom
  )
    open.value = false
}
watch(
  [open, dialog],
  async ([visible, element]) => {
    if (!element) return
    if (visible && !element.open) {
      previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
      if (previousOverflow === undefined) previousOverflow = document.body.style.overflow
      document.body.style.overflow = 'hidden'
      element.showModal()
      await nextTick()
      if (!disposed && open.value && element.open) heading.value?.focus()
    } else if (!visible && element.open) element.close()
  },
  { flush: 'post' },
)
onScopeDispose(() => {
  disposed = true
  previousFocus = null
  dialog.value?.close()
  unlockScroll()
})
</script>

<template>
  <dialog
    ref="dialog"
    class="conversation-history-panel"
    aria-labelledby="conversation-history-title"
    @cancel.prevent="open = false"
    @close="closed"
    @click="backdrop"
  >
    <header class="history-panel-header">
      <h2 id="conversation-history-title" ref="heading" tabindex="-1">
        {{ t('Conversation history') }}
      </h2>
      <button
        class="history-panel-close"
        type="button"
        :aria-label="t('Close')"
        :title="t('Close')"
        @click="open = false"
      >
        <AppIcon name="close" />
      </button>
    </header>
    <div class="history-panel-body">
      <ConversationList
        :history="history"
        @open="emit('open', $event)"
        @deleted="emit('deleted', $event)"
      />
    </div>
    <p class="history-panel-note">
      {{ t('Saved chats expire 90 days after the last message.') }}
    </p>
  </dialog>
</template>

<style scoped>
.conversation-history-panel {
  position: fixed;
  inset: 0 0 0 auto;
  margin: 0;
  padding: 0;
  width: min(420px, 100vw);
  max-width: none;
  height: 100dvh;
  max-height: none;
  border: 0;
  border-left: 1px solid var(--divider);
  background: var(--surface);
  color: var(--text);
  overflow: hidden;
  box-shadow: -12px 0 48px #08060d26;
}
.conversation-history-panel[open] {
  display: flex;
  flex-direction: column;
}
.conversation-history-panel::backdrop {
  background: #08060d70;
}
.history-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: max(18px, env(safe-area-inset-top)) 20px 16px;
  border-bottom: 1px solid var(--divider);
  flex-shrink: 0;
}
.history-panel-header h2 {
  font-size: 20px;
  line-height: 1.35;
  letter-spacing: -0.4px;
  margin: 0;
  outline: none;
}
.history-panel-close {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  flex: 0 0 44px;
  padding: 0;
  border: 0;
  border-radius: 9px;
  color: var(--muted);
  background: transparent;
  cursor: pointer;
}
.history-panel-close:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: -2px;
}
.history-panel-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
  padding: 8px 12px 20px;
  scrollbar-width: thin;
}
.history-panel-note {
  flex-shrink: 0;
  margin: 0;
  padding: 16px 24px max(18px, env(safe-area-inset-bottom));
  border-top: 1px solid var(--divider);
  color: var(--muted);
  font-size: 12px;
  line-height: 1.6;
}
@media (hover: hover) {
  .history-panel-close:hover {
    background: var(--subtle);
    color: var(--text);
  }
}
@media (max-width: 600px) {
  .conversation-history-panel {
    width: 100vw;
    border-left: 0;
  }
  .history-panel-header {
    padding-inline: 16px;
  }
}
</style>
