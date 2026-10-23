<script setup lang="ts">
import AppIcon from '../AppIcon.vue'
defineProps<{ url: string | null; filename: string | null; loading: boolean }>()
</script>

<template>
  <section class="preview-panel" aria-labelledby="preview-title">
    <div class="preview-header">
      <h2 id="preview-title" class="preview-title">Your character</h2>
      <span class="preview-label">{{ url ? 'Original image' : 'Image preview' }}</span>
    </div>
    <div class="preview-stage" :class="{ 'has-image': url }">
      <img v-if="url" class="preview-image" :src="url" :alt="`Preview of ${filename}`" />
      <div v-else class="preview-empty">
        <AppIcon class="preview-icon" name="image" />
        <p class="empty-title">{{ loading ? 'Checking your image' : 'Your portrait goes here' }}</p>
        <p class="empty-description">
          {{
            loading
              ? 'The preview will appear when the file is validated.'
              : 'Upload a portrait to make it your own.'
          }}
        </p>
      </div>
    </div>
    <div class="preview-footer">
      <span class="preview-caption">{{ filename || 'Your original image stays unchanged.' }}</span>
      <span v-if="url" class="image-note">Static image</span>
    </div>
  </section>
</template>

<style scoped>
.preview-panel {
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--surface);
  border: 1px solid var(--divider);
  border-radius: 14px;
  overflow: hidden;
}
.preview-header {
  min-height: 58px;
  padding: 12px 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  border-bottom: 1px solid var(--divider);
}
.preview-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}
.preview-label {
  font-size: 12px;
  color: var(--muted);
}
.preview-stage {
  flex: 1;
  min-height: 0;
  padding: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--subtle);
  overflow: hidden;
}
.preview-stage.has-image {
  padding: 16px;
}
.preview-image {
  display: block;
  max-width: 100%;
  max-height: 100%;
  width: auto;
  height: auto;
  object-fit: contain;
  image-orientation: from-image;
  outline: 1px solid rgb(0 0 0 / 10%);
  outline-offset: -1px;
}
:global([data-theme='dark']) .preview-image {
  outline-color: rgb(255 255 255 / 10%);
}
.preview-empty {
  text-align: center;
  max-width: 260px;
}
.preview-icon {
  color: var(--muted);
  width: 40px;
  height: 40px;
  stroke-width: 1.2;
}
.empty-title {
  margin: 16px 0 6px;
  font-size: 16px;
  font-weight: 500;
}
.empty-description {
  color: var(--muted);
  font-size: 14px;
  margin: 0;
}
.preview-footer {
  min-height: 48px;
  padding: 12px 20px;
  border-top: 1px solid var(--divider);
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  color: var(--muted);
  font-size: 12px;
}
.preview-caption {
  overflow-wrap: anywhere;
  min-width: 0;
}
.image-note {
  flex-shrink: 0;
}
@media (max-width: 800px) {
  .preview-stage {
    flex: none;
    height: 360px;
  }
  .preview-stage:not(.has-image) {
    height: 160px;
    padding: 16px;
  }
  .preview-header,
  .preview-footer {
    padding-left: 16px;
    padding-right: 16px;
  }
  .preview-footer {
    min-height: 42px;
  }
  .preview-icon {
    width: 28px;
    height: 28px;
  }
  .empty-title {
    margin-top: 8px;
    font-size: 15px;
  }
  .empty-description {
    font-size: 13px;
  }
}
</style>
