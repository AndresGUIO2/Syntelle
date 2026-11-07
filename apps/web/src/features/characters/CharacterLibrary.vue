<script setup lang="ts">
import { shallowRef } from 'vue'
import { useLocaleStore } from '../../stores/locale'
import { characterFailure, characterPortrait, type PersonalCharacter } from './api'
defineProps<{ items: PersonalCharacter[]; loading: boolean }>()
const emit = defineEmits<{ remove: [id: string]; navigate: [event: MouseEvent]; refresh: [] }>()
const { t } = useLocaleStore()
const confirming = shallowRef('')
const failedPortraits = shallowRef<string[]>([])
function remove(id: string) {
  emit('remove', id)
  confirming.value = ''
}
const labels = {
  queued: 'Waiting to prepare',
  processing: 'Preparing your character',
  ready: 'Ready to talk',
  failed: 'Needs attention',
}
</script>

<template>
  <section class="library" aria-labelledby="character-library-title">
    <header class="library-heading">
      <div>
        <h2 id="character-library-title">{{ t('Your characters') }}</h2>
        <p>{{ t('Preparation continues even when you leave this page.') }}</p>
      </div>
      <button class="quiet" type="button" :disabled="loading" @click="emit('refresh')">
        {{ t('Refresh') }}
      </button>
    </header>
    <p v-if="loading" class="notice" role="status">{{ t('Loading…') }}</p>
    <div v-else-if="!items.length" class="empty">
      <h3>{{ t('Your first character starts here') }}</h3>
      <p>
        {{
          t(
            'Upload a portrait and give it a name. Once it is ready, open it to start a conversation.',
          )
        }}
      </p>
    </div>
    <ul v-else class="character-list">
      <li v-for="item in items" :key="item.id" class="character-card" :data-character-id="item.id">
        <img
          v-if="item.portrait_available && !failedPortraits.includes(item.id)"
          class="portrait"
          :src="characterPortrait(item.id)"
          :alt="item.name"
          @error="failedPortraits = [...failedPortraits, item.id]"
        />
        <div v-else class="portrait placeholder" aria-hidden="true">
          {{ item.name.slice(0, 1).toLocaleUpperCase() }}
        </div>
        <div class="character-details">
          <h3>{{ item.name }}</h3>
          <p class="status" :class="item.status" role="status">{{ t(labels[item.status]) }}</p>
          <p v-if="item.status === 'failed'" class="detail">
            {{ t(characterFailure(item.failure_code)) }}
          </p>
          <p v-else-if="item.status === 'processing'" class="detail">
            {{ t('Building speech, blinking and head movement, then checking the result.') }}
          </p>
          <p v-else-if="item.status === 'queued'" class="detail">
            {{ t('Your portrait is saved. Preparation will start when capacity is available.') }}
          </p>
          <div class="actions">
            <a
              v-if="item.status === 'ready'"
              class="open"
              :href="`/?character=${item.id}`"
              @click="emit('navigate', $event)"
              >{{ t('Open character') }}</a
            ><button class="quiet" type="button" @click="confirming = item.id">
              {{
                t(
                  item.status === 'queued' || item.status === 'processing'
                    ? 'Cancel preparation'
                    : 'Remove',
                )
              }}
            </button>
          </div>
          <div v-if="confirming === item.id" class="confirm">
            <p>{{ t('Remove this character from your account?') }}</p>
            <button class="quiet" type="button" @click="confirming = ''">
              {{ t('Keep character') }}</button
            ><button class="quiet destructive" type="button" @click="remove(item.id)">
              {{ t('Remove character') }}
            </button>
          </div>
        </div>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.library {
  min-width: 0;
}
.library-heading {
  display: flex;
  align-items: start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
}
.library-heading h2 {
  margin: 0 0 8px;
  font-size: 20px;
}
.library-heading p,
.detail,
.empty p {
  color: var(--muted);
  font-size: 14px;
  line-height: 1.6;
  margin: 0;
}
.character-list {
  display: grid;
  gap: 16px;
  list-style: none;
  padding: 0;
  margin: 0;
}
.character-card {
  display: grid;
  grid-template-columns: 96px minmax(0, 1fr);
  gap: 20px;
  padding: 20px;
  border: 1px solid var(--divider);
  border-radius: 18px;
  background: var(--surface);
}
.portrait {
  width: 96px;
  height: 126px;
  border-radius: 8px;
  object-fit: cover;
  outline: 1px solid oklch(1 0 0 / 0.1);
  outline-offset: -1px;
}
.placeholder {
  display: grid;
  place-items: center;
  background: var(--subtle);
  color: var(--accent);
  font-size: 32px;
}
.character-details h3 {
  margin: 0 0 6px;
  overflow-wrap: anywhere;
  font-size: 18px;
}
.status {
  margin: 0 0 12px;
  font-size: 12px;
  color: var(--muted);
}
.status.ready {
  color: var(--accent);
}
.status.failed,
.destructive {
  color: var(--error);
}
.actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
}
.quiet,
.open {
  min-height: 40px;
  padding: 9px 12px;
  border-radius: 8px;
  font: inherit;
  font-size: 13px;
  cursor: pointer;
}
.quiet {
  border: 1px solid var(--divider);
  background: transparent;
  color: var(--text);
}
.open {
  display: inline-flex;
  align-items: center;
  border: 1px solid var(--accent);
  background: var(--user-surface);
  color: var(--text);
  text-decoration: none;
}
.confirm {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--divider);
  font-size: 13px;
}
.confirm .quiet {
  margin-right: 8px;
}
.empty {
  padding: 32px;
  border: 1px dashed var(--divider);
  border-radius: 18px;
}
.empty h3 {
  margin: 0 0 8px;
  font-size: 18px;
}
@media (max-width: 520px) {
  .character-card {
    grid-template-columns: 64px minmax(0, 1fr);
    gap: 12px;
    padding: 16px;
  }
  .portrait {
    width: 64px;
    height: 86px;
  }
}
</style>
