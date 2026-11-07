<script setup lang="ts">
import { computed, onScopeDispose, shallowRef, useTemplateRef } from 'vue'
import { useLocaleStore } from '../../stores/locale'
const props = defineProps<{ busy: boolean; blocked: boolean }>()
const emit = defineEmits<{ submit: [name: string, image: File] }>()
const { t } = useLocaleStore()
const name = shallowRef('')
const file = shallowRef<File | null>(null)
const preview = shallowRef('')
const error = shallowRef('')
const input = useTemplateRef<HTMLInputElement>('input')
const canSubmit = computed(() => name.value.trim() && file.value && !props.busy && !props.blocked)
function choose(event: Event) {
  const selected = (event.target as HTMLInputElement).files?.[0]
  if (preview.value) URL.revokeObjectURL(preview.value)
  file.value = null
  preview.value = ''
  error.value = ''
  if (!selected) return
  if (!['image/jpeg', 'image/png', 'image/webp'].includes(selected.type))
    error.value = 'Choose a JPEG, PNG, or WebP portrait.'
  else if (selected.size > 5_000_000) error.value = 'Choose a portrait no larger than 5 MB.'
  else {
    file.value = selected
    preview.value = URL.createObjectURL(selected)
  }
}
function submit() {
  if (canSubmit.value && file.value) emit('submit', name.value.trim(), file.value)
}
function reset() {
  name.value = ''
  file.value = null
  if (preview.value) URL.revokeObjectURL(preview.value)
  preview.value = ''
  if (input.value) input.value.value = ''
}
defineExpose({ reset })
onScopeDispose(() => {
  if (preview.value) URL.revokeObjectURL(preview.value)
})
</script>

<template>
  <form class="upload-card" @submit.prevent="submit">
    <div class="upload-intro">
      <h2>{{ t('Create a character') }}</h2>
      <p>{{ t('Start with a clear portrait. We will prepare its face, speech and movement.') }}</p>
    </div>
    <div class="portrait-preview">
      <img v-if="preview" :src="preview" :alt="t('Selected portrait')" />
      <div v-else class="portrait-placeholder" aria-hidden="true">
        <svg viewBox="0 0 64 64" fill="none" stroke="currentColor" stroke-width="1.5">
          <circle cx="32" cy="23" r="11" />
          <path d="M13 56v-6a19 19 0 0 1 38 0v6M6 15V6h9M49 6h9v9M6 49v9h9M49 58h9v-9" />
        </svg>
      </div>
    </div>
    <label class="field" for="character-photo"
      >{{ t('Portrait')
      }}<input
        id="character-photo"
        ref="input"
        type="file"
        accept="image/jpeg,image/png,image/webp"
        :disabled="busy || blocked"
        required
        aria-describedby="portrait-help"
        @change="choose"
    /></label>
    <p id="portrait-help" class="field-help">
      {{
        t(
          'One person facing forward, with the full head visible. JPEG, PNG or WebP, up to 5 MB and 4 megapixels. Use a photo you have permission to use.',
        )
      }}
    </p>
    <label class="field" for="character-name"
      >{{ t('Character name')
      }}<input
        id="character-name"
        v-model="name"
        maxlength="100"
        required
        autocomplete="off"
        :disabled="busy || blocked"
        :placeholder="t('Give your character a name')"
    /></label>
    <p v-if="error" class="error" role="alert">{{ t(error) }}</p>
    <button class="primary" type="submit" :disabled="!canSubmit">
      {{ t(busy ? 'Uploading…' : 'Create character') }}
    </button>
    <p class="field-help">
      {{
        t(
          blocked
            ? 'Your current character is still being prepared. You can leave this page and return later.'
            : 'Your character stays private to your account. The uploaded original is removed after 24 hours.',
        )
      }}
    </p>
  </form>
</template>

<style scoped>
.upload-card {
  display: grid;
  gap: 16px;
  padding: 24px;
  border: 1px solid var(--divider);
  border-radius: 20px;
  background: var(--surface);
}
.upload-intro h2 {
  margin: 0 0 8px;
  font-size: 20px;
}
.upload-intro p,
.field-help {
  margin: 0;
  color: var(--muted);
  font-size: 13px;
  line-height: 1.55;
}
.portrait-preview {
  height: 190px;
  overflow: hidden;
  display: grid;
  place-items: center;
  border-radius: 12px;
  background: var(--subtle);
}
.portrait-preview img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  outline: 1px solid oklch(1 0 0 / 0.1);
  outline-offset: -1px;
}
.portrait-placeholder {
  width: 70px;
  color: var(--muted);
}
.field {
  display: grid;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
}
.field input {
  min-width: 0;
  width: 100%;
  min-height: 44px;
  padding: 10px 12px;
  border: 1px solid var(--divider);
  border-radius: 8px;
  background: var(--page);
  color: var(--text);
  font: inherit;
}
.field input[type='file'] {
  font-size: 12px;
  padding: 8px;
}
.field input {
  border-color: var(--control-border);
}
.field input:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 3px;
}
.primary {
  min-height: 44px;
  border: 1px solid transparent;
  border-radius: 10px;
  background: var(--accent);
  color: var(--on-accent, #21192d);
  font: inherit;
  font-weight: 600;
  cursor: pointer;
}
.primary:disabled {
  opacity: 0.5;
  cursor: default;
}
.error {
  color: var(--error);
  font-size: 13px;
}
@media (prefers-reduced-motion: no-preference) {
  .primary {
    transition: scale 150ms ease-out;
  }
  .primary:not(:disabled):active {
    scale: 0.96;
  }
}
</style>
