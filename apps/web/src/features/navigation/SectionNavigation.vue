<script setup lang="ts">
import { useId } from 'vue'
defineProps<{ label: string; items: { value: string; label: string }[]; selected: string }>()
const emit = defineEmits<{ select: [value: string] }>()
const selectId = useId()
</script>

<template>
  <div class="section-navigation">
    <nav class="section-list" :aria-label="label">
      <button
        v-for="item in items"
        :key="item.value"
        class="section-link"
        type="button"
        :aria-current="selected === item.value ? 'page' : undefined"
        @click="emit('select', item.value)"
      >
        {{ item.label }}
      </button>
    </nav>
    <div class="section-picker">
      <label :for="selectId">{{ label }}</label
      ><select
        :id="selectId"
        :value="selected"
        @change="emit('select', ($event.target as HTMLSelectElement).value)"
      >
        <option v-for="item in items" :key="item.value" :value="item.value">
          {{ item.label }}
        </option>
      </select>
    </div>
  </div>
</template>

<style scoped>
.section-navigation {
  min-width: 0;
}
.section-list {
  display: grid;
  gap: 4px;
}
.section-link {
  min-height: 44px;
  padding: 10px 14px;
  border: 1px solid transparent;
  border-radius: 9px;
  background: transparent;
  color: var(--muted);
  font: inherit;
  font-size: 14px;
  font-weight: 500;
  text-align: start;
  cursor: pointer;
}
.section-link[aria-current='page'] {
  background: var(--subtle);
  color: var(--accent);
  font-weight: 650;
}
.section-link:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: -2px;
}
.section-picker {
  display: none;
  gap: 8px;
}
.section-picker label {
  color: var(--muted);
  font-size: 13px;
  font-weight: 500;
}
.section-picker select {
  min-height: 44px;
  width: 100%;
  padding: 10px 12px;
  border: 1px solid var(--control-border);
  border-radius: 9px;
  background: var(--surface);
  color: var(--text);
  font: inherit;
  font-size: 14px;
}
@media (hover: hover) {
  .section-link:hover {
    background: var(--subtle);
    color: var(--text);
  }
}
@media (max-width: 1023px) {
  .section-list {
    display: none;
  }
  .section-picker {
    display: grid;
  }
}
</style>
