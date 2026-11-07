<script setup lang="ts">
import { nextTick, shallowRef, useTemplateRef, watch } from 'vue'
import { useLocaleStore } from '../../stores/locale'
import type { HistoryItem, useConversationHistory } from './useConversationHistory'

const props = defineProps<{ history: ReturnType<typeof useConversationHistory> }>()
const emit = defineEmits<{
  open: [item: HistoryItem]
  deleted: [id: string]
}>()
const { t } = useLocaleStore()
const root = useTemplateRef<HTMLElement>('root')
const confirming = shallowRef('')
watch(
  () => props.history.items.value,
  (items) => {
    if (!items.some((item) => item.id === confirming.value)) confirming.value = ''
  },
)
function select(item: HistoryItem) {
  if (props.history.mutatingId.value) return
  confirming.value = ''
  emit('open', item)
}
async function remove(item: HistoryItem) {
  if (await props.history.remove(item)) {
    confirming.value = ''
    emit('deleted', item.id)
    await nextTick()
    root.value?.focus()
  }
}
async function confirm(id: string) {
  confirming.value = id
  await nextTick()
  root.value?.querySelector<HTMLButtonElement>('.delete-confirm')?.focus()
}
async function cancel(id: string) {
  confirming.value = ''
  await nextTick()
  root.value?.querySelector<HTMLButtonElement>(`[data-delete-id="${CSS.escape(id)}"]`)?.focus()
}
function focus() {
  root.value?.focus()
}
defineExpose({ focus })
</script>

<template>
  <section
    ref="root"
    class="conversation-list"
    tabindex="-1"
    :aria-label="t('Recent conversations')"
  >
    <h2 class="list-heading">{{ t('Recent conversations') }}</h2>
    <p v-if="history.loading.value && !history.items.value.length" class="list-note" role="status">
      {{ t('Loading conversations…') }}
    </p>
    <div v-if="history.error.value" class="list-error" role="alert">
      <p>{{ t(history.error.value) }}</p>
      <button
        class="text-action"
        type="button"
        :disabled="history.loading.value"
        @click="history.refresh"
      >
        {{ t('Try again') }}
      </button>
    </div>
    <p v-else-if="!history.loading.value && !history.items.value.length" class="list-note">
      {{ t('Your conversations will appear here.') }}
    </p>
    <div v-for="group in history.groups.value" :key="group.label" class="date-group">
      <h3 class="date-heading">{{ t(group.label) }}</h3>
      <ul class="conversation-rows">
        <li
          v-for="item in group.items"
          :key="item.id"
          class="conversation-row"
          :class="{ selected: history.selectedId.value === item.id }"
        >
          <div class="row-controls">
            <button
              class="conversation-title"
              type="button"
              :title="item.title || t('Untitled conversation')"
              :aria-current="history.selectedId.value === item.id ? 'true' : undefined"
              :aria-busy="history.previewingId.value === item.id"
              :disabled="Boolean(history.mutatingId.value)"
              @click="select(item)"
            >
              <span>{{ item.title || t('Untitled conversation') }}</span>
            </button>
            <button
              class="row-action"
              type="button"
              :aria-label="
                t('Delete conversation: {title}', {
                  title: item.title || t('Untitled conversation'),
                })
              "
              :title="t('Delete conversation')"
              :data-delete-id="item.id"
              :disabled="
                Boolean(
                  history.mutatingId.value ||
                  history.previewingId.value ||
                  history.loadingOlder.value,
                )
              "
              @click="confirm(item.id)"
            >
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="1.7"
                stroke-linecap="round"
                stroke-linejoin="round"
                aria-hidden="true"
              >
                <path d="M3 6h18M9 6V4h6v2M5 6l1 14h12l1-14M10 10v6m4-6v6" />
              </svg>
            </button>
          </div>
          <div
            v-if="confirming === item.id"
            class="delete-question"
            @keydown.esc.prevent="cancel(item.id)"
          >
            <p>{{ t('Delete this conversation?') }}</p>
            <div class="confirm-actions">
              <button
                type="button"
                class="text-action"
                :disabled="Boolean(history.mutatingId.value)"
                @click="cancel(item.id)"
              >
                {{ t('Cancel') }}
              </button>
              <button
                type="button"
                class="text-action delete-confirm"
                :disabled="Boolean(history.mutatingId.value)"
                @click="remove(item)"
              >
                {{ t('Delete') }}
              </button>
            </div>
          </div>
        </li>
      </ul>
    </div>
    <button
      v-if="history.hasMore.value"
      class="text-action load-more"
      type="button"
      :disabled="history.loading.value || history.loadingMore.value"
      @click="history.loadMore"
    >
      {{ t(history.loadingMore.value ? 'Loading…' : 'More conversations') }}
    </button>
    <div v-if="history.claimable.value.length" class="guest-conversations">
      <h3 class="date-heading">{{ t('Save guest conversations') }}</h3>
      <div v-for="item in history.claimable.value" :key="item.id" class="guest-row">
        <span class="guest-title" :title="item.title">{{
          item.title || t('Untitled conversation')
        }}</span>
        <button
          class="text-action"
          type="button"
          :aria-label="
            t('Save conversation: {title}', { title: item.title || t('Untitled conversation') })
          "
          :disabled="
            Boolean(
              history.mutatingId.value || history.previewingId.value || history.loadingOlder.value,
            )
          "
          @click="history.claim(item)"
        >
          {{ t('Save') }}
        </button>
      </div>
    </div>
  </section>
</template>

<style scoped>
.conversation-list {
  min-width: 0;
  outline-offset: -2px;
}
.list-heading {
  margin: 0;
  padding: 18px 12px 8px;
  color: var(--muted);
  font-size: 12px;
  font-weight: 600;
}
.date-heading {
  margin: 14px 12px 4px;
  color: var(--muted);
  font-size: 11px;
  font-weight: 500;
}
.conversation-rows {
  list-style: none;
  margin: 0;
  padding: 0;
}
.conversation-row {
  border-radius: 8px;
  color: var(--text);
}
.row-controls {
  display: flex;
  min-width: 0;
  align-items: center;
}
.conversation-title {
  flex: 1;
  min-width: 0;
  min-height: 44px;
  border: 0;
  padding: 10px 0 10px 12px;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: pointer;
  font: inherit;
  font-size: 13px;
}
.conversation-title span,
.guest-title {
  display: block;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}
.row-action {
  width: 44px;
  height: 44px;
  flex: 0 0 44px;
  display: grid;
  place-items: center;
  border: 0;
  border-radius: 8px;
  color: var(--muted);
  background: transparent;
  cursor: pointer;
}
.selected {
  background: color-mix(in srgb, var(--accent) 12%, transparent);
}
.selected .conversation-title {
  color: var(--text);
  font-weight: 600;
}
.conversation-row:focus-within {
  background: color-mix(in srgb, var(--accent) 9%, transparent);
}
.conversation-title:focus-visible,
.row-action:focus-visible,
.text-action:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: -2px;
  border-radius: 6px;
}
.text-action {
  min-height: 44px;
  border: 0;
  padding: 8px 12px;
  color: var(--muted);
  background: transparent;
  font: inherit;
  font-size: 12px;
  cursor: pointer;
  text-align: left;
}
.text-action:disabled,
.row-action:disabled {
  cursor: default;
  opacity: 0.45;
}
.load-more {
  width: 100%;
}
.list-note,
.list-error {
  margin: 4px 12px 12px;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.5;
}
.list-error p {
  margin: 0;
}
.list-error .text-action {
  padding-left: 0;
}
.delete-question {
  padding: 0 12px 4px;
}
.delete-question p {
  margin: 0;
  font-size: 12px;
  line-height: 1.5;
}
.confirm-actions {
  display: flex;
  justify-content: flex-end;
}
.delete-confirm {
  color: var(--text);
  font-weight: 600;
}
.guest-row {
  display: flex;
  align-items: center;
  min-width: 0;
  padding-left: 12px;
}
.guest-title {
  flex: 1;
  min-width: 0;
  color: var(--muted);
  font-size: 12px;
}
@media (hover: hover) {
  .conversation-row:hover {
    background: color-mix(in srgb, var(--text) 5%, transparent);
  }
  .conversation-row.selected:hover {
    background: color-mix(in srgb, var(--accent) 15%, transparent);
  }
  .row-action {
    opacity: 0;
  }
  .conversation-row:hover .row-action,
  .conversation-row:focus-within .row-action {
    opacity: 1;
  }
  .text-action:hover,
  .row-action:hover {
    color: var(--text);
  }
}
</style>
