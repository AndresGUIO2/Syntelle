<script setup lang="ts">
import { useLocaleStore } from './stores/locale'
import { defineAsyncComponent, shallowRef, useTemplateRef, watchEffect } from 'vue'
import AppHeader from './components/AppHeader.vue'
import PricingPage from './features/pricing/PricingPage.vue'
import { usePageNavigation } from './composables/usePageNavigation'
const { t } = useLocaleStore()
const accountWorkspace = useTemplateRef<{ openAccount: () => void }>('accountWorkspace')
const signedIn = shallowRef(false)
const navigation = usePageNavigation()
const streamWorkspace = defineAsyncComponent(
  () => import('./features/streaming/StreamWorkspace.vue'),
)
function openAccount() {
  if (navigation.pricing.value) navigation.visit('/?auth')
  else accountWorkspace.value?.openAccount()
}
watchEffect(() => {
  document.title = navigation.pricing.value ? `${t('Pricing')} · Syntelle` : 'Syntelle'
})
</script>

<template>
  <div class="app-shell demo-shell">
    <a class="skip-link" href="#main-content"> {{ t('Skip to content') }} </a>
    <AppHeader
      :show-upload="false"
      :show-account="true"
      :show-pricing="true"
      :signed-in="signedIn"
      :pricing-active="navigation.pricing.value"
      @account="openAccount"
      @navigate="navigation.follow"
    />
    <main id="main-content" class="main-content demo-content" tabindex="-1">
      <PricingPage v-if="navigation.pricing.value" @navigate="navigation.follow" />
      <template v-else>
        <h1 class="visually-hidden">{{ t('Syntelle workspace') }}</h1>
        <component
          :is="streamWorkspace"
          ref="accountWorkspace"
          @account-change="signedIn = $event"
        />
      </template>
    </main>
  </div>
</template>

<style scoped>
.app-shell {
  height: 100dvh;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
}
.main-content {
  width: 100%;
  max-width: 1440px;
  min-height: 0;
  margin: 0 auto;
  padding: 24px 32px 32px;
}
.main-content:focus {
  outline: none;
}
.demo-shell {
  --workspace-max-width: 1520px;
  --workspace-gutter: clamp(24px, 3vw, 40px);
  background:
    radial-gradient(ellipse at 92% 100%, var(--workspace-glow), transparent 55%),
    linear-gradient(145deg, var(--page), var(--subtle));
}
.demo-content {
  max-width: var(--workspace-max-width);
  overflow-y: auto;
  padding: 32px var(--workspace-gutter);
}
@media (max-width: 800px) {
  .app-shell {
    height: auto;
    min-height: 100svh;
    grid-template-rows: auto auto;
  }
  .main-content {
    padding: 16px;
  }
}
@media (max-width: 899px) {
  .demo-shell {
    --workspace-gutter: 16px;
    height: 100dvh;
    min-height: 0;
    grid-template-rows: auto minmax(0, 1fr);
  }
  .demo-content {
    padding: 20px var(--workspace-gutter);
  }
}
</style>
