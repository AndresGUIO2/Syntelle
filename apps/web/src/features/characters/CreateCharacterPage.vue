<script setup lang="ts">
import { useTemplateRef } from 'vue'
import { useLocaleStore } from '../../stores/locale'
import CharacterUpload from './CharacterUpload.vue'
import CharacterLibrary from './CharacterLibrary.vue'
import { useCharacters } from './useCharacters'
const emit = defineEmits<{ navigate: [event: MouseEvent] }>()
const { t } = useLocaleStore()
const state = useCharacters()
const form = useTemplateRef<InstanceType<typeof CharacterUpload>>('form')
async function submit(name: string, image: File) {
  if (await state.submit(name, image)) form.value?.reset()
}
</script>

<template>
  <section class="characters-page">
    <header class="page-heading">
      <div>
        <p class="eyebrow">{{ t('Your collection') }}</p>
        <h1>{{ t('Characters') }}</h1>
        <p>{{ t('Bring a portrait into the conversation.') }}</p>
      </div>
      <a href="/" @click="emit('navigate', $event)">{{ t('Back to conversation') }}</a>
    </header>
    <p v-if="state.error.value" class="page-error" role="alert">{{ t(state.error.value) }}</p>
    <div class="character-layout">
      <CharacterUpload
        ref="form"
        :busy="state.submitting.value"
        :blocked="state.active.value"
        @submit="submit"
      />
      <CharacterLibrary
        :items="state.items.value"
        :loading="state.loading.value"
        @remove="state.remove"
        @refresh="state.refresh"
        @navigate="emit('navigate', $event)"
      />
    </div>
  </section>
</template>

<style scoped>
.characters-page {
  max-width: 1120px;
  margin: 0 auto;
  padding: 12px 0 36px;
}
.page-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 32px;
}
.eyebrow {
  color: var(--accent);
  font-size: 12px;
  font-weight: 600;
  margin: 0 0 8px;
}
.page-heading h1 {
  font-size: clamp(28px, 4vw, 36px);
  margin: 0 0 8px;
  letter-spacing: -0.025em;
}
.page-heading p:not(.eyebrow) {
  color: var(--muted);
  margin: 0;
}
.page-heading a {
  color: var(--text);
  font-size: 14px;
  white-space: nowrap;
}
.character-layout {
  display: grid;
  grid-template-columns: minmax(260px, 360px) minmax(0, 1fr);
  gap: 40px;
  align-items: start;
}
.page-error {
  padding: 14px 18px;
  border: 1px solid var(--divider);
  border-radius: 10px;
  color: var(--error);
  margin-bottom: 24px;
}
@media (max-width: 760px) {
  .character-layout {
    grid-template-columns: 1fr;
    gap: 32px;
  }
  .page-heading {
    align-items: start;
  }
  .page-heading a {
    white-space: normal;
    max-width: 110px;
    text-align: right;
  }
}
</style>
