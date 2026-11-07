<script setup lang="ts">
import { useLocaleStore } from '../../stores/locale'
import CharacterCard from './CharacterCard.vue'
import CreateCharacterCard from './CreateCharacterCard.vue'
import type { PersonalCharacter } from './api'
import type { CharacterConnection } from './useCharacterJourney'
withDefaults(
  defineProps<{
    items: PersonalCharacter[]
    loading: boolean
    error?: string
    removingId?: string
    publishable?: boolean
  }>(),
  { error: '', removingId: '' },
)
const emit = defineEmits<{
  remove: [id: string]
  navigate: [event: MouseEvent]
  refresh: []
  publish: [id: string]
  connect: [selection: CharacterConnection]
}>()
const { t } = useLocaleStore()
</script>
<template>
  <section class="character-library" :aria-label="t('Character collection')">
    <p v-if="error" class="console-notice error" role="alert">
      {{ t(error) }}
      <button class="retry-link" @click="emit('refresh')">{{ t('Try again') }}</button>
    </p>
    <p v-else-if="loading && !items.length" class="library-loading" role="status">
      {{ t('Loading your characters…') }}
    </p>
    <div v-if="items.length" class="collection-tools">
      <span
        >{{ t('Personal collection')
        }}<span class="collection-count">{{ items.length }}</span></span
      >
      <button class="console-button quiet" :disabled="loading" @click="emit('refresh')">
        {{ t('Refresh') }}
      </button>
    </div>
    <ul class="character-grid">
      <li v-if="$slots.included" class="included-slot"><slot name="included" /></li>
      <li v-for="item in items" :key="item.id">
        <CharacterCard
          :item="item"
          :removing="removingId === item.id"
          @remove="emit('remove', $event)"
          @navigate="emit('navigate', $event)"
          @connect="emit('connect', $event)"
        />
      </li>
      <li v-if="!items.length && !loading && !error" class="first-character">
        <CreateCharacterCard @navigate="emit('navigate', $event)" />
      </li>
    </ul>
  </section>
</template>
<style scoped>
.collection-tools {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  margin: -6px 0 16px;
  color: var(--muted);
  font-size: 13px;
}
.collection-tools > span {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}
.collection-count {
  display: inline-grid;
  place-items: center;
  min-width: 24px;
  min-height: 24px;
  padding-inline: 5px;
  border-radius: 6px;
  background: var(--subtle);
  color: var(--text);
  font-size: 12px;
}
.character-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 300px), 1fr));
  gap: 24px;
  list-style: none;
  margin: 0;
  padding: 0;
}
.character-grid > li {
  min-width: 0;
}
.library-loading {
  color: var(--muted);
  padding: 0 0 20px;
  margin: 0;
}
.retry-link {
  border: 0;
  padding: 8px;
  min-height: 44px;
  background: transparent;
  color: inherit;
  text-decoration: underline;
  cursor: pointer;
  font: inherit;
}
@media (max-width: 600px) {
  .character-grid {
    grid-template-columns: minmax(0, 1fr);
    gap: 20px;
  }
}
</style>
