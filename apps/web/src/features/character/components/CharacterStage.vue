<script setup lang="ts">
import { computed, useTemplateRef, watch } from 'vue'
const props = defineProps<{
  frame: HTMLCanvasElement | null
  background: string
  blur: number
  aspect: number
  focus?: { x: number; y: number; width: number; height: number }
  poster?: string
  fill?: boolean
}>()
const sceneStyle = computed(() =>
  props.focus
    ? {
        left: `${(-props.focus.x / props.focus.width) * 100}%`,
        top: `${(-props.focus.y / props.focus.height) * 100}%`,
        right: 'auto',
        bottom: 'auto',
        width: `${100 / props.focus.width}%`,
        height: `${100 / props.focus.height}%`,
      }
    : {},
)
const aspect = computed(() =>
  props.focus ? (props.aspect * props.focus.width) / props.focus.height : props.aspect,
)
const host = useTemplateRef<HTMLDivElement>('host')
watch(
  [host, () => props.frame],
  ([element, canvas]) => {
    element?.replaceChildren(...(canvas ? [canvas] : []))
    if (canvas) {
      canvas.setAttribute('role', 'img')
      canvas.setAttribute('aria-label', 'Animated character')
      canvas.className = 'character-canvas'
    }
  },
  { flush: 'post' },
)
</script>

<template>
  <div
    class="composition"
    :class="{ 'fill-stage': fill }"
    data-testid="motion-composition"
    :style="{ aspectRatio: aspect }"
  >
    <div class="scene" :style="sceneStyle">
      <div
        class="background-layer"
        data-testid="background-layer"
        :style="{ inset: `${-Math.ceil(blur * 3)}px`, filter: `blur(${blur}px)` }"
      >
        <img :src="background" alt="" />
      </div>
      <img v-if="!frame && poster" class="poster" :src="poster" alt="" />
      <div ref="host" class="character-layer" />
    </div>
  </div>
</template>

<style scoped>
.composition {
  position: relative;
  isolation: isolate;
  overflow: hidden;
  width: 100%;
  background: var(--surface);
}
.fill-stage {
  height: 100%;
  aspect-ratio: auto !important;
}
.scene {
  position: absolute;
  inset: 0;
}
.background-layer {
  position: absolute;
  z-index: 0;
  transform: translateZ(0);
}
.background-layer img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.poster {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: var(--character-stage-fit, cover);
  object-position: var(--character-stage-position, center top);
}
.character-layer {
  position: absolute;
  inset: 0;
  z-index: 1;
  contain: paint;
}
.character-layer :deep(.character-canvas) {
  display: block;
  width: 100%;
  height: 100%;
}
.fill-stage .character-layer :deep(.character-canvas) {
  object-fit: var(--character-stage-fit, cover);
  object-position: var(--character-stage-position, center top);
}
</style>
