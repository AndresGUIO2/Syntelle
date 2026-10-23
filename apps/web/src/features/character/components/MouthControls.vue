<script setup lang="ts">
defineProps<{ poses: string[]; amount: number; playing: boolean; disabled: boolean }>()
const source = defineModel<string>('source', { required: true })
const target = defineModel<string>('target', { required: true })
const emit = defineEmits<{ toggle: []; pose: [amount: number] }>()
</script>
<template>
  <div class="mouth-controls" aria-label="Mouth controls">
    <div class="row">
      <label
        >From<select v-model="source" aria-label="From pose">
          <option v-for="id in poses" :key="id">{{ id }}</option>
        </select></label
      >
      <label
        >To<select v-model="target" aria-label="To pose">
          <option v-for="id in poses" :key="id">{{ id }}</option>
        </select></label
      >
    </div>
    <div class="row">
      <button class="button" :disabled="disabled" :aria-pressed="playing" @click="emit('toggle')">
        {{ playing ? 'Pause mouth' : 'Play mouth' }}
      </button>
      <button class="button" @click="emit('pose', 0)">{{ source }}</button>
      <button class="button" @click="emit('pose', 1)">{{ target }}</button>
    </div>
    <label
      >Mouth blend · {{ Math.round(amount * 100) }}%<input
        aria-label="Mouth blend"
        type="range"
        min="0"
        max="1"
        step="0.01"
        :value="amount"
        @input="emit('pose', Number(($event.target as HTMLInputElement).value))"
    /></label>
  </div>
</template>
<style scoped>
.mouth-controls {
  border-top: 1px solid var(--control-border);
  padding-top: 20px;
  display: grid;
  gap: 16px;
}
.row {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}
.row label {
  flex: 1;
  min-width: 100px;
}
.mouth-controls label {
  display: grid;
  gap: 8px;
  font-size: 14px;
}
select {
  font: inherit;
  background: var(--surface);
  color: var(--text);
  border: 1px solid var(--control-border);
  border-radius: 6px;
  padding: 8px;
  min-height: 44px;
}
input[type='range'] {
  width: 100%;
  accent-color: var(--accent);
  min-height: 28px;
}
</style>
