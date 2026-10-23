<script setup lang="ts">
import type { useSpeechPlayback } from '../composables/useSpeechPlayback'
defineProps<{ speech: ReturnType<typeof useSpeechPlayback>; ready: boolean }>()
const text = defineModel<string>('text', { required: true })
const language = defineModel<string>('language', { required: true })
const voice = defineModel<string>('voice', { required: true })
</script>

<template>
  <section class="speech-controls" aria-labelledby="speech-label">
    <h3 id="speech-label">Dragon HD speech</h3>
    <p v-if="speech.checking.value" role="status">Checking speech availability…</p>
    <p v-else-if="!speech.configured.value" class="note">
      Azure Speech is not configured. Manual animation is available.
    </p>
    <label
      >Text to speak<textarea
        v-model="text"
        maxlength="500"
        rows="3"
        :disabled="speech.loading.value || speech.active.value"
      />
    </label>
    <div class="fields">
      <label
        >Speech language<select
          v-model="language"
          :disabled="speech.loading.value || speech.active.value"
        >
          <option value="en-US">English</option>
          <option value="es-CO">Spanish</option>
        </select></label
      >
      <label
        >Dragon HD voice<select
          v-model="voice"
          :disabled="speech.loading.value || speech.active.value"
        >
          <option value="en-US-Ava:DragonHDLatestNeural">Ava</option>
          <option value="en-US-Andrew:DragonHDLatestNeural">Andrew</option>
        </select></label
      >
    </div>
    <div class="actions">
      <button
        class="button"
        :disabled="
          !ready ||
          !speech.configured.value ||
          speech.loading.value ||
          speech.active.value ||
          !speech.text.value.trim()
        "
        @click="speech.speak"
      >
        {{ speech.loading.value ? 'Preparing speech…' : 'Speak' }}
      </button>
      <button
        v-if="speech.active.value"
        class="button"
        @click="speech.paused.value ? speech.resume() : speech.pause()"
      >
        {{ speech.paused.value ? 'Resume speech' : 'Pause speech' }}
      </button>
      <button
        class="button"
        :disabled="!speech.active.value && !speech.loading.value"
        @click="speech.stop"
      >
        Stop speech
      </button>
    </div>
    <p v-if="speech.error.value" role="alert">{{ speech.error.value }}</p>
  </section>
</template>

<style scoped>
.speech-controls {
  border-top: 1px solid var(--control-border);
  margin-top: 24px;
  padding-top: 20px;
}
h3 {
  margin: 0 0 16px;
  font-size: 16px;
}
label {
  display: block;
  font-size: 14px;
}
textarea,
select {
  display: block;
  width: 100%;
  margin-top: 8px;
  padding: 10px;
  border: 1px solid var(--control-border);
  border-radius: 6px;
  color: var(--text);
  background: var(--surface);
  font: inherit;
}
textarea {
  resize: vertical;
  box-sizing: border-box;
}
.fields {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  margin: 12px 0;
}
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.note {
  color: var(--muted);
  font-size: 13px;
}
</style>
