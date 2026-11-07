<script setup lang="ts">
import { shallowRef, watch } from 'vue'
import AppIcon from '../../components/AppIcon.vue'
import { useLocaleStore } from '../../stores/locale'
import { characterFailure, characterPortrait, type PersonalCharacter } from './api'
import type { CharacterConnection } from './useCharacterJourney'
const props = withDefaults(
  defineProps<{ item: PersonalCharacter; removable?: boolean; removing?: boolean }>(),
  { removable: true, removing: false },
)
const emit = defineEmits<{
  navigate: [event: MouseEvent]
  connect: [selection: CharacterConnection]
  remove: [id: string]
}>()
const { t } = useLocaleStore()
const failed = shallowRef(false)
const confirming = shallowRef(false)
const labels = {
  queued: 'Waiting to prepare',
  processing: 'Preparing your character',
  ready: 'Ready',
  failed: 'Needs attention',
}
watch(
  () => props.item.id,
  () => {
    failed.value = false
    confirming.value = false
  },
)
</script>
<template>
  <article class="character-card" :data-character-id="item.id">
    <div class="character-portrait">
      <span class="ownership-label">{{ t('Private character') }}</span>
      <img
        v-if="item.portrait_available && !failed"
        :src="characterPortrait(item.id)"
        :alt="item.name"
        @error="failed = true"
      />
      <div v-else class="portrait-fallback">
        <span>{{ item.name.slice(0, 1).toLocaleUpperCase() }}</span
        ><small>{{ t('Portrait unavailable') }}</small>
      </div>
      <span v-if="item.status !== 'ready'" class="character-status" :class="item.status"
        ><span aria-hidden="true" />{{ t(labels[item.status]) }}</span
      >
    </div>
    <div class="character-details">
      <div class="character-title">
        <h3>{{ item.name }}</h3>
        <span v-if="item.status === 'ready'" class="ready-label"
          ><span aria-hidden="true" />{{ t('Ready') }}</span
        >
      </div>
      <p v-if="item.status === 'failed'" class="character-description failure">
        {{ t(characterFailure(item.failure_code)) }}
      </p>
      <p v-else-if="item.status === 'processing'" class="character-description">
        {{ t('Preparing speech and movement. You can come back later.') }}
      </p>
      <p v-else-if="item.status === 'queued'" class="character-description">
        {{ t('Your portrait is saved. Waiting for preparation capacity.') }}
      </p>
      <p v-else class="character-description">
        {{ t('Ready for a conversation or your website.') }}
      </p>
      <div v-if="item.status === 'ready'" class="character-actions">
        <a class="console-button" :href="`/?character=${item.id}`" @click="emit('navigate', $event)"
          ><AppIcon name="play" />{{ t('Test') }}</a
        >
        <button class="console-button quiet" @click="emit('connect', { characterId: item.id })">
          {{ t('Connect to website') }}<AppIcon class="point-right" name="chevron-left" />
        </button>
      </div>
      <button
        v-if="removable && !confirming"
        class="remove-action"
        :disabled="removing"
        @click="confirming = true"
      >
        {{
          t(
            item.status === 'queued' || item.status === 'processing'
              ? 'Cancel preparation'
              : 'Remove',
          )
        }}
      </button>
      <div v-if="confirming" class="remove-confirmation">
        <p>
          {{
            t(
              item.status === 'queued' || item.status === 'processing'
                ? 'Cancel this preparation and remove the character?'
                : 'Remove this character from your account?',
            )
          }}
        </p>
        <div class="character-actions">
          <button class="console-button quiet" :disabled="removing" @click="confirming = false">
            {{ t('Keep character') }}</button
          ><button
            class="console-button danger"
            :disabled="removing"
            @click="emit('remove', item.id)"
          >
            {{ t(removing ? 'Removing…' : 'Remove character') }}
          </button>
        </div>
      </div>
    </div>
  </article>
</template>
<style scoped>
.character-card {
  height: 100%;
  min-width: 0;
  background: var(--surface);
  border-radius: 18px;
  overflow: hidden;
}
.character-portrait {
  position: relative;
  aspect-ratio: 1.12;
  background: var(--subtle);
}
.character-portrait img {
  width: 100%;
  height: 100%;
  position: absolute;
  object-fit: cover;
  object-position: center 25%;
  outline: 1px solid oklch(0 0 0 / 0.1);
  outline-offset: -1px;
}
.ownership-label {
  position: absolute;
  top: 16px;
  left: 16px;
  z-index: 1;
  padding: 6px 9px;
  border-radius: 6px;
  background: var(--portrait-ownership, #fffffff0);
  color: var(--portrait-text, #26212d);
  font-size: 11px;
  font-weight: 600;
}
.character-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 9px;
}
.ready-label {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: var(--muted);
  font-size: 11px;
}
.ready-label > span {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--accent);
}
.point-right {
  transform: rotate(180deg);
}
:global([data-theme='dark'] .character-portrait img) {
  outline-color: oklch(1 0 0 / 0.1);
}
.portrait-fallback {
  display: grid;
  place-content: center;
  justify-items: center;
  width: 100%;
  height: 100%;
  gap: 12px;
  color: var(--muted);
}
.portrait-fallback > span {
  font-size: 64px;
  color: var(--accent);
  font-weight: 500;
}
.portrait-fallback small {
  font-size: 12px;
}
.character-status {
  position: absolute;
  bottom: 14px;
  left: 14px;
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 6px 10px;
  background: var(--portrait-status, #15131bee);
  color: var(--portrait-text, #f3f0f7);
  border-radius: 6px;
  font-size: 12px;
  line-height: 1.4;
}
.character-status > span {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}
.character-status.ready > span {
  background: var(--portrait-accent, #bba3f5);
}
.character-status.failed > span {
  background: var(--portrait-error, #ff9d95);
}
.character-details {
  padding: 20px;
}
.character-details h3 {
  margin: 0;
  font-size: 23px;
  overflow-wrap: anywhere;
}
.character-description {
  margin: 0 0 18px;
  color: var(--muted);
  font-size: 13px;
  line-height: 1.6;
}
.failure {
  color: var(--error);
}
.character-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.character-actions .console-button {
  font-size: 12px;
  padding: 9px 10px;
}
.character-actions .console-button svg {
  width: 16px;
  height: 16px;
}
.remove-action {
  margin-top: 0;
  min-height: 44px;
  padding: 8px 0;
  border: 0;
  background: none;
  color: var(--muted);
  font: inherit;
  font-size: 12px;
  cursor: pointer;
}
.remove-action:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 3px;
}
.remove-confirmation {
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--divider);
}
.remove-confirmation p {
  color: var(--muted);
  font-size: 13px;
  line-height: 1.5;
  margin: 0 0 10px;
}
@media (hover: hover) {
  .remove-action:hover {
    color: var(--error);
  }
}
</style>
