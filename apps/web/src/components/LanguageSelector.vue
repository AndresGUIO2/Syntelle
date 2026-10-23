<script setup lang="ts">
import {
  computed,
  nextTick,
  onMounted,
  onScopeDispose,
  shallowRef,
  useId,
  useTemplateRef,
} from 'vue'
import { useLocaleStore, type PlatformLocale } from '../stores/locale'
import AppIcon from './AppIcon.vue'

const language = useLocaleStore()
const open = shallowRef(false)
const picker = useTemplateRef<HTMLElement>('picker')
const trigger = useTemplateRef<HTMLButtonElement>('trigger')
const menu = useTemplateRef<HTMLElement>('menu')
const menuId = useId()
const choices = [
  { id: 'en', name: 'English' },
  { id: 'es', name: 'Español' },
] as const
const selected = computed(() => choices.find((choice) => choice.id === language.locale)!)
const label = computed(() => `${language.t('Platform language')}: ${selected.value.name}`)

function items() {
  return Array.from(menu.value?.querySelectorAll<HTMLButtonElement>('[role="menuitemradio"]') ?? [])
}
async function openMenu(index = choices.findIndex((choice) => choice.id === language.locale)) {
  open.value = true
  await nextTick()
  items()[index]?.focus()
}
function closeMenu(restoreFocus = false) {
  open.value = false
  if (restoreFocus) trigger.value?.focus()
}
function choose(locale: PlatformLocale) {
  language.locale = locale
  closeMenu(true)
}
function navigate(event: KeyboardEvent) {
  if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
  event.preventDefault()
  const options = items()
  const index = options.indexOf(document.activeElement as HTMLButtonElement)
  const next =
    event.key === 'Home'
      ? 0
      : event.key === 'End'
        ? options.length - 1
        : (index + (event.key === 'ArrowDown' ? 1 : -1) + options.length) % options.length
  options[next]?.focus()
}
function outside(event: PointerEvent) {
  if (event.target instanceof Node && !picker.value?.contains(event.target)) closeMenu()
}
function focusLeft(event: FocusEvent) {
  if (!(event.relatedTarget instanceof Node) || !picker.value?.contains(event.relatedTarget))
    closeMenu()
}
onMounted(() => document.addEventListener('pointerdown', outside))
onScopeDispose(() => document.removeEventListener('pointerdown', outside))
</script>

<template>
  <div
    ref="picker"
    class="language-picker"
    @focusout="focusLeft"
    @keydown.esc.stop.prevent="closeMenu(true)"
  >
    <button
      ref="trigger"
      class="language-trigger"
      type="button"
      :aria-label="label"
      :title="language.t('Platform language')"
      aria-haspopup="menu"
      :aria-expanded="open"
      :aria-controls="open ? menuId : undefined"
      @click="open ? closeMenu() : openMenu()"
      @keydown.down.prevent="openMenu()"
      @keydown.up.prevent="openMenu(choices.length - 1)"
    >
      <AppIcon name="globe" class="language-globe" />
      <span :lang="selected.id">{{ selected.name }}</span>
      <AppIcon name="chevron-down" class="language-chevron" :class="{ expanded: open }" />
    </button>
    <div
      v-if="open"
      :id="menuId"
      ref="menu"
      class="language-menu"
      role="menu"
      :aria-label="language.t('Platform language')"
      @keydown="navigate"
    >
      <button
        v-for="choice in choices"
        :key="choice.id"
        class="language-option"
        type="button"
        role="menuitemradio"
        :aria-checked="choice.id === language.locale"
        :lang="choice.id"
        tabindex="-1"
        @click="choose(choice.id)"
      >
        <span>{{ choice.name }}</span>
        <AppIcon v-if="choice.id === language.locale" name="check" class="language-check" />
      </button>
    </div>
  </div>
</template>

<style scoped>
.language-picker {
  position: relative;
  z-index: 5;
  flex-shrink: 0;
}
.language-trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 44px;
  gap: 8px;
  padding: 8px 10px;
  border: 1px solid transparent;
  border-radius: 6px;
  color: var(--muted);
  background: transparent;
  font: inherit;
  font-size: 13px;
  font-weight: 500;
}
.language-trigger[aria-expanded='true'] {
  color: var(--text);
  background: var(--subtle);
}
.language-trigger .language-globe {
  width: 16px;
  height: 16px;
}
.language-trigger .language-chevron {
  width: 12px;
  height: 12px;
}
.language-chevron.expanded {
  transform: rotate(180deg);
}
.language-menu {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  min-width: 176px;
  padding: 4px;
  border: 1px solid var(--divider);
  border-radius: 9px;
  background: var(--surface-raised);
  color: var(--text);
  box-shadow: 0 2px 8px #0000001a;
}
.language-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  width: 100%;
  min-height: 44px;
  padding: 10px 12px;
  border: 1px solid transparent;
  border-radius: 4px;
  color: inherit;
  background: transparent;
  font: inherit;
  font-size: 13px;
  text-align: start;
}
.language-option[aria-checked='true'] {
  background: var(--subtle);
}
.language-option:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: -2px;
}
.language-option .language-check {
  width: 16px;
  height: 16px;
  color: var(--accent);
}
@media (hover: hover) {
  .language-trigger:hover {
    color: var(--text);
    background: var(--subtle);
  }
  .language-option:hover {
    background: var(--subtle);
  }
}
@media (prefers-reduced-motion: no-preference) {
  .language-trigger {
    transition:
      background-color 150ms ease-out,
      color 150ms ease-out,
      transform 150ms ease-out;
  }
  .language-trigger:active {
    transform: scale(0.96);
  }
  .language-chevron {
    transition: transform 150ms ease-out;
  }
}
</style>
