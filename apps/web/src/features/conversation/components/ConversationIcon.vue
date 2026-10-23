<script setup lang="ts">
defineProps<{
  name: 'mic' | 'mic-off' | 'adjust' | 'close' | 'play' | 'stop' | 'alert' | 'send' | 'new-chat'
}>()
</script>

<template>
  <svg
    class="conversation-icon"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.8"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
    focusable="false"
  >
    <template v-if="name === 'mic'">
      <rect x="8" y="2" width="8" height="13" rx="4" fill="currentColor" stroke="none" />
      <path d="M5 10v1a7 7 0 0 0 14 0v-1M12 18v4M8 22h8" stroke-width="2" />
    </template>
    <template v-else-if="name === 'mic-off'">
      <path
        d="m3 3 18 18M9 9v2a3 3 0 0 0 5 2M9 4a3 3 0 0 1 6 0v5M5 10v1a7 7 0 0 0 12 5m2-5v-1M12 18v4M8 22h8"
      />
    </template>
    <template v-else-if="name === 'adjust'">
      <path d="M5 3v7m0 4v7M12 3v2m0 4v12M19 3v11m0 4v3" />
      <rect x="2.5" y="10" width="5" height="4" rx="1.5" />
      <rect x="9.5" y="5" width="5" height="4" rx="1.5" />
      <rect x="16.5" y="14" width="5" height="4" rx="1.5" />
    </template>
    <path v-else-if="name === 'close'" d="m6 6 12 12M6 18 18 6" />
    <path v-else-if="name === 'send'" d="M12 19V5m-6 6 6-6 6 6" stroke-width="2" />
    <template v-else-if="name === 'new-chat'">
      <path d="M12 4H6a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-6" />
      <path d="m16 3 5 5-9 9-6 1 1-6Z" />
    </template>
    <path v-else-if="name === 'play'" d="M9 5.5v13l10-6.5Z" fill="currentColor" stroke="none" />
    <rect
      v-else-if="name === 'stop'"
      x="6"
      y="6"
      width="12"
      height="12"
      rx="2"
      fill="currentColor"
      stroke="none"
    />
    <template v-else-if="name === 'alert'">
      <circle cx="12" cy="12" r="9" />
      <path d="M12 7v6m0 4h.01" />
    </template>
  </svg>
</template>

<style scoped>
.conversation-icon {
  width: 20px;
  height: 20px;
  flex-shrink: 0;
}
.conversation-icon :deep(path),
.conversation-icon :deep(rect),
.conversation-icon :deep(circle) {
  vector-effect: non-scaling-stroke;
}
</style>
