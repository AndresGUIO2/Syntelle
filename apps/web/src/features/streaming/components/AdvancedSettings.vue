<script setup lang="ts">
import { useLocaleStore } from '../../../stores/locale'
import type { StreamSettings } from '../types'
import ExpressionPreview from './ExpressionPreview.vue'
const settings = defineModel<StreamSettings>({ required: true })
defineProps<{ previewDisabled?: boolean; laughterBusy?: boolean; laughterAvailable?: boolean }>()
const emit = defineEmits<{ preview: [kind: string, amount?: number, repetitions?: number] }>()
const { t } = useLocaleStore()
function update<K extends keyof StreamSettings>(key: K, value: StreamSettings[K]) {
  settings.value = { ...settings.value, [key]: value }
}
</script>

<template>
  <div class="customization-section">
    <div>
      <label class="customization-check"
        ><input
          type="checkbox"
          :checked="settings.emotion_regulation?.enabled ?? true"
          @change="
            update('emotion_regulation', {
              temperament: settings.emotion_regulation?.temperament ?? 'calm',
              enabled: ($event.target as HTMLInputElement).checked,
            })
          "
        />{{ t('Emotion stability') }}</label
      >
      <p class="customization-note">
        {{
          (settings.emotion_regulation?.enabled ?? true)
            ? t('Keeps emotional reactions steady and within temperament limits.')
            : t('The language model directs temporary emotions without stability rules.')
        }}
      </p>
    </div>
    <ExpressionPreview
      :smile="settings.smile"
      :brows="settings.brows"
      :disabled="previewDisabled ?? false"
      :laughter-busy="laughterBusy"
      :laughter-available="laughterAvailable ?? false"
      @update:smile="update('smile', $event)"
      @update:brows="update('brows', $event)"
      @preview="(kind, amount, repetitions) => emit('preview', kind, amount, repetitions)"
    />
  </div>
</template>
