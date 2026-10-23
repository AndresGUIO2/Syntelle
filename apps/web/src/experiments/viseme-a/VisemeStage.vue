<script setup lang="ts">
import { useTemplateRef, watchEffect } from 'vue'
import { posePackage } from './usePosePackage'
import type { PoseImages } from './poseTypes'

const props = defineProps<{
  base: HTMLImageElement
  source: PoseImages
  target: PoseImages
  amount: number
  detail?: boolean
  showBounds: boolean
  morphFrame: HTMLCanvasElement | null
}>()
const canvas = useTemplateRef<HTMLCanvasElement>('canvas')
watchEffect(() => {
  const context = canvas.value?.getContext('2d')
  if (!context) return
  const crop = posePackage.crop
  // The renderer mutates one canvas in place; its identity does not change.
  // Track playback explicitly even while the shared frame is present.
  const frameAmount = props.amount
  const view = props.detail ? posePackage.detail : { x: 0, y: 0, ...posePackage.canvas }
  context.clearRect(0, 0, view.width, view.height)
  context.drawImage(props.base, -view.x, -view.y)
  const intermediate = props.morphFrame
  context.globalAlpha = 1
  if (intermediate) context.drawImage(props.morphFrame!, crop.x - view.x, crop.y - view.y)
  else {
    context.drawImage(props.source.frame, crop.x - view.x, crop.y - view.y)
    context.globalAlpha = frameAmount
    context.drawImage(props.target.frame, crop.x - view.x, crop.y - view.y)
  }
  context.globalAlpha = 1
  if (props.showBounds) {
    context.strokeStyle = '#6D4BD0'
    context.lineWidth = props.detail ? 1 : 3
    context.setLineDash([8, 5])
    context.strokeRect(crop.x - view.x, crop.y - view.y, crop.width, crop.height)
    context.setLineDash([])
  }
})
</script>

<template>
  <canvas
    ref="canvas"
    class="stage"
    :class="{ detail }"
    :width="detail ? posePackage.detail.width : posePackage.canvas.width"
    :height="detail ? posePackage.detail.height : posePackage.canvas.height"
    :aria-label="detail ? 'Mouth detail' : 'Character transition'"
    role="img"
    >Portrait with locally prepared mouth layers.</canvas
  >
</template>

<style scoped>
.stage {
  display: block;
  width: 100%;
  height: auto;
  outline: 1px solid oklch(0 0 0 / 0.1);
  outline-offset: -1px;
}
.stage:not(.detail) {
  max-height: 67dvh;
  width: auto;
  max-width: 100%;
  object-fit: contain;
  margin-inline: auto;
}
:root[data-theme='dark'] .stage {
  outline-color: oklch(1 0 0 / 0.1);
}
</style>
