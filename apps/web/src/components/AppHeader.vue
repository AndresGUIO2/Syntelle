<script setup lang="ts">
import { useLocaleStore } from '../stores/locale'
import ThemeToggle from './ThemeToggle.vue'
import LanguageSelector from './LanguageSelector.vue'
import AppIcon from './AppIcon.vue'
import { useThemeStore } from '../stores/theme'
import { useTemplateRef } from 'vue'

const { t } = useLocaleStore()
const emit = defineEmits<{ uploadImage: []; account: []; navigate: [event: MouseEvent] }>()
const mobileMenu = useTemplateRef<HTMLDetailsElement>('mobileMenu')
function closeMobileMenu() {
  mobileMenu.value?.removeAttribute('open')
}
function navigateFromMobile(event: MouseEvent) {
  closeMobileMenu()
  emit('navigate', event)
}
function accountFromMobile() {
  closeMobileMenu()
  emit('account')
}
function uploadFromMobile() {
  closeMobileMenu()
  emit('uploadImage')
}
withDefaults(
  defineProps<{
    showUpload?: boolean
    showTheme?: boolean
    showAccount?: boolean
    showPricing?: boolean
    showCreateCharacter?: boolean
    pricingActive?: boolean
    characterCreationActive?: boolean
    signedIn?: boolean
  }>(),
  {
    showUpload: true,
    showTheme: true,
  },
)
const theme = useThemeStore()
const companyAssets = {
  wordmark: '/brand/colors-studio.svg',
  lightWordmark: '/brand/colors-studio-light.svg',
  symbol: '/brand/colors-studio-symbol.svg',
}
</script>

<template>
  <header class="app-header">
    <div class="header-inner">
      <div class="brand">
        <a
          class="wordmark"
          href="/"
          :aria-label="t('Syntelle home')"
          @click="$emit('navigate', $event)"
          >Syntelle</a
        >
        <div class="company-signature">
          <span class="attribution">{{ t('by') }}</span>
          <picture class="company-logo">
            <source media="(max-width: 600px)" :srcset="companyAssets.symbol" />
            <img
              class="company-logo-image"
              :src="theme.theme === 'light' ? companyAssets.lightWordmark : companyAssets.wordmark"
              alt="Colors Studio"
              width="460"
              height="160"
            />
          </picture>
        </div>
      </div>
      <LanguageSelector class="header-language" />
      <ThemeToggle v-if="showTheme" class="mobile-theme" />
      <details ref="mobileMenu" class="mobile-menu">
        <summary :aria-label="t('Menu')">
          <span class="menu-lines" aria-hidden="true"><span /><span /><span /></span>
        </summary>
        <nav class="mobile-menu-panel" :aria-label="t('Menu')">
          <a
            v-if="showPricing"
            href="/pricing"
            :aria-current="pricingActive ? 'page' : undefined"
            @click="navigateFromMobile"
            >{{ t('Pricing') }}</a
          >
          <a
            v-if="showCreateCharacter && signedIn"
            href="/characters/new"
            :aria-current="characterCreationActive ? 'page' : undefined"
            @click="navigateFromMobile"
            >{{ t('Create') }}</a
          >
          <button v-if="showAccount" type="button" @click="accountFromMobile">
            {{ t(signedIn ? 'Your account' : 'Sign in') }}
          </button>
          <button v-if="showUpload" type="button" @click="uploadFromMobile">
            {{ t('Upload photo') }}
          </button>
        </nav>
      </details>
      <div class="header-actions">
        <a
          v-if="showPricing"
          class="header-pricing"
          href="/pricing"
          :aria-current="pricingActive ? 'page' : undefined"
          @click="$emit('navigate', $event)"
        >
          {{ t('Pricing') }}
        </a>
        <a
          v-if="showCreateCharacter && signedIn"
          class="header-create-character"
          href="/characters/new"
          :aria-label="t('Create a character')"
          :aria-current="characterCreationActive ? 'page' : undefined"
          @click="$emit('navigate', $event)"
        >
          <AppIcon name="plus" />
          <span>{{ t('Create') }}</span>
        </a>
        <button
          v-if="showAccount"
          class="header-account"
          type="button"
          aria-haspopup="dialog"
          @click="$emit('account')"
        >
          {{ t(signedIn ? 'Your account' : 'Sign in') }}
        </button>
        <ThemeToggle v-if="showTheme" />
        <button
          v-if="showUpload"
          class="button button-primary header-upload"
          type="button"
          :aria-label="t('Upload photo')"
          @click="$emit('uploadImage')"
        >
          <AppIcon name="upload" /><span class="upload-label"> {{ t('Upload photo') }} </span>
        </button>
      </div>
    </div>
  </header>
</template>

<style scoped>
.app-header {
  height: 48px;
  background: var(--surface);
  border-bottom: 1px solid var(--divider);
}
.header-inner {
  max-width: var(--workspace-max-width, none);
  height: 100%;
  margin: 0 auto;
  padding: 0 var(--workspace-gutter, 20px);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}
.brand {
  display: flex;
  align-items: center;
  gap: 16px;
  min-width: 0;
}
.wordmark {
  color: var(--text);
  text-decoration: none;
  font-size: 20px;
  font-weight: 700;
  letter-spacing: -0.4px;
}
.company-signature {
  display: flex;
  align-items: center;
  gap: 8px;
  border-left: 1px solid var(--divider);
  padding-left: 16px;
}
.attribution {
  color: var(--muted);
  font-size: 12px;
}
.company-logo {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 120px;
  height: 44px;
}
.company-logo-image {
  display: block;
  width: 120px;
  height: 42px;
}
.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}
.header-language {
  margin-left: auto;
}
.mobile-theme,
.mobile-menu {
  display: none;
}
.header-account,
.header-pricing,
.header-create-character {
  display: inline-flex;
  align-items: center;
  min-height: 44px;
  padding: 8px 12px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--text);
  font: inherit;
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
  cursor: pointer;
  text-decoration: none;
}
.header-create-character {
  gap: 6px;
  color: var(--accent);
}
.header-create-character :deep(.icon) {
  width: 16px;
  height: 16px;
}
.header-pricing[aria-current='page'],
.header-create-character[aria-current='page'] {
  color: var(--accent);
}
@media (hover: hover) {
  .header-account:hover,
  .header-pricing:hover,
  .header-create-character:hover {
    background: var(--subtle);
  }
}
@media (max-width: 800px) {
  .header-inner {
    padding: 0 var(--workspace-gutter, 16px);
  }
}
@media (max-width: 760px) {
  .app-header {
    height: auto;
    min-height: 48px;
  }
  .header-inner {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 0 12px;
  }
  .header-actions {
    grid-column: 1 / -1;
    justify-content: flex-end;
  }
  .brand {
    gap: 12px;
  }
  .company-signature {
    padding-left: 12px;
    gap: 6px;
  }
  .company-logo {
    width: 32px;
    height: 36px;
  }
  .company-logo-image {
    width: 32px;
    height: 32px;
  }
  .header-actions {
    gap: 8px;
  }
  .header-upload {
    width: 44px;
    padding: 10px;
  }
  .upload-label {
    display: none;
  }
}
@media (max-width: 420px) {
  .brand {
    gap: 10px;
  }
  .company-signature {
    padding-left: 10px;
  }
  .company-logo,
  .company-logo-image {
    width: 28px;
  }
  .header-actions {
    gap: 4px;
  }
  .header-account,
  .header-pricing,
  .header-create-character {
    padding-right: 8px;
    padding-left: 8px;
  }
}
@media (max-width: 760px) {
  .header-inner {
    display: flex;
    flex-wrap: nowrap;
    gap: 4px;
    padding: 0 12px;
  }
  .app-header {
    position: relative;
    z-index: 10;
    height: 48px;
    min-height: 48px;
  }
  .brand {
    gap: 0;
  }
  .company-signature {
    display: none;
  }
  .wordmark {
    font-size: 18px;
  }
  .header-language {
    margin-left: auto;
  }
  .header-actions {
    display: none;
  }
  .mobile-theme,
  .mobile-menu {
    display: block;
  }
  .mobile-menu {
    flex: 0 0 auto;
  }
  .mobile-menu summary {
    display: grid;
    width: 44px;
    height: 44px;
    place-items: center;
    border-radius: 8px;
    color: var(--text);
    cursor: pointer;
    list-style: none;
  }
  .mobile-menu summary::-webkit-details-marker {
    display: none;
  }
  .mobile-menu summary:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }
  .mobile-menu[open] summary {
    background: var(--subtle);
  }
  .menu-lines {
    display: grid;
    gap: 4px;
  }
  .menu-lines span {
    display: block;
    width: 18px;
    height: 2px;
    border-radius: 2px;
    background: currentColor;
  }
  .mobile-menu-panel {
    position: absolute;
    top: calc(100% + 4px);
    right: 12px;
    display: grid;
    width: min(220px, calc(100vw - 24px));
    padding: 6px;
    border: 1px solid var(--divider);
    border-radius: 12px;
    background: var(--surface-raised);
    box-shadow: 0 12px 28px #0004;
  }
  .mobile-menu-panel :is(a, button) {
    display: flex;
    align-items: center;
    min-height: 44px;
    padding: 10px 12px;
    border: 0;
    border-radius: 7px;
    color: var(--text);
    background: transparent;
    font: inherit;
    font-size: 13px;
    text-align: left;
    text-decoration: none;
    cursor: pointer;
  }
  .mobile-menu-panel :is(a, button):hover,
  .mobile-menu-panel :is(a, button):focus-visible {
    background: var(--subtle);
  }
  .header-account,
  .header-pricing,
  .header-create-character {
    padding-right: 6px;
    padding-left: 6px;
    font-size: 12px;
  }
  .header-language :deep(.language-trigger) {
    gap: 4px;
    padding: 6px;
    font-size: 12px;
  }
  .header-language :deep(.language-globe) {
    display: none;
  }
}
@media (max-width: 360px) {
  .header-inner {
    gap: 0 4px;
    padding: 0 8px;
  }
  .wordmark {
    font-size: 17px;
  }
  .header-account,
  .header-pricing,
  .header-create-character {
    padding-right: 4px;
    padding-left: 4px;
    font-size: 11px;
  }
}
</style>
