<script setup lang="ts">
import { onScopeDispose, useTemplateRef, watch } from 'vue'
import ConversationIcon from '../../conversation/components/ConversationIcon.vue'
import { demoBackgrounds, demoBackgroundUrl } from '../catalog'
import ExpressionControls from './ExpressionControls.vue'
const open = defineModel<boolean>('open', { required: true })
const background = defineModel<string>('background', { required: true })
const blur = defineModel<number>('blur', { required: true })
const voice = defineModel<string>('voice', { required: true })
const blinking = defineModel<boolean>('blinking', { required: true })
const movement = defineModel<boolean>('movement', { required: true })
const smile = defineModel<number>('smile', { required: true })
const brows = defineModel<number>('brows', { required: true })
const automaticExpressions = defineModel<boolean>('automaticExpressions', { required: true })
const instructions = defineModel<string>('instructions', { default: '' })
const props = defineProps<{
  voiceLocked: boolean
  reducedMotion: boolean
  returnFocus: HTMLElement | null
  expressionsAvailable: boolean
  smileAvailable: boolean
}>()
const dialog = useTemplateRef<HTMLDialogElement>('dialog')
let previousFocus: HTMLElement | null = null
function restore() {
  previousFocus?.focus()
  previousFocus = null
}
watch([open, dialog], ([visible, element]) => {
  if (!element) return
  if (visible && !element.open) {
    previousFocus =
      props.returnFocus ??
      (document.activeElement instanceof HTMLElement ? document.activeElement : null)
    element.show()
  } else if (!visible && element.open) element.close()
})
function closed() {
  open.value = false
  restore()
}
onScopeDispose(() => {
  dialog.value?.close()
  restore()
})
</script>

<template>
  <dialog
    ref="dialog"
    class="customization"
    aria-labelledby="customization-title"
    @close="closed"
  >
    <header class="drawer-header">
      <h2 id="customization-title">Customize</h2>
      <button
        class="workspace-action workspace-action-icon close-button"
        type="button"
        aria-label="Close customization"
        autofocus
        @click="open = false"
      >
        <ConversationIcon name="close" />
      </button>
    </header>
    <div class="drawer-content">
      <section class="settings-section" aria-labelledby="instructions-heading">
        <h3 id="instructions-heading">Character instructions</h3>
        <label class="select-label" for="character-instructions"
          >How should this character respond?</label
        >
        <textarea
          id="character-instructions"
          v-model="instructions"
          class="instructions-input"
          maxlength="2000"
          rows="5"
          :disabled="voiceLocked"
        />
        <p class="setting-note">
          Applies to the next reply. No company knowledge is included automatically.
        </p>
      </section>
      <fieldset class="settings-section">
        <legend>Background</legend>
        <div class="background-options">
          <button
            v-for="item in demoBackgrounds"
            :key="item.id"
            class="background-option"
            type="button"
            :aria-pressed="background === item.id"
            @click="background = item.id"
          >
            <img :src="demoBackgroundUrl(item.thumbnail)" alt="" width="108" height="61" /><span>{{
              item.name
            }}</span>
          </button>
        </div>
        <div class="blur-label">
          <label for="demo-blur">Background blur</label
          ><span>{{ blur === 0 ? 'Off' : `${blur} px` }}</span>
        </div>
        <input id="demo-blur" v-model.number="blur" type="range" min="0" max="20" step="1" />
      </fieldset>
      <section class="settings-section" aria-labelledby="voice-heading">
        <h3 id="voice-heading">Voice</h3>
        <label class="select-label" for="demo-voice">Choose a voice</label
        ><select id="demo-voice" v-model="voice" :disabled="voiceLocked">
          <option value="en-US-Ava:DragonHDLatestNeural">Ava</option>
          <option value="en-US-Andrew:DragonHDLatestNeural">Andrew</option>
        </select>
        <p v-if="voiceLocked" class="setting-note">Stop playback to change the voice.</p>
      </section>
      <fieldset class="settings-section">
        <legend>Animation</legend>
        <label class="check-label"
          ><input v-model="blinking" type="checkbox" :disabled="reducedMotion" />Blinking</label
        >
        <label class="check-label"
          ><input v-model="movement" type="checkbox" :disabled="reducedMotion" />Natural
          movement</label
        >
        <p v-if="reducedMotion" class="setting-note">
          Animation is reduced to match your device settings. Speech remains available.
        </p>
      </fieldset>
      <ExpressionControls
        v-model:smile="smile"
        v-model:brows="brows"
        v-model:automatic="automaticExpressions"
        :available="expressionsAvailable"
        :smile-available="smileAvailable"
        :reduced-motion="reducedMotion"
      />
    </div>
  </dialog>
</template>

<style scoped>
.customization {
  position: fixed;
  inset: 0 0 0 auto;
  width: min(420px, 100%);
  height: 100dvh;
  max-width: 100%;
  max-height: none;
  margin: 0;
  padding: 0;
  border: 0;
  border-left: 1px solid var(--divider);
  background: var(--surface);
  color: var(--text);
  overflow-y: auto;
  overscroll-behavior: contain;
}
.instructions-input {
  width: 100%;
  padding: 10px 12px;
  resize: vertical;
  background: var(--page);
  color: var(--text);
  border: 1px solid var(--control-border);
  border-radius: 10px;
  font: inherit;
  font-size: 16px;
  line-height: 1.5;
}
.drawer-header {
  position: sticky;
  top: 0;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 18px 24px;
  background: var(--surface);
  border-bottom: 1px solid var(--divider);
  z-index: 1;
}
.drawer-header h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  letter-spacing: -0.3px;
}
.close-button {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border: 1px solid transparent;
  border-radius: 10px;
  background: transparent;
  color: var(--text);
}
.drawer-content {
  padding: 0 24px 28px;
}
.settings-section {
  margin: 0;
  padding: 24px 0;
  border: 0;
  border-bottom: 1px solid var(--divider);
  min-width: 0;
}
.settings-section:last-child {
  border-bottom: 0;
}
legend {
  float: left;
  width: 100%;
  padding: 0;
  margin-bottom: 16px;
}
legend,
h3 {
  font-size: 15px;
  font-weight: 600;
}
h3 {
  margin: 0 0 16px;
}
.background-options {
  clear: both;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}
.background-option {
  padding: 3px;
  background: var(--subtle);
  color: var(--muted);
  border: 1px solid transparent;
  border-radius: 10px;
  font-size: 11px;
  line-height: 1.4;
}
.background-option[aria-pressed='true'] {
  border-color: var(--accent);
  color: var(--text);
  background: var(--user-surface);
}
.background-option img {
  display: block;
  width: 100%;
  height: auto;
  aspect-ratio: 16 / 9;
  border-radius: 6px;
  object-fit: cover;
  outline: 1px solid rgb(0 0 0 / 0.1);
  outline-offset: -1px;
}
.background-option span {
  display: block;
  padding: 7px 0 4px;
}
.blur-label {
  display: flex;
  justify-content: space-between;
  margin-top: 24px;
  font-size: 13px;
}
.blur-label span {
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}
input[type='range'] {
  width: 100%;
  min-height: 44px;
  margin: 0;
  accent-color: var(--accent);
}
.select-label {
  display: block;
  margin-bottom: 8px;
  font-size: 13px;
  color: var(--muted);
}
select {
  width: 100%;
  min-height: 44px;
  border: 1px solid var(--control-border);
  border-radius: 10px;
  padding: 10px 12px;
  color: var(--text);
  background: var(--surface);
  font: inherit;
  font-size: 14px;
}
.check-label {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 44px;
  clear: both;
  font-size: 14px;
}
input[type='checkbox'] {
  width: 17px;
  height: 17px;
  margin: 0;
  accent-color: var(--accent);
}
.setting-note {
  font-size: 13px;
  color: var(--muted);
  line-height: 1.6;
  margin: 10px 0 0;
}
input:focus-visible,
select:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 3px;
}
@media (hover: hover) {
  .close-button:hover,
  .background-option:hover {
    background: var(--subtle);
  }
}
@media (max-width: 899px) {
  .customization {
    inset: auto 0 0;
    width: 100%;
    height: auto;
    max-height: 85dvh;
    border: 0;
    border-top: 1px solid var(--divider);
    border-radius: 12px 12px 0 0;
  }
  .drawer-content {
    padding-bottom: max(24px, env(safe-area-inset-bottom));
  }
}
:global(:root[data-theme='dark']) .background-option img {
  outline-color: rgb(255 255 255 / 0.1);
}
</style>
