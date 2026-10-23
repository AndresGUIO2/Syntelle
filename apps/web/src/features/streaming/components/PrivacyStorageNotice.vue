<script setup lang="ts">
import { useLocaleStore } from '../../../stores/locale'

const { t } = useLocaleStore()
</script>

<template>
  <aside class="storage-notice" aria-labelledby="storage-notice-title">
    <h3 id="storage-notice-title">{{ t('Cookies and browser storage') }}</h3>
    <p>
      {{
        t(
          'Syntelle uses an essential sign-in cookie for guest and account sessions. This browser also stores your language, appearance, instructions and conversation settings, plus a conversation ID. This storage is used to run the service, not for advertising.',
        )
      }}
    </p>
    <a href="/privacy.html" target="_blank" rel="noopener noreferrer">{{ t('Privacy Policy') }}</a>
  </aside>
</template>

<style scoped>
.storage-notice {
  padding: 14px;
  border: 1px solid var(--divider);
  border-radius: 10px;
  background: var(--subtle);
  color: var(--muted);
  font-size: 12px;
  line-height: 1.55;
}

.storage-notice h3 {
  margin: 0 0 6px;
  color: var(--text);
  font-size: 13px;
  font-weight: 600;
}

.storage-notice p {
  margin: 0 0 8px;
}

.storage-notice a {
  color: var(--accent);
  font-weight: 600;
  text-underline-offset: 3px;
}
</style>
