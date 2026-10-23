<script setup lang="ts">
import { useTemplateRef } from 'vue'
import { useImageValidation } from '../../composables/useImageValidation'
import AppIcon from '../AppIcon.vue'
import ImageUpload from './ImageUpload.vue'
import ImagePreview from './ImagePreview.vue'
import ValidationResult from './ValidationResult.vue'
import ConversationPreview from './ConversationPreview.vue'

const { file, status, result, error, previewUrl, validate, clear, rejectSelection, retry } =
  useImageValidation()
const uploader = useTemplateRef('uploader')
function chooseImage() {
  uploader.value?.chooseImage()
}
defineExpose({ chooseImage })
</script>

<template>
  <div class="workspace-grid">
    <ImagePreview
      :url="previewUrl"
      :filename="file?.name ?? null"
      :loading="status === 'loading'"
    />
    <section class="workspace-panel" aria-labelledby="conversation-title">
      <div class="workspace-panel-header">
        <h2 id="conversation-title" class="panel-title"><AppIcon name="message" />Conversation</h2>
        <span class="availability">Not available yet</span>
      </div>
      <div class="setup-scroll-area">
        <section class="image-setup" aria-labelledby="upload-title">
          <div class="setup-heading">
            <h3 id="upload-title" class="setup-title">Start with a portrait</h3>
            <p class="setup-description">Upload your image to begin setting up your character.</p>
          </div>
          <ImageUpload
            ref="uploader"
            :filename="file?.name ?? null"
            @select="validate"
            @selection-error="rejectSelection"
          />
          <ValidationResult :status="status" :result="result" :error="error" @retry="retry" />
          <button v-if="file" class="clear-button" type="button" @click="clear">
            Remove image
          </button>
          <p class="setup-note">
            We currently check file format, size, and dimensions. Facial compatibility and animation
            come next.
          </p>
          <p class="privacy-note">No account needed. Uploads are discarded after validation.</p>
        </section>
      </div>
      <ConversationPreview />
    </section>
  </div>
</template>

<style scoped>
.workspace-grid {
  height: 100%;
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1.15fr);
  gap: 28px;
}
.workspace-panel {
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--surface);
  border: 1px solid var(--divider);
  border-radius: 14px;
  overflow: hidden;
}
.workspace-panel-header {
  min-height: 58px;
  padding: 12px 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  border-bottom: 1px solid var(--divider);
  background: var(--subtle);
}
.panel-title {
  font-size: 15px;
  font-weight: 600;
  margin: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}
.panel-title .icon {
  width: 18px;
  height: 18px;
}
.availability {
  color: var(--muted);
  font-size: 12px;
}
.setup-scroll-area {
  min-height: 0;
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
}
.image-setup {
  width: 100%;
  max-width: 484px;
  margin: auto;
  padding: 32px 24px;
}
.setup-heading {
  margin-bottom: 24px;
}
.setup-title {
  font-size: 23px;
  font-weight: 600;
  line-height: 1.3;
  letter-spacing: -0.5px;
  margin: 0 0 8px;
}
.setup-description {
  margin: 0;
  color: var(--muted);
  font-size: 14px;
}
.setup-note {
  margin: 20px 0 0;
  color: var(--muted);
  font-size: 13px;
  line-height: 1.6;
}
.privacy-note {
  margin: 12px 0 0;
  color: var(--muted);
  font-size: 12px;
}
.clear-button {
  background: none;
  border: 0;
  color: var(--accent);
  min-height: 44px;
  padding: 8px 0;
  text-decoration: underline;
  font-size: 14px;
  text-underline-offset: 3px;
}
@media (max-width: 800px) {
  .workspace-grid {
    height: auto;
    grid-template-columns: minmax(0, 1fr);
    gap: 16px;
  }
  .setup-scroll-area {
    overflow: visible;
  }
  .image-setup {
    max-width: 520px;
    padding: 24px 20px;
  }
  .setup-heading {
    margin-bottom: 20px;
  }
  .setup-title {
    font-size: 21px;
  }
  .workspace-panel-header {
    padding: 12px 16px;
  }
}
</style>
