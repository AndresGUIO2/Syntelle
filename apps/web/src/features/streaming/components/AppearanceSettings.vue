<script setup lang="ts">
import { useLocaleStore } from '../../../stores/locale'
import type { StreamSettings } from '../types'
import { backgrounds } from '../settingsCatalog'
const settings = defineModel<StreamSettings>({ required: true })
const { t } = useLocaleStore()
function update<K extends keyof StreamSettings>(key: K, value: StreamSettings[K]) {
  settings.value = { ...settings.value, [key]: value }
}
</script>

<template>
  <div class="customization-section">
    <fieldset class="customization-group">
      <legend>{{ t('Background') }}</legend>
      <div class="customization-backgrounds">
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
      <label class="customization-slider"
        ><span>{{ t('Background blur') }}</span
        ><output>{{ settings.blur }} px</output
        ><input
          type="range"
          :aria-label="t('Background blur')"
          min="0"
          max="20"
          step="1"
          :value="settings.blur"
          @input="update('blur', Number(($event.target as HTMLInputElement).value))"
      /></label>
    </fieldset>
    <fieldset class="customization-group">
      <legend>{{ t('Animation') }}</legend>
      <label class="customization-check"
        ><input
          type="checkbox"
          :checked="settings.blinking"
          :disabled="settings.reduced_motion"
          @change="update('blinking', ($event.target as HTMLInputElement).checked)"
        />{{ t('Natural blinking') }}</label
      >
      <label class="customization-check"
        ><input
          type="checkbox"
          :checked="settings.movement"
          :disabled="settings.reduced_motion"
          @change="update('movement', ($event.target as HTMLInputElement).checked)"
        />{{ t('Head movement and breathing') }}</label
      >
      <p v-if="settings.reduced_motion" class="customization-note">
        {{ t('Autonomous movement is disabled to match your device preferences.') }}
      </p>
    </fieldset>
  </div>
</template>
