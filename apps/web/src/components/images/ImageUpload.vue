<script setup lang="ts">
import { shallowRef, useTemplateRef } from 'vue'
import AppIcon from '../AppIcon.vue'

defineProps<{ filename: string | null }>()
const emit = defineEmits<{ select: [file: File]; selectionError: [message: string] }>()
const input = useTemplateRef<HTMLInputElement>('fileInput')
const dragging = shallowRef(false)

function choose(files: FileList | null) {
  if (!files?.length) return
  if (files.length !== 1) {
    emit('selectionError', 'Choose one image at a time.')
    return
  }
  emit('select', files[0])
}
function onChange(event: Event) {
  const target = event.target as HTMLInputElement
  choose(target.files)
  target.value = ''
}
function onDrop(event: DragEvent) {
  dragging.value = false
  choose(event.dataTransfer?.files ?? null)
}
function chooseImage() {
  input.value?.click()
}

defineExpose({ chooseImage })
</script>

<template>
  <div
    class="upload-zone"
    :class="{ dragging }"
    @dragover.prevent="dragging = true"
    @dragleave.prevent="dragging = false"
    @drop.prevent="onDrop"
  >
    <AppIcon class="upload-icon" name="upload" />
    <p class="upload-title">
      {{ filename ? 'Choose a different image' : 'Drop your portrait here' }}
    </p>
    <p class="upload-description">or choose a file from your device</p>
    <label class="visually-hidden" for="portrait-file">Portrait image</label>
    <input
      id="portrait-file"
      ref="fileInput"
      class="visually-hidden"
      type="file"
      accept="image/jpeg,image/png,image/webp,.jpg,.jpeg,.png,.webp"
      tabindex="-1"
      @change="onChange"
    />
    <button class="button button-primary" type="button" @click="chooseImage">
      {{ filename ? 'Replace image' : 'Choose an image' }}
    </button>
    <p class="file-guidance">JPEG, PNG, or WebP · Up to 5 MB</p>
  </div>
</template>

<style scoped>
.upload-zone {
  border: 1px dashed var(--control-border);
  border-radius: 10px;
  padding: 24px 16px 20px;
  text-align: center;
  background: var(--subtle);
}
.upload-zone.dragging {
  border-style: solid;
  border-color: var(--accent);
  background: var(--subtle);
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
.upload-icon {
  color: var(--accent);
  width: 26px;
  height: 26px;
}
.upload-title {
  margin: 12px 0 4px;
  font-size: 15px;
  font-weight: 600;
}
.upload-description {
  margin: 0 0 20px;
  color: var(--muted);
  font-size: 13px;
}
.file-guidance {
  font-size: 12px;
  color: var(--muted);
  margin: 16px 0 0;
}
</style>
