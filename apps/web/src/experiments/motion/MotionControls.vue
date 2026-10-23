<script setup lang="ts">
import { backgroundUrl } from './useMotionAssets'
import type { Background } from './motionTypes'
defineProps<{
  backgrounds: Background[]
  closure: number
  reducedMotion: boolean
  doubleBlinkEnabled?: boolean
  doubleBlinkAvailable?: boolean
}>()
const selected = defineModel<string>('background', { required: true })
const blur = defineModel<number>('blur', { required: true })
const blinking = defineModel<boolean>('blinking', { required: true })
const moving = defineModel<boolean>('moving', { required: true })
const paused = defineModel<boolean>('paused', { required: true })
const emit = defineEmits<{ blink: []; 'double-blink': []; closure: [value: number] }>()
</script>

<template>
  <div class="motion-controls">
    <fieldset class="backgrounds">
      <legend>Background</legend>
      <button
        v-for="item in backgrounds"
        :key="item.id"
        class="background-option"
        :aria-pressed="selected === item.id"
        @click="selected = item.id"
      >
        <img :src="backgroundUrl(item.thumbnail)" alt="" /><span>{{ item.name }}</span>
      </button>
    </fieldset>
    <label class="setting"
      >Background blur <span>{{ blur }} px</span>
      <input
        v-model.number="blur"
        aria-label="Background blur"
        type="range"
        min="0"
        max="20"
        step="1"
      />
    </label>
    <div class="toggles">
      <label
        ><input v-model="blinking" type="checkbox" :disabled="reducedMotion" /> Automatic
        blinking</label
      >
      <label
        ><input v-model="moving" type="checkbox" :disabled="reducedMotion" /> Subtle movement</label
      >
    </div>
    <div class="buttons">
      <button
        class="button"
        :aria-pressed="paused"
        :disabled="reducedMotion"
        @click="paused = !paused"
      >
        {{ paused ? 'Resume animation' : 'Pause animation' }}
      </button>
      <button class="button" :disabled="paused || reducedMotion" @click="emit('blink')">
        Blink once
      </button>
      <button
        v-if="doubleBlinkEnabled"
        class="button"
        :disabled="!doubleBlinkAvailable"
        @click="emit('double-blink')"
      >
        Blink twice
      </button>
    </div>
    <label class="setting"
      >Inspect eyelids <span>{{ Math.round(closure * 100) }}%</span>
      <input
        aria-label="Eyelid closure"
        type="range"
        min="0"
        max="1"
        step="0.01"
        :value="closure"
        @input="emit('closure', Number(($event.target as HTMLInputElement).value))"
      />
    </label>
    <p v-if="reducedMotion" class="note">
      Reduced motion is on. Use the eyelid and mouth sliders to inspect still poses.
    </p>
  </div>
</template>

<style scoped>
.backgrounds {
  display: flex;
  gap: 8px;
  border: 0;
  padding: 0;
  margin: 0 0 20px;
  min-width: 0;
}
legend {
  margin-bottom: 10px;
  font-size: 14px;
}
.background-option {
  padding: 3px;
  border: 1px solid var(--control-border);
  border-radius: 6px;
  background: var(--surface);
  color: var(--text);
  cursor: pointer;
  flex: 1;
  min-width: 0;
}
.background-option[aria-pressed='true'] {
  outline: 2px solid var(--accent);
  outline-offset: 1px;
}
.background-option img {
  width: 100%;
  display: block;
  aspect-ratio: 16/9;
  object-fit: cover;
  border-radius: 3px;
}
.background-option span {
  display: block;
  font-size: 12px;
  padding: 7px 1px 4px;
}
.setting {
  display: block;
  font-size: 14px;
  margin: 16px 0;
}
.setting span {
  color: var(--muted);
  margin-left: 8px;
}
.setting input {
  display: block;
  width: 100%;
  margin: 10px 0;
  accent-color: var(--accent);
}
.toggles {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 20px;
  font-size: 14px;
}
.toggles input {
  accent-color: var(--accent);
}
.buttons {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 16px;
}
.note {
  color: var(--muted);
  font-size: 13px;
}
</style>
