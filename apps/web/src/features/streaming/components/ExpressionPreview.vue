<script setup lang="ts">
import { computed, onScopeDispose, shallowRef } from 'vue'
import { useLocaleStore } from '../../../stores/locale'

defineProps<{
  disabled: boolean
  laughterAvailable?: boolean
  laughterBusy?: boolean
  smile: number
  brows: number
}>()
const { t } = useLocaleStore()
const emit = defineEmits<{
  preview: [kind: string, amount?: number, repetitions?: number]
  'update:smile': [value: number]
  'update:brows': [value: number]
}>()
const busy = shallowRef('')
const gesture = shallowRef('nod')
const strength = shallowRef(1)
const repetitions = shallowRef(1)
let timer: ReturnType<typeof setTimeout> | undefined
const choices = [
  { kind: 'nod', label: 'Nod' },
  { kind: 'shake', label: 'Shake head' },
  { kind: 'tilt', label: 'Lean head toward shoulder' },
]
const selectedLabel = computed(() => choices.find(({ kind }) => kind === gesture.value)!.label)
function preview(kind: string, amount = 1) {
  if (kind === 'laugh') {
    emit('preview', kind, amount, 1)
    return
  }
  clearTimeout(timer)
  busy.value = kind
  const count = kind === 'nod' || kind === 'shake' ? repetitions.value : 1
  emit('preview', kind, amount, count)
  timer = setTimeout(
    () => {
      busy.value = ''
    },
    kind === 'nod' ? count * 420 + 200 : kind === 'shake' ? count * 580 + 200 : 1400,
  )
}
onScopeDispose(() => clearTimeout(timer))
</script>

<template>
  <fieldset class="expression-controls">
    <legend class="section-title">{{ t('Facial expression') }}</legend>
    <label class="slider-field">
      <span>{{ t('Soft smile') }}</span
      ><output>{{ Math.round(smile * 100) }}%</output>
      <input
        :value="smile"
        :aria-label="t('Soft smile')"
        type="range"
        min="0"
        max="1"
        step="0.05"
        @input="emit('update:smile', Number(($event.target as HTMLInputElement).value))"
      />
    </label>
    <label class="slider-field">
      <span>{{ t('Raise eyebrows') }}</span
      ><output>{{ Math.round(brows * 100) }}%</output>
      <input
        :value="brows"
        :aria-label="t('Raise eyebrows')"
        type="range"
        min="0"
        max="1"
        step="0.05"
        @input="emit('update:brows', Number(($event.target as HTMLInputElement).value))"
      />
    </label>
    <div class="laugh-row">
      <span>{{ t('Laughter with sound') }}</span>
      <button
        type="button"
        :disabled="disabled || !!busy || laughterBusy || !laughterAvailable"
        @click="preview('laugh')"
      >
        <svg aria-hidden="true" viewBox="0 0 20 20"><path d="m7 4 8 6-8 6Z" /></svg>
        {{ laughterBusy ? t('Playing…') : t('Play laughter') }}
      </button>
    </div>
  </fieldset>
  <fieldset class="expression-controls">
    <legend class="section-title">{{ t('Try a head gesture') }}</legend>
    <label class="select-field">
      {{ t('Gesture') }}
      <select v-model="gesture" :disabled="disabled || !!busy || laughterBusy">
        <option v-for="choice in choices" :key="choice.kind" :value="choice.kind">
          {{ t(choice.label) }}
        </option>
      </select>
    </label>
    <label class="slider-field">
      <span>{{ t('Intensity') }}</span
      ><output>{{ Math.round(strength * 100) }}%</output>
      <input
        v-model.number="strength"
        :aria-label="t('Intensity')"
        :disabled="disabled || !!busy || laughterBusy"
        type="range"
        min="0.2"
        max="1"
        step="0.05"
      />
    </label>
    <label v-if="gesture === 'nod' || gesture === 'shake'" class="slider-field">
      <span>{{ t('Repetitions') }}</span
      ><output>{{ repetitions }}</output>
      <input
        v-model.number="repetitions"
        :aria-label="t('Repetitions')"
        :disabled="disabled || !!busy || laughterBusy"
        type="range"
        min="1"
        max="8"
        step="1"
      />
    </label>
    <button
      class="gesture-play"
      type="button"
      :disabled="disabled || !!busy || laughterBusy"
      @click="preview(gesture, strength)"
    >
      <svg aria-hidden="true" viewBox="0 0 20 20"><path d="m7 4 8 6-8 6Z" /></svg>
      {{ busy && busy !== 'laugh' ? t('Playing…') : t('Play gesture') }}
      <span class="sr-only">: {{ t(selectedLabel) }}</span>
    </button>
  </fieldset>
</template>

<style scoped>
.expression-controls {
  display: grid;
  gap: 16px;
  margin: 0;
  padding: 0;
  border: 0;
  min-width: 0;
}
.section-title {
  padding: 0;
  margin-bottom: 16px;
  font-size: 14px;
  font-weight: 600;
}
.slider-field {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 8px;
  font-size: 13px;
  font-weight: 500;
}
output {
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}
input[type='range'] {
  grid-column: 1 / -1;
  width: 100%;
  margin: 0;
  accent-color: var(--accent);
}
.select-field {
  display: grid;
  gap: 8px;
  font-size: 13px;
  font-weight: 500;
}
select {
  width: 100%;
  min-height: 44px;
  padding: 10px 12px;
  background: var(--page);
  color: var(--text);
  border: 1px solid var(--control-border);
  border-radius: 7px;
  font: inherit;
}
.laugh-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-size: 13px;
}
button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 44px;
  border: 1px solid var(--control-border);
  border-radius: 7px;
  padding: 8px 12px;
  background: var(--surface);
  color: var(--text);
  font: inherit;
  font-size: 13px;
  cursor: pointer;
}
button svg {
  width: 18px;
  height: 18px;
  fill: currentColor;
  flex-shrink: 0;
}
.gesture-play {
  justify-self: start;
}
button:disabled {
  opacity: 0.5;
  cursor: default;
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
  white-space: nowrap;
}
@media (hover: hover) {
  button:enabled:hover {
    background: var(--subtle);
  }
}
@media (prefers-reduced-motion: no-preference) {
  button {
    transition:
      background-color 150ms ease-out,
      transform 150ms ease-out;
  }
  button:enabled:active {
    transform: scale(0.96);
  }
}
</style>
