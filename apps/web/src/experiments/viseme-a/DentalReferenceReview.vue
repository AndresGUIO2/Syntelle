<script setup lang="ts">
import { computed } from 'vue'
import type { DentalSelection } from './poseTypes'

const props = defineProps<{ selection: DentalSelection; originalUrl: string; maskUrl: string }>()
const reviewed = computed(() => Boolean(props.selection.mask_review))
const argon = computed(() => props.selection.mask_review?.method.includes('Argon') === true)
const maskLabel = computed(() =>
  argon.value ? 'Argon contours' : reviewed.value ? 'Reviewed contours' : 'Mask proposal',
)
const candidates = computed(() =>
  Object.entries(props.selection.candidates).sort(
    ([a, x], [b, y]) =>
      Number(y.eligible) - Number(x.eligible) || y.score - x.score || a.localeCompare(b),
  ),
)
</script>

<template>
  <details class="reference-review">
    <summary class="reference-summary" data-testid="dental-reference">
      Dental reference: {{ selection.selected_pose }} · automatically selected from
      {{ candidates.length }} original poses
    </summary>
    <div class="reference-content">
      <p class="reference-note">
        Selected from cached original images. No teeth were generated, recolored or stretched.
        Scores measure extraction utility, not visual approval.
      </p>
      <div class="reference-images">
        <figure class="reference-figure">
          <img
            class="reference-image"
            :src="originalUrl"
            :alt="`Original ${selection.selected_pose} dental source`"
          />
          <figcaption>Original {{ selection.selected_pose }} mouth</figcaption>
        </figure>
        <figure class="reference-figure">
          <img
            class="reference-image"
            :src="maskUrl"
            :alt="
              argon
                ? 'Argon dental and tongue contours'
                : reviewed
                  ? 'Reviewed dental contours'
                  : 'Automatic dental mask proposal'
            "
          />
          <figcaption>
            {{ maskLabel }} · upper in blue, lower in pink<span v-if="argon"
              >, visible tongue in orange</span
            >
          </figcaption>
        </figure>
      </div>
      <p class="reference-note">
        {{ selection.review_status }} These masks are not validated for arbitrary uploads.
      </p>
      <div class="reference-table-wrap">
        <table class="reference-table">
          <caption class="reference-caption">
            Candidate comparison · utility score is not a confidence percentage
          </caption>
          <thead>
            <tr>
              <th scope="col">Pose</th>
              <th scope="col">Utility score</th>
              <th scope="col">Decision</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="[id, candidate] in candidates"
              :key="id"
              :class="{ selected: id === selection.selected_pose }"
            >
              <th scope="row">{{ id }}</th>
              <td>{{ candidate.score.toFixed(1) }}</td>
              <td>
                {{
                  id === selection.selected_pose
                    ? 'Selected'
                    : candidate.eligible
                      ? 'Eligible alternative'
                      : candidate.reasons.join(' ')
                }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </details>
</template>

<style scoped>
.reference-review {
  border: 1px solid var(--control-border);
  border-radius: 6px;
  margin-block: 16px 24px;
  font-size: 14px;
}
.reference-summary {
  cursor: pointer;
  padding: 12px 16px;
  min-height: 44px;
  box-sizing: border-box;
}
.reference-summary:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
.reference-content {
  padding: 0 16px 16px;
}
.reference-note,
.reference-caption {
  color: var(--muted);
  line-height: 1.5;
}
.reference-images {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
  max-width: 720px;
}
.reference-figure {
  margin: 0;
}
.reference-image {
  width: 100%;
  height: auto;
  display: block;
}
.reference-figure figcaption {
  margin-top: 8px;
  color: var(--muted);
}
.reference-table-wrap {
  overflow-x: auto;
}
.reference-table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
}
.reference-caption {
  text-align: left;
  margin-bottom: 8px;
}
.reference-table th,
.reference-table td {
  border-bottom: 1px solid var(--control-border);
  padding: 8px;
  vertical-align: top;
}
.reference-table .selected {
  background: var(--subtle);
}
@media (max-width: 560px) {
  .reference-images {
    grid-template-columns: 1fr;
  }
}
</style>
