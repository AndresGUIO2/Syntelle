<script setup lang="ts">
import { computed, useId } from 'vue'
import { useLocaleStore } from '../../../stores/locale'
import type { StreamSettings } from '../types'
import EmotionControls from './EmotionControls.vue'
import AdvancedSettings from './AdvancedSettings.vue'
const settings = defineModel<StreamSettings>({ required: true })
const expanded = defineModel<boolean>('expanded', { required: true })
defineProps<{ disabled: boolean; previewDisabled?: boolean }>()
const emit = defineEmits<{ preview: [kind: string, amount?: number, repetitions?: number] }>()
const { t } = useLocaleStore()
const id = useId()
const regulation = computed(
  () => settings.value.emotion_regulation ?? { enabled: true, temperament: 'calm' as const },
)
const styles = [
  { value: 'calm', label: 'Calm', description: 'Gentler, less frequent reactions.' },
  { value: 'warm', label: 'Warm', description: 'More noticeable reactions with a steady pace.' },
  { value: 'expressive', label: 'Expressive', description: 'Stronger, more frequent reactions.' },
] as const
const selectedDescription = computed(
  () => styles.find((style) => style.value === regulation.value.temperament)!.description,
)
function update<K extends keyof StreamSettings>(key: K, value: StreamSettings[K]) {
  settings.value = { ...settings.value, [key]: value }
}
</script>

<template>
  <div class="customization-section">
    <EmotionControls
      :model-value="settings.emotion_baseline"
      @update:model-value="update('emotion_baseline', $event)"
    />
    <fieldset class="customization-group">
      <legend class="sr-only">{{ t('Automatic reactions') }}</legend>
      <label class="customization-toggle"
        ><span>{{ t('Automatic reactions') }}</span
        ><input
          type="checkbox"
          :aria-label="t('React to the conversation')"
          :checked="settings.automatic_expressions"
          @change="
            update('automatic_expressions', ($event.target as HTMLInputElement).checked)
          " /><span class="customization-toggle-track" aria-hidden="true"
      /></label>
      <label class="customization-field"
        >{{ t('Reaction style') }}
        <select
          :value="regulation.temperament"
          :disabled="!settings.automatic_expressions || !regulation.enabled"
          :title="t(selectedDescription)"
          :aria-describedby="`${id}-reaction-help`"
          @change="
            update('emotion_regulation', {
              ...regulation,
              temperament: ($event.target as HTMLSelectElement)
                .value as typeof regulation.temperament,
            })
          "
        >
          <option v-for="style in styles" :key="style.value" :value="style.value">
            {{ t(style.label) }}
          </option>
        </select>
      </label>
      <p
        v-if="!settings.automatic_expressions || !regulation.enabled"
        :id="`${id}-reaction-help`"
        class="customization-note"
      >
        {{
          !settings.automatic_expressions
            ? t('Enable automatic reactions to change the style.')
            : t('Enable emotion stability in Advanced.')
        }}
      </p>
      <p v-else :id="`${id}-reaction-help`" class="sr-only">{{ t(selectedDescription) }}</p>
    </fieldset>
    <label class="customization-field"
      >{{ t('Character instructions') }}
      <textarea
        :value="settings.instructions"
        rows="2"
        maxlength="2000"
        :disabled="disabled"
        :aria-describedby="disabled ? `${id}-instructions-help` : undefined"
        @input="update('instructions', ($event.target as HTMLTextAreaElement).value)"
      />
    </label>
    <p v-if="disabled" :id="`${id}-instructions-help`" class="customization-note">
      {{
        t(
          'Turn off the microphone and wait for any reply or connection to finish before editing instructions.',
        )
      }}
    </p>
    <div class="customization-advanced">
      <button
        class="customization-disclosure"
        type="button"
        :aria-expanded="expanded"
        :aria-controls="`${id}-advanced`"
        @click="expanded = !expanded"
      >
        <span>{{ t('Advanced') }}</span
        ><span aria-hidden="true">{{ expanded ? '−' : '+' }}</span>
      </button>
      <div v-show="expanded" :id="`${id}-advanced`" class="customization-advanced-content">
        <AdvancedSettings
          v-model="settings"
          :preview-disabled="previewDisabled"
          @preview="(kind, amount, repetitions) => emit('preview', kind, amount, repetitions)"
        />
      </div>
    </div>
  </div>
</template>
