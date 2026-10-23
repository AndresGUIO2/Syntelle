<script setup lang="ts">
import { useLocaleStore } from './stores/locale'
import {
  defineAsyncComponent,
  onMounted,
  shallowRef,
  useTemplateRef,
  watchEffect,
} from 'vue'
import AppHeader from './components/AppHeader.vue'
import PricingPage from './features/pricing/PricingPage.vue'
import CreateCharacterPage from './features/characters/CreateCharacterPage.vue'
import { request } from './features/streaming/api'
import { usePageNavigation } from './composables/usePageNavigation'
const { t } = useLocaleStore()
const accountWorkspace = useTemplateRef<{ openAccount: () => void }>('accountWorkspace')
const signedIn = shallowRef(false)
const navigation = usePageNavigation()
const characterSessionChecked = shallowRef(!navigation.characterCreation.value)
const streamWorkspace = defineAsyncComponent(
  () => import('./features/streaming/StreamWorkspace.vue'),
)
function openAccount() {
  if (navigation.pricing.value || navigation.characterCreation.value) navigation.visit('/?auth')
  else accountWorkspace.value?.openAccount()
}
onMounted(async () => {
  if (!navigation.characterCreation.value) return
  try {
    const session = await request<{ account: unknown | null }>('/api/v1/auth/me')
    signedIn.value = Boolean(session.account)
  } catch {
    signedIn.value = false
  } finally {
    characterSessionChecked.value = true
  }
  if (!signedIn.value && navigation.characterCreation.value) {
    navigation.visit('/?auth')
  }
})
watchEffect(() => {
  document.title = navigation.pricing.value
    ? `${t('Pricing')} · Syntelle`
    : navigation.characterCreation.value
      ? `${t('Create a character')} · Syntelle`
      : 'Syntelle'
})
</script>

<template>
  <div class="app-shell demo-shell">
    <a class="skip-link" href="#main-content"> {{ t('Skip to content') }} </a>
    <AppHeader
      :show-upload="false"
      :show-account="true"
      :show-pricing="true"
      :show-create-character="true"
      :signed-in="signedIn"
      :pricing-active="navigation.pricing.value"
      :character-creation-active="navigation.characterCreation.value"
      @account="openAccount"
      @navigate="navigation.follow"
    />
    <main id="main-content" class="main-content demo-content" tabindex="-1">
      <PricingPage v-if="navigation.pricing.value" @navigate="navigation.follow" />
      <CreateCharacterPage
        v-else-if="navigation.characterCreation.value && characterSessionChecked && signedIn"
        @navigate="navigation.follow"
      />
      <div
        v-else-if="navigation.characterCreation.value"
        class="character-session-check"
        role="status"
      >
        {{ t('Checking your account…') }}
      </div>
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
.character-session-check {
  display: grid;
  min-height: 100%;
  place-items: center;
  color: var(--muted);
  font-size: 14px;
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
