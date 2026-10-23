<script setup lang="ts">
import { useLocaleStore } from '../../../stores/locale'
import { computed, useTemplateRef } from 'vue'
import ConversationIcon from './ConversationIcon.vue'

const { t } = useLocaleStore()
const props = defineProps<{
  active: boolean
  disabled: boolean
  starting: boolean
  state: 'off' | 'starting' | 'listening' | 'preparing' | 'speaking' | 'error'
  inputLevel: number
}>()
defineEmits<{ toggle: [] }>()
const button = useTemplateRef<HTMLButtonElement>('button')
const level = computed(() => (props.active ? Math.min(1, Math.max(0, props.inputLevel)) : 0))
const haloStyle = computed(() => ({ '--input-level': level.value }))
defineExpose({ focus: () => button.value?.focus({ preventScroll: true }) })
</script>

<template>
  <div class="microphone-control" :data-state="state" :data-capture="active" :style="haloStyle">
    <span class="microphone-halo" aria-hidden="true" />
    <span class="microphone-orbit" aria-hidden="true" />
    <span class="microphone-progress" aria-hidden="true" />
    <button
      ref="button"
      class="microphone-button"
      type="button"
      :aria-label="t(active ? 'Microphone on' : 'Enable microphone')"
      :aria-pressed="active"
      :aria-busy="starting"
      :title="
        t(starting ? 'Starting microphone…' : active ? 'Turn microphone off' : 'Start talking')
      "
      :disabled="disabled"
      @click="$emit('toggle')"
    >
      <ConversationIcon :name="active || starting ? 'mic' : 'mic-off'" />
    </button>
  </div>
</template>

<style scoped>
.microphone-control {
  position: relative;
  isolation: isolate;
  width: 48px;
  height: 48px;
}
.microphone-button {
  display: grid;
  place-items: center;
  position: relative;
  width: 48px;
  height: 48px;
  padding: 0;
  color: #21192d;
  border: 1px solid var(--control-border);
  border-radius: 50%;
  background: linear-gradient(145deg, #fff 25%, #f8f5ff);
  box-shadow:
    0 6px 18px #22103926,
    0 0 24px #a17be847,
    inset 0 2px 1px #fff;
}
.microphone-button .conversation-icon {
  width: 24px;
  height: 24px;
}
.microphone-button:disabled {
  opacity: 0.6;
}
[data-capture='false']:not([data-state='starting']) .microphone-button {
  color: var(--muted);
  background: var(--surface);
  box-shadow: none;
}
[data-capture='false']:not([data-state='starting']) .microphone-halo {
  opacity: 0;
}
[data-capture='true'] .microphone-button {
  color: #21192d;
  background: #c4aaf5;
  border-color: #c4aaf5;
}
.microphone-halo {
  position: absolute;
  z-index: -1;
  pointer-events: none;
  inset: -12px;
  border-radius: 50%;
  background:
    radial-gradient(ellipse at 28% 35%, #9b5be4a6, transparent 65%),
    radial-gradient(ellipse at 75% 65%, #c1a3ff99, transparent 65%);
  mask-image: radial-gradient(closest-side, #000 50%, transparent 100%);
  opacity: 0.36;
}
.microphone-orbit {
  position: absolute;
  pointer-events: none;
  inset: -5px;
  border: 1px solid var(--orb-ring);
  border-radius: 50%;
  box-shadow: 0 0 0 3px var(--orb-wash);
}
[data-capture='true']:not([data-state='error']) .microphone-halo {
  opacity: calc(0.4 + var(--input-level) * 0.6);
}
[data-capture='true'] .microphone-orbit {
  border-color: var(--accent);
}
.microphone-progress {
  position: absolute;
  pointer-events: none;
  inset: -5px;
  border: 2px solid transparent;
  border-radius: 50%;
}
[data-state='starting'] .microphone-progress,
[data-state='preparing'] .microphone-progress {
  border-top-color: var(--accent);
  border-right-color: var(--accent);
}
[data-state='speaking'] .microphone-progress {
  border-left-color: var(--accent);
  border-right-color: var(--accent);
}
[data-state='error'] .microphone-progress {
  border-color: var(--error);
}
@media (hover: hover) {
  [data-capture='false'] .microphone-button:enabled:hover {
    background: var(--subtle);
  }
  [data-capture='true'] .microphone-button:enabled:hover {
    background: #d1bdf7;
    box-shadow:
      0 6px 22px #22103933,
      0 0 30px #a17be866,
      inset 0 2px 1px #fff;
  }
}
@media (prefers-reduced-motion: no-preference) {
  .microphone-halo {
    transition:
      transform 120ms cubic-bezier(0.2, 0, 0, 1),
      opacity 120ms cubic-bezier(0.2, 0, 0, 1);
  }
  [data-capture='true']:not([data-state='error']) .microphone-halo {
    transform: scale(calc(1 + var(--input-level) * 0.45));
  }
  .microphone-button {
    transition:
      transform 150ms ease-out,
      box-shadow 150ms ease-out;
  }
  .microphone-button:enabled:active {
    transform: scale(0.96);
  }
  [data-state='starting'] .microphone-progress {
    animation: progress 1.8s linear infinite;
  }
  [data-state='preparing'] .microphone-progress {
    animation: progress 3.6s linear infinite;
  }
}
@media (prefers-reduced-motion: reduce) {
  [data-capture='true']:not([data-state='error']) .microphone-halo {
    opacity: 0.6;
  }
}
@media (forced-colors: active) {
  .microphone-button {
    border-color: ButtonText;
    color: ButtonText;
  }
  [data-capture='true'] .microphone-orbit {
    border-color: Highlight;
  }
}
@keyframes progress {
  to {
    transform: rotate(360deg);
  }
}
</style>
