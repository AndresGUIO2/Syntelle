<script setup lang="ts">
import { computed, onMounted, onScopeDispose, shallowRef, watch } from 'vue'
import CharacterStage from '../../character/components/CharacterStage.vue'
import { useCharacterMotion } from '../../character/composables/useCharacterMotion'
import type { PreparedDemoCharacter } from '../catalog'

const props = defineProps<{
  prepared: PreparedDemoCharacter
  text: string
  language: string
  voice: string
  blinking: boolean
  movement: boolean
  smile: number
  brows: number
  automaticExpressions: boolean
  background: string
  blur: number
}>()
const emit = defineEmits<{ session: [value: ReturnType<typeof useCharacterMotion> | null] }>()
const detail = shallowRef<HTMLCanvasElement | null>(null)
const session = useCharacterMotion(props.prepared.oral, props.prepared.motion, detail)
// Presentation only: a common face-relative crop, using the existing source anatomy.
// Anchor its bottom to available upper-body material and preserve the canvas aspect.
const portraitStyle = computed(() => {
  const { canvas, anatomy } = props.prepared.motion.definition!
  const bottom = Math.min(canvas.height, anatomy.chin_y + anatomy.face_height * 0.9)
  const height = anatomy.face_height * 2.6
  return {
    '--portrait-height': canvas.height / height,
    '--portrait-width': canvas.width / height,
    '--portrait-top': (height - bottom) / height,
  }
})
watch(
  () => [props.smile, props.brows, props.automaticExpressions] as const,
  ([smile, brows, automatic]) => {
    session.expressions.smile.value = smile
    session.expressions.brows.value = brows
    session.expressions.automatic.value = automatic
  },
  { immediate: true },
)
watch(
  () => [props.text, props.language, props.voice],
  ([text, language, voice]) => {
    session.speech.text.value = text!
    session.speech.language.value = language!
    session.speech.voice.value = voice!
  },
  { immediate: true, flush: 'sync' },
)
watch(
  () => [props.blinking, props.movement, session.clock.reducedMotion.value],
  () => {
    session.clock.automaticBlink.value = props.blinking && !session.clock.reducedMotion.value
    session.clock.movement.value = props.movement && !session.clock.reducedMotion.value
  },
  { immediate: true },
)
onMounted(() => emit('session', session))
onScopeDispose(() => emit('session', null))
</script>

<template>
  <CharacterStage
    class="demo-character-stage"
    :style="portraitStyle"
    :frame="session.ready.value ? (session.body.value?.canvas ?? null) : null"
    :background="background"
    :blur="blur"
    :aspect="prepared.motion.definition!.canvas.width / prepared.motion.definition!.canvas.height"
    fill
    :poster="prepared.motion.assetUrl(prepared.motion.definition!.foreground)"
  />
</template>

<style scoped>
.demo-character-stage.fill-stage {
  position: absolute;
  inset: 0;
  height: auto;
  container-type: size;
  --character-stage-fit: contain;
}
.demo-character-stage :deep(.character-layer),
.demo-character-stage :deep(.poster) {
  /* The stage fills the viewport; only the character has a size limit. */
  --portrait-frame: min(88cqh, 135cqw, 800px);
  left: 50%;
  right: auto;
  bottom: auto;
  top: calc(100cqh - var(--portrait-frame) + var(--portrait-frame) * var(--portrait-top));
  width: calc(var(--portrait-frame) * var(--portrait-width));
  height: calc(var(--portrait-frame) * var(--portrait-height));
  transform: translateX(-50%);
}
@media (max-width: 899px) {
  .demo-character-stage :deep(.character-layer),
  .demo-character-stage :deep(.poster) {
    --portrait-frame: min(100cqh, 150cqw, 960px);
  }
}
</style>
