<script setup lang="ts">
import { characterThumbnail, demoCharacters, type DemoCharacterId } from '../catalog'
const selected = defineModel<DemoCharacterId>({ required: true })
</script>

<template>
  <div class="character-picker" role="group" aria-label="Choose a character">
    <div class="character-options">
      <button
        v-for="character in demoCharacters"
        :key="character.id"
        type="button"
        class="character-option"
        :aria-pressed="selected === character.id"
        @click="selected = character.id"
      >
        <img :src="characterThumbnail(character.id)" alt="" width="36" height="36" />
        <span>{{ character.name }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.character-picker {
  display: flex;
  align-items: center;
  min-width: 0;
}
.character-options {
  display: flex;
  gap: 6px;
  width: 100%;
}
.character-option {
  display: inline-flex;
  align-items: center;
  gap: 9px;
  padding: 5px 10px 5px 5px;
  min-height: 48px;
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  color: var(--muted);
  font-size: 14px;
  font-weight: 500;
  min-width: 0;
}
.character-option img {
  object-fit: cover;
  object-position: 50% 30%;
  border-radius: 3px;
}
.character-option[aria-pressed='true'] {
  border-color: var(--accent);
  color: var(--text);
  background: var(--surface);
}
@media (hover: hover) {
  .character-option:hover {
    background: var(--subtle);
    color: var(--text);
  }
}
@media (max-width: 1100px) and (min-width: 900px) {
  .character-option {
    gap: 6px;
    padding-right: 6px;
    font-size: 12px;
  }
  .character-option img {
    width: 28px;
    height: 32px;
  }
}
@media (max-width: 899px) {
  .character-picker {
    gap: 10px;
  }
  .character-options {
    flex: 1;
    gap: 6px;
  }
  .character-option {
    flex: 1;
    justify-content: center;
    gap: 6px;
    padding-right: 8px;
    font-size: 13px;
  }
  .character-option img {
    width: 30px;
    height: 32px;
  }
}
</style>
