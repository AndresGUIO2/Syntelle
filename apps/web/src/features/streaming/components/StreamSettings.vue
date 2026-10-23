<script setup lang="ts">
import { useLocaleStore } from '../../../stores/locale'
import type { StreamSettings } from '../types'
import { backgrounds } from '../settingsCatalog'
import ExpressionPreview from './ExpressionPreview.vue'
import PrivacyStorageNotice from './PrivacyStorageNotice.vue'
const { t } = useLocaleStore()
const settings = defineModel<StreamSettings>({ required: true })
defineProps<{ disabled: boolean; previewDisabled?: boolean }>()
const emit = defineEmits<{ preview: [kind: string, amount?: number, repetitions?: number] }>()
function update<K extends keyof StreamSettings>(key: K, value: StreamSettings[K]) {
  settings.value = { ...settings.value, [key]: value }
}
</script>

<template>
  <div class="stream-settings">
    <label class="field">
      {{ t('Character instructions') }}
      <textarea
        :value="settings.instructions"
        maxlength="2000"
        rows="4"
        :disabled="disabled"
        @input="update('instructions', ($event.target as HTMLTextAreaElement).value)"
      />
    </label>
    <fieldset>
      <legend>{{ t('Background') }}</legend>
      <div class="backgrounds">
        <button
          v-for="item in backgrounds"
          :key="item.id"
          type="button"
          :aria-pressed="settings.background === item.id"
          @click="update('background', item.id)"
        >
          <img :src="item.thumbnail" alt="" width="108" height="61" /><span>{{
            t(item.name)
          }}</span>
        </button>
      </div>
      <label class="field blur">
        {{ t('Blur') }} <span>{{ settings.blur }} px</span
        ><input
          :value="settings.blur"
          type="range"
          min="0"
          max="20"
          step="1"
          @input="update('blur', Number(($event.target as HTMLInputElement).value))"
      /></label>
    </fieldset>
    <fieldset>
      <legend>{{ t('Animation') }}</legend>
      <label class="check"
        ><input
          :checked="settings.blinking"
          type="checkbox"
          :disabled="settings.reduced_motion"
          @change="update('blinking', ($event.target as HTMLInputElement).checked)"
        />
        {{ t('Natural blinking') }}
      </label>
      <label class="check"
        ><input
          :checked="settings.movement"
          type="checkbox"
          :disabled="settings.reduced_motion"
          @change="update('movement', ($event.target as HTMLInputElement).checked)"
        />
        {{ t('Head movement and breathing') }}
      </label>
      <label class="check"
        ><input
          :checked="settings.automatic_expressions"
          type="checkbox"
          @change="update('automatic_expressions', ($event.target as HTMLInputElement).checked)"
        />
        {{ t('Expressions during speech') }}
      </label>
      <p v-if="settings.reduced_motion" class="note">
        {{ t('Autonomous movement is disabled to match your device preferences.') }}
      </p>
    </fieldset>
    <ExpressionPreview
      :smile="settings.smile"
      :brows="settings.brows"
      :disabled="previewDisabled"
      @update:smile="update('smile', $event)"
      @update:brows="update('brows', $event)"
      @preview="(kind, amount, repetitions) => emit('preview', kind, amount, repetitions)"
    />
    <PrivacyStorageNotice />
  </div>
</template>

<style scoped>
.stream-settings {
  display: grid;
  gap: 22px;
}
.field {
  display: grid;
  gap: 8px;
  font-size: 13px;
  font-weight: 500;
}
textarea {
  width: 100%;
  min-height: 44px;
  padding: 10px 12px;
  background: var(--page);
  color: var(--text);
  border: 1px solid var(--control-border);
  border-radius: 7px;
  font: inherit;
}
textarea {
  resize: vertical;
}
fieldset {
  margin: 0;
  padding: 0;
  border: 0;
}
legend {
  margin-bottom: 12px;
  font-size: 14px;
  font-weight: 600;
}
.backgrounds {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}
.backgrounds button {
  padding: 3px;
  overflow: hidden;
  color: var(--text);
  background: var(--surface);
  border: 1px solid var(--divider);
  border-radius: 7px;
  cursor: pointer;
}
.backgrounds button[aria-pressed='true'] {
  border-color: var(--accent);
  outline: 1px solid var(--accent);
}
.backgrounds img {
  display: block;
  width: 100%;
  height: auto;
  border-radius: 4px;
}
.backgrounds span {
  display: block;
  padding: 8px 2px;
  font-size: 11px;
}
.blur {
  margin-top: 16px;
  grid-template-columns: 1fr auto;
}
.blur input {
  grid-column: 1 / -1;
}
.check {
  display: flex;
  align-items: center;
  min-height: 40px;
  gap: 8px;
  font-size: 13px;
}
.note {
  font-size: 12px;
  color: var(--muted);
  line-height: 1.5;
}
input[type='range'] {
  width: 100%;
  accent-color: var(--accent);
}
</style>
