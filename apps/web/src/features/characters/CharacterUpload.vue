<script setup lang="ts">
import { computed, onScopeDispose, shallowRef, useId, useTemplateRef } from 'vue'
import { useLocaleStore } from '../../stores/locale'
import AppIcon from '../../components/AppIcon.vue'
import { clearCharacterDraft, inspectPortrait, useCharacterJourney } from './useCharacterJourney'
const props = withDefaults(
  defineProps<{ busy: boolean; blocked: boolean; unavailable?: boolean; ownerId?: string }>(),
  { ownerId: '', unavailable: false },
)
const emit = defineEmits<{ submit: [name: string, image: File] }>()
const { t } = useLocaleStore()
const draft = useCharacterJourney(props.ownerId)
const input = useTemplateRef<HTMLInputElement>('input')
const heading = useTemplateRef<HTMLElement>('heading')
const dragging = shallowRef(false)
const id = useId()
let selection = 0
let disposed = false
const locked = computed(() => props.busy || props.blocked || props.unavailable)
const canReview = computed(() =>
  Boolean(draft.image && !draft.error && !draft.checking && !locked.value),
)
const canSubmit = computed(() => canReview.value && draft.name.trim().length > 0)
async function choose(file?: File) {
  if (!file || locked.value) return
  const revision = ++selection
  draft.checking = true
  draft.error = ''
  try {
    const dimensions = await inspectPortrait(file)
    if (disposed || !draft.valid || revision !== selection) return
    if (draft.preview) URL.revokeObjectURL(draft.preview)
    draft.image = file
    draft.preview = URL.createObjectURL(file)
    draft.width = dimensions.width
    draft.height = dimensions.height
    draft.pending = undefined
    draft.submitError = ''
  } catch (failure) {
    if (!disposed && draft.valid && revision === selection)
      draft.error =
        failure instanceof Error
          ? failure.message
          : 'The portrait could not be opened. Choose another image.'
  } finally {
    if (!disposed && draft.valid && revision === selection) draft.checking = false
    if (input.value) input.value.value = ''
  }
}
function drop(event: DragEvent) {
  dragging.value = false
  if (event.dataTransfer?.files.length !== 1) {
    draft.error = 'Choose one portrait at a time.'
    return
  }
  void choose(event.dataTransfer.files[0])
}
function step(value: 'portrait' | 'review') {
  if (props.busy || (value === 'review' && !canReview.value)) return
  draft.step = value
  heading.value?.focus()
}
function submit() {
  if (canSubmit.value && draft.image) emit('submit', draft.name.trim(), draft.image)
}
function reset() {
  clearCharacterDraft(draft)
}
defineExpose({ reset })
onScopeDispose(() => {
  disposed = true
  selection++
  draft.checking = false
})
</script>

<template>
  <section class="character-creator" :aria-label="t('Character preparation')">
    <ol class="creation-steps" :aria-label="t('Creation steps')">
      <li>
        <button
          type="button"
          :aria-current="draft.step === 'portrait' ? 'step' : undefined"
          :class="{ active: draft.step === 'portrait' }"
          :disabled="busy"
          @click="step('portrait')"
        >
          <span>1</span>{{ t('Portrait') }}<AppIcon v-if="draft.image" name="check" />
        </button>
      </li>
      <li>
        <button
          type="button"
          :aria-current="draft.step === 'review' ? 'step' : undefined"
          :class="{ active: draft.step === 'review' }"
          :disabled="!canReview"
          @click="step('review')"
        >
          <span>2</span>{{ t('Review and create') }}
        </button>
      </li>
    </ol>
    <div class="creator-layout">
      <div
        class="portrait-stage"
        :class="{ dragging, 'has-portrait': draft.preview }"
        @dragover.prevent="dragging = !locked"
        @dragleave.prevent="dragging = false"
        @drop.prevent="drop"
      >
        <img v-if="draft.preview" :src="draft.preview" :alt="t('Selected portrait')" />
        <div v-else class="portrait-empty">
          <AppIcon name="image" />
          <span>{{ t('Drop one image here, or choose a photo below.') }}</span>
          <button
            class="console-button"
            type="button"
            :disabled="locked || draft.checking"
            @click="input?.click()"
          >
            <AppIcon name="upload" />{{ t('Choose photo') }}
          </button>
        </div>
        <button
          v-if="draft.image && draft.step === 'portrait'"
          class="console-button replace-photo"
          type="button"
          :disabled="locked || draft.checking"
          @click="input?.click()"
        >
          <AppIcon name="image" />{{ t('Replace photo') }}
        </button>
        <span v-if="draft.image" class="portrait-file">{{ draft.image.name }}</span>
      </div>
      <form
        class="creator-form"
        @submit.prevent="draft.step === 'portrait' ? step('review') : submit()"
      >
        <div>
          <span class="step-caption">{{
            t(draft.step === 'portrait' ? 'Step 1 of 2' : 'Step 2 of 2')
          }}</span>
          <h2 ref="heading" tabindex="-1">
            {{
              t(draft.step === 'portrait' ? 'Choose a clear portrait' : 'Meet your new character')
            }}
          </h2>
        </div>
        <template v-if="draft.step === 'portrait'">
          <p class="creator-intro">
            {{
              t('Use a front-facing photo of one person, with the full head and shoulders visible.')
            }}
          </p>
          <ul class="portrait-tips">
            <li>{{ t('Keep the eyes and mouth unobstructed.') }}</li>
            <li>{{ t('Choose even lighting and a clear face.') }}</li>
            <li>{{ t('Use a portrait you have permission to use.') }}</li>
          </ul>
          <input
            :id="id"
            ref="input"
            class="file-input"
            type="file"
            accept="image/jpeg,image/png,image/webp"
            :disabled="locked"
            :aria-label="t('Portrait')"
            @change="choose(($event.target as HTMLInputElement).files?.[0])"
          />
          <p class="format-note">
            {{ t('JPEG, PNG or WebP · up to 5 MB and 4 megapixels · maximum 4096 px per side') }}
          </p>
        </template>
        <template v-else>
          <p class="creator-intro">
            {{ t('Give it a name, then confirm to start preparing speech and movement.') }}
          </p>
          <label class="console-field" :for="`${id}-name`"
            >{{ t('Character name')
            }}<input
              :id="`${id}-name`"
              v-model="draft.name"
              maxlength="100"
              autocomplete="off"
              required
              :disabled="locked"
              :readonly="Boolean(draft.pending)"
              :placeholder="t('Give your character a name')"
          /></label>
          <div class="portrait-summary">
            <AppIcon name="image" />
            <div>
              <strong>{{ draft.image?.name }}</strong
              ><small
                >{{ draft.width }} × {{ draft.height }} px ·
                {{ t('Original photo preserved') }}</small
              >
            </div>
          </div>
          <p class="privacy-note">
            <AppIcon name="shield" />{{
              t('Private to your account. Connecting it to a website is a separate choice.')
            }}
          </p>
          <p class="format-note">{{ t('The uploaded original is removed after 24 hours.') }}</p>
        </template>
        <p v-if="draft.checking" class="format-note" role="status">
          {{ t('Checking your photo…') }}
        </p>
        <p v-if="draft.error || draft.submitError" class="console-notice error" role="alert">
          {{ t(draft.error || draft.submitError) }}
        </p>
        <p v-if="blocked" class="console-notice">
          {{
            t(
              'Your current character is still being prepared. You can leave this page and return later.',
            )
          }}
        </p>
        <div class="creator-actions">
          <button
            v-if="draft.step === 'review'"
            class="console-button quiet"
            type="button"
            :disabled="busy"
            @click="step('portrait')"
          >
            <AppIcon name="chevron-left" />{{ t('Change portrait') }}
          </button>
          <button
            class="console-button primary"
            type="submit"
            :disabled="draft.step === 'portrait' ? !canReview : !canSubmit"
          >
            {{
              t(busy ? 'Uploading…' : draft.step === 'portrait' ? 'Continue' : 'Create character')
            }}
          </button>
        </div>
      </form>
    </div>
  </section>
</template>

<style scoped>
.character-creator {
  max-width: 920px;
}
.creation-steps {
  display: flex;
  gap: 32px;
  list-style: none;
  padding: 0;
  margin: 0 0 28px;
  border-bottom: 1px solid var(--divider);
}
.creation-steps button {
  display: flex;
  gap: 10px;
  align-items: center;
  min-height: 52px;
  padding: 0 0 16px;
  border: 0;
  border-bottom: 2px solid transparent;
  background: none;
  color: var(--muted);
  font: inherit;
  font-size: 14px;
  cursor: pointer;
}
.creation-steps button.active {
  border-color: var(--accent);
  color: var(--text);
  font-weight: 600;
}
.creation-steps button:disabled {
  cursor: default;
}
.creation-steps button > span {
  display: grid;
  place-items: center;
  width: 24px;
  height: 24px;
  border: 1px solid var(--control-border);
  border-radius: 50%;
  font-size: 12px;
}
.creation-steps svg {
  width: 16px;
  height: 16px;
  color: var(--accent);
}
.creator-layout {
  display: grid;
  grid-template-columns: minmax(240px, 360px) minmax(300px, 1fr);
  gap: 48px;
  align-items: start;
}
.portrait-stage {
  display: grid;
  position: relative;
  place-items: center;
  min-height: 300px;
  aspect-ratio: 4/5;
  border: 1px dashed var(--control-border);
  border-radius: 16px;
  background: var(--surface);
  overflow: hidden;
}
.portrait-stage.dragging {
  outline: 3px solid var(--accent);
  outline-offset: 4px;
}
.portrait-stage img {
  width: 100%;
  height: 100%;
  position: absolute;
  object-fit: contain;
}
.portrait-empty {
  display: grid;
  justify-items: center;
  gap: 20px;
  padding: 32px;
  text-align: center;
}
.portrait-empty > svg {
  width: 44px;
  height: 44px;
  color: var(--accent);
}
.portrait-empty span {
  color: var(--muted);
  font-size: 14px;
  line-height: 1.6;
}
.portrait-empty .console-button {
  min-width: 152px;
  justify-content: center;
}
.replace-photo {
  position: absolute;
  top: 12px;
  right: 12px;
  background: var(--surface);
}
.portrait-file {
  position: absolute;
  bottom: 12px;
  left: 12px;
  right: 12px;
  padding: 8px 12px;
  border-radius: 8px;
  background: var(--portrait-upload, #15131be6);
  color: var(--portrait-text, #f3f0f7);
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.creator-form {
  display: grid;
  gap: 20px;
  padding: 8px 0;
}
.creator-form h2 {
  font-size: 24px;
  margin: 10px 0 0;
  letter-spacing: -0.5px;
  outline: none;
}
.step-caption {
  color: var(--muted);
  font-size: 12px;
}
.creator-intro {
  margin: 0;
  color: var(--muted);
  line-height: 1.6;
  font-size: 15px;
}
.portrait-tips {
  margin: 0;
  padding-left: 20px;
  color: var(--muted);
  font-size: 14px;
  line-height: 1.6;
  display: grid;
  gap: 10px;
}
.file-input {
  position: absolute;
  width: 1px;
  height: 1px;
  opacity: 0;
  overflow: hidden;
}
.format-note {
  font-size: 12px;
  line-height: 1.6;
  color: var(--muted);
  margin: 0;
}
.creator-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--divider);
}
.portrait-summary {
  display: flex;
  gap: 12px;
  align-items: center;
  padding: 16px 0;
  border-block: 1px solid var(--divider);
  min-width: 0;
}
.portrait-summary svg {
  flex-shrink: 0;
  color: var(--muted);
}
.portrait-summary div {
  min-width: 0;
}
.portrait-summary strong {
  display: block;
  overflow-wrap: anywhere;
  font-size: 13px;
}
.portrait-summary small {
  display: block;
  margin-top: 6px;
  color: var(--muted);
  font-size: 12px;
}
.privacy-note {
  display: flex;
  align-items: start;
  gap: 10px;
  color: var(--muted);
  font-size: 13px;
  line-height: 1.6;
  margin: 0;
}
.privacy-note svg {
  flex-shrink: 0;
  margin-top: 2px;
}
.creation-steps button:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 4px;
}
@media (max-width: 760px) {
  .creator-layout {
    grid-template-columns: 1fr;
    gap: 28px;
  }
  .portrait-stage {
    min-height: 0;
    height: 200px;
    aspect-ratio: auto;
  }
  .portrait-stage.has-portrait {
    height: 240px;
  }
  .portrait-empty {
    padding: 20px 28px;
    gap: 12px;
  }
  .portrait-empty > svg {
    width: 28px;
    height: 28px;
  }
  .creation-steps {
    gap: 20px;
    margin-bottom: 20px;
  }
  .creation-steps button {
    font-size: 13px;
  }
  .creator-form {
    padding: 0;
    gap: 18px;
  }
}
</style>
