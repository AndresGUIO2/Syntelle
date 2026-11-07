<script setup lang="ts">
import { computed } from 'vue'
import AppIcon from '../../components/AppIcon.vue'
import { useLocaleStore } from '../../stores/locale'
import type { Workspace } from '../workspaces/useWorkspaces'
import { useCharacters } from '../characters/useCharacters'
import CharacterLibrary from '../characters/CharacterLibrary.vue'
import CharacterGallerySample from '../characters/CharacterGallerySample.vue'
import SharedCharacterCollection from '../characters/SharedCharacterCollection.vue'
import type { CharacterConnection } from '../characters/useCharacterJourney'
import ConsoleHeading from './ConsoleHeading.vue'
const props = defineProps<{ workspace?: Workspace | null; ownerId: string }>()
const emit = defineEmits<{
  navigate: [event: MouseEvent]
  connect: [selection: CharacterConnection]
}>()
const { t } = useLocaleStore()
const personal = useCharacters(props.ownerId)
const showShared = computed(() => props.workspace && props.workspace.role !== 'billing')
const galleryColumns = computed(() => Math.min(4, Math.max(2, personal.items.value.length + 1)))
</script>
<template>
  <section
    class="console-page character-collection"
    :style="{ '--console-page-width': `${galleryColumns * 364 - 24}px` }"
  >
    <ConsoleHeading title="Characters" description="Give your ideas a face and a voice."
      ><a class="console-button primary" href="/characters/new" @click="emit('navigate', $event)"
        ><AppIcon name="plus" />{{ t('Create character') }}</a
      ></ConsoleHeading
    >
    <CharacterLibrary
      :items="personal.items.value"
      :loading="personal.loading.value"
      :error="personal.error.value"
      :removing-id="personal.removingId.value"
      @refresh="personal.refresh"
      @remove="personal.remove"
      @navigate="emit('navigate', $event)"
      @connect="emit('connect', $event)"
    >
      <template #included>
        <CharacterGallerySample
          @navigate="emit('navigate', $event)"
          @connect="emit('connect', $event)"
        />
      </template>
    </CharacterLibrary>
    <SharedCharacterCollection
      v-if="showShared && workspace"
      :key="workspace.id"
      :workspace="workspace"
      @connect="emit('connect', $event)"
    />
  </section>
</template>
<style scoped>
.character-collection {
  width: 100%;
  max-width: var(--console-page-width);
  margin-inline: auto;
}
</style>
