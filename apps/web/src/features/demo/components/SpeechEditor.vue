<script setup lang="ts">
import { computed } from 'vue'
import AppIcon from '../../../components/AppIcon.vue'
const text = defineModel<string>('text', { required: true })
const props = defineProps<{
  ready: boolean
  checking: boolean
  configured: boolean
  loading: boolean
  active: boolean
  paused: boolean
  characterError: boolean
  error: string
}>()
const emit = defineEmits<{ speak: []; pause: []; resume: []; stop: [] }>()
const status = computed(() => {
  if (props.error) return ''
  if (props.characterError) return 'Reload this page to try again.'
  if (props.checking) return 'Checking voice availability…'
  if (!props.configured)
    return 'Voice playback is unavailable right now. You can still customize your character.'
  if (!props.ready) return 'Loading your character…'
  if (props.loading) return 'Preparing your audio…'
  if (props.paused && props.active) return 'Playback paused'
  if (props.active) return 'Speaking'
  return 'Ready when you are'
})
function submit() {
  if (props.ready && props.configured && !props.loading && !props.active && text.value.trim())
    emit('speak')
}
</script>

<template>
  <section class="speech-editor" aria-labelledby="editor-title">
    <div class="editor-heading">
      <h2 id="editor-title">Write and listen</h2>
      <p>Write a short text for your character to say.</p>
    </div>
    <form class="speech-form" @submit.prevent="submit">
      <div class="text-label">
        <label for="demo-text">Text to speak</label
        ><span id="text-count">{{ 500 - text.length }} characters left</span>
      </div>
      <textarea
        id="demo-text"
        v-model="text"
        maxlength="500"
        rows="7"
        aria-describedby="text-count"
        placeholder="What would you like your character to say?"
        @keydown.ctrl.enter.prevent="submit"
        @keydown.meta.enter.prevent="submit"
      />
      <div class="editor-options">
        <span class="keyboard-hint">Ctrl / ⌘ + Enter to speak</span>
      </div>
      <div class="playback-controls">
        <button
          v-if="!active"
          class="button button-primary speak-button"
          type="submit"
          :disabled="!ready || !configured || checking || loading || !text.trim()"
        >
          <AppIcon name="play" />{{ loading ? 'Preparing…' : 'Speak' }}
        </button>
        <button
          v-else
          class="button button-primary speak-button"
          type="button"
          @click="paused ? emit('resume') : emit('pause')"
        >
          <AppIcon :name="paused ? 'play' : 'pause'" />{{ paused ? 'Resume' : 'Pause' }}
        </button>
        <button
          v-if="active || loading"
          class="button stop-button"
          type="button"
          @click="emit('stop')"
        >
          <AppIcon name="stop" />Stop
        </button>
      </div>
      <p v-if="error" class="playback-status playback-error" role="alert">{{ error }}</p>
      <p v-else class="playback-status" role="status">{{ status }}</p>
    </form>
  </section>
</template>

<style scoped>
.speech-editor {
  padding: 24px;
  display: flex;
  flex-direction: column;
  min-height: 0;
  min-width: 0;
  height: 100%;
}
.editor-heading {
  margin-bottom: 20px;
}
.editor-heading h2 {
  margin: 0 0 8px;
  max-width: 360px;
  font-size: 22px;
  line-height: 1.2;
  letter-spacing: -0.4px;
  font-weight: 600;
}
.editor-heading p {
  margin: 0;
  font-size: 15px;
  color: var(--muted);
}
.speech-form {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}
.text-label {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 10px;
  margin-bottom: 10px;
  font-size: 13px;
}
.text-label label {
  font-weight: 600;
}
.text-label span {
  color: var(--muted);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}
textarea {
  resize: vertical;
  min-height: 100px;
  flex: 1;
  width: 100%;
  padding: 14px;
  border: 1px solid var(--control-border);
  border-radius: 6px;
  color: var(--text);
  background: var(--page);
  font: inherit;
  line-height: 1.65;
}
textarea::placeholder {
  color: var(--muted);
  opacity: 1;
}
textarea:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 3px;
}
.editor-options {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  margin-top: 14px;
}
.keyboard-hint {
  font-size: 12px;
  color: var(--muted);
  padding-bottom: 12px;
}
.playback-controls {
  display: flex;
  gap: 10px;
  margin-top: 16px;
}
.speak-button {
  min-width: 136px;
  min-height: 44px;
}
.button:disabled {
  opacity: 0.55;
}
.playback-status {
  color: var(--muted);
  font-size: 13px;
  margin: 14px 0 0;
  min-height: 20px;
}
.playback-error {
  color: var(--error);
}
@media (max-width: 899px) {
  .speech-editor {
    padding: 24px 20px;
  }
  .editor-heading {
    margin-bottom: 24px;
  }
  .editor-heading h2 {
    max-width: none;
    font-size: 22px;
  }
  textarea {
    min-height: 150px;
    height: 156px;
    max-height: none;
    flex: auto;
    font-size: 16px;
  }
  .keyboard-hint {
    display: none;
  }
  .editor-options {
    margin-top: 14px;
  }
  .playback-controls {
    margin-top: 20px;
  }
  .playback-status {
    min-height: 20px;
  }
}
@media (prefers-reduced-motion: no-preference) {
  .button:not(:disabled):active {
    transform: scale(0.96);
    transition: transform 150ms ease-out;
  }
}
</style>
