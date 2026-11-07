<script setup lang="ts">
import { useLocaleStore } from '../../stores/locale'
import CharacterUpload from './CharacterUpload.vue'
import CharacterLibrary from './CharacterLibrary.vue'
import CharacterPreparation from './CharacterPreparation.vue'
import ConsoleHeading from '../console/ConsoleHeading.vue'
import { useCharacters } from './useCharacters'
import { useCharacterJourney, type CharacterConnection } from './useCharacterJourney'
const emit = defineEmits<{
  navigate: [event: MouseEvent]
  connect: [selection: CharacterConnection]
}>()
const props = withDefaults(
  defineProps<{ mode?: 'library' | 'create'; conversationHref?: string; ownerId?: string }>(),
  { mode: 'library', conversationHref: '/', ownerId: '' },
)
const { t } = useLocaleStore()
const state = useCharacters(props.ownerId)
const draft = useCharacterJourney(props.ownerId)
async function submit(name: string, image: File) {
  await state.submit(name, image)
}
</script>
<template>
  <section
    class="console-page characters-page"
    :class="{ 'characters-page--create': mode === 'create' }"
  >
    <nav v-if="mode === 'create'" class="character-breadcrumb" :aria-label="t('Breadcrumb')">
      <a href="/characters" @click="emit('navigate', $event)">{{ t('Characters') }}</a
      ><span aria-hidden="true">/</span><span>{{ t('Create character') }}</span>
    </nav>
    <ConsoleHeading
      :title="mode === 'create' ? 'Create character' : 'Characters'"
      :description="
        mode === 'create'
          ? 'From a portrait to a character you can test and connect.'
          : 'Create, test and manage your personal characters.'
      "
    />
    <CharacterPreparation
      v-if="mode === 'create' && draft.receipt"
      :item="draft.receipt"
      :loading="state.loading.value"
      :error="state.error.value"
      :removing="state.removingId.value === draft.receipt.id"
      @refresh="state.refresh"
      @remove="state.remove"
      @navigate="emit('navigate', $event)"
      @connect="emit('connect', $event)"
      @another="draft.receipt = null"
    />
    <template v-else-if="mode === 'create'"
      ><p v-if="state.error.value && !draft.submitError" class="console-notice error" role="alert">
        {{ t(state.error.value) }}
        <button class="console-button quiet" @click="state.refresh">{{ t('Try again') }}</button>
      </p>
      <CharacterUpload
        :owner-id="ownerId"
        :busy="state.submitting.value"
        :blocked="state.active.value && !state.resubmittingPending.value"
        :unavailable="state.loading.value || Boolean(state.error.value && !draft.submitError)"
        @submit="submit"
    /></template>
    <CharacterLibrary
      v-else
      :items="state.items.value"
      :loading="state.loading.value"
      :error="state.error.value"
      :removing-id="state.removingId.value"
      @remove="state.remove"
      @refresh="state.refresh"
      @navigate="emit('navigate', $event)"
      @connect="emit('connect', $event)"
    />
  </section>
</template>
<style scoped>
.characters-page--create {
  --console-page-width: 920px;
}
.character-breadcrumb {
  display: flex;
  gap: 10px;
  align-items: center;
  margin: 0 0 16px;
  color: var(--muted);
  font-size: 12px;
}
.character-breadcrumb a {
  color: var(--muted);
  text-decoration: none;
  min-height: 44px;
  display: inline-flex;
  align-items: center;
}
.characters-page > .console-notice {
  max-width: 920px;
}
.character-breadcrumb a:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 3px;
}
@media (hover: hover) {
  .character-breadcrumb a:hover {
    color: var(--text);
    text-decoration: underline;
  }
}
</style>
