<script setup lang="ts">
import { computed } from 'vue'
import AppIcon from '../AppIcon.vue'
import type { ImageMetadata, ValidationError, ValidationStatus } from '../../types/images'

const props = defineProps<{
  status: ValidationStatus
  result: ImageMetadata | null
  error: ValidationError | null
}>()
defineEmits<{ retry: [] }>()
const retryable = computed(() =>
  ['processor_unavailable', 'connection_error'].includes(props.error?.code ?? ''),
)
const sizeLabel = computed(() =>
  props.result ? `${props.result.size_bytes.toLocaleString('en-US')} bytes` : '',
)
</script>

<template>
  <div
    class="validation-result"
    aria-live="polite"
    aria-atomic="true"
    :aria-busy="status === 'loading'"
  >
    <div v-if="status === 'loading'" class="validation-message" role="status">
      <p class="result-title">Validating image…</p>
      <p class="result-description">Checking the file content, size, and orientation.</p>
    </div>
    <div v-else-if="status === 'success' && result" class="validation-message">
      <p class="result-title success"><AppIcon name="check" /> Image file validated</p>
      <dl class="image-details">
        <div class="detail-row">
          <dt class="detail-label">Format</dt>
          <dd class="detail-value">{{ result.format }}</dd>
        </div>
        <div class="detail-row">
          <dt class="detail-label">Dimensions</dt>
          <dd class="detail-value">{{ result.width }} × {{ result.height }} px</dd>
        </div>
        <div class="detail-row">
          <dt class="detail-label">File size</dt>
          <dd class="detail-value">{{ sizeLabel }}</dd>
        </div>
      </dl>
      <p class="result-description">Dimensions reflect the image’s orientation.</p>
    </div>
    <div v-else-if="status === 'error' && error" class="validation-message" role="alert">
      <p class="result-title error"><AppIcon name="alert" /> Image could not be validated</p>
      <p class="result-description">{{ error.message }}</p>
      <button v-if="retryable" class="button retry-button" type="button" @click="$emit('retry')">
        Try again
      </button>
    </div>
  </div>
</template>

<style scoped>
.validation-message {
  padding-top: 24px;
}
.result-title {
  margin: 0 0 8px;
  font-size: 15px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 8px;
}
.success {
  color: var(--accent);
}
.error {
  color: var(--error);
}
.result-description {
  font-size: 13px;
  color: var(--muted);
  margin: 0;
}
.image-details {
  margin: 16px 0;
  font-size: 14px;
}
.detail-row {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 16px;
  margin: 8px 0;
}
.detail-label {
  color: var(--muted);
}
.detail-value {
  margin: 0;
  text-align: right;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
}
.retry-button {
  margin-top: 16px;
}
</style>
