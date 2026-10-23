<script setup lang="ts">
import { shallowRef, useTemplateRef, watch } from 'vue'
import { useLocaleStore } from '../../../stores/locale'
import { request } from '../api'
import type { StreamSettings } from '../types'
import type { ConversationTurn } from '../../conversation/types'

type Item = {
  id: string
  title: string
  language: string
  updated_at: string
  expires_at?: string
  settings?: StreamSettings
}
const open = defineModel<boolean>({ required: true })
const props = defineProps<{ signedIn: boolean }>()
const emit = defineEmits<{ continue: [item: Item, turns: ConversationTurn[]] }>()
const { t } = useLocaleStore()
const dialog = useTemplateRef<HTMLDialogElement>('dialog')
const items = shallowRef<Item[]>([])
const claimable = shallowRef<Item[]>([])
const selected = shallowRef<Item | null>(null)
const turns = shallowRef<ConversationTurn[]>([])
const next = shallowRef('')
const older = shallowRef(0)
const busy = shallowRef(false)
const error = shallowRef('')
const confirmDelete = shallowRef(false)

async function load() {
  if (!props.signedIn) return
  busy.value = true
  error.value = ''
  try {
    const [history, guest] = await Promise.all([
      request<{ items: Item[]; next: string }>('/api/v1/conversations'),
      request<{ items: Item[] }>('/api/v1/conversations/claimable'),
    ])
    items.value = history.items
    next.value = history.next
    claimable.value = guest.items
  } catch (failure) {
    error.value = failure instanceof Error ? failure.message : 'History is unavailable.'
  } finally {
    busy.value = false
  }
}
async function more() {
  if (!next.value || busy.value) return
  busy.value = true
  try {
    const page = await request<{ items: Item[]; next: string }>(
      `/api/v1/conversations?before=${encodeURIComponent(next.value)}`,
    )
    items.value = [...items.value, ...page.items]
    next.value = page.next
  } catch (failure) {
    error.value = failure instanceof Error ? failure.message : 'History is unavailable.'
  } finally {
    busy.value = false
  }
}
async function select(item: Item) {
  busy.value = true
  error.value = ''
  try {
    const page = await request<{ conversation: Item; turns: ConversationTurn[]; before: number }>(
      `/api/v1/conversations/${encodeURIComponent(item.id)}`,
    )
    selected.value = page.conversation
    turns.value = page.turns
    older.value = page.before
    confirmDelete.value = false
  } catch (failure) {
    error.value = failure instanceof Error ? failure.message : 'Conversation is unavailable.'
  } finally {
    busy.value = false
  }
}
async function moreTurns() {
  if (!selected.value || !older.value || busy.value) return
  busy.value = true
  try {
    const page = await request<{ turns: ConversationTurn[]; before: number }>(
      `/api/v1/conversations/${encodeURIComponent(selected.value.id)}?before=${older.value}`,
    )
    turns.value = [...page.turns, ...turns.value]
    older.value = page.before
  } catch (failure) {
    error.value = failure instanceof Error ? failure.message : 'Older messages are unavailable.'
  } finally {
    busy.value = false
  }
}
async function claim(item: Item) {
  busy.value = true
  try {
    await request(`/api/v1/conversations/${encodeURIComponent(item.id)}/claim`, 'POST', {})
    await load()
  } catch (failure) {
    error.value = failure instanceof Error ? failure.message : 'This chat could not be saved.'
  } finally {
    busy.value = false
  }
}
async function remove() {
  if (!selected.value || !confirmDelete.value) {
    confirmDelete.value = true
    return
  }
  busy.value = true
  try {
    await request(`/api/v1/conversations/${encodeURIComponent(selected.value.id)}`, 'DELETE')
    selected.value = null
    turns.value = []
    await load()
  } catch (failure) {
    error.value = failure instanceof Error ? failure.message : 'This chat could not be deleted.'
  } finally {
    busy.value = false
    confirmDelete.value = false
  }
}
watch([open, dialog], ([visible, element]) => {
  if (!element) return
  if (visible && !element.open) {
    element.showModal()
    void load()
  } else if (!visible && element.open) element.close()
})
watch(
  () => props.signedIn,
  (value) => {
    if (!value) {
      items.value = []
      claimable.value = []
      selected.value = null
      turns.value = []
      open.value = false
    }
  },
)
</script>

<template>
  <dialog
    ref="dialog"
    class="history-dialog"
    :aria-label="t('Conversation history')"
    @close="open = false"
  >
    <header>
      <h2>{{ t('Conversation history') }}</h2>
      <button type="button" @click="open = false">{{ t('Close') }}</button>
    </header>
    <p>{{ t('Saved chats expire 90 days after the last message.') }}</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <section v-if="claimable.length" aria-label="Guest chats">
      <h3>{{ t('Save guest conversations') }}</h3>
      <div v-for="item in claimable" :key="item.id" class="history-row">
        <span>{{ item.title }}</span
        ><button type="button" :disabled="busy" @click="claim(item)">
          {{ t('Save to account') }}
        </button>
      </div>
    </section>
    <div class="history-layout">
      <section :aria-label="t('Saved conversations')">
        <h3>{{ t('Saved conversations') }}</h3>
        <p v-if="!items.length && !busy">{{ t('No saved conversations yet.') }}</p>
        <button
          v-for="item in items"
          :key="item.id"
          class="history-row"
          type="button"
          :disabled="busy"
          @click="select(item)"
        >
          <span>{{ item.title }}</span
          ><small>{{ new Date(item.updated_at).toLocaleDateString() }}</small>
        </button>
        <button v-if="next" type="button" :disabled="busy" @click="more">
          {{ t('More conversations') }}
        </button>
      </section>
      <section v-if="selected" :aria-label="t('Conversation transcript')">
        <h3>{{ selected.title }}</h3>
        <button v-if="older" type="button" :disabled="busy" @click="moreTurns">
          {{ t('Older messages') }}
        </button>
        <div class="history-turns">
          <div v-for="turn in turns" :key="turn.id" class="history-turn">
            <strong>{{ t('You') }}</strong>
            <p>{{ turn.user }}</p>
            <strong>Syntelle</strong>
            <p>{{ turn.reply }}</p>
          </div>
        </div>
        <div class="history-actions">
          <button
            type="button"
            :disabled="busy || !selected.settings"
            @click="selected && emit('continue', selected, turns)"
          >
            {{ t(selected.language === 'es-CO' ? 'Continue in Spanish' : 'Continue in English') }}
          </button>
          <button type="button" :disabled="busy" @click="remove">
            {{ t(confirmDelete ? 'Confirm delete' : 'Delete conversation') }}
          </button>
        </div>
      </section>
    </div>
  </dialog>
</template>

<style scoped>
.history-dialog {
  width: min(940px, calc(100% - 24px));
  max-height: calc(100dvh - 24px);
  overflow: auto;
  color: var(--text);
  background: var(--surface);
  border: 1px solid var(--divider);
  border-radius: 16px;
  padding: 24px;
}
.history-dialog::backdrop {
  background: #08060db3;
}
header,
.history-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}
h2 {
  margin: 0 0 16px;
}
h3 {
  font-size: 15px;
}
.history-layout {
  display: grid;
  grid-template-columns: minmax(180px, 1fr) minmax(0, 2fr);
  gap: 20px;
}
.history-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
  padding: 12px;
  text-align: left;
  border: 1px solid var(--divider);
  background: var(--surface);
  color: var(--text);
  border-radius: 10px;
  margin-bottom: 8px;
}
.history-turns {
  max-height: 50vh;
  overflow: auto;
}
.history-turn {
  padding: 12px 0;
  border-bottom: 1px solid var(--divider);
}
.history-turn p {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  margin: 4px 0 12px;
}
button {
  cursor: pointer;
  min-height: 44px;
  color: var(--text);
}
button:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
@media (max-width: 620px) {
  .history-layout {
    grid-template-columns: 1fr;
  }
}
</style>
