<script setup lang="ts">
import { useLocaleStore } from '../../stores/locale'
import AppIcon from '../../components/AppIcon.vue'
import CharacterCard from './CharacterCard.vue'
import { characterFailure, type PersonalCharacter } from './api'
import type { CharacterConnection } from './useCharacterJourney'
defineProps<{ item: PersonalCharacter; loading: boolean; error: string; removing: boolean }>()
const emit = defineEmits<{
  navigate: [event: MouseEvent]
  connect: [selection: CharacterConnection]
  refresh: []
  remove: [id: string]
  another: []
}>()
const { t } = useLocaleStore()
</script>
<template>
  <section class="preparation-receipt" :aria-label="t('Character preparation')">
    <div class="receipt-copy">
      <span class="receipt-symbol"
        ><AppIcon
          :name="item.status === 'ready' ? 'check' : item.status === 'failed' ? 'alert' : 'image'"
      /></span>
      <h2>
        {{
          t(
            item.status === 'ready'
              ? 'Your character is ready'
              : item.status === 'failed'
                ? 'Preparation needs attention'
                : 'Your portrait is saved',
          )
        }}
      </h2>
      <p :class="{ 'receipt-failure': item.status === 'failed' }">
        {{
          t(
            item.status === 'ready'
              ? 'Try its speech and movement, or connect it to a website.'
              : item.status === 'failed'
                ? characterFailure(item.failure_code)
                : 'Preparation continues when you leave this page. Return to Characters to check its status.',
          )
        }}
      </p>
      <p v-if="error" class="console-notice error" role="alert">{{ t(error) }}</p>
      <div class="receipt-actions">
        <button class="console-button" :disabled="loading" @click="emit('refresh')">
          {{ t('Refresh status') }}</button
        ><a class="console-button quiet" href="/characters" @click="emit('navigate', $event)">{{
          t('View characters')
        }}</a
        ><button
          v-if="item.status === 'ready' || item.status === 'failed'"
          class="console-button quiet"
          @click="emit('another')"
        >
          {{ t('Create another character') }}
        </button>
      </div>
    </div>
    <CharacterCard
      class="receipt-card"
      :item="item"
      :removing="removing"
      @navigate="emit('navigate', $event)"
      @connect="emit('connect', $event)"
      @remove="emit('remove', $event)"
    />
  </section>
</template>
<style scoped>
.preparation-receipt {
  display: grid;
  grid-template-columns: minmax(240px, 340px) minmax(300px, 1fr);
  gap: 48px;
  align-items: start;
  max-width: 920px;
}
.receipt-copy {
  grid-column: 2;
  grid-row: 1;
  padding-top: 24px;
}
.receipt-card {
  grid-column: 1;
  grid-row: 1;
}
.receipt-card :deep(.character-description.failure) {
  display: none;
}
.receipt-copy > .receipt-failure {
  color: var(--error);
}
.receipt-symbol {
  display: grid;
  place-items: center;
  width: 48px;
  height: 48px;
  border-radius: 12px;
  background: var(--subtle);
  color: var(--accent);
  margin-bottom: 24px;
}
.receipt-copy h2 {
  margin: 0 0 16px;
  font-size: 28px;
  letter-spacing: -0.6px;
}
.receipt-copy > p {
  margin: 0 0 24px;
  color: var(--muted);
  line-height: 1.7;
  font-size: 15px;
  max-width: 480px;
}
.receipt-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}
@media (max-width: 760px) {
  .preparation-receipt {
    grid-template-columns: 1fr;
    gap: 28px;
  }
  .receipt-copy {
    grid-column: 1;
    grid-row: 1;
    padding-top: 0;
  }
  .receipt-card {
    grid-column: 1;
    grid-row: 2;
  }
  .preparation-receipt > :deep(.character-card) {
    max-width: 380px;
    width: 100%;
  }
}
</style>
