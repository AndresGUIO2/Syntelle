<script setup lang="ts">
import AppIcon from '../../components/AppIcon.vue'
import { useLocaleStore } from '../../stores/locale'
import type { CharacterConnection } from './useCharacterJourney'
const emit = defineEmits<{
  navigate: [event: MouseEvent]
  connect: [selection: CharacterConnection]
}>()
const { t } = useLocaleStore()
</script>
<template>
  <article class="included-character gallery-sample" aria-labelledby="included-character-title">
    <div class="sample-portrait">
      <img src="/characters/posters/syntelle-light-studio-portrait.webp" alt="Syntelle" />
      <span class="sample-label"><AppIcon name="check" />{{ t('Included character') }}</span>
    </div>
    <div class="sample-details">
      <div class="sample-title">
        <h2 id="included-character-title">Syntelle</h2>
        <span class="sample-ready"><span aria-hidden="true" />{{ t('Ready') }}</span>
      </div>
      <p>{{ t('Ready for a conversation or your website.') }}</p>
      <div class="sample-actions">
        <a
          class="console-button primary"
          href="/?character=syntelle"
          @click="emit('navigate', $event)"
        >
          <AppIcon name="play" />{{ t('Test') }}
        </a>
        <button class="console-button quiet" @click="emit('connect', { characterId: 'syntelle' })">
          {{ t('Connect to website') }}<AppIcon class="point-right" name="chevron-left" />
        </button>
      </div>
    </div>
  </article>
</template>
<style scoped>
.point-right {
  transform: rotate(180deg);
}
.gallery-sample {
  --accent: #d5c2ff;
  --primary-border: #a88bd0;
  position: relative;
  isolation: isolate;
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
  min-height: 500px;
  height: 100%;
  overflow: hidden;
  border: 1px solid transparent;
  border-radius: 22px;
  background: #1b1823;
  color: #f8f6fc;
  box-shadow:
    0 0 0 1px #ffffff14,
    0 4px 8px #15131b0a,
    0 16px 32px #15131b14;
}
.sample-portrait {
  position: absolute;
  inset: 0;
}
.sample-portrait::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(180deg, #1b182300 30%, #1b182330 48%, #1b1823ed 76%, #1b1823 100%);
}
.sample-portrait img {
  position: absolute;
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: center 28%;
  outline: 1px solid oklch(1 0 0 / 0.1);
  outline-offset: -1px;
}
.sample-label {
  position: absolute;
  z-index: 1;
  top: 18px;
  left: 18px;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 7px 10px;
  border: 1px solid #ffffff26;
  border-radius: 100px;
  color: #faf8ff;
  background: #211c2be8;
  font-size: 11px;
  font-weight: 500;
}
.sample-label svg {
  width: 14px;
  height: 14px;
  color: #d5c2ff;
}
.sample-details {
  position: relative;
  padding: 100px 24px 22px;
}
.sample-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.sample-title h2 {
  margin: 0;
  font-size: 28px;
  line-height: 1.15;
  font-weight: 600;
  letter-spacing: -0.7px;
}
.sample-ready {
  display: inline-flex;
  gap: 7px;
  align-items: center;
  font-size: 12px;
  color: #e0dce8;
  flex-shrink: 0;
}
.sample-ready > span {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #d5c2ff;
  box-shadow: 0 0 0 4px #d5c2ff14;
}
.sample-details p {
  margin: 10px 0 20px;
  color: #e0dce8;
  font-size: 13px;
  line-height: 1.6;
}
.sample-actions {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: center;
  gap: 10px;
  padding-top: 16px;
  border-top: 1px solid #ffffff26;
}
.sample-actions .console-button {
  font-size: 12px;
  padding-inline: 12px;
  border-radius: 10px;
}
.sample-actions .console-button.quiet {
  gap: 5px;
  min-width: 0;
  padding-inline: 2px;
  color: #f1ecf8;
}
.sample-actions .console-button svg {
  width: 16px;
  height: 16px;
}
.sample-actions > a {
  min-width: 82px;
  justify-content: center;
}
@media (hover: hover) {
  .sample-actions .console-button.quiet:hover {
    background: #ffffff14;
  }
}
</style>
