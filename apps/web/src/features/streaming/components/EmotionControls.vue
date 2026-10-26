<script setup lang="ts">
import { computed, useId } from 'vue'
import { useLocaleStore } from '../../../stores/locale'
import { emotionStates, type EmotionBaseline, type EmotionState } from '../types'
const baseline = defineModel<EmotionBaseline>({ required: true })
const { t } = useLocaleStore()
const id = useId()
const labels: Record<EmotionState, string> = {
  neutral: 'Neutral',
  joyful: 'Joyful',
  playful: 'Playful',
  affectionate: 'Affectionate',
  curious: 'Curious',
  sad: 'Sad',
  angry: 'Angry',
  fearful: 'Fearful',
  surprised: 'Surprised',
  disgusted: 'Disgusted',
}
const selectedLabel = computed(() => labels[baseline.value.state])
// Interface symbols only; these never drive the prepared character geometry.
const faces: Record<EmotionState, string> = {
  neutral: 'M8 9.5h.01M16 9.5h.01M8.5 15.5h7',
  joyful: 'M6.5 10q1.5-2 3 0m5 0q1.5-2 3 0M7.5 14q4.5 6 9 0',
  playful: 'M6.5 10h3M16 9.5h.01M8 14.5q4.5 5 9-1',
  affectionate:
    'M8 11 5.8 8.7C3.8 6.5 7 5.8 8 7.5c1-1.7 4.2-1 2.2 1.2ZM16 11l-2.2-2.3C11.8 6.5 15 5.8 16 7.5c1-1.7 4.2-1 2.2 1.2ZM8 15q4 4 8 0',
  curious: 'M6.5 7.5q1.5-2 3 0m4.5 1h3M8 11h.01M16 11h.01M10 16q2-1.5 4 0',
  sad: 'M6.5 8.5 9 7m6 0 2.5 1.5M8 10.5h.01M16 10.5h.01M8 17q4-4.5 8 0',
  angry: 'm6.5 7 3 2m5 0 3-2M8 11h.01M16 11h.01M8.5 16q3.5-2 7 0',
  fearful:
    'M6 7.5 9 6m6 0 3 1.5M8 9a1 1.5 0 1 0 0 3 1 1.5 0 1 0 0-3m8 0a1 1.5 0 1 0 0 3 1 1.5 0 1 0 0-3M9 17v-1q3-3 6 0v1Z',
  surprised:
    'M6.5 6.5h3m5 0h3M8 9a.8 1 0 1 0 0 2 .8 1 0 1 0 0-2m8 0a.8 1 0 1 0 0 2 .8 1 0 1 0 0-2M12 14a2 2.5 0 1 0 0 5 2 2.5 0 1 0 0-5',
  disgusted: 'm6.5 8 3 1m5-2 3-1M7 11h2m6-1h2m-6 2 1.5-1 1 1M8 16q2-3 4-1l4 1',
}
function select(state: EmotionState) {
  baseline.value = { state, intensity: state === 'neutral' ? 0 : baseline.value.intensity || 0.5 }
}
</script>

<template>
  <fieldset class="customization-group">
    <legend class="sr-only">{{ t('Mood') }}</legend>
    <div class="customization-mood-heading">
      <span>{{ t('Mood') }}</span>
      <span class="customization-current-mood">{{ t(selectedLabel) }}</span>
      <span
        class="customization-preview"
        :title="t('Preview: some emotions still need visual calibration.')"
        >{{ t('Preview') }}</span
      >
      <details class="customization-help">
        <summary :aria-label="t('About moods')" :title="t('About moods')">
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <circle cx="12" cy="12" r="9" />
            <path d="M12 11v6m0-10h.01" />
          </svg>
        </summary>
        <div class="customization-help-copy">
          <p>{{ t('This is her usual mood. Automatic reactions can temporarily change it.') }}</p>
          <p>{{ t('Preview: some emotions still need visual calibration.') }}</p>
        </div>
      </details>
    </div>
    <div class="customization-moods" role="radiogroup" :aria-label="t('Mood')">
      <label
        v-for="state in emotionStates"
        :key="state"
        class="customization-mood"
        :class="{ selected: baseline.state === state }"
        :title="t(labels[state])"
        ><input
          type="radio"
          :aria-label="t(labels[state])"
          :name="`${id}-mood`"
          :value="state"
          :checked="baseline.state === state"
          @change="select(state)" /><svg viewBox="0 0 24 24" aria-hidden="true">
          <circle cx="12" cy="12" r="9.5" />
          <path :d="faces[state]" /></svg
      ></label>
    </div>
    <div class="customization-intensity">
      <label class="customization-slider"
        ><span>{{ t('Intensity') }}</span
        ><output>{{ Math.round(baseline.intensity * 100) }}%</output
        ><input
          type="range"
          min="0"
          max="1"
          step="0.05"
          :aria-label="t('Emotion intensity')"
          :disabled="baseline.state === 'neutral'"
          :value="baseline.intensity"
          @input="
            baseline = { ...baseline, intensity: Number(($event.target as HTMLInputElement).value) }
          "
      /></label>
      <button
        class="customization-reset customization-reset-icon"
        type="button"
        :aria-label="t('Reset emotion')"
        :title="t('Reset emotion')"
        @click="baseline = { state: 'neutral', intensity: 0 }"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 10a8 8 0 1 1 2 8M4 4v6h6" /></svg>
      </button>
    </div>
  </fieldset>
</template>
