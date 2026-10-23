<script setup lang="ts">
defineProps<{
  yaw: number
  pitch: number
  manual: boolean
  playing: boolean
  reducedMotion: boolean
  neckDetailAvailable?: boolean
}>()
const compare = defineModel<boolean>('compare', { required: true })
const detail = defineModel<boolean>('detail', { default: false })
const emit = defineEmits<{
  inspect: [value: number, axis: 'yaw' | 'pitch']
  replay: []
  automatic: []
}>()
</script>

<template>
  <fieldset class="projection-controls">
    <legend>Head depth comparison</legend>
    <label class="toggle"
      ><input v-model="compare" type="checkbox" /> Compare all three versions</label
    >
    <label v-if="neckDetailAvailable" class="toggle"
      ><input v-model="detail" type="checkbox" /> Magnify neck</label
    >
    <label class="angle"
      >Head yaw <span>{{ yaw.toFixed(1) }}°</span>
      <input
        aria-label="Head yaw"
        type="range"
        min="-3"
        max="3"
        step="0.1"
        :value="yaw"
        @input="emit('inspect', Number(($event.target as HTMLInputElement).value), 'yaw')"
      />
    </label>
    <div class="presets">
      <button
        v-for="value in [-3, -2, -1, 0, 1, 2, 3]"
        :key="value"
        class="button"
        :aria-pressed="manual && yaw === value"
        @click="emit('inspect', value, 'yaw')"
      >
        {{ value }}° yaw
      </button>
    </div>
    <label class="angle"
      >Head pitch <span>{{ pitch.toFixed(1) }}°</span>
      <input
        aria-label="Head pitch"
        type="range"
        min="-1"
        max="1"
        step="0.1"
        :value="pitch"
        @input="emit('inspect', Number(($event.target as HTMLInputElement).value), 'pitch')"
      />
    </label>
    <div class="presets">
      <button class="button" :disabled="playing || reducedMotion" @click="emit('replay')">
        Replay comparison
      </button>
      <button class="button" :aria-pressed="!manual && !playing" @click="emit('automatic')">
        Automatic head motion
      </button>
    </div>
  </fieldset>
</template>

<style scoped>
.projection-controls {
  border: 1px solid var(--control-border);
  padding: 16px;
  margin: 0 0 24px;
  min-width: 0;
}
.projection-controls legend {
  font-size: 14px;
  padding: 0 6px;
}
.toggle,
.angle {
  display: block;
  font-size: 14px;
  margin: 12px 0;
}
.angle span {
  color: var(--muted);
  margin-left: 8px;
}
.angle input {
  display: block;
  width: 100%;
  margin: 10px 0;
  accent-color: var(--accent);
}
.presets {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
</style>
