<script setup lang="ts">
import { useLocaleStore } from '../../../stores/locale'
const { t } = useLocaleStore()
</script>

<template>
  <aside class="customization-privacy">
    <p>
      {{ t('Your preferences are saved in this browser. Signing in uses an essential cookie.') }}
    </p>
    <a href="/privacy.html" target="_blank" rel="noopener noreferrer">{{ t('Privacy Policy') }}</a>
  </aside>
</template>
