<script setup lang="ts">
import { computed } from 'vue'
import type { PoseImages } from './poseTypes'

const props = defineProps<{ pose: PoseImages; assetUrl: (name: string) => string }>()
const current = computed(() => props.pose.frame.toDataURL('image/png'))
</script>

<template>
  <details v-if="pose.data.comparison" class="comparison">
    <summary>Compare preparation · {{ pose.data.id }}</summary>
    <div class="comparison-grid">
      <figure>
        <img :src="assetUrl(pose.data.comparison.original)" alt="Original generated mouth" />
        <figcaption>Original generation</figcaption>
      </figure>
      <figure v-if="pose.data.comparison.previous">
        <img :src="assetUrl(pose.data.comparison.previous)" alt="Preceding mouth composition" />
        <figcaption>Preceding composition</figcaption>
      </figure>
      <figure>
        <img :src="assetUrl(pose.data.comparison.semantic)" alt="Automatic semantic boundaries" />
        <figcaption>Automatic boundaries</figcaption>
      </figure>
      <figure>
        <img :src="current" alt="Current target mouth endpoint" />
        <figcaption>Current endpoint</figcaption>
      </figure>
    </div>
    <p>
      Green: outer lips. Yellow: opening. Blue: upper teeth. Violet: lower teeth. Orange: tongue.
      Target endpoints are shown here; use the main controls for motion.
    </p>
  </details>
</template>

<style scoped>
.comparison {
  margin-block: 20px;
  border: 1px solid var(--control-border);
  border-radius: 6px;
  padding: 12px 16px;
}
summary {
  cursor: pointer;
  font-size: 14px;
}
.comparison-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-top: 16px;
}
figure {
  margin: 0;
}
img {
  display: block;
  width: 100%;
  height: auto;
}
figcaption,
p {
  color: var(--muted);
  font-size: 13px;
}
figcaption {
  margin-top: 6px;
}
@media (max-width: 800px) {
  .comparison-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
