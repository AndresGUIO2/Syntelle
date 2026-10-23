<script setup lang="ts">
const smile = defineModel<number>('smile', { required: true })
const brows = defineModel<number>('brows', { required: true })
const automatic = defineModel<boolean>('automatic', { required: true })
defineProps<{ available: boolean; smileAvailable: boolean; reducedMotion: boolean }>()
function reset() {
  smile.value = 0
  brows.value = 0
}
</script>

<template>
  <fieldset class="settings-section expressions" :disabled="!available">
    <legend>Expressions</legend>
    <template v-if="smileAvailable">
      <div class="expression-label">
        <label for="demo-smile">Smile</label><span>{{ Math.round(smile * 100) }}%</span>
      </div>
      <input id="demo-smile" v-model.number="smile" type="range" min="0" max="1" step="0.05" />
      <p class="expression-note">Smiles pause during speech.</p>
    </template>
    <div class="expression-label">
      <label for="demo-brows">Raise eyebrows</label><span>{{ Math.round(brows * 100) }}%</span>
    </div>
    <input id="demo-brows" v-model.number="brows" type="range" min="0" max="1" step="0.05" />
    <button
      class="workspace-action reset-expression"
      type="button"
      :disabled="smile === 0 && brows === 0"
      @click="reset"
    >
      Reset expression
    </button>
    <label class="automatic-expression"
      ><input v-model="automatic" type="checkbox" :disabled="reducedMotion" />Automatic expressions
      during speech</label
    >
    <p v-if="!available" class="expression-note">Expressions are unavailable in this view.</p>
    <p v-else-if="reducedMotion" class="expression-note">
      Automatic expressions are off with reduced motion. Manual expressions remain available.
    </p>
  </fieldset>
</template>

<style scoped>
.expressions {
  display: grid;
  gap: 12px;
}
.expressions legend {
  font-size: 14px;
  font-weight: 600;
}
.expressions input {
  accent-color: var(--accent);
}
.expressions input[type='range'] {
  width: 100%;
  margin: 0;
}
.expression-label {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  font-size: 14px;
}
.expression-label span,
.expression-note {
  color: var(--muted);
}
.automatic-expression {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  font-size: 14px;
  line-height: 1.5;
}
.automatic-expression input {
  margin-top: 4px;
}
.reset-expression {
  justify-self: start;
  padding: 10px 14px;
}
.reset-expression:disabled {
  opacity: 0.5;
  cursor: default;
}
.expression-note {
  margin: 0;
  font-size: 13px;
  line-height: 1.5;
}
</style>
