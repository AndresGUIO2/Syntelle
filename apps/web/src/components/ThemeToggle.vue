<script setup lang="ts">
import { useLocaleStore } from '../stores/locale'
import { useThemeStore } from '../stores/theme'
const { t } = useLocaleStore()
const theme = useThemeStore()
</script>

<template>
  <button
    class="button theme-toggle"
    type="button"
    :aria-pressed="theme.theme === 'dark'"
    :aria-label="t('Dark mode')"
    :title="t(theme.theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode')"
    @click="theme.toggle"
  >
    <svg class="theme-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path
        v-if="theme.theme === 'dark'"
        fill="currentColor"
        d="M20.7 14.1A8.8 8.8 0 0 1 9.9 3.3a.75.75 0 0 0-.93-.94A10 10 0 1 0 21.64 15a.75.75 0 0 0-.94-.9Z"
      />
      <template v-else>
        <circle cx="12" cy="12" r="4.5" fill="currentColor" />
        <path
          d="M12 2v2m0 16v2M2 12h2m16 0h2M4.93 4.93l1.42 1.42m11.3 11.3 1.42 1.42M4.93 19.07l1.42-1.42m11.3-11.3 1.42-1.42"
          fill="none"
          stroke="currentColor"
          stroke-width="1.8"
          stroke-linecap="round"
        />
      </template>
    </svg>
  </button>
</template>

<style scoped>
.theme-toggle {
  width: 44px;
  padding: 10px;
  border-color: transparent;
  border-radius: 50%;
  background: transparent;
  color: var(--muted);
}
.theme-icon {
  display: block;
  width: 20px;
  height: 20px;
}
@media (hover: hover) {
  .theme-toggle:hover {
    background: var(--subtle);
    color: var(--text);
  }
}
</style>
