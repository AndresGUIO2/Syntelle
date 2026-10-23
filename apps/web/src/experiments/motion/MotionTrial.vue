<script setup lang="ts">
import { computed, shallowRef, useTemplateRef } from 'vue'
import MotionStage from './MotionStage.vue'
import MotionControls from './MotionControls.vue'
import SpeechControls from '../../features/character/components/SpeechControls.vue'
import MouthControls from '../../features/character/components/MouthControls.vue'
import {
  backgroundCatalog,
  backgroundUrl,
  motionCharacters,
  resolveMotionContext,
} from './motionCatalog'
import { resolvePoseContext } from '../viseme-a/poseCatalog'
import { useCharacterMotion } from '../../features/character/composables/useCharacterMotion'
import { useBackgroundPreference } from '../../features/character/composables/useBackgroundPreference'
import type { TrialSelection } from '../../features/character/types/packageContext'
import type { HeadPose } from '../../features/character/animation/headMotion'
import { useProjectionReview } from '../../features/character/composables/useProjectionReview'
import ProjectionControls from './ProjectionControls.vue'
import ProjectionComparison from './ProjectionComparison.vue'
const props = defineProps<{ selection?: TrialSelection }>()
const context = resolvePoseContext({ ...props.selection, trial: 'motion' })
const character = context.character ?? 'syntelle'
const posePackage = context.package
const eyeView = useTemplateRef<HTMLCanvasElement>('eyeView')
const previousNatural = props.selection?.head === 'natural-previous'
const rotationPreview = props.selection?.head === 'rotation-candidate'
const contextualPreview = props.selection?.head === 'contextual-candidate'
const vitalityPreview = props.selection?.head === 'vitality-candidate'
const neckPreview =
  props.selection?.head === 'neck-candidate' || props.selection?.head === 'neck-compare'
const projectionPreview =
  props.selection?.head === 'rgbd-candidate' || props.selection?.head === 'rgbd-compare'
const compare = shallowRef(
  props.selection?.head === 'rgbd-compare' || props.selection?.head === 'neck-compare',
)
const manualPose = shallowRef<HeadPose | null>(null)
const neckDetail = shallowRef(false)
const natural =
  props.selection?.head === 'natural' ||
  previousNatural ||
  rotationPreview ||
  contextualPreview ||
  vitalityPreview ||
  projectionPreview ||
  neckPreview
const { clock, mouth, speech, sourceId, targetId, definition, body, comparison, ready, error } =
  useCharacterMotion(
    context,
    resolveMotionContext(
      character,
      natural,
      neckPreview
        ? 'neck'
        : projectionPreview
          ? 'rgbd'
          : vitalityPreview
            ? 'vitality'
            : contextualPreview
              ? 'contextual'
              : rotationPreview
                ? 'rotation'
                : previousNatural
                  ? 'previous'
                  : 'current',
    ),
    eyeView,
    { compare, pose: manualPose, legacy: props.selection?.render === 'legacy' },
  )
const review = useProjectionReview(clock.elapsed, manualPose)
const neckFocus = computed(() => {
  const neck = definition?.projection?.articulation
  if (!neckDetail.value || !neck || !definition) return undefined
  const { width, height } = definition.canvas
  const fw = definition.anatomy.face_width,
    fh = definition.anatomy.face_height
  const x = Math.max(0, neck.chest_center_px[0]! - fw * 0.55)
  const y = Math.max(0, neck.localization.chin_y - fh * 0.18)
  return {
    x: x / width,
    y: y / height,
    width: Math.min(width - x, fw * 1.1) / width,
    height:
      Math.min(height - y, neck.localization.base_y - neck.localization.chin_y + fh * 0.5) / height,
  }
})
const frames = computed(() => [
  {
    name: definition?.projection?.articulation ? 'Previous RGB-D' : 'Current movement',
    canvas: comparison.value?.baseline.canvas ?? null,
  },
  {
    name: definition?.projection?.articulation
      ? 'Articulated neck'
      : 'Same turns, current renderer',
    canvas: comparison.value?.matched.canvas ?? null,
  },
  {
    name: definition?.projection?.articulation ? 'Neck + depth breathing' : 'Perspective RGB-D',
    canvas: body.value?.canvas ?? null,
  },
])
function replayProjection() {
  if (!clock.reducedMotion.value) {
    clock.paused.value = false
    review.replay()
  }
}
const { automaticBlink, movement, paused, closure, reducedMotion, doubleBlinkAvailable } = clock
const { background, blur, selectedBackground } = useBackgroundPreference(
  backgroundCatalog,
  backgroundUrl,
)
</script>

<template>
  <section class="motion-trial" aria-labelledby="motion-title">
    <div class="heading">
      <h2 id="motion-title">Character motion trial</h2>
      <a :href="`?trial=viseme-a&oral=semantic&character=${character}`">Back to mouth trial</a>
    </div>
    <nav class="character-navigation" aria-label="Prepared characters">
      <a
        v-for="name in motionCharacters"
        :key="name"
        :aria-current="name === character ? 'page' : undefined"
        :href="`?trial=motion&character=${name}${natural ? `&head=${neckPreview ? 'neck-compare' : projectionPreview ? 'rgbd-compare' : vitalityPreview ? 'vitality-candidate' : contextualPreview ? 'contextual-candidate' : rotationPreview ? 'rotation-candidate' : previousNatural ? 'natural-previous' : 'natural'}` : ''}`"
        >{{ name }}</a
      >
    </nav>
    <p class="candidate-navigation">
      <a :href="`?trial=motion&character=${character}&head=neck-compare`"
        >Compare neck and breathing</a
      >
    </p>
    <p class="candidate-navigation">
      <a
        :href="`?trial=motion&character=${character}&head=${definition?.projection ? 'rgbd-compare' : definition?.breathing ? 'contextual-candidate' : definition?.behavior ? 'rotation-candidate' : 'natural'}`"
        >{{
          definition?.projection
            ? 'Compare depth renderers'
            : definition?.breathing
              ? 'Compare previous breathing and blinking'
              : definition?.behavior
                ? 'Compare previous head turns'
                : 'Try contextual movement'
        }}</a
      >
    </p>
    <p v-if="definition?.projection" class="candidate-navigation">
      <a :href="`?trial=motion&character=${character}&head=vitality-candidate`"
        >Previous movement</a
      >
    </p>
    <p v-if="error" role="alert">{{ error }}</p>
    <p v-else-if="!ready" role="status">Loading the prepared character…</p>
    <div v-if="definition" class="layout">
      <ProjectionComparison
        v-if="compare && definition.projection"
        :frames="frames"
        :focus="neckFocus"
        :background="selectedBackground"
        :blur="blur"
        :aspect="definition.canvas.width / definition.canvas.height"
      />
      <MotionStage
        v-else
        :frame="body?.canvas ?? null"
        :focus="neckFocus"
        :background="selectedBackground"
        :blur="blur"
        :aspect="definition.canvas.width / definition.canvas.height"
      />
      <div class="settings">
        <ProjectionControls
          v-if="definition.projection"
          v-model:compare="compare"
          v-model:detail="neckDetail"
          :neck-detail-available="Boolean(definition.projection.articulation)"
          :yaw="review.yaw.value"
          :pitch="review.pitch.value"
          :manual="review.manual.value"
          :playing="review.playing.value"
          :reduced-motion="reducedMotion"
          @inspect="review.inspect"
          @replay="replayProjection"
          @automatic="review.automatic"
        />
        <MotionControls
          v-model:background="background"
          v-model:blur="blur"
          v-model:blinking="automaticBlink"
          v-model:moving="movement"
          v-model:paused="paused"
          :backgrounds="backgroundCatalog"
          :closure="closure"
          :reduced-motion="reducedMotion"
          :double-blink-enabled="Boolean(definition.behavior?.blink.double)"
          :double-blink-available="doubleBlinkAvailable"
          @blink="clock.blink"
          @double-blink="clock.blinkTwice"
          @closure="clock.setClosure"
        />
        <figure class="eye-detail">
          <canvas ref="eyeView" role="img" aria-label="Eyelid detail" />
          <figcaption>Eyelid detail</figcaption>
        </figure>
        <MouthControls
          v-model:source="sourceId"
          v-model:target="targetId"
          :poses="Object.keys(posePackage.poses)"
          :amount="mouth.amount.value"
          :playing="mouth.playing.value"
          :disabled="
            !ready || reducedMotion || paused || speech.active.value || speech.loading.value
          "
          @toggle="mouth.toggle"
          @pose="mouth.pose"
        />
        <SpeechControls
          v-model:text="speech.text.value"
          v-model:language="speech.language.value"
          v-model:voice="speech.voice.value"
          :speech="speech"
          :ready="ready"
        />
      </div>
    </div>
  </section>
</template>

<style scoped>
.motion-trial {
  max-width: 1120px;
  margin-inline: auto;
}
.heading {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 16px;
}
.heading h2 {
  font-size: 20px;
  margin: 0;
}
.heading a,
.character-navigation a {
  color: var(--accent);
  font-size: 14px;
}
.character-navigation {
  display: flex;
  gap: 16px;
  margin: 16px 0 24px;
}
.character-navigation a[aria-current='page'] {
  font-weight: 700;
}
.layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 32px;
  align-items: start;
}
.settings {
  min-width: 0;
}
.eye-detail {
  margin: 24px 0;
  background: var(--subtle);
}
.eye-detail canvas {
  width: 100%;
  height: auto;
  display: block;
}
.eye-detail figcaption {
  padding: 8px;
  text-align: center;
  color: var(--muted);
  font-size: 13px;
}
@media (max-width: 800px) {
  .layout {
    grid-template-columns: 1fr;
    gap: 24px;
  }
  .heading {
    align-items: start;
  }
}
</style>
