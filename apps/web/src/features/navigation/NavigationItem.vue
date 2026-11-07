<script setup lang="ts">
import { onMounted, onScopeDispose, shallowRef, useId, watch } from 'vue'
import AppIcon from '../../components/AppIcon.vue'

defineOptions({ inheritAttrs: false })
const props = defineProps<{
  label: string
  href?: string
  icon?: InstanceType<typeof AppIcon>['$props']['name']
  active?: boolean
  compact?: boolean
  popup?: 'dialog'
}>()
const emit = defineEmits<{ click: [event: MouseEvent] }>()
const tooltipId = useId()
const tooltip = shallowRef<{ left: number; top: number } | null>(null)
let anchor: HTMLElement | null = null
function position() {
  if (!anchor || !props.compact) return
  const bounds = anchor.getBoundingClientRect()
  const edge = anchor.closest('.desktop-sidebar')?.getBoundingClientRect().right ?? bounds.right
  tooltip.value = {
    left: Math.min(Math.max(bounds.right, edge) + 12, window.innerWidth - 208),
    top: Math.max(12, Math.min(bounds.top + bounds.height / 2, window.innerHeight - 28)),
  }
}
function show(event: Event) {
  if (!props.compact || !(event.currentTarget instanceof HTMLElement)) return
  if (event instanceof PointerEvent && event.pointerType !== 'mouse') return
  anchor = event.currentTarget
  position()
}
function hide() {
  tooltip.value = null
  anchor = null
}
function leavePointer() {
  if (document.activeElement !== anchor) hide()
}
function viewportChanged() {
  // Keyboard focus may scroll the short sidebar after focus was delivered.
  // Keep its label anchored instead of dismissing it before it can be read.
  if (anchor && document.activeElement === anchor) position()
  else hide()
}
function follow(event: MouseEvent) {
  hide()
  emit('click', event)
}
watch(() => props.compact, hide)
onMounted(() => {
  window.addEventListener('scroll', viewportChanged, true)
  window.addEventListener('resize', viewportChanged)
})
onScopeDispose(() => {
  window.removeEventListener('scroll', viewportChanged, true)
  window.removeEventListener('resize', viewportChanged)
})
</script>

<template>
  <component
    :is="href ? 'a' : 'button'"
    v-bind="$attrs"
    class="nav-item"
    :class="{ compact }"
    :href="href"
    :type="href ? undefined : 'button'"
    :aria-current="active ? 'page' : undefined"
    :aria-label="label"
    :aria-haspopup="popup"
    :aria-describedby="tooltip ? tooltipId : undefined"
    @click="follow"
    @pointerenter="show"
    @pointerleave="leavePointer"
    @focus="show"
    @blur="hide"
    @keydown.esc="hide"
  >
    <slot name="icon"><AppIcon v-if="icon" :name="icon" /></slot>
    <span v-if="!compact" class="item-label"
      ><slot>{{ label }}</slot></span
    >
  </component>
  <Teleport to="body">
    <span
      v-if="tooltip"
      :id="tooltipId"
      role="tooltip"
      class="navigation-tooltip"
      :style="{ left: `${tooltip.left}px`, top: `${tooltip.top}px` }"
      >{{ label }}</span
    >
  </Teleport>
</template>

<style scoped>
.nav-item {
  position: relative;
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  min-height: 44px;
  padding: 11px 12px;
  border: 1px solid transparent;
  border-radius: 8px;
  color: var(--muted);
  background: transparent;
  font: inherit;
  font-size: 14px;
  font-weight: 500;
  line-height: 1.4;
  text-align: start;
  text-decoration: none;
  cursor: pointer;
}
.nav-item :deep(svg) {
  width: 20px;
  height: 20px;
  flex-shrink: 0;
}
.item-label {
  min-width: 0;
  overflow-wrap: anywhere;
}
.nav-item[aria-current='page'] {
  background: var(--navigation-selected, #393047);
  color: var(--navigation-text, #f6f2ff);
  font-weight: 650;
}
.nav-item[aria-current='page']::before {
  content: '';
  position: absolute;
  left: -7px;
  top: 12px;
  bottom: 12px;
  width: 3px;
  border-radius: 3px;
  background: var(--navigation-accent, #bba3f5);
}
.nav-item[aria-current='page'] :deep(svg) {
  color: var(--navigation-accent, #cbb8fa);
}
.nav-item:focus-visible {
  outline: 2px solid var(--navigation-accent, #bba3f5);
  outline-offset: 1px;
}
.compact {
  justify-content: center;
  padding: 11px;
}
.navigation-tooltip {
  position: fixed;
  z-index: 1000;
  transform: translateY(-50%);
  max-width: 196px;
  padding: 9px 12px;
  border: 1px solid var(--navigation-control-border, #6f657d);
  border-radius: 8px;
  color: var(--navigation-text, #f3f0f7);
  background: var(--navigation-tooltip, #25212d);
  box-shadow: 0 6px 24px #0004;
  font:
    500 13px/1.4 'DM Sans Variable',
    sans-serif;
  pointer-events: none;
  overflow-wrap: anywhere;
}
@media (hover: hover) {
  .nav-item:hover {
    background: var(--navigation-hover, #292430);
    color: var(--navigation-text, #f3f0f7);
  }
  .nav-item[aria-current='page']:hover {
    background: var(--navigation-selected-hover, #443951);
  }
}
@media (prefers-reduced-motion: no-preference) {
  .nav-item {
    transition:
      background-color 150ms ease-out,
      color 150ms ease-out;
  }
}
@media (forced-colors: active) {
  .nav-item[aria-current='page'] {
    border-color: Highlight;
  }
}
</style>
