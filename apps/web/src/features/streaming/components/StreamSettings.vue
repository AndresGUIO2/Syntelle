<script setup lang="ts">
import { shallowRef, useId, useTemplateRef, watch } from 'vue'
import { useLocaleStore } from '../../../stores/locale'
import type { StreamSettings } from '../types'
import BehaviorSettings from './BehaviorSettings.vue'
import AppearanceSettings from './AppearanceSettings.vue'
import PrivacyStorageNotice from './PrivacyStorageNotice.vue'
const settings = defineModel<StreamSettings>({ required: true })
const props = defineProps<{
  active: boolean
  disabled: boolean
  previewDisabled?: boolean
  laughterBusy?: boolean
  laughterAvailable?: boolean
}>()
const emit = defineEmits<{ preview: [kind: string, amount?: number, repetitions?: number] }>()
const { t } = useLocaleStore()
const selected = shallowRef(0)
const expanded = shallowRef(false)
const scroller = useTemplateRef<HTMLElement>('scroller')
const id = useId()
const tabs = ['Behavior', 'Appearance']
watch(
  () => props.active,
  (active) => {
    if (active) {
      selected.value = 0
      expanded.value = false
      if (scroller.value) scroller.value.scrollTop = 0
    }
  },
)
function select(index: number) {
  selected.value = index
  if (scroller.value) scroller.value.scrollTop = 0
}
function navigate(event: KeyboardEvent, index: number) {
  const next =
    event.key === 'Home'
      ? 0
      : event.key === 'End'
        ? 1
        : event.key === 'ArrowRight' || event.key === 'ArrowLeft'
          ? 1 - index
          : undefined
  if (next === undefined) return
  event.preventDefault()
  select(next)
  if (event.currentTarget instanceof HTMLElement) {
    event.currentTarget.parentElement
      ?.querySelectorAll<HTMLButtonElement>('[role="tab"]')
      [next]?.focus()
  }
}
</script>

<template>
  <section v-show="active" class="customization" :aria-label="t('Customize Syntelle')">
    <div class="customization-tabs" role="tablist" :aria-label="t('Customization sections')">
      <button
        v-for="(tab, index) in tabs"
        :id="`${id}-tab-${index}`"
        :key="tab"
        type="button"
        role="tab"
        :aria-selected="selected === index"
        :aria-controls="`${id}-panel-${index}`"
        :tabindex="selected === index ? 0 : -1"
        @click="select(index)"
        @keydown="navigate($event, index)"
      >
        {{ t(tab) }}
      </button>
    </div>
    <div ref="scroller" class="customization-scroll">
      <div
        v-show="selected === 0"
        :id="`${id}-panel-0`"
        role="tabpanel"
        :aria-labelledby="`${id}-tab-0`"
        tabindex="0"
      >
        <BehaviorSettings
          v-model="settings"
          v-model:expanded="expanded"
          :disabled="disabled"
          :preview-disabled="previewDisabled"
          :laughter-busy="laughterBusy"
          :laughter-available="laughterAvailable"
          @preview="(kind, amount, repetitions) => emit('preview', kind, amount, repetitions)"
        />
      </div>
      <div
        v-show="selected === 1"
        :id="`${id}-panel-1`"
        role="tabpanel"
        :aria-labelledby="`${id}-tab-1`"
        tabindex="0"
      >
        <AppearanceSettings v-model="settings" />
      </div>
      <PrivacyStorageNotice />
    </div>
  </section>
</template>

<style src="./customization.css"></style>
