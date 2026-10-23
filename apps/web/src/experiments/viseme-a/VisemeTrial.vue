<script setup lang="ts">
import VisemeStage from './VisemeStage.vue'
import DentalReferenceReview from './DentalReferenceReview.vue'
import OralComparison from './OralComparison.vue'
import { resolvePoseContext, developmentCharacters } from './poseCatalog'
import { useOralTrial } from '../../features/character/composables/useOralTrial'
import type { TrialSelection } from '../../features/character/types/packageContext'
const props = defineProps<{ selection?: TrialSelection }>()
const context = resolvePoseContext(props.selection)
const {
  package: posePackage,
  assetUrl: poseAssetUrl,
  character: developmentCharacter,
  semantic: semanticTrial,
  completed: completedTrial,
} = context
const {
  amount,
  playing,
  reducedMotion,
  duration,
  hold,
  pose,
  toggle,
  images,
  error,
  sourceId,
  targetId,
  source,
  target,
  pair,
  pairSafe,
  showBounds,
  mode,
  morphFrame,
  morphUnavailable,
  poseLabel,
  scrub,
} = useOralTrial(context)
</script>

<template>
  <section class="trial" aria-labelledby="trial-title">
    <div class="trial-heading">
      <h2 id="trial-title" class="trial-title">Mouth transition trial</h2>
      <a class="back-link" href="/">Back to workspace</a>
    </div>
    <p class="trial-note">
      One immutable portrait, shared teeth and pose-specific mouth layers. Silent preview; no new
      generation.
    </p>
    <p class="trial-note">
      <a class="back-link" :href="`?trial=motion&character=${developmentCharacter ?? 'syntelle'}`"
        >Try blinking, motion and backgrounds</a
      >
    </p>
    <p v-if="developmentCharacter" class="trial-note">
      Development character: {{ developmentCharacter }} ·
      <a
        v-for="name in developmentCharacters"
        :key="name"
        class="character-link"
        :href="`?trial=viseme-a&oral=semantic&character=${name}`"
        >{{ name }}</a
      >
    </p>
    <template v-else>
      <p class="trial-note">
        <a
          :href="semanticTrial ? '?trial=viseme-a&oral=completed' : '?trial=viseme-a&oral=semantic'"
          >{{
            semanticTrial
              ? 'Compare preceding completed package'
              : 'Inspect automatic semantic preparation'
          }}</a
        >
        ·
        {{
          semanticTrial
            ? 'Measured lip boundaries; automatic dentition placement'
            : 'Preceding completed package'
        }}
      </p>
      <p class="trial-note">
        <a
          :href="
            completedTrial ? '?trial=viseme-a&oral=previous' : '?trial=viseme-a&oral=completed'
          "
          >{{ completedTrial ? 'Compare previous dentition' : 'Inspect completed dentition' }}</a
        >
        ·
        {{
          semanticTrial
            ? 'Shared EI teeth; local attachment coverage'
            : completedTrial
              ? 'Completed EI material'
              : 'Previous package'
        }}
      </p>
    </template>
    <p v-if="error" role="alert">{{ error }}</p>
    <p v-else-if="!images" role="status">Loading the portrait…</p>
    <template v-else-if="images && source && target">
      <OralComparison v-if="semanticTrial" :pose="target" :asset-url="poseAssetUrl" />
      <p
        v-if="target.data.preparation && !target.data.preparation.ready"
        class="trial-note"
        role="alert"
      >
        Preparation failed: {{ target.data.preparation.issues.join(' ') }} This experiment remains
        available for inspection; the character is not ready for publication.
      </p>
      <DentalReferenceReview
        v-if="posePackage.reference.selection"
        :selection="posePackage.reference.selection"
        :original-url="poseAssetUrl(posePackage.reference.selection.original_preview)"
        :mask-url="poseAssetUrl(posePackage.reference.selection.mask_preview)"
      />
      <div class="trial-grid">
        <figure class="portrait-figure">
          <VisemeStage
            :base="images.base"
            :source="source"
            :target="target"
            :amount="amount"
            :show-bounds="showBounds"
            :morph-frame="morphFrame"
          />
          <figcaption class="caption">Full portrait</figcaption>
        </figure>
        <div class="inspection">
          <figure class="detail-figure">
            <VisemeStage
              detail
              :base="images.base"
              :source="source"
              :target="target"
              :amount="amount"
              :show-bounds="showBounds"
              :morph-frame="morphFrame"
            />
            <figcaption class="caption">
              Mouth detail · <span data-testid="pose">{{ poseLabel }}</span>
            </figcaption>
          </figure>
          <div class="controls" aria-label="Transition controls">
            <div class="setting-row">
              <label class="setting"
                >From
                <select v-model="sourceId" aria-label="From pose">
                  <option
                    v-for="definition in posePackage.poses"
                    :key="definition.id"
                    :value="definition.id"
                  >
                    {{ definition.id }}
                  </option>
                </select>
              </label>
              <label class="setting"
                >To
                <select v-model="targetId" aria-label="To pose">
                  <option
                    v-for="definition in posePackage.poses"
                    :key="definition.id"
                    :value="definition.id"
                  >
                    {{ definition.id }}
                  </option>
                </select>
              </label>
            </div>
            <p v-if="!pairSafe" class="trial-note" role="status">
              {{
                pair?.reason
                  ? 'Direct morph unavailable: ' + pair.reason
                  : 'Choose two different poses.'
              }}
              The endpoint buttons remain available.
            </p>
            <label class="setting"
              >Method
              <select v-model="mode" aria-label="Method">
                <option value="morph" :disabled="morphUnavailable">Landmark morph</option>
                <option value="crossfade">Crossfade only</option>
              </select>
            </label>
            <p v-if="morphUnavailable" class="trial-note" role="status">
              Mouth deformation is unavailable in this browser. Crossfade still works; reload to
              retry.
            </p>
            <div class="button-row">
              <button
                class="button button-primary"
                :disabled="reducedMotion || !pairSafe"
                :aria-pressed="playing"
                @click="toggle"
              >
                {{ playing ? 'Pause' : 'Play' }}
              </button>
              <button class="button" @click="pose(0)">{{ sourceId }}</button>
              <button class="button" @click="pose(1)">{{ targetId }}</button>
            </div>
            <div class="setting-row">
              <label class="setting"
                >Transition
                <select v-model.number="duration" aria-label="Transition">
                  <option :value="0">Direct cut</option>
                  <option :value="120">120 ms</option>
                  <option :value="300">300 ms</option>
                </select>
              </label>
              <label class="setting"
                >Hold each pose
                <select v-model.number="hold">
                  <option :value="500">500 ms</option>
                  <option :value="1200">1,200 ms</option>
                </select>
              </label>
            </div>
            <label class="setting"
              >Inspect blend · {{ Math.round(amount * 100) }}%
              <input
                aria-label="Mouth blend"
                type="range"
                min="0"
                max="1"
                step="0.01"
                :disabled="!pairSafe"
                :value="amount"
                @input="scrub"
              />
            </label>
            <label class="check-setting"
              ><input v-model="showBounds" type="checkbox" /> Show crop boundary</label
            >
            <p v-if="reducedMotion" class="trial-note">
              Reduced motion is on. Use Rest, A or the blend slider to inspect still frames.
            </p>
          </div>
          <p class="trial-note">
            {{
              semanticTrial
                ? 'Shared tooth layers, measured lip occlusion and color-only shadows. Hidden attachment coverage continues the selected enamel locally. Crossfade is a diagnostic comparison; landmark morph keeps the dental material opaque.'
                : completedTrial
                  ? 'Completed EI upper and lower arches, lip occlusion and contact shadows. Crossfade is a diagnostic comparison; landmark morph keeps the dental material opaque.'
                  : 'Previous cutout package, retained for comparison.'
            }}
          </p>
        </div>
      </div>
    </template>
  </section>
</template>

<style scoped>
.trial {
  max-width: 1120px;
  margin-inline: auto;
}
.trial-heading {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 16px;
}
.trial-title {
  font-size: 20px;
  margin: 0;
}
.back-link {
  color: var(--accent);
  font-size: 14px;
}
.trial-note {
  color: var(--muted);
  font-size: 14px;
  margin: 12px 0 20px;
}
.character-link {
  margin-inline-start: 8px;
}
.trial-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 32px;
  align-items: start;
}
.portrait-figure,
.detail-figure {
  margin: 0;
}
.portrait-figure {
  background: var(--subtle);
}
.caption {
  padding: 8px 0;
  font-size: 14px;
  color: var(--muted);
  text-align: center;
}
.controls {
  display: grid;
  gap: 16px;
  margin-top: 16px;
}
.button-row,
.setting-row {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}
.setting {
  display: grid;
  gap: 6px;
  font-size: 14px;
  flex: 1;
}
.setting select {
  font: inherit;
  background: var(--surface);
  color: var(--text);
  padding: 8px;
  border: 1px solid var(--control-border);
  border-radius: 6px;
  min-height: 44px;
}
.setting input {
  width: 100%;
  margin: 0;
  min-height: 32px;
  accent-color: var(--accent);
}
.check-setting {
  display: flex;
  gap: 8px;
  align-items: center;
  min-height: 32px;
  font-size: 14px;
}
.check-setting input {
  accent-color: var(--accent);
  width: 18px;
  height: 18px;
}
.setting select:focus-visible,
.setting input:focus-visible,
.check-setting input:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 3px;
}
@media (max-width: 800px) {
  .trial-grid {
    grid-template-columns: 1fr;
    gap: 24px;
  }
  .stage:not(.detail) {
    max-height: 48svh;
  }
  .trial-heading {
    align-items: start;
  }
  .trial-title {
    font-size: 18px;
  }
}
</style>
