<script setup lang="ts">
import MotionStage from './MotionStage.vue'
defineProps<{
  frames: readonly { name: string; canvas: HTMLCanvasElement | null }[]
  background: string
  blur: number
  aspect: number
  focus?: { x: number; y: number; width: number; height: number }
}>()
</script>

<template>
  <div class="projection-comparison">
    <figure v-for="frame in frames" :key="frame.name" class="version">
      <figcaption class="caption">{{ frame.name }}</figcaption>
      <MotionStage
        :frame="frame.canvas"
        :background="background"
        :blur="blur"
        :aspect="aspect"
        :focus="focus"
      />
    </figure>
  </div>
</template>

<style scoped>
.projection-comparison {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
  grid-column: 1/-1;
}
.version {
  margin: 0;
  min-width: 0;
}
.caption {
  font-size: 14px;
  margin-bottom: 12px;
  color: var(--muted);
}
@media (max-width: 650px) {
  .projection-comparison {
    grid-template-columns: 1fr;
  }
}
</style>
