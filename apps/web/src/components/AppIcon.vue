<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  name:
    | 'upload'
    | 'image'
    | 'check'
    | 'alert'
    | 'sun'
    | 'moon'
    | 'message'
    | 'mic'
    | 'send'
    | 'play'
    | 'pause'
    | 'stop'
    | 'close'
    | 'adjust'
    | 'plus'
    | 'volume'
    | 'volume-off'
    | 'pip'
    | 'globe'
    | 'chevron-down'
    | 'chevron-left'
    | 'menu'
    | 'sidebar'
    | 'history'
    | 'workspace'
    | 'shield'
    | 'tag'
    | 'user'
    | 'users'
    | 'grid'
    | 'code'
    | 'chart'
    | 'card'
    | 'copy'
    | 'external'
}>()
const shapes = {
  grid: ['M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h7v7h-7z'],
  code: ['m8 6-6 6 6 6m8-12 6 6-6 6m-3-15-2 18'],
  chart: ['M4 3v18h17', 'M8 16v-4m5 4V8m5 8V5'],
  card: ['M3 5h18v14H3z', 'M3 10h18M6 15h4'],
  users: [
    'M15 8a4 4 0 1 1-8 0 4 4 0 0 1 8 0Z',
    'M3 21v-2a7 7 0 0 1 14 0v2m1-16a4 4 0 0 1 0 8m2 3a5 5 0 0 1 2 4v1',
  ],
  copy: ['M8 8h13v13H8z', 'M16 8V3H3v13h5'],
  external: ['M14 3h7v7m0-7L10 14', 'M10 3H3v18h18v-7'],
  upload: ['M12 16V3m-4 4 4-4 4 4', 'M4 16v4a1 1 0 0 0 1 1h14a1 1 0 0 0 1-1v-4'],
  image: [
    'M4 3h16a1 1 0 0 1 1 1v16a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1Z',
    'm3 17 6-6 4 4 3-3 5 5',
    'M8 7h.01',
  ],
  check: ['m5 12 4 4L19 6'],
  alert: [
    'M12 8v5m0 3h.01',
    'M10.3 3.8 2 18a1.3 1.3 0 0 0 1.1 2h17.8a1.3 1.3 0 0 0 1.1-2L13.7 3.8a2 2 0 0 0-3.4 0Z',
  ],
  sun: [
    'M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8Z',
    'M12 2v2m0 16v2M2 12h2m16 0h2M5 5l1.5 1.5m11 11L19 19M5 19l1.5-1.5m11-11L19 5',
  ],
  moon: ['M20.8 13a9 9 0 0 1-9.8-9.8A9 9 0 1 0 20.8 13Z'],
  message: [
    'M21 11.5a8.5 8.5 0 0 1-8.5 8.5H4l-2 2v-9.5A8.5 8.5 0 0 1 10.5 4H13a8 8 0 0 1 8 7.5Z',
    'M7 10h10M7 14h6',
  ],
  mic: ['M9 5a3 3 0 0 1 6 0v7a3 3 0 0 1-6 0V5Z', 'M5 10v2a7 7 0 0 0 14 0v-2M12 19v3m-4 0h8'],
  send: ['m21 3-7 18-4-7-7-4 18-7Z', 'm10 14 5-5'],
  play: ['m8 5 11 7-11 7V5Z'],
  pause: ['M8 5v14M16 5v14'],
  stop: ['M6 6h12v12H6z'],
  close: ['m6 6 12 12M6 18 18 6'],
  adjust: ['M4 7h5m4 0h7M4 17h9m4 0h3', 'M9 4v6m4 4v6'],
  plus: ['M12 5v14M5 12h14'],
  volume: ['M11 5 6 9H3v6h3l5 4V5Z', 'M15 8a6 6 0 0 1 0 8m3-11a10 10 0 0 1 0 14'],
  'volume-off': ['M11 5 6 9H3v6h3l5 4V5Z', 'm16 9 6 6m0-6-6 6'],
  pip: ['M3 4h18v16H3z', 'M12 11h7v7h-7z'],
  globe: [
    'M21 12a9 9 0 1 0-18 0 9 9 0 0 0 18 0Z',
    'M3 12h18',
    'M12 3a18 18 0 0 1 0 18 18 18 0 0 1 0-18Z',
  ],
  'chevron-down': ['m6 9 6 6 6-6'],
  'chevron-left': ['m14 6-6 6 6 6'],
  menu: ['M4 6h16M4 12h16M4 18h16'],
  sidebar: ['M4 3h16a1 1 0 0 1 1 1v16a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1Z', 'M9 3v18'],
  history: ['M3 11a9 9 0 1 1 2.6 7', 'M3 4v7h7', 'M12 7v5l3 2'],
  workspace: ['M3 7h18v13H3z', 'M8 7V4h8v3', 'M3 12h18', 'M10 11v3h4v-3'],
  shield: ['m12 3 8 3v6c0 4-4 7-8 9-4-2-8-5-8-9V6z', 'm8 12 3 3 5-6'],
  tag: ['M3 3h8l10 10-8 8L3 11z', 'M7 7h.01'],
  user: ['M16 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0Z', 'M4 21v-2a8 8 0 0 1 16 0v2'],
}
const paths = computed(() => shapes[props.name])
</script>

<template>
  <svg
    class="icon"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.8"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
    focusable="false"
  >
    <path v-for="path in paths" :key="path" :d="path" />
  </svg>
</template>

<style scoped>
.icon {
  width: 20px;
  height: 20px;
  flex-shrink: 0;
}
</style>
