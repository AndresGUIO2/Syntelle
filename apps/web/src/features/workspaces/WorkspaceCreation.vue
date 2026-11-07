<script setup lang="ts">
import { shallowRef, useId } from 'vue'
import { useLocaleStore } from '../../stores/locale'

withDefaults(defineProps<{ busy: boolean; error?: string; dismissible?: boolean }>(), {
  error: '',
  dismissible: false,
})
const emit = defineEmits<{ create: [name: string]; cancel: [] }>()
const { t } = useLocaleStore()
const name = shallowRef('')
const id = useId()
</script>

<template>
  <section class="workspace-creation">
    <header>
      <h2>{{ t('Create a workspace') }}</h2>
      <p>{{ t('Group website connections, shared access and usage under one project.') }}</p>
    </header>
    <form
      class="creation-form"
      :aria-busy="busy"
      @submit.prevent="name.trim() && emit('create', name.trim())"
    >
      <label :for="id">{{ t('Workspace name') }}</label>
      <input
        :id="id"
        v-model="name"
        maxlength="100"
        required
        autocomplete="organization"
        :disabled="busy"
        :placeholder="t('Your business or project name')"
      />
      <p v-if="error" class="creation-error" role="alert">{{ t(error) }}</p>
      <div class="creation-actions">
        <button class="console-button primary" type="submit" :disabled="busy || !name.trim()">
          {{ t(busy ? 'Creating workspace…' : 'Create workspace') }}</button
        ><button
          v-if="dismissible"
          class="console-button quiet"
          type="button"
          :disabled="busy"
          @click="emit('cancel')"
        >
          {{ t('Cancel') }}
        </button>
      </div>
    </form>
    <p class="privacy-note">
      {{ t('Your personal characters stay private. You choose what to share.') }}
    </p>
  </section>
</template>
<style scoped>
.workspace-creation {
  padding: 32px;
  color: var(--text);
  width: 100%;
  min-width: 0;
  max-width: 520px;
}
.workspace-creation h2 {
  margin: 0 0 12px;
  font-size: 24px;
  letter-spacing: -0.5px;
  line-height: 1.25;
}
.workspace-creation header p {
  margin: 0 0 28px;
  color: var(--muted);
  font-size: 15px;
  line-height: 1.6;
}
.creation-form {
  display: grid;
  gap: 10px;
}
.creation-form label {
  font-size: 14px;
  font-weight: 600;
}
.creation-form input {
  width: 100%;
  min-width: 0;
  min-height: 48px;
  padding: 12px;
  border: 1px solid var(--control-border);
  border-radius: 8px;
  background: var(--page);
  color: var(--text);
  font: inherit;
}
.creation-form input:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 3px;
}
.creation-actions {
  display: flex;
  gap: 12px;
  margin-top: 14px;
  flex-wrap: wrap;
}
.creation-error {
  color: var(--error);
  font-size: 14px;
  line-height: 1.5;
  overflow-wrap: anywhere;
}
.privacy-note {
  margin: 24px 0 0;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.6;
}
@media (max-width: 480px) {
  .workspace-creation {
    padding: 24px;
  }
}
</style>
